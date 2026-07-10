# Design: Parità funzionale con Jira Cloud — roadmap iterativa

> **Data**: 2026-07-10
> **Stato**: Approvato dall'utente
> **Ambito**: governa i round di sviluppo del progetto; le feature di dettaglio restano in `jira-opensource-spec.md`.

## 1. Obiettivo e criteri di successo

Un project management tool open source (licenza **AGPL-3.0**) con:

- **API drop-in compatibile** con Jira Cloud REST API v3 (`/rest/api/3/*`) e Agile API (`/rest/agile/1.0/*`): stessi path, payload, formato errori, paginazione, `expand`/`fields`, JQL.
  - Criterio misurabile: i **test di contratto generati dall'OpenAPI ufficiale Atlassian** passano; uno script/SDK Jira reale funziona puntato al nostro server.
- **UI fedele a Jira Cloud** (layout, flussi, densità visiva) con identità visiva propria: palette, icone e nome diversi — nessun asset o logo Atlassian (trademark).
  - Riferimento visivo: istanza reale `harpaitalia.atlassian.net` via sessione Chrome dell'utente.
- Copertura funzionale delle feature 🟢 (fondamentali) e 🟡 (importanti) di `jira-opensource-spec.md`; le 🔴 (enterprise) come round opzionali finali.
- Prodotto **pubblicabile su GitHub**: nome proprio (l'attuale "open-jira" contiene il trademark "Jira" e va sostituito prima della pubblicazione), README, CONTRIBUTING, Code of Conduct, CI, Helm chart, demo docker-compose.

## 2. Decisioni prese

| Decisione | Scelta |
|---|---|
| Scope parità | 🟢 + 🟡 nei round principali; 🔴 in round opzionali finali |
| Compatibilità API | Drop-in con Jira Cloud REST API v3 + Agile API 1.0 |
| Codebase | Evolvere l'esistente (backend Go + frontend-next) |
| Frontend | Solo `frontend-next`; il vecchio `frontend` (SPA React) viene rimosso quando superato |
| Fedeltà UI | Fedele a Jira nei layout/flussi, identità visiva propria |
| Licenza | AGPL-3.0 |
| Strategia round | Slice verticali per area funzionale (API + UI + test per round) |

## 3. Architettura (evoluzione dell'esistente)

- **Backend Go** (`cmd/server`, `cmd/worker`, `internal/`): resta. Si aggiunge un layer "piattaforma v3" trasversale:
  - formato errori Jira (`errorMessages` + `errors`),
  - paginazione `startAt`/`maxResults`/`total`/`isLast`,
  - supporto `expand` e selezione `fields`,
  - **ADF** (Atlassian Document Format) come modello canonico per rich-text (descrizioni, commenti),
  - **JQL engine** come modulo dedicato (parser → AST → query builder SQL).
- **Frontend**: `frontend-next` (Next.js 14, Tailwind, shadcn/Radix, TanStack Query, dnd-kit, TipTap con serializzazione da/verso ADF, Recharts).
- **Dati**: PostgreSQL primario (JSONB per custom field e ADF), MariaDB/SQLite secondari, Redis per code e notifiche, migrazioni golang-migrate.
- **Ranking board/backlog**: LexoRank-like (come Jira), non interi sequenziali.
- **Real-time**: WebSocket (`internal/api/ws`) per aggiornamenti board/issue.

## 4. Metodo di verifica della parità (gate a tre livelli)

Ogni round è chiuso solo se passano tutti e tre i livelli:

1. **Contract test**: suite generata dall'OpenAPI ufficiale Jira v3 + snapshot di risposte reali dell'istanza `harpaitalia.atlassian.net` (catturate una tantum) usati come golden file.
2. **Test funzionali**: TDD sul backend (unit + integration con DB reale) e **E2E Playwright** sul frontend.
3. **Confronto UI**: screenshot side-by-side delle stesse schermate tra la nostra app e Jira reale via sessione Chrome.

## 5. Round di sviluppo (slice verticali)

Ogni round produce: endpoint v3 conformi + UI + test dei tre livelli + documentazione dell'area. Il prodotto è funzionante e dimostrabile a fine round.

| Round | Area | Contenuto principale |
|---|---|---|
| **0** | Audit & fondamenta | Gap analysis endpoint esistenti vs OpenAPI ufficiale; harness contract test; CI (lint + test); Playwright; seed dati demo; piattaforma v3 (errori, paginazione, expand, ADF); auth (login, API token, sessione); shell UI (top nav + sidebar fedeli a Jira) |
| **1** | Progetti | CRUD progetti v3, tipi (Scrum/Kanban/Business), lead, default assignee, categorie, avatar, archiviazione; UI: lista progetti, creazione con template, settings progetto |
| **2** | Issue core | Issue CRUD, tipi gerarchici (Epic → Story/Task/Bug → Subtask), priorità, labels, resolutions, createmeta/editmeta, custom field (testo, numero, data, select, multi-select); UI: vista issue completa, modal creazione, edit inline |
| **3** | Collaborazione | Commenti ADF con @menzioni, allegati, watchers, voti, issue links, remote links, changelog/history, time tracking; UI integrata nella vista issue |
| **4** | Ricerca & JQL | Parser JQL completo (campi, operatori, funzioni tipo `currentUser()`, `ORDER BY`), `/rest/api/3/search/jql`, filtri salvati e condivisi; UI: ricerca globale, list view con colonne configurabili, filtri |
| **5** | Board, Backlog, Sprint | Agile API 1.0 (board, sprint, epic, ranking LexoRank); UI: board drag&drop con filtri e group-by, backlog con sprint collassabili e creazione inline, start/complete sprint |
| **6** | Workflow | Stati custom con categorie, transizioni con condizioni/validator/post-function base, workflow per progetto; UI: editor workflow in settings, colonne board mappate su stati |
| **7** | Viste & Report | Timeline/Gantt, Calendar, Summary progetto, Dashboard con gadget, report (Burndown, Velocity, Cumulative Flow, pie, created vs resolved) |
| **8** | Utenti & permessi | Gruppi, ruoli progetto, permission scheme, profilo utente, notifiche in-app + email (worker Redis), preferenze notifica |
| **9** | Integrazioni | Webhook in uscita, integrazione Git (interfaccia `GitProvider`: Forgejo/Gitea, GitLab, GitHub — branch/commit/PR collegati alle issue), automation base (regole trigger → condizione → azione) |
| **10** | Release 1.0 | Rimozione vecchio `frontend`, security review, performance/load test, rename progetto (nome senza trademark), LICENSE AGPL-3.0, README/CONTRIBUTING/CoC, Helm chart, docker-compose demo, CI/CD release, pubblicazione GitHub |
| **11+** | Enterprise 🔴 (opzionali) | Automation avanzata, Forms/Intake, Goals/OKR, Teams, Standups, approvazioni su transizione |

## 6. Orchestrazione degli agenti (uguale per ogni round)

1. **Piano di round**: dal design + spec si scrive il piano dettagliato del round (skill `writing-plans`) con task piccoli, indipendenti e verificabili.
2. **Esecuzione**: task indipendenti dispatchati ad agenti in parallelo (subagent-driven development, TDD). Backend e frontend della stessa area lavorano su contratti API concordati a inizio round.
3. **Gate di chiusura**: code review (agente revisore) → contract test + E2E verdi → confronto UI con Jira reale → commit/merge. Un round non si apre se il precedente non è verde.

## 7. Rischi e mitigazioni

- **JQL e ADF sono i moduli più difficili** e attraversano tutto il sistema: ADF si implementa nel Round 0 (piattaforma), la grammatica JQL si definisce già al Round 2 anche se l'engine completo arriva al Round 4.
- **Deriva dal contratto v3**: mitigata dai contract test in CI su ogni PR, non solo a fine round.
- **Trademark Atlassian**: nessun asset/logo/nome Atlassian nel codice o nella UI; rename prima della pubblicazione (Round 10).
- **Scope creep**: le feature 🔴 sono esplicitamente fuori dai round 0–10.

## 8. Fonti

- Feature di dettaglio: `jira-opensource-spec.md` (sezioni 4.x, MVP in sezione 5).
- Contratto API: OpenAPI ufficiale Jira Cloud v3 + Agile 1.0 (da scaricare nel Round 0 e versionare in `docs/contracts/`).
- Riferimento UI: istanza `harpaitalia.atlassian.net` via Chrome.
