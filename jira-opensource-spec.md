# Specifiche: Clone Open-Source di Jira su Kubernetes

> **Documento di specifica** per la reimplementazione open-source di Jira.  
> Basato su analisi diretta dell'istanza Jira `harpaitalia.atlassian.net` — esplorata il 13/05/2026.  
> Destinato a essere usato come prompt di contesto per **Claude Code**.

---

## 1. Obiettivo del Progetto

Realizzare un'applicazione web di **project management agile** open-source, autosufficiente, distribuibile su **Kubernetes**, con supporto nativo a **PostgreSQL** (primario), **MariaDB** e **SQLite** (sviluppo/testing), e integrazione con qualsiasi piattaforma Git (Forgejo, GitLab, Gitea, GitHub, ecc.) al posto di Bitbucket.

Il sistema deve essere una valida alternativa a Jira Cloud per team software che adottano metodologie Scrum o Kanban.

---

## 2. Stack Tecnico

### Backend
- **Linguaggio**: Go (consigliato per performance e binario singolo) oppure Python (FastAPI)
- **API**: REST + WebSocket per aggiornamenti real-time
- **Auth**: JWT + OAuth2 (OIDC — compatibile con Keycloak, Auth0, Forgejo, GitLab)
- **Task queue**: Redis + worker per automation/notifiche async
- **Search**: PostgreSQL full-text search (base); opzionale Elasticsearch/Meilisearch per ricerca avanzata

### Frontend
- **Framework**: React (TypeScript) + Vite
- **UI**: shadcn/ui o Radix UI + Tailwind CSS
- **State management**: TanStack Query (React Query) + Zustand
- **Drag & drop**: dnd-kit
- **Rich text editor**: TipTap (compatibile con Markdown e formato Atlassian Document Format-like)
- **Charts/Reports**: Recharts o Chart.js
- **Gantt/Timeline**: custom su canvas o libreria DHTMLX Gantt open-source

### Database
| DB | Uso | Note |
|----|-----|-------|
| **PostgreSQL 15+** | Produzione primaria | JSONB per campi custom, full-text search nativo |
| **MariaDB 10.6+** | Alternativa produzione | Via ORM con dialect separato |
| **SQLite** | Sviluppo / demo locale | Via driver `modernc.org/sqlite` (no CGO) |

ORM: **GORM** (Go) o **SQLAlchemy** (Python) con migration via **golang-migrate** o **Alembic**.

### Kubernetes & Infrastruttura
```
├── Deployment: API server (scalabile orizzontalmente)
├── Deployment: Frontend (Nginx + SPA)
├── Deployment: Worker (code automation/notification)
├── StatefulSet: PostgreSQL (o referenziato esterno)
├── Deployment: Redis (cache + queue)
├── ConfigMap/Secret: config DB, JWT secret, Git provider OAuth
├── Ingress: nginx-ingress o Traefik
├── PersistentVolumeClaim: upload file/allegati
└── CronJob: cleanup sessioni scadute, archivio issue
```

**Helm chart** incluso nel repository per deploy one-command.

### Integrazione Git
Interfaccia generica `GitProvider` con implementazioni per:
- **Forgejo / Gitea** (API compatibile)
- **GitLab** (API v4)
- **GitHub** (REST API v3 / GraphQL)
- **Bitbucket** (per retrocompatibilità)
- Configurabile via webhook + OAuth App

---

## 3. Feature Classification

### Legenda
| Simbolo | Categoria | Significato |
|---------|-----------|-------------|
| 🟢 | **FONDAMENTALE** | Necessario per MVP — il sistema non è usabile senza |
| 🟡 | **IMPORTANTE** | Aggiunge valore significativo — da implementare nella v1.x |
| 🔴 | **NON NECESSARIO** | Avanzato/enterprise — da valutare in versioni future |

---

## 4. Feature per Area

### 4.1 Gestione Progetti (Spaces/Projects)

| Feature | Priorità | Note |
|---------|----------|-------|
| Creazione progetto con nome, chiave univoca, icona | 🟢 | |
| Tipi progetto: Software (Scrum), Software (Kanban), Business | 🟢 | |
| Project lead / owner | 🟢 | |
| Default assignee (Unassigned / Project Lead) | 🟢 | |
| Elenco progetti con filtro e ricerca | 🟢 | |
| Archivio progetto (soft-delete) | 🟡 | |
| Categorie progetto | 🟡 | |
| Template di progetto predefiniti | 🟡 | |
| Project key multipli (per migrazioni) | 🔴 | |

---

### 4.2 Work Items (Issue / Ticket)

| Feature | Priorità | Note |
|---------|----------|-------|
| Creazione issue con tipo (Epic, Story, Task, Bug, Subtask) | 🟢 | |
| Campo Summary (titolo) obbligatorio | 🟢 | |
| Descrizione rich-text (grassetto, liste, codice inline, link) | 🟢 | |
| Status con workflow configurabile (es. TO DO → IN PROGRESS → DONE) | 🟢 | |
| Assignee (utente singolo) | 🟢 | |
| Reporter | 🟢 | |
| Priority (Highest, High, Medium, Low, Lowest) | 🟢 | |
| Labels / Tag (multi-valore, liberi) | 🟢 | |
| Parent/Child (gerarchia: Epic → Story/Task → Subtask) | 🟢 | |
| Sprint (assegnazione a sprint) | 🟢 | |
| Story point estimate | 🟢 | |
| Linked work items (blocca, è bloccato da, duplica, si collega a) | 🟢 | |
| Subtask inline | 🟢 | |
| Commenti con menzioni @utente | 🟢 | |
| History / Activity log (chi ha cambiato cosa e quando) | 🟢 | |
| Timestamp creazione / modifica | 🟢 | |
| Watchers (seguire un issue) | 🟢 | |
| Fix versions / Release | 🟡 | |
| Time tracking (original estimate, time spent, remaining) | 🟡 | |
| Allegati file (upload) | 🟡 | |
| Ambiente (Environment field) | 🟡 | |
| Due date / Start date | 🟡 | |
| Campi custom numerici, testo, data, select, multi-select | 🟡 | |
| Bulk edit issue | 🟡 | |
| Issue template | 🟡 | |
| Commenti con formattazione Markdown | 🟡 | |
| Shortcut azioni rapide da tastiera | 🟡 | |
| Archiviazione issue | 🔴 | |
| Voto (Vote) su issue | 🔴 | |
| Planning Poker integrato | 🔴 | |

---

### 4.3 Tipi di Lavoro (Work Types)

| Feature | Priorità | Note |
|---------|----------|-------|
| Epic | 🟢 | |
| Story | 🟢 | |
| Task | 🟢 | |
| Bug | 🟢 | |
| Subtask | 🟢 | |
| Aggiunta work type custom | 🟡 | |
| Icone / colori per work type | 🟡 | |
| Campi specifici per work type | 🟡 | |
| Workflow separato per work type | 🔴 | |

---

### 4.4 Workflow

| Feature | Priorità | Note |
|---------|----------|-------|
| Workflow default: TO DO → IN PROGRESS → DONE | 🟢 | |
| Workflow personalizzato per progetto (stati custom) | 🟢 | |
| Transizioni tra stati (definite graficamente o via config) | 🟢 | |
| Colori per colonna/status | 🟢 | |
| Stato TO BE TESTED (o intermedi custom) | 🟡 | |
| Condizioni sulle transizioni (es. "richiede commento") | 🟡 | |
| Trigger automatici su transizione (es. assegna automaticamente) | 🟡 | |
| Workflow globale condiviso tra progetti | 🔴 | |
| Approvazioni su transizione | 🔴 | |

---

### 4.5 Board (Scrum / Kanban)

| Feature | Priorità | Note |
|---------|----------|-------|
| Board Scrum con colonne per stato | 🟢 | |
| Board Kanban (senza sprint) | 🟢 | |
| Drag & drop card tra colonne | 🟢 | |
| Filtro per assignee, label, priority, tipo | 🟢 | |
| Ricerca nella board | 🟢 | |
| Card con: titolo, ID, tipo, priority, assignee avatar, story points | 🟢 | |
| Colonna "DONE" con WIP configurabile | 🟡 | |
| Group by (raggruppamento per Epic, Assignee, Label) | 🟡 | |
| Swimlanes | 🟡 | |
| Visualizzazione "Epics" in board | 🟡 | |
| Board condivisa tra più progetti | 🔴 | |

---

### 4.6 Backlog

| Feature | Priorità | Note |
|---------|----------|-------|
| Backlog con tutti gli issue non assegnati a sprint | 🟢 | |
| Sprint attivo nel backlog (collassabile) | 🟢 | |
| Drag & drop per riordinare e spostare in sprint | 🟢 | |
| Creazione inline issue nel backlog | 🟢 | |
| Story points per sprint (budget / velocità) | 🟢 | |
| Filtro backlog per assignee, tipo, label, epic | 🟢 | |
| Creazione nuovo sprint | 🟢 | |
| Start sprint (con nome, date, goal) | 🟢 | |
| Complete sprint (con opzione di spostare issue aperti) | 🟢 | |
| Sprint multipli attivi contemporaneamente | 🟡 | |
| Epics nel backlog come raggruppatori | 🟡 | |
| Stima velocità sprint suggerita | 🔴 | |

---

### 4.7 Timeline (Gantt / Roadmap)

| Feature | Priorità | Note |
|---------|----------|-------|
| Vista timeline con epics come barre | 🟡 | |
| Visualizzazione sprint su timeline | 🟡 | |
| Filtro per Epic, Tipo, Label, Status | 🟡 | |
| Zoom: Settimane / Mesi / Trimestri | 🟡 | |
| Drag & drop date su timeline | 🟡 | |
| Dipendenze tra issue (frecce) | 🔴 | |
| Cross-project timeline (Advanced Roadmaps) | 🔴 | |
| Roadmap pubblica (share link) | 🔴 | |

---

### 4.8 Calendar

| Feature | Priorità | Note |
|---------|----------|-------|
| Vista calendario mensile con issue che hanno due date | 🟡 | |
| Sprint come eventi su calendario | 🟡 | |
| Pannello "Unscheduled work" drag onto calendar | 🟡 | |
| Filtro per Assignee, Tipo, Status | 🟡 | |
| Vista settimanale | 🔴 | |

---

### 4.9 List View

| Feature | Priorità | Note |
|---------|----------|-------|
| Lista tabellare di tutti gli issue del progetto | 🟢 | |
| Colonne: Tipo, ID, Titolo, Assignee, Reporter, Priority, Status | 🟢 | |
| Ordinamento per colonna | 🟢 | |
| Filtro e ricerca | 🟢 | |
| Modalità colonne configurabili | 🟡 | |
| Export CSV/Excel | 🟡 | |
| Inline edit (modifica direttamente nella lista) | 🟡 | |
| Saved views | 🟡 | |

---

### 4.10 Report e Analytics

| Feature | Priorità | Note |
|---------|----------|-------|
| Sprint Burndown Chart (story points rimanenti nel tempo) | 🟢 | |
| Velocity Report (story points completati per sprint) | 🟡 | |
| Burnup Chart (lavoro completato vs scope totale) | 🟡 | |
| Cumulative Flow Diagram (distribuzione stati nel tempo) | 🟡 | |
| Cycle Time Report (tempo medio issue → done) | 🔴 | |
| Deployment Frequency Report | 🔴 | |
| Distribuzione per tipo, priorità, assignee | 🟡 | |
| Epic Progress Report | 🟡 | |

---

### 4.11 Summary Progetto

| Feature | Priorità | Note |
|---------|----------|-------|
| Stats: completati/aggiornati/creati (ultimi 7 gg) | 🟢 | |
| Status overview (conteggio per stato) | 🟢 | |
| Activity feed (chi ha fatto cosa) | 🟢 | |
| Priority breakdown | 🟡 | |
| Distribuzione per tipo di lavoro | 🟡 | |
| Team workload (issue per membro) | 🟡 | |
| Epic progress (% completamento per epic) | 🟡 | |

---

### 4.12 Ricerca e Filtri

| Feature | Priorità | Note |
|---------|----------|-------|
| Ricerca globale full-text (titolo, descrizione, commenti) | 🟢 | |
| Filtri avanzati: progetto, tipo, status, assignee, sprint, label, epic | 🟢 | |
| JQL-like query language (sintassi strutturata) | 🟡 | |
| Filtri salvati per utente | 🟡 | |
| Filtri condivisi con il team | 🟡 | |
| Ricerca globale cross-project | 🟡 | |
| AI search (Ask AI) | 🔴 | |

---

### 4.13 Dashboard

| Feature | Priorità | Note |
|---------|----------|-------|
| Dashboard personale configurabile | 🟡 | |
| Widget: "Assigned to Me" | 🟢 | |
| Widget: Activity Streams (feed aggiornamenti) | 🟡 | |
| Widget: Projects list | 🟡 | |
| Widget: Burndown / Sprint stats | 🟡 | |
| Crea nuove dashboard | 🟡 | |
| Condivisione dashboard (pubblico/privato/gruppo) | 🟡 | |
| Dashboard starred | 🔴 | |
| Widget: Introduction / Welcome text | 🔴 | |

---

### 4.14 Utenti, Ruoli e Permessi

| Feature | Priorità | Note |
|---------|----------|-------|
| Registrazione / login utenti | 🟢 | |
| OAuth2 / OIDC (GitLab, Forgejo, Keycloak, GitHub) | 🟢 | |
| Ruoli progetto: Administrator, Member, Viewer | 🟢 | |
| Invito utenti per email | 🟢 | |
| Access requests (richiesta accesso al progetto) | 🟡 | |
| Gruppi / Team con permessi aggregati | 🟡 | |
| Ruoli globali (admin istanza, utente normale) | 🟢 | |
| LDAP / SAML (SSO enterprise) | 🔴 | |
| Permessi granulari per singola operazione | 🔴 | |
| Audit log accessi | 🟡 | |

---

### 4.15 Notifiche

| Feature | Priorità | Note |
|---------|----------|-------|
| Notifiche in-app (campanella) | 🟢 | |
| Email notifiche (assegnazione, commenti, menzioni, cambio stato) | 🟢 | |
| Configurazione notifiche per progetto | 🟡 | |
| Configurazione notifiche per utente (opt-out) | 🟡 | |
| Webhook outbound (per integrazioni esterne) | 🟡 | |
| Notifiche push (PWA) | 🔴 | |

---

### 4.16 Automation

| Feature | Priorità | Note |
|---------|----------|-------|
| Regole base: trigger → condizione → azione | 🟡 | |
| Trigger: issue creato, aggiornato, transizione stato | 🟡 | |
| Azioni: cambia assignee, aggiungi label, transiziona, commenta | 🟡 | |
| Automazioni predefinite (template) | 🟡 | |
| Automazioni cross-project | 🔴 | |
| Automazione con AI | 🔴 | |
| Scheduled automation (cron-based) | 🔴 | |

---

### 4.17 Integrazione Git

| Feature | Priorità | Note |
|---------|----------|-------|
| Collegamento branch/commit a issue (via branch naming `PROJ-123`) | 🟢 | |
| Visualizzazione commit linkati nell'issue | 🟢 | |
| Visualizzazione PR/MR linkate nell'issue | 🟢 | |
| Webhook receiver (push events, PR events) | 🟢 | |
| Transizione automatica status su merge PR | 🟡 | |
| Supporto Forgejo / Gitea | 🟢 | |
| Supporto GitLab | 🟢 | |
| Supporto GitHub | 🟢 | |
| Supporto Bitbucket (retrocompatibilità) | 🔴 | |
| Code view (browse repository dentro Jira) | 🔴 | |
| Deployment tracking (da CI/CD) | 🔴 | |

---

### 4.18 Releases e Versioni

| Feature | Priorità | Note |
|---------|----------|-------|
| Creazione release/versione per progetto | 🟡 | |
| Associazione issue a una versione (Fix version) | 🟡 | |
| Release notes auto-generate | 🔴 | |
| Deploy tracking per versione | 🔴 | |

---

### 4.19 Security

| Feature | Priorità | Note |
|---------|----------|-------|
| HTTPS obbligatorio (TLS tramite ingress) | 🟢 | |
| CSRF protection | 🟢 | |
| Rate limiting API | 🟢 | |
| Secret management via Kubernetes Secrets | 🟢 | |
| Security level per issue (visibilità ristretta) | 🔴 | |
| Vulnerability scanning integrazione (Snyk-like) | 🔴 | |

---

### 4.20 Forms / Intake

| Feature | Priorità | Note |
|---------|----------|-------|
| Form pubblico per raccogliere richieste dall'esterno | 🔴 | |
| Form interno per creare issue con campi guidati | 🟡 | |

---

### 4.21 Docs / Wiki integrato

| Feature | Priorità | Note |
|---------|----------|-------|
| Pagine wiki per progetto (tipo Confluence light) | 🔴 | |
| Link a Confluence esterno | 🔴 | |

---

### 4.22 Goals / OKR

| Feature | Priorità | Note |
|---------|----------|-------|
| Obiettivi (Goals/OKR) collegati a progetti | 🔴 | |

---

### 4.23 Teams

| Feature | Priorità | Note |
|---------|----------|-------|
| Gestione team con membri | 🟡 | |
| Workload view per team | 🟡 | |
| Capacity planning | 🔴 | |

---

### 4.24 Standups

| Feature | Priorità | Note |
|---------|----------|-------|
| Standup view (filtro board su chi lavora a cosa) | 🔴 | |

---

## 5. MVP Scope (Versione 1.0)

Tutte le feature **🟢 FONDAMENTALI** costituiscono l'MVP. Il seguente elenco riassume cosa deve funzionare per il rilascio iniziale:

### Core
- [x] Multi-project con tipi Software (Scrum/Kanban)
- [x] Work items: Epic, Story, Task, Bug, Subtask con gerarchia
- [x] Workflow configurabile per progetto (stati + transizioni)
- [x] Campi standard: Summary, Description, Status, Assignee, Reporter, Priority, Labels, Parent, Sprint, Story Points, Linked items
- [x] Commenti, History/Activity log, Watchers
- [x] Subtask

### Board & Backlog
- [x] Board Scrum con drag & drop colonne
- [x] Board Kanban
- [x] Backlog con sprint management (crea, avvia, completa sprint)

### Viste
- [x] List view tabellare con filtro/ricerca
- [x] Summary progetto con stats e activity feed
- [x] Widget "Assigned to Me"

### Report
- [x] Sprint Burndown Chart

### Ricerca
- [x] Ricerca full-text globale
- [x] Filtri per progetto, tipo, stato, assignee, sprint, label

### Utenti & Auth
- [x] Login con email/password + OAuth2 (Forgejo, GitLab, GitHub)
- [x] Ruoli: Admin istanza, Admin progetto, Member, Viewer
- [x] Invito utenti per email
- [x] Notifiche in-app + email base

### Git Integration
- [x] Webhook receiver (push/PR events)
- [x] Link branch/commit/PR a issue
- [x] Supporto Forgejo, GitLab, GitHub

### Infra
- [x] PostgreSQL + MariaDB + SQLite
- [x] Kubernetes Helm chart
- [x] Docker image ufficiale
- [x] Configurazione via env variables + ConfigMap

---

## 6. Architettura Applicativa

```
┌─────────────────────────────────────────────────────────┐
│                     FRONTEND (React SPA)                 │
│  Board │ Backlog │ Timeline │ Reports │ Settings │ Admin  │
└────────────────────────┬────────────────────────────────┘
                         │ REST API + WebSocket
┌────────────────────────▼────────────────────────────────┐
│                   API SERVER (Go / FastAPI)              │
│                                                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │ Projects │  │  Issues  │  │  Users   │  │  Auth   │ │
│  └──────────┘  └──────────┘  └──────────┘  └─────────┘ │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │ Sprints  │  │Workflows │  │  Search  │  │Webhooks │ │
│  └──────────┘  └──────────┘  └──────────┘  └─────────┘ │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │ Reports  │  │Notifiche │  │   Git    │  │Automaz. │ │
│  └──────────┘  └──────────┘  │Providers │  └─────────┘ │
│                               └──────────┘              │
└──────┬──────────────┬──────────────────────┬────────────┘
       │              │                      │
┌──────▼──────┐ ┌─────▼────┐         ┌──────▼──────┐
│  PostgreSQL  │ │  Redis   │         │   Worker    │
│  MariaDB     │ │ (cache + │         │  (async:    │
│  SQLite      │ │  queue)  │         │  email,     │
└─────────────┘ └──────────┘         │  webhooks,  │
                                      │  automation)│
                                      └─────────────┘
```

---

## 7. Modello Dati (Schema DB Principale)

```sql
-- Organizzazione / Istanza
organizations (id, name, slug, settings_json)

-- Utenti
users (id, email, username, display_name, avatar_url, password_hash, 
       is_admin, is_active, created_at, updated_at)

oauth_tokens (id, user_id, provider, access_token, refresh_token, expires_at)

-- Progetti
projects (id, org_id, name, key, description, type [scrum|kanban|business],
          lead_user_id, default_assignee, icon_url, is_archived, created_at)

project_members (project_id, user_id, role [admin|member|viewer])

-- Workflow
workflows (id, project_id, name)
workflow_statuses (id, workflow_id, name, category [todo|inprogress|done], 
                   color, position)
workflow_transitions (id, from_status_id, to_status_id)

-- Sprint
sprints (id, project_id, name, goal, state [active|closed|future],
         start_date, end_date, created_at)

-- Versioni / Release
versions (id, project_id, name, description, release_date, released)

-- Work Items (Issues)
issues (id, project_id, key [PROJ-123], title, description_json,
        type_id, status_id, priority [highest|high|medium|low|lowest],
        assignee_id, reporter_id, parent_id, sprint_id, version_id,
        story_points, original_estimate, time_spent,
        start_date, due_date, environment,
        is_archived, position, created_at, updated_at)

issue_labels (issue_id, label_id)
labels (id, project_id, name, color)

issue_links (id, source_id, target_id, link_type [blocks|is_blocked|duplicates|relates])

-- Campi custom
custom_fields (id, project_id, name, field_type [text|number|date|select|multiselect|user])
custom_field_options (id, field_id, value, position)
issue_custom_values (issue_id, field_id, value_text, value_number, value_date, option_id)

-- Work Types (Issue Types)
issue_types (id, project_id, name, description, icon, color, is_subtask)

-- Subtask / Allegati / Commenti
issue_attachments (id, issue_id, filename, file_path, file_size, uploader_id, created_at)

comments (id, issue_id, author_id, body_json, created_at, updated_at, is_deleted)

-- History
issue_history (id, issue_id, actor_id, field_name, old_value, new_value, created_at)

-- Watchers
issue_watchers (issue_id, user_id)

-- Dashboard
dashboards (id, name, owner_id, is_public, layout_json)
dashboard_widgets (id, dashboard_id, widget_type, config_json, position_json)

-- Filtri salvati
saved_filters (id, project_id, owner_id, name, jql, is_shared)

-- Integrazione Git
git_providers (id, project_id, provider_type [forgejo|gitlab|github|gitea],
               base_url, token_encrypted, webhook_secret)

issue_commits (id, issue_id, provider_id, commit_sha, message, author, committed_at)
issue_branches (id, issue_id, provider_id, branch_name, repo_url)
issue_pull_requests (id, issue_id, provider_id, pr_number, title, url, 
                     state [open|merged|closed], created_at, merged_at)

-- Automation
automation_rules (id, project_id, name, is_active, trigger_type, 
                  conditions_json, actions_json)
automation_runs (id, rule_id, issue_id, triggered_at, status, log)

-- Notifiche
notifications (id, user_id, type, title, body, link, is_read, created_at)
notification_settings (user_id, project_id, event_type, via_email, via_app)

-- Webhook outbound
webhooks (id, project_id, url, secret, events_json, is_active)
```

---

## 8. API REST — Endpoint Principali

```
# Auth
POST   /api/v1/auth/register
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
GET    /api/v1/auth/oauth/{provider}/redirect
GET    /api/v1/auth/oauth/{provider}/callback
POST   /api/v1/auth/refresh

# Utenti
GET    /api/v1/users/me
PATCH  /api/v1/users/me
GET    /api/v1/users/{id}

# Progetti
GET    /api/v1/projects
POST   /api/v1/projects
GET    /api/v1/projects/{key}
PATCH  /api/v1/projects/{key}
DELETE /api/v1/projects/{key}
GET    /api/v1/projects/{key}/members
POST   /api/v1/projects/{key}/members
DELETE /api/v1/projects/{key}/members/{userId}

# Issues
GET    /api/v1/projects/{key}/issues          # con filtri JQL-like
POST   /api/v1/projects/{key}/issues
GET    /api/v1/issues/{issueKey}
PATCH  /api/v1/issues/{issueKey}
DELETE /api/v1/issues/{issueKey}
POST   /api/v1/issues/{issueKey}/transition
GET    /api/v1/issues/{issueKey}/comments
POST   /api/v1/issues/{issueKey}/comments
GET    /api/v1/issues/{issueKey}/history
POST   /api/v1/issues/{issueKey}/watch
DELETE /api/v1/issues/{issueKey}/watch
POST   /api/v1/issues/{issueKey}/attachments
GET    /api/v1/issues/{issueKey}/links
POST   /api/v1/issues/{issueKey}/links
DELETE /api/v1/issues/{issueKey}/links/{linkId}
POST   /api/v1/issues/rank                    # drag & drop ordering

# Sprint
GET    /api/v1/projects/{key}/sprints
POST   /api/v1/projects/{key}/sprints
GET    /api/v1/projects/{key}/sprints/{id}
PATCH  /api/v1/projects/{key}/sprints/{id}
POST   /api/v1/projects/{key}/sprints/{id}/start
POST   /api/v1/projects/{key}/sprints/{id}/complete
GET    /api/v1/projects/{key}/backlog

# Workflow
GET    /api/v1/projects/{key}/workflow
POST   /api/v1/projects/{key}/workflow/statuses
PATCH  /api/v1/projects/{key}/workflow/statuses/{id}
DELETE /api/v1/projects/{key}/workflow/statuses/{id}
POST   /api/v1/projects/{key}/workflow/transitions

# Report
GET    /api/v1/projects/{key}/reports/burndown?sprintId=
GET    /api/v1/projects/{key}/reports/velocity
GET    /api/v1/projects/{key}/reports/burnup
GET    /api/v1/projects/{key}/reports/cfd         # cumulative flow

# Dashboard
GET    /api/v1/dashboards
POST   /api/v1/dashboards
GET    /api/v1/dashboards/{id}
PATCH  /api/v1/dashboards/{id}

# Filtri
GET    /api/v1/filters
POST   /api/v1/filters
GET    /api/v1/filters/{id}

# Search
GET    /api/v1/search?q=&project=&type=&status=&assignee=&sprint=&label=

# Git
POST   /api/v1/projects/{key}/git/providers
POST   /api/v1/webhooks/git/{token}              # receiver webhook Git

# Notifiche
GET    /api/v1/notifications
PATCH  /api/v1/notifications/read-all
GET    /api/v1/notifications/settings
PATCH  /api/v1/notifications/settings

# WebSocket
WS     /ws/v1/projects/{key}/board              # real-time board updates
WS     /ws/v1/notifications                     # notifiche in tempo reale
```

---

## 9. Configurazione (Environment Variables)

```env
# Server
APP_PORT=8080
APP_ENV=production
APP_SECRET=<jwt-secret-min-32-chars>
APP_BASE_URL=https://your-instance.example.com

# Database
DB_DRIVER=postgres          # postgres | mysql | sqlite
DB_DSN=postgres://user:pass@host:5432/dbname?sslmode=require

# Redis
REDIS_URL=redis://redis:6379/0

# Email (SMTP)
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=noreply@example.com
SMTP_PASS=secret
SMTP_FROM=noreply@example.com

# OAuth Providers (configurare i necessari)
OAUTH_FORGEJO_CLIENT_ID=
OAUTH_FORGEJO_CLIENT_SECRET=
OAUTH_FORGEJO_BASE_URL=https://git.example.com

OAUTH_GITLAB_CLIENT_ID=
OAUTH_GITLAB_CLIENT_SECRET=
OAUTH_GITLAB_BASE_URL=https://gitlab.com     # o self-hosted

OAUTH_GITHUB_CLIENT_ID=
OAUTH_GITHUB_CLIENT_SECRET=

# Storage allegati
STORAGE_DRIVER=local          # local | s3 | minio
STORAGE_PATH=/data/uploads    # per local
S3_BUCKET=
S3_ENDPOINT=
S3_ACCESS_KEY=
S3_SECRET_KEY=

# Feature flags
FEATURE_AUTOMATION=true
FEATURE_TIMELINE=true
FEATURE_CALENDAR=true
```

---

## 10. Struttura Repository

```
/
├── cmd/
│   ├── server/           # main API server
│   └── worker/           # async worker (email, automation)
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
│   ├── store/            # DB layer (GORM / SQLAlchemy)
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
│   ├── helm/             # Helm chart Kubernetes
│   │   ├── Chart.yaml
│   │   ├── values.yaml
│   │   └── templates/
│   └── docker/
│       ├── Dockerfile.api
│       ├── Dockerfile.worker
│       └── docker-compose.yml  # per sviluppo locale
├── docs/
│   └── api/              # OpenAPI / Swagger spec
└── README.md
```

---

## 11. Priorità di Sviluppo (Roadmap)

### Sprint 1-2 — Fondamenta
- Setup progetto (repo, CI/CD, Docker, DB migrations)
- Autenticazione (JWT + OAuth2 con Forgejo/GitLab/GitHub)
- CRUD Progetti, Utenti, Ruoli
- CRUD Issues con campi base
- API REST documentata con Swagger

### Sprint 3-4 — Board & Backlog
- Workflow configurabile + transizioni
- Board Scrum drag & drop
- Backlog con sprint management (crea/avvia/completa)
- Board Kanban
- Frontend React: Board + Backlog + Issue detail

### Sprint 5-6 — Collaborazione & Ricerca
- Commenti, History, Watchers
- Linked issues, Subtask
- Full-text search + filtri avanzati
- Notifiche in-app + email base
- Filtri salvati

### Sprint 7-8 — Reports & Dashboard
- Summary progetto
- Sprint Burndown chart
- List view con filtri
- Dashboard con widget "Assigned to Me" e Activity Stream

### Sprint 9-10 — Git Integration & Automazione
- Webhook receiver (Forgejo, GitLab, GitHub)
- Visualizzazione commit/PR/branch nell'issue
- Transizione automatica su merge
- Regole automation base (trigger → azione)

### Sprint 11-12 — Feature v1.x
- Timeline / Gantt
- Calendar view
- Velocity Report, Burnup, CFD
- Campi custom
- Export CSV

---

## 12. Note per Claude Code

### Approccio architetturale raccomandato
1. **Inizia con il backend Go**: struttura `internal/domain` → `internal/store` → `internal/api`, in quest'ordine. Non mescolare logica di business con DB layer.
2. **Usa interfacce per il DB layer** in modo da supportare PostgreSQL/MariaDB/SQLite tramite lo stesso codice di business.
3. **Migrations prima del codice**: definisci lo schema completo (vedi Sezione 7) nelle migration SQL prima di scrivere i repository.
4. **API-first**: implementa gli endpoint REST + test di integrazione prima del frontend.
5. **Frontend modulare**: ogni pagina (Board, Backlog, Issue Detail) è un modulo autonomo con i suoi hook React Query.

### Considerazioni specifiche
- **Drag & drop board**: usa `@dnd-kit/core` con `SortableContext`. Persisti l'ordine nel campo `position` (float) per evitare re-rank frequenti.
- **WebSocket**: implementa un hub pub/sub in-memory (Go channels) con Redis come message bus per scalabilità orizzontale.
- **JQL-like search**: implementa un parser query semplice (lexer + parser) che genera query SQL parametrizzate. Non usare `eval` o concatenazione stringa.
- **Git webhooks**: verifica sempre la firma HMAC del payload prima di processarlo.
- **Multi-database**: usa GORM con tag `gorm:"-"` per colonne DB-specific, e usa `COALESCE`, `ILIKE` etc. con adapter per dialect.
- **Story points ranking**: usa `LexoRank` o float gap (es. 1000, 2000, 3000) per il drag & drop nell'backlog senza riscrivere tutti i rank.

### Test
- Unit test per domain logic (no dipendenze esterne)
- Integration test con `testcontainers-go` (PostgreSQL reale)
- E2E test con Playwright per i flussi critici (crea issue, drag board, complete sprint)

---

*Documento generato automaticamente da analisi diretta di Jira Cloud (harpaitalia.atlassian.net) — 13/05/2026*
