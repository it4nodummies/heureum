# Open Jira — Design Document

> **Data:** 2026-05-14
> **Decisioni:** Go backend, React frontend, PostgreSQL primario, step atomici per Ralph-loop
> **Riferimento:** `jira-opensource-spec.md` (analisi harpaitalia.atlassian.net — 13/05/2026)

---

## Obiettivo

Clone open-source di Jira Cloud, deployabile su Kubernetes, con supporto PostgreSQL/MariaDB/SQLite e integrazione Git multi-provider (Forgejo, GitLab, GitHub). Il sistema rimpiazza l'intero stack Atlassian (Jira + Bitbucket) per team Scrum/Kanban.

---

## Stack Tecnico

| Layer | Tecnologia |
|-------|-----------|
| Backend | Go 1.22+ |
| ORM | GORM |
| Migrazioni | golang-migrate |
| API | REST + WebSocket (gorilla/mux, gorilla/websocket) |
| Auth | JWT + OAuth2 (Forgejo, GitLab, GitHub) |
| Frontend | React 18 + TypeScript + Vite |
| UI | shadcn/ui + Tailwind CSS |
| State | TanStack Query + Zustand |
| Drag & Drop | dnd-kit |
| Editor | TipTap |
| Charts | Recharts |
| DB Primario | PostgreSQL 15+ |
| DB Alternativi | MariaDB 10.6+, SQLite (sviluppo) |
| Cache/Queue | Redis |
| Container | Docker + Kubernetes + Helm |

---

## Architettura

```
┌─────────────────────────────────────────────────────────┐
│                     FRONTEND (React SPA)                │
│  Board │ Backlog │ Timeline │ Reports │ Settings │ Admin│
└────────────────────────┬────────────────────────────────┘
                         │ REST API + WebSocket
┌────────────────────────▼────────────────────────────────┐
│                   API SERVER (Go)                       │
│  Projects │ Issues │ Workflows │ Sprints │ Search       │
│  Reports  │ Users  │ Auth      │ Notifiche │ Git        │
│  Webhooks │ Dashboard │ Automation                      │
└──────┬──────────────┬──────────────────────┬────────────┘
       │              │                      │
┌──────▼───────┐ ┌─────▼────┐         ┌──────▼──────┐
│  PostgreSQL  │ │  Redis   │         │   Worker    │
│  MariaDB     │ │ (cache + │         │  (async:    │
│  SQLite      │ │  queue)  │         │  email,     │
└──────────────┘ └──────────┘         │  webhooks,  │
                                      │  automation)│
                                      └─────────────┘
```

---

## Struttura Repository

```
/
├── cmd/
│   ├── server/           # main API server
│   └── worker/           # async worker
├── internal/
│   ├── api/              # HTTP handlers e routing
│   │   ├── middleware/   # auth, rate-limit, cors
│   │   └── handlers/     # per ogni dominio
│   ├── domain/           # business logic pura
│   │   ├── issue/
│   │   ├── sprint/
│   │   ├── project/
│   │   ├── workflow/
│   │   ├── notification/
│   │   └── git/
│   ├── store/            # DB layer (GORM)
│   │   ├── postgres/
│   │   ├── mysql/
│   │   └── sqlite/
│   ├── worker/           # task definitions
│   └── config/           # config loading
├── migrations/           # SQL migration files
├── frontend/             # React SPA
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── hooks/
│   │   ├── store/
│   │   └── lib/
│   └── package.json
├── deploy/
│   ├── helm/             # Helm chart
│   └── docker/           # Dockerfiles + compose
├── docs/
│   └── api/              # OpenAPI spec
└── README.md
```

---

## Approccio: Vertical Slices (Step Atomici per Ralph-loop)

12 step sequenziali con dipendenze. Ogni step = un modulo completo (DB → domain → API → frontend dove applicabile), eseguibile da un ralph-loop indipendente.

```
Step 1: Infrastructure Core
  └──→ Step 2: Auth & Users
        └──→ Step 3: Projects Core
              └──→ Step 4: Issues Core
                    ├──→ Step 5: Workflow Engine
                    ├──→ Step 6: Board & Backlog
                    ├──→ Step 7: Comments & History
                    ├──→ Step 8: Search & Filters
                    ├──→ Step 9: Reports & Dashboard
                    ├──→ Step 10: Notifications
                    ├──→ Step 11: Git Integration
                    └──→ Step 12: Advanced Features & Deploy
```

## Contenuto di ogni Step

### Step 1: Infrastructure Core
- Inizializzazione modulo Go, directory structure
- Config loader (env vars + YAML), logger (slog)
- DB connection pool (PostgreSQL + SQLite), Redis connection
- Migration files SQL completi (schema intero della Sezione 7 delle specifiche)
- Docker Compose per dev (PostgreSQL + Redis)
- GitHub Actions CI (lint, test, build)

### Step 2: Auth & Users
- Registrazione/login email/password con bcrypt + JWT
- OAuth2 flow (Forgejo, GitLab, GitHub)
- Middleware auth JWT, refresh token
- CRUD utenti, ruoli globali (admin/user)
- Tabella `oauth_tokens`

### Step 3: Projects Core
- CRUD progetti (name, key, type, lead, icon, description)
- Tipi: Software/Scrum, Software/Kanban, Business
- Project members con ruoli (admin/member/viewer)
- Invito utenti via token email
- Archivio progetto (soft-delete)

### Step 4: Issues Core
- CRUD issue (summary, description, priority, status, assignee, reporter)
- Tipi issue: Epic, Story, Task, Bug, Subtask
- Gerarchia parent/child (Epic → Story/Task → Subtask)
- Labels multi-valore, story points, due date/start date
- Linked issues (blocks, is_blocked, duplicates, relates)
- Issue key generazione automatica (PROJ-123)
- Archiviazione issue (soft-delete)

### Step 5: Workflow Engine
- Workflow CRUD e associazione a progetto
- Status CRUD (category: todo/inprogress/done, colore, posizione)
- Transizioni configurabili tra status
- Workflow default: TO DO → IN PROGRESS → DONE
- API `/issues/{key}/transition` con validazione
- Condizioni base su transizioni

### Step 6: Board & Backlog
- Board Scrum con colonne per status
- Board Kanban (senza sprint)
- Drag & drop card tra colonne (dnd-kit + persist `position` float)
- Filtro (assignee, label, priority, tipo) e ricerca
- Backlog con sprint management (crea, avvia, completa)
- Drag & drop backlog per ordinamento e assegnazione sprint
- WebSocket per real-time board updates

### Step 7: Comments & History
- Commenti con editor TipTap (rich-text)
- Menzioni @utente con link a notifica
- History/Activity log automatico
- Watchers (segui/smetti di seguire issue)
- Allegati file (upload locale + opzione S3/MinIO)
- Soft-delete commenti

### Step 8: Search & Filters
- Full-text search PostgreSQL (titolo, descrizione, commenti)
- Filtri avanzati (progetto, tipo, status, assignee, sprint, label, epic)
- Ricerca cross-project
- JQL-like query parser (sintassi → SQL parametrizzato)
- Filtri salvati (personali e condivisi)

### Step 9: Reports & Dashboard
- Sprint Burndown Chart (story points vs tempo)
- Summary progetto (stats 7gg, conteggio per status, activity feed)
- Dashboard personale configurabile
- Widget: Assigned to Me, Activity Streams
- Charts con Recharts

### Step 10: Notifications
- Notifiche in-app (campanella + lista)
- Email notifications (assegnazione, commenti, menzioni, cambio stato)
- Configurazione per utente (opt-out per tipo evento) e per progetto
- SMTP integration
- WebSocket per notifiche real-time
- Webhook outbound configurabili

### Step 11: Git Integration
- Interfaccia `GitProvider` con implementazioni (Forgejo, GitLab, GitHub)
- Webhook receiver con verifica firma HMAC
- Auto-link branch/commit a issue (pattern `PROJ-123` nel nome branch)
- Visualizzazione commit, branch, PR/MR nell'issue
- Transizione automatica status su merge PR

### Step 12: Advanced Features & Deploy
- Timeline / Gantt view (barre per epic, zoom settimane/mesi/trimestri)
- Calendar view (issue con date, sprint come eventi)
- Campi custom (text, number, date, select, multi-select)
- Automation base (trigger → condizione → azione)
- Export CSV
- Helm chart Kubernetes (API server, Worker, Frontend, PostgreSQL, Redis, Ingress, PVC)
- Dockerfile (API, Worker, Frontend) + Docker Compose dev
- Worker async (email, webhook processing, automation trigger)

---

## Convezioni

| Aspetto | Standard |
|---------|----------|
| Modulo Go | `github.com/open-jira/open-jira` |
| Branch | `feature/step-N-<nome-modulo>` |
| Commit | `feat(step-N): <descrizione>` |
| Test | Unit (Go test), Integration (testcontainers-go), E2E (Playwright) |
| API | REST `/api/v1/`, WebSocket `/ws/v1/` |
| TDD | Test → impl → refactor → commit per ogni feature |

---

## Testing Strategy

- **Unit test:** Domain logic pura, nessuna dipendenza esterna
- **Integration test:** `testcontainers-go` con PostgreSQL reale, test HTTP con `httptest`
- **E2E test:** Playwright per flussi critici (crea issue, drag board, completa sprint)
- **Coverage target:** 80%+ per dominio, 60%+ complessivo

---

## Decisioni Chiave

1. **Go** come backend — performance, binario singolo, GORM multi-dialect
2. **Step atomici** — ogni step completato e testato prima di passare al prossimo
3. **Ralph-loop** — ogni step eseguito da un agente ralph-loop autonomo con TDD
4. **PostgreSQL-first** — ma con astrazione DB layer per supportare MariaDB e SQLite
5. **WebSocket** — hub pub/sub in-memory (Go channels) con Redis message bus per scalabilità
6. **Position field** (float) per drag & drop ordering (LexoRank pattern)
7. **JQL parser** — lexer/parser che genera SQL parametrizzato (no string concatenation)
