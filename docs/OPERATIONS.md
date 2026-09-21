# Operations: Backup & Restore

This document covers the one operational prerequisite before real data goes into a Heureum
instance: backing up and restoring the Docker Compose deployment
(`deploy/docker/docker-compose.yml`).

All commands below assume you run them from the repository root, against the default Compose
project (no `-p` flag). If you run multiple tenants with distinct project names (see the
"Multi-tenancy" section of the README), substitute your own `-p <project>` / container names
throughout.

## What to back up

Heureum's Docker stack keeps state in exactly two places, plus one file that is not stack state
but is required to bring the stack back up at all:

1. **The Postgres database** (`postgres` service, database `openjira`, user `openjira`) — all
   projects, issues, comments, users, permissions, workflow state, etc.
2. **The `uploads` named volume**, mounted at `/data/uploads` in the `api` container — issue
   attachments.
3. **`deploy/docker/.env`**, specifically `APP_SECRET`. It is gitignored and exists only on the
   host. `docker-compose.yml` requires it for both the `api` and `worker` services
   (`APP_SECRET: "${APP_SECRET:?APP_SECRET is required — set it in .env}"`, lines 44 and 66) — with
   it missing, `docker compose up` refuses to start either service. And even if you generate a
   fresh secret instead of restoring the old one, the stack does start, but `APP_SECRET` signs
   session tokens: a new value invalidates every session issued by the lost instance, logging out
   every user.

**Both database state and uploads must be backed up together.** A restore of only the database
leaves the `attachments` rows in place but the files on disk gone, so every issue attachment
404s after restore. A restore of only the `uploads` volume leaves orphaned files nothing
references. Always back up and restore the database and the `uploads` volume as one unit, from
the same point in time. In practice "the same point in time" means two sequential commands (the
`pg_dump` below, then the tarball), not one atomic snapshot — an attachment uploaded in the
window between them can end up recorded in the database but missing from the tarball, or present
in the tarball but not yet referenced by any row. For a busy instance, pause the `api` container
(or accept this narrow race) if you need a truly consistent pair.

## What is NOT backed up

Nothing else holds state. The `api`, `worker`, `frontend` and `nginx` containers are stateless —
they can be rebuilt/restarted freely and hold no data of their own. Redis is used only as an
ephemeral job queue for the worker (webhook delivery / automation), not as a data store; it does
not need to be backed up.

## Backup

Run this periodically (e.g. from a cron job on the Docker host) while the stack is up:

```bash
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

# 1. Database dump (custom format, compressed, safe for pg_restore)
docker compose -f deploy/docker/docker-compose.yml exec -T postgres \
  pg_dump -U openjira -d openjira -F c > "heureum-db-${TIMESTAMP}.dump"

# 2. Uploads volume, as a tarball, via a throwaway container that shares the api
#    container's mounts (avoids having to know the volume's project-qualified name)
API_CID=$(docker compose -f deploy/docker/docker-compose.yml ps -q api)
docker run --rm --volumes-from "$API_CID" -v "$(pwd)":/backup alpine \
  tar czf "/backup/heureum-uploads-${TIMESTAMP}.tar.gz" -C /data/uploads .
```

This produces two files in the current directory: `heureum-db-<timestamp>.dump` and
`heureum-uploads-<timestamp>.tar.gz`. Copy both off the host (object storage, another machine,
etc.) — a backup that lives only on the same disk as the data it protects is not a backup.

## Restore

Restoring into a database that already has data is **not supported** by this procedure — it
assumes a fresh, empty target. If the instance is already running with data you want to
discard, back it up first (see above) in case you need to roll back, then proceed.

```bash
# 1. Tear down the stack, including its volumes, so both pgdata and uploads start
#    empty. This is the destructive step — make sure you actually have both backup
#    files from a point in time you want before running it.
docker compose -f deploy/docker/docker-compose.yml down -v

# 2. Bring up just Postgres against the fresh, empty pgdata volume, and wait for it
docker compose -f deploy/docker/docker-compose.yml up -d postgres
until docker compose -f deploy/docker/docker-compose.yml exec -T postgres \
  pg_isready -U openjira >/dev/null 2>&1; do sleep 1; done

# 3. Restore the database dump into the fresh, empty database
docker compose -f deploy/docker/docker-compose.yml exec -T postgres \
  pg_restore -U openjira -d openjira --clean --if-exists < heureum-db-<timestamp>.dump

# 4. Bring up the rest of the stack (creates the fresh uploads volume and mounts it
#    into the api container)
docker compose -f deploy/docker/docker-compose.yml up -d

# 5. Load the uploads tarball into the running api container's uploads mount
API_CID=$(docker compose -f deploy/docker/docker-compose.yml ps -q api)
docker run --rm --volumes-from "$API_CID" -v "$(pwd)":/backup alpine \
  tar xzf "/backup/heureum-uploads-<timestamp>.tar.gz" -C /data/uploads
```

## Verify a backup

A backup that has never been restored is not a backup. Verify a backup by restoring it into a
throwaway stack under a different Compose project name, so it uses its own volumes and does not
touch your real deployment.

You need Basic-auth credentials to call `/rest/api/3` afterwards: an email plus a real **API
token** (`POST /rest/api/3/auth/api-tokens` with a Bearer session token, or the "API tokens"
section of your profile in the UI) — Basic auth checks against an API token, not a login
password. This `docker-compose.yml` stack does not seed any demo user/project (that only happens
via `cmd/seed` against SQLite, see the Quick start above), so sign up and create a project/issue
first if you don't already have one to check against.

Your real deployment is (presumably) still running its `api`/`nginx` services on the default
`API_PORT`/`APP_PORT` (8080/80). The throwaway project must **not** try to publish those same
host ports, or `up -d` fails before you ever reach the curl below — export distinct ports for
this project only (don't write them into `deploy/docker/.env`, which is shared with the real
deployment):

```bash
export CP="docker compose -p heureum-backup-verify -f deploy/docker/docker-compose.yml"
export API_PORT=18080   # any free port distinct from the real deployment's — required
export APP_PORT=18000   # only needed for the optional browser check below

$CP down -v   # ensure a clean slate — safe: only this throwaway project's volumes
$CP up -d postgres
until $CP exec -T postgres pg_isready -U openjira >/dev/null 2>&1; do sleep 1; done

$CP exec -T postgres pg_restore -U openjira -d openjira --clean --if-exists < heureum-db-<timestamp>.dump

$CP up -d
API_CID=$($CP ps -q api)
docker run --rm --volumes-from "$API_CID" -v "$(pwd)":/backup alpine \
  tar xzf "/backup/heureum-uploads-<timestamp>.tar.gz" -C /data/uploads

curl -s -u "<your-email>:<your-api-token>" "http://localhost:${API_PORT}/rest/api/3/project"

# then, in a browser at http://localhost:${APP_PORT}/app, open a project and confirm an
# issue's attachment loads (not a 404) — or curl the attachment content endpoint directly:
# GET /rest/api/3/attachment/content/{attachmentId}

$CP down -v   # clean up the throwaway project entirely
```

Expected: the project list comes back with your project(s) exactly as they were before the
backup, and the attachment's content loads byte-for-byte, not a 404.

**Verified:** this exact backup → teardown → restore sequence was run end to end against a
throwaway Compose project seeded with a real project, issue and attachment; after restore the
project list and the attachment's content both came back identical to the pre-backup state.
