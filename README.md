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

## API REST (Jira Cloud v3 aligned)

Documentazione OpenAPI completa: `docs/api/openapi.yaml`

Base URL: `/rest/api/3`

### Auth
```
POST   /rest/api/3/auth/register
POST   /rest/api/3/auth/login
GET    /rest/api/3/auth/oauth/{provider}/redirect
GET    /rest/api/3/auth/oauth/{provider}/callback
```

### Myself
```
GET    /rest/api/3/myself
```

### Users
```
GET    /rest/api/3/users/search?query={name}
GET    /rest/api/3/user?accountId={id}
```

### Projects
```
GET    /rest/api/3/project                              # list all
GET    /rest/api/3/project/search                       # paginated search
POST   /rest/api/3/project                              # create
GET    /rest/api/3/project/{projectIdOrKey}              # get
PUT    /rest/api/3/project/{projectIdOrKey}              # update
DELETE /rest/api/3/project/{projectIdOrKey}              # delete
GET    /rest/api/3/project/{projectIdOrKey}/statuses     # project statuses
```

### Issues
```
GET    /rest/api/3/issue/{issueIdOrKey}                  # get issue
POST   /rest/api/3/issue                                 # create issue
PUT    /rest/api/3/issue/{issueIdOrKey}                  # edit issue
DELETE /rest/api/3/issue/{issueIdOrKey}                  # delete issue
PUT    /rest/api/3/issue/{issueIdOrKey}/assignee         # assign issue
GET    /rest/api/3/issue/{issueIdOrKey}/changelog        # get changelog
GET    /rest/api/3/issue/{issueIdOrKey}/transitions      # get available transitions
POST   /rest/api/3/issue/{issueIdOrKey}/transitions      # transition issue
POST   /rest/api/3/issue/{issueIdOrKey}/notify           # send notification
```

### Comments
```
GET    /rest/api/3/issue/{issueIdOrKey}/comment           # list comments
POST   /rest/api/3/issue/{issueIdOrKey}/comment           # add comment
GET    /rest/api/3/issue/{issueIdOrKey}/comment/{id}      # get comment
PUT    /rest/api/3/issue/{issueIdOrKey}/comment/{id}      # update comment
DELETE /rest/api/3/issue/{issueIdOrKey}/comment/{id}      # delete comment
POST   /rest/api/3/comment/list                           # get comments by IDs
```

### Attachments
```
POST   /rest/api/3/issue/{issueIdOrKey}/attachments       # upload attachment
GET    /rest/api/3/attachment/{id}                        # get metadata
GET    /rest/api/3/attachment/content/{id}                # download content
DELETE /rest/api/3/attachment/{id}                        # delete attachment
GET    /rest/api/3/attachment/meta                        # attachment settings
```

### Issue Links
```
POST   /rest/api/3/issueLink                              # create link
GET    /rest/api/3/issueLink/{linkId}                     # get link
DELETE /rest/api/3/issueLink/{linkId}                     # delete link
```

### Watchers
```
GET    /rest/api/3/issue/{issueIdOrKey}/watchers          # list watchers
POST   /rest/api/3/issue/{issueIdOrKey}/watchers          # add watcher
DELETE /rest/api/3/issue/{issueIdOrKey}/watchers?username=X # remove watcher
```

### Workflows & Statuses
```
GET    /rest/api/3/workflow/search?projectId={id}         # search workflows
GET    /rest/api/3/status                                 # all statuses
GET    /rest/api/3/status/{idOrName}                      # get status
```

### Dashboards
```
GET    /rest/api/3/dashboard                              # list dashboards
POST   /rest/api/3/dashboard                              # create dashboard
GET    /rest/api/3/dashboard/{id}                         # get dashboard
PUT    /rest/api/3/dashboard/{id}                         # update dashboard
DELETE /rest/api/3/dashboard/{id}                         # delete dashboard
GET    /rest/api/3/dashboard/search                       # search dashboards
POST   /rest/api/3/dashboard/{id}/copy                    # copy dashboard
POST   /rest/api/3/dashboard/{dashboardId}/gadget         # add gadget
DELETE /rest/api/3/dashboard/{dashboardId}/gadget/{id}    # remove gadget
```

### Filters
```
GET    /rest/api/3/filter/my                              # my filters
GET    /rest/api/3/filter/favourite                       # favorite filters
GET    /rest/api/3/filter/search                          # search filters
POST   /rest/api/3/filter                                 # create filter
GET    /rest/api/3/filter/{id}                            # get filter
PUT    /rest/api/3/filter/{id}                            # update filter
DELETE /rest/api/3/filter/{id}                            # delete filter
PUT    /rest/api/3/filter/{id}/favourite                  # add favorite
DELETE /rest/api/3/filter/{id}/favourite                  # remove favorite
```

### Search
```
POST   /rest/api/3/search                                 # JQL search
GET    /rest/api/3/search/jql?jql=...                     # JQL search GET
```

### Board & Backlog
```
GET    /rest/api/3/project/{key}/board                    # board view
POST   /rest/api/3/issue/rank                             # reorder issues
GET    /rest/api/3/project/{key}/sprints                  # list sprints
POST   /rest/api/3/project/{key}/sprints                  # create sprint
POST   /rest/api/3/project/{key}/sprints/{id}/start       # start sprint
POST   /rest/api/3/project/{key}/sprints/{id}/complete    # complete sprint
```

### Reports
```
GET    /rest/api/3/project/{key}/reports/burndown?sprintId=...
GET    /rest/api/3/project/{key}/reports/velocity
GET    /rest/api/3/project/{key}/summary
GET    /rest/api/3/project/{key}/issues/export?format=csv
```

### Notifications
```
GET    /rest/api/3/notifications
PATCH  /rest/api/3/notifications/read-all
GET    /rest/api/3/notifications/settings
PATCH  /rest/api/3/notifications/settings
```

### Git Integration
```
POST   /rest/api/3/project/{key}/git/providers
POST   /rest/api/3/webhooks/git/{token}
```

### WebSocket
```
WS     /ws/v1/project/{key}/board
```

### Autenticazione

Tutti gli endpoint protetti richiedono header:

```
Authorization: Bearer <jwt-token>
```

## Configurazione OAuth

### Forgejo / Gitea

1. Vai su **Settings → Applications → Manage OAuth2 Applications**
2. Crea una nuova applicazione con redirect URI: `<BASE_URL>/rest/api/3/auth/oauth/forgejo/callback`
3. Imposta le env vars:
   ```bash
   OAUTH_FORGEJO_CLIENT_ID=<client-id>
   OAUTH_FORGEJO_CLIENT_SECRET=<client-secret>
   OAUTH_FORGEJO_BASE_URL=https://git.example.com
   ```

### GitLab

1. Vai su **Settings → Applications**
2. Crea una nuova applicazione con redirect URI: `<BASE_URL>/rest/api/3/auth/oauth/gitlab/callback`
3. Scope: `read_user`

### GitHub

1. Vai su **Settings → Developer settings → OAuth Apps**
2. Crea una nuova app con callback URL: `<BASE_URL>/rest/api/3/auth/oauth/github/callback`

## Integrazione Git

Collega un repository Git al progetto per tracciare commit, branch e PR.

```bash
# Configura un provider Git per il progetto
curl -X POST /rest/api/3/project/PROJ/git/providers \
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
URL: <BASE_URL>/rest/api/3/webhooks/git/<webhook-secret>
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

**Versione:** 2.0.0 | **Tag:** [v2.0.0](https://github.com/open-jira/open-jira/releases/tag/v2.0.0)
