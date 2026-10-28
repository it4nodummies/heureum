# Heureum

**Issue & project tracking that's easy for product owners.**

A self-hostable, open-source project & issue tracker with a drop-in Jira Cloud REST API v3-compatible surface.

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](LICENSE)

## Compatibility note

Heureum implements a subset of the Jira Cloud REST API v3 (`/rest/api/3`) and Agile API
(`/rest/agile/1.0`) for drop-in compatibility. *Jira* and *Atlassian* are trademarks of
Atlassian; Heureum is an independent project, not affiliated with or endorsed by Atlassian.

See [API compatibility & gap report](#api-compatibility--gap-report) below for exactly which
endpoints are covered today.

## Features

- Projects, issues (Epic/Story/Task/Bug/Subtask), boards, sprints and backlog
- Workflows, labels, links, watchers, comments
- Search, dashboards/reports, notifications
- Users, groups and permissions
- Webhooks and an async worker for automation rules
- A REST surface compatible with a subset of the Jira Cloud v3 / Agile 1.0 APIs

## Tech stack

| Layer       | Technology |
|-------------|------------|
| Backend     | Go, standard `net/http`, GORM, `golang-migrate` |
| Frontend    | Next.js (App Router) + React, served under the `/app` route prefix |
| Database    | PostgreSQL or MySQL/MariaDB in production, SQLite for local development |
| Cache/queue | Redis (optional) |
| Deployment  | Docker, Kubernetes (Helm) |

## Repository structure

```
cmd/
  server/     # HTTP API server
  worker/     # background worker (automation rules, async jobs)
  seed/       # populates the database with demo users, a project and sample issues
  gapreport/  # compares implemented routes against the official OpenAPI specs
              # and regenerates docs/contracts/gap-report.md
internal/      # application code (config, domain packages, API handlers, store)
frontend-next/ # Next.js (App Router) frontend, UI served under /app
migrations/    # SQL migrations (golang-migrate)
deploy/
  docker/     # docker-compose files
  helm/       # Helm chart
docs/          # contracts, gap reports and project documentation
```

## Quick start (SQLite)

```bash
# 1. seed a local SQLite database with demo data
APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=./dev.db go run ./cmd/seed

# 2. start the API server
APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=./dev.db go run ./cmd/server

# 3. start the frontend
cd frontend-next && npm install && npm run dev
```

Open **http://localhost:3000/app** and sign in with the demo account:

- Email: `admin@example.com`
- Password: `admin-demo-123`

## Configuration

Only the environment variables actually read by the server are listed below.

| Variable | Required | Default | Description |
|----------|----------|---------|--------------|
| `APP_SECRET` | yes | — | JWT signing secret; also derives the key that encrypts git provider tokens at rest. **Must be at least 32 characters when `APP_ENV=production`** (shorter is a startup error) |
| `DB_DSN` | yes | — | Database connection string / file path |
| `APP_PORT` | no | `8080` | HTTP port the API server listens on |
| `APP_ENV` | no | `development` | Environment name (affects logging, the `APP_SIGNUP` default and the `APP_SECRET` length check) |
| `APP_BASE_URL` | no | `http://localhost:<APP_PORT>` | Base URL used to build absolute links; its origin is the only one accepted on WebSocket connections |
| `DB_DRIVER` | no | `postgres` | One of `postgres`, `mysql`, `sqlite` |
| `REDIS_URL` | no | `redis://localhost:6379/0` | Redis connection URL |
| `APP_UPLOADS_DIR` | no | `./data/uploads` | Local disk directory where issue attachments are stored |
| `APP_SIGNUP` | no | `closed` when `APP_ENV=production`, `open` otherwise | `open` or `closed` — whether public registration is enabled. An explicit value always wins over the default; **any other value is a startup error**. Regardless of the policy, the first registration on an instance with **no active admin** is accepted and that user becomes global admin (bootstrap, see below) |
| `APP_TRUSTED_PROXIES` | no | — | Comma-separated IPs/CIDRs of the reverse proxies in front of the API (e.g. `127.0.0.1,10.42.0.0/16`). Only when the connecting peer is one of them is `X-Forwarded-For` believed — the client is the rightmost non-trusted hop — for auth rate limiting; otherwise the header is ignored and the peer address is the client. A malformed entry is a startup error |
| `APP_WEBHOOK_ALLOW_PRIVATE_HOSTS` | no | — | Comma-separated hosts, IPs or CIDRs that outgoing webhooks may target even though they resolve to private/loopback/link-local ranges (SSRF guard). A **hostname** entry is trusted wherever it resolves; an **IP/CIDR** entry lifts the restriction only for that address range |

SMTP is wired: set `SMTP_HOST`/`SMTP_PORT`/`SMTP_USER`/`SMTP_PASS`/`SMTP_FROM` to have the
worker deliver notification emails (respecting each user's `via_email` preference, once per
notification); leaving `SMTP_HOST` empty disables email. OAuth and S3 object-storage settings
are still planned and not yet wired.

### Signup policy and first-admin bootstrap

In production (`APP_ENV=production`) public registration is **closed by default**; in any other
environment it is open. `APP_SIGNUP=open|closed` overrides the default either way. Whatever the
policy, when the instance has **no active global admin** (fresh install, or the only admin was
removed) the next registration is accepted and that user becomes global admin, so a production
instance is never stranded without an administrator. Once an active admin exists, a closed
instance only grows through invitations.

### API tokens and git providers

Personal API tokens can be created with an optional expiry, listed and revoked from the profile
page (or via `GET`/`DELETE /rest/api/3/auth/api-tokens[/{id}]`); an expired or revoked token is
rejected immediately.

Git provider configuration (`/rest/api/3/project/{key}/git/providers`) is readable by project
admins only and never returns the access token or the HMAC secret (`has_token` says whether one is
stored; the token is encrypted at rest with a key derived from `APP_SECRET`). The inbound webhook
URL (`webhook_url` in the response) carries a dedicated random URL token, distinct from the HMAC
secret. `webhook_secret` is required and must be at least 16 characters; supported providers are
`github`, `gitlab`, `forgejo` and `gitea` — `bitbucket` is refused (no signature validator).

Attachments are stored on local disk under `APP_UPLOADS_DIR`; when running in Docker, mount a volume (see below) so uploads survive container restarts.

## Docker

For backup, restore and day-2 operations, see [docs/OPERATIONS.md](docs/OPERATIONS.md).

### Prebuilt images (GHCR)

Official images are published to the GitHub Container Registry on every release and are
publicly pullable:

```bash
docker pull ghcr.io/it4nodummies/heureum-api:1.1.0
docker pull ghcr.io/it4nodummies/heureum-worker:1.1.0
docker pull ghcr.io/it4nodummies/heureum-frontend:1.1.0
```

Each image is also tagged `latest` and with the major.minor (`1.1`).

### Run the full stack from GHCR

`deploy/docker/docker-compose.prod.yml` wires the three images together with Postgres, Redis
and an nginx reverse proxy. It references `nginx.conf` from the repo, so clone the repo first:

```bash
git clone https://github.com/it4nodummies/heureum.git
cd heureum
APP_SECRET="$(openssl rand -hex 32)" \
  docker compose -f deploy/docker/docker-compose.prod.yml up -d
```

The API runs its migrations automatically on first start. Open **http://localhost** and
create the first account via the sign-up screen: it becomes the global admin, and since this
stack runs with `APP_ENV=production` registration is closed from then on (invite further users
from the admin UI, or set `APP_SIGNUP=open` explicitly). `APP_SECRET` must be at least 32
characters here. No demo data is seeded in this stack; use the SQLite quick start above if you
want the sample project.

### Build locally instead

Development stack that builds the images from source (Postgres + Redis + API + frontend + nginx):

```bash
docker compose -f deploy/docker/docker-compose.yml up --build
```

## Kubernetes / Helm

```bash
helm install heureum deploy/helm/heureum
```

See the chart's `values.yaml` for configurable settings.

## Multi-tenancy

Heureum is **single-tenant per instance**: users, projects, groups and API keys are global
to the running instance, and there is no application-level isolation between
organizations (the `organizations` table exists in the schema but is currently vestigial
and unused). To serve multiple organizations, run **one instance per tenant**, each with
its own database, its own `APP_SECRET`, and its own hostname:

- **Helm**: install one release per tenant/namespace, each with its own `ingress.host`
  (chart resource names are release-aware, so multiple releases coexist without collisions).
- **Docker Compose**: use a distinct Compose project name (`-p`/`COMPOSE_PROJECT_NAME`) per
  tenant stack, and set `API_PORT` to avoid host port collisions between stacks.

Run every internet-facing instance with `APP_ENV=production`: registration is then closed by
default (the first account still bootstraps as admin), so sign-ups can't leak into a shared
instance. Set `APP_SIGNUP=open` only when you deliberately want public registration.

## API compatibility & gap report

Heureum's route coverage against the official Jira Cloud v3 / Agile 1.0 OpenAPI specs is
tracked automatically. Run `go run ./cmd/gapreport` to regenerate it, or read the current
snapshot at [`docs/contracts/gap-report.md`](docs/contracts/gap-report.md).

### What is stable

Heureum implements a subset of the Jira Cloud v3 surface deliberately, as a migration bridge
rather than a full re-implementation — see
[ADR 0001](docs/adr/0001-superficie-jira-compat-congelata.md). Within that subset, four areas are
**stable within a major version**, because they are what a migration from Jira actually goes
through:

1. **Issues** — CRUD, fields, transitions, comments, worklogs, links, watchers
2. **Projects** — CRUD, search, categories
3. **Search / JQL** — `/search/jql` and the language the parser accepts
4. **Agile** — boards, sprints, backlog (`/rest/agile/1.0/*`)

An incompatible change in those four areas — removing a route, changing a response shape,
narrowing a field — requires a major version and an announced deprecation period. Everything else
in the surface is best-effort: consult the gap report above for what exists today, and expect it
to change in a minor release. See
[ADR 0003](docs/adr/0003-promessa-di-stabilita-selettiva.md) for the reasoning.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and our [Code of Conduct](CODE_OF_CONDUCT.md).

## Security

See [SECURITY.md](SECURITY.md) for how to report vulnerabilities.

## License

Heureum is licensed under the [GNU AGPL v3.0](LICENSE). See [CHANGELOG.md](CHANGELOG.md) for
release history.
