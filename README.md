# Open Jira

Clone open-source di [Jira](https://www.atlassian.com/software/jira), distribuibile su Kubernetes, con supporto PostgreSQL/MariaDB/SQLite e integrazione Git multi-provider.

## Architettura

```
┌──────────────────────────────────────────────┐
│            FRONTEND (React SPA)              │
│  Board · Backlog · Timeline · Reports · Admin│
└──────────────────┬───────────────────────────┘
                   │ REST + WebSocket
┌──────────────────▼───────────────────────────┐
│              API Server (Go)                 │
│  Projects · Issues · Workflows · Sprints     │
│  Search · Reports · Notifications · Git      │
└──────┬──────────────┬────────────────────────┘
       │              │
┌──────▼──────┐ ┌─────▼─────┐
│ PostgreSQL   │ │  Redis    │
│ MariaDB      │ │(cache+Q)  │
│ SQLite       │ └───────────┘
└─────────────┘
```

## Stack Tecnologico

| Layer | Tecnologia |
|-------|-----------|
| Backend | Go 1.22+ |
| ORM | GORM |
| API | REST + WebSocket |
| Auth | JWT + OAuth2 (Forgejo, GitLab, GitHub) |
| Frontend | React 19 + TypeScript + Vite |
| UI | Tailwind CSS + Lucide React |
| State | TanStack Query + Zustand |
| Charts | Recharts |
| DB Primario | PostgreSQL 15+ |
| DB Alternativi | MariaDB 10.6+, SQLite |
| Cache | Redis 7 |
| Deploy | Docker + Kubernetes + Helm |

## Moduli

| Step | Modulo | Descrizione |
|------|--------|-------------|
| 1 | Infrastruttura | Config, logging, GORM multi-DB, migrations, CI |
| 2 | Auth & Utenti | JWT, OAuth2, registrazione, login, ruoli |
| 3 | Progetti | CRUD progetti, membri, inviti |
| 4 | Issues | Epic, Story, Task, Bug, Subtask, gerarchia, label, link, watcher |
| 5 | Workflow | Stati, transizioni configurabili, default TO DO → DONE |
| 6 | Board & Backlog | Scrum/Kanban, drag&drop, sprint, WebSocket real-time |
| 7 | Commenti & History | Commenti, menzioni @utente, activity log, allegati |
| 8 | Ricerca & Filtri | Full-text search, JQL-like parser, filtri salvati |
| 9 | Report & Dashboard | Burndown, Velocity, Burnup, CFD, dashboard configurabile |
| 10 | Notifiche | In-app, email, WebSocket, impostazioni per evento |
| 11 | Git Integration | Webhook, auto-link branch/commit/PR, Forgejo/GitLab/GitHub |
| 12 | Advanced & Deploy | Timeline, Calendar, Custom Fields, Automation, Helm, Docker |

## Prerequisiti

- **Go** 1.22+ ([installazione](https://go.dev/dl/))
- **Node.js** 20+ ([installazione](https://nodejs.org/))
- **Docker** e **Docker Compose** (per sviluppo locale)
- **PostgreSQL** 15+ (o SQLite per sviluppo/test)
- **Redis** 7 (opzionale in sviluppo, richiesto in produzione)
- **kubectl** e **Helm** (solo per deploy Kubernetes)

## Quick Start — Sviluppo Locale

```bash
# 1. Clona il repository
git clone https://github.com/open-jira/open-jira.git
cd open-jira

# 2. Avvia PostgreSQL e Redis con Docker Compose
docker compose -f deploy/docker/docker-compose.yml up -d

# 3. Configura le variabili d'ambiente
cp .env.example .env
# Modifica .env con i tuoi valori

# 4. Avvia il backend
go run ./cmd/server

# 5. In un altro terminale, avvia il frontend
cd frontend
npm install
npm run dev
```

L'applicazione sarà disponibile su `http://localhost:5173` (frontend) e `http://localhost:8080` (API).

## Configurazione

Tutte le variabili d'ambiente:

```bash
# Server
APP_PORT=8080                    # Porta del server API
APP_ENV=development              # development | production
APP_SECRET=<jwt-secret>          # Chiave JWT (min 32 caratteri) — OBBLIGATORIA
APP_BASE_URL=http://localhost:8080

# Database
DB_DRIVER=postgres               # postgres | mysql | sqlite
DB_DSN=postgres://user:pass@host:5432/dbname?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379/0

# Email (SMTP)
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=noreply@example.com
SMTP_PASS=secret
SMTP_FROM=noreply@example.com

# OAuth Providers
OAUTH_FORGEJO_CLIENT_ID=
OAUTH_FORGEJO_CLIENT_SECRET=
OAUTH_FORGEJO_BASE_URL=

OAUTH_GITLAB_CLIENT_ID=
OAUTH_GITLAB_CLIENT_SECRET=
OAUTH_GITLAB_BASE_URL=

OAUTH_GITHUB_CLIENT_ID=
OAUTH_GITHUB_CLIENT_SECRET=

# Storage allegati
STORAGE_DRIVER=local             # local | s3 | minio
STORAGE_PATH=./data/uploads
S3_BUCKET=
S3_ENDPOINT=
S3_ACCESS_KEY=
S3_SECRET_KEY=
```

### Driver Database

| Driver | DSN Example |
|--------|-------------|
| `postgres` | `postgres://user:pass@localhost:5432/openjira?sslmode=disable` |
| `mysql` | `mysql://user:pass@tcp(localhost:3306)/openjira?charset=utf8mb4` |
| `sqlite` | `file:./openjira.db?cache=shared` |

## Docker

### Build immagini

```bash
# API Server
docker build -t open-jira/api:latest -f Dockerfile .

# Worker (email, webhook, automation)
docker build -t open-jira/worker:latest -f Dockerfile.worker .

# Frontend (Nginx + SPA)
docker build -t open-jira/frontend:latest -f Dockerfile.frontend .
```

### Run con Docker Compose (stack completo)

```bash
docker compose -f deploy/docker/docker-compose.yml up -d
```

## Kubernetes (Helm)

```bash
# Installa il chart
helm install open-jira ./deploy/helm/open-jira \
  --set config.appSecret=<your-jwt-secret> \
  --set config.baseUrl=https://jira.example.com \
  --set config.smtpHost=smtp.example.com

# Verifica lo stato
kubectl get pods -l app.kubernetes.io/name=open-jira

# Upgrade
helm upgrade open-jira ./deploy/helm/open-jira --set config.appSecret=<new-secret>

# Rimozione
helm uninstall open-jira
```

### Parametri Helm principali

| Parametro | Default | Descrizione |
|-----------|---------|-------------|
| `replicaCount` | `1` | Numero repliche API server |
| `image.api` | `open-jira/api:latest` | Immagine API server |
| `image.worker` | `open-jira/worker:latest` | Immagine worker |
| `image.frontend` | `open-jira/frontend:latest` | Immagine frontend |
| `config.appSecret` | `""` | Chiave JWT — **obbligatorio** |
| `config.dbDriver` | `postgres` | Driver database |
| `config.dbDsn` | `postgres://...` | DSN database |
| `config.redisUrl` | `redis://redis:6379/0` | URL Redis |
| `config.baseUrl` | `http://localhost` | Base URL pubblico |
| `config.smtpHost` | `""` | Host SMTP |
| `postgresql.enabled` | `true` | Installa PostgreSQL nel cluster |
| `redis.enabled` | `true` | Installa Redis nel cluster |

### Stack Kubernetes

```
├── Deployment: API server (scalabile orizzontalmente)
├── Deployment: Frontend (Nginx + SPA)
├── Deployment: Worker (email, automation, webhook)
├── StatefulSet: PostgreSQL (o referenziato esterno)
├── Deployment: Redis (cache + queue)
├── ConfigMap / Secret: config, JWT secret, OAuth credentials
├── Ingress: nginx-ingress o Traefik
└── PersistentVolumeClaim: upload allegati
```

## API REST

Documentazione OpenAPI completa: `docs/api/openapi.yaml`

### Endpoint principali

```
# Auth
POST   /api/v1/auth/register
POST   /api/v1/auth/login
GET    /api/v1/auth/oauth/{provider}/redirect
GET    /api/v1/auth/oauth/{provider}/callback

# Utenti
GET    /api/v1/users/me

# Progetti
GET    /api/v1/projects
POST   /api/v1/projects
GET    /api/v1/projects/{key}
PATCH  /api/v1/projects/{key}
DELETE /api/v1/projects/{key}

# Issues
GET    /api/v1/projects/{key}/issues
POST   /api/v1/projects/{key}/issues
GET    /api/v1/issues/{issueKey}
PATCH  /api/v1/issues/{issueKey}
DELETE /api/v1/issues/{issueKey}

# Workflow
GET    /api/v1/projects/{key}/workflow
POST   /api/v1/projects/{key}/workflow/statuses
POST   /api/v1/projects/{key}/workflow/transitions
POST   /api/v1/issues/{issueKey}/transition

# Sprint
GET    /api/v1/projects/{key}/sprints
POST   /api/v1/projects/{key}/sprints/{id}/start
POST   /api/v1/projects/{key}/sprints/{id}/complete

# Board
GET    /api/v1/projects/{key}/board
POST   /api/v1/issues/rank

# Ricerca
GET    /api/v1/search?q=project=PROJ type=Bug

# Reports
GET    /api/v1/projects/{key}/reports/burndown?sprintId=...

# Dashboard
GET    /api/v1/dashboards
POST   /api/v1/dashboards

# Notifiche
GET    /api/v1/notifications
PATCH  /api/v1/notifications/read-all

# Git
POST   /api/v1/projects/{key}/git/providers
POST   /api/v1/webhooks/git/{token}

# WebSocket
WS     /ws/v1/projects/{key}/board
```

### Autenticazione

Tutti gli endpoint protetti richiedono header:

```
Authorization: Bearer <jwt-token>
```

## Configurazione OAuth

### Forgejo / Gitea

1. Vai su **Settings → Applications → Manage OAuth2 Applications**
2. Crea una nuova applicazione con redirect URI: `<BASE_URL>/api/v1/auth/oauth/forgejo/callback`
3. Imposta le env vars:
   ```bash
   OAUTH_FORGEJO_CLIENT_ID=<client-id>
   OAUTH_FORGEJO_CLIENT_SECRET=<client-secret>
   OAUTH_FORGEJO_BASE_URL=https://git.example.com
   ```

### GitLab

1. Vai su **Settings → Applications**
2. Crea una nuova applicazione con redirect URI: `<BASE_URL>/api/v1/auth/oauth/gitlab/callback`
3. Scope: `read_user`

### GitHub

1. Vai su **Settings → Developer settings → OAuth Apps**
2. Crea una nuova app con callback URL: `<BASE_URL>/api/v1/auth/oauth/github/callback`

## Integrazione Git

Collega un repository Git al progetto per tracciare commit, branch e PR.

```bash
# Configura un provider Git per il progetto
curl -X POST /api/v1/projects/PROJ/git/providers \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "provider_type": "forgejo",
    "base_url": "https://git.example.com",
    "token": "<api-token>",
    "webhook_secret": "<webhook-secret>"
  }'
```

Aggiungi il webhook sul tuo repository Git:
```
URL: <BASE_URL>/api/v1/webhooks/git/<webhook-secret>
Eventi: Push, Pull Request
Content type: application/json
Secret: <webhook-secret>
```

I branch con formato `PROJ-123-fix-bug` vengono automaticamente linkati all'issue `PROJ-123`.

## Sviluppo

### Struttura Repository

```
/
├── cmd/
│   ├── server/           # API server
│   └── worker/           # Worker async
├── internal/
│   ├── api/              # HTTP handlers, middleware, WebSocket
│   ├── domain/           # Business logic
│   │   ├── issue/        # Issue, commenti, allegati
│   │   ├── sprint/       # Sprint
│   │   ├── project/      # Progetto, membri, inviti
│   │   ├── workflow/     # Workflow, stati, transizioni
│   │   ├── notification/ # Notifiche, impostazioni
│   │   ├── git/          # Integrazione Git
│   │   ├── auth/         # JWT, password, OAuth
│   │   ├── search/       # Ricerca, filtri, JQL
│   │   ├── report/       # Burndown, velocity, CFD
│   │   ├── dashboard/    # Dashboard, widget
│   │   ├── customfield/  # Campi custom
│   │   ├── automation/   # Automazioni
│   │   ├── timeline/     # Timeline/Gantt
│   │   └── calendar/     # Vista calendario
│   ├── store/            # DB connection + migrations runner
│   ├── config/           # Config loader
│   └── log/              # Logger strutturato (slog)
├── migrations/           # SQL migration files
├── frontend/             # React SPA
│   └── src/
│       ├── components/   # Componenti UI riusabili
│       ├── pages/        # Pagine (Board, Backlog, IssueDetail, ...)
│       ├── hooks/        # Custom hooks
│       ├── store/        # Zustand store
│       └── lib/          # Utility, API client
├── deploy/
│   ├── helm/             # Helm chart Kubernetes
│   └── docker/           # Docker Compose sviluppo
├── docs/
│   └── api/              # OpenAPI spec
└── .github/workflows/    # CI/CD
```

### Test

```bash
# Unit e integration test
go test ./... -v -count=1

# Lint
go vet ./...

# Frontend typecheck + build
cd frontend
npx tsc -b
npx vite build
```

### Database Migrations

Le migration sono in `migrations/` e vengono eseguite automaticamente all'avvio del server tramite `golang-migrate`.

Per generare nuove migration:

```bash
migrate create -ext sql -dir migrations/ -seq <nome-migration>
```

## CI/CD

La GitHub Actions pipeline (`lint` + `test` + `build`) si trova in `.github/workflows/ci.yml`.

## Licenza

MIT

---

**Versione:** 1.0.0 | **Tag:** [v1.0.0](https://github.com/open-jira/open-jira/releases/tag/v1.0.0)
