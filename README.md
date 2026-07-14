# Heureum

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
| `APP_SECRET` | yes | — | JWT signing secret |
| `DB_DSN` | yes | — | Database connection string / file path |
| `APP_PORT` | no | `8080` | HTTP port the API server listens on |
| `APP_ENV` | no | `development` | Environment name (affects logging) |
| `APP_BASE_URL` | no | `http://localhost:<APP_PORT>` | Base URL used to build absolute links |
| `DB_DRIVER` | no | `postgres` | One of `postgres`, `mysql`, `sqlite` |
| `REDIS_URL` | no | `redis://localhost:6379/0` | Redis connection URL |
| `APP_UPLOADS_DIR` | no | `./data/uploads` | Local disk directory where issue attachments are stored |
| `APP_SIGNUP` | no | `open` | `open` or `closed` — set `closed` to disable public registration |

SMTP / OAuth / object-storage settings are planned and not yet wired.

Attachments are stored on local disk under `APP_UPLOADS_DIR`; when running in Docker, mount a volume (see below) so uploads survive container restarts.

## Docker

Local development stack (Postgres + Redis + API):

```bash
docker compose -f deploy/docker/docker-compose.yml up --build
```

A production compose file with pre-built `heureum/*` images is provided as a starting point
at `deploy/docker/docker-compose.prod.yml`.

## Kubernetes / Helm

```bash
helm install heureum deploy/helm/heureum
```

See the chart's `values.yaml` for configurable settings.

## API compatibility & gap report

Heureum's route coverage against the official Jira Cloud v3 / Agile 1.0 OpenAPI specs is
tracked automatically. Run `go run ./cmd/gapreport` to regenerate it, or read the current
snapshot at [`docs/contracts/gap-report.md`](docs/contracts/gap-report.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and our [Code of Conduct](CODE_OF_CONDUCT.md).

## Security

See [SECURITY.md](SECURITY.md) for how to report vulnerabilities.

## License

Heureum is licensed under the [GNU AGPL v3.0](LICENSE). See [CHANGELOG.md](CHANGELOG.md) for
release history.
