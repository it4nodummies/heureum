# Stato del progetto — punto di ripresa

> Aggiornato: 2026-07-14 (dopo Round 7). Questo file è il punto di ingresso per riprendere lo sviluppo in una nuova sessione di Claude Code.

## Obiettivo

Clone open source di Jira con **API drop-in compatibile con Jira Cloud REST API v3** (`/rest/api/3/*`) e UI fedele a Jira. Backend Go, frontend Next.js (`frontend-next/`). Metodo: round iterativi a slice verticali (API + UI + test), con gate a tre livelli (contract test vs OpenAPI ufficiale + TDD backend + E2E Playwright).

- Design/roadmap: `docs/superpowers/specs/2026-07-10-jira-parity-roadmap-design.md`
- Feature di dettaglio: `jira-opensource-spec.md`
- Contratto ufficiale versionato: `docs/contracts/jira-platform-v3.json` (+ `jira-agile-1.0.json`)
- Mappa di conformità: `docs/contracts/gap-report.md` (rigenerabile con `go run ./cmd/gapreport`)

## Branch e stato git

- Branch di lavoro: `feat/frontend-next` (pushato su `origin`, un Forgejo self-hosted `http://192.168.1.58:3000` a volte non raggiungibile — se il push fallisce, riprovare quando torna su).
- Master: `master`.

## Round completati

- **Round 0 — Fondamenta** ✅: piattaforma v3 (errori/paginazione/expand/fields), package ADF, auth Basic+API token, harness contract test (kin-openapi), gap report tool, seed demo, CI GitHub Actions, Playwright. Primo endpoint certificato: `GET /rest/api/3/myself`.
- **Round 1 — Progetti** ✅ (piano: `docs/superpowers/plans/2026-07-10-round-1-progetti.md`): `GET/POST /project`, `GET/PUT/DELETE /project/{idOrKey}`, `/project/search`, `/project/type`, `/projectCategory`, archive/restore. **Id numerici tipo Jira (`seq_id` da 10000)**, UUID resta PK interna, `GET` risolve id-o-key. UI: lista, creazione con template, impostazioni.
- **Round 2 — Issue core** ✅ (piano: `docs/superpowers/plans/2026-07-10-round-2-issue-core.md`): `GET/POST/PUT/DELETE /issue`, `/issue/createmeta`, `/issue/{id}/editmeta`, `/priority`, `/issuetype`, `/status`, `/resolution`, `/field`, `/label`. Issue con `seq_id` (10001+), `description` in ADF, assignee/reporter utenti v3, status con `statusCategory`. UI: vista issue (`/jira/browse/{key}`) con rendering ADF, edit inline summary, modale creazione.
- **Round 3 — Collaborazione** ✅ (piano: `docs/superpowers/plans/2026-07-11-round-3-collaborazione.md`): commenti ADF con @menzioni (`GET/POST /issue/{idOrKey}/comment`, `GET/PUT/DELETE .../comment/{id}`, `POST /comment/list`), worklog/time tracking (`GET/POST/DELETE /issue/{idOrKey}/worklog`), voti (`.../votes`), watchers (`.../watchers`, riscritti conformi), issue link (`POST /issueLink` + `GET/DELETE /issueLink/{linkId}`), changelog (`GET /issue/{idOrKey}/changelog` → PageBeanChangelog), remote link (`GET/POST/DELETE .../remotelink`, Delete scoped per issue). Timestamp Jira RFC3339 con offset `:` (`v3.JiraTime`). Nuovi mapping in `internal/api/v3/collab.go` (Votes/Watchers/IssueLinkV3/Changelog/RemoteLink) + `comment.go`/`worklog.go`. Migrazioni 000008-000010. UI: sezione Commenti + toggle watch/vote nella vista issue. E2E `collaboration.spec.ts` (8/8 suite verde).
- **Round 4 — Ricerca & JQL** ✅ (piano: `docs/superpowers/plans/2026-07-13-round-4-ricerca-jql.md`): motore JQL vero nel package **`internal/jql`** (lexer → parser a discesa ricorsiva → compiler AST→SQL con `Resolver`); supporta AND/OR/NOT, parentesi, `= != > >= < <= ~ !~ IN "NOT IN" IS [NOT] EMPTY`, `currentUser()`, `ORDER BY`. Endpoint: `GET/POST /search/jql` (**token-paginato** `nextPageToken`/`isLast` via cursore base64), legacy `GET/POST /search` (**offset** `startAt/maxResults/total`), `POST /search/approximate-count` (count-only), `GET /jql/autocompletedata`. Filtri salvati conformi (`Filter`/`PageBeanFilterDetails`): `POST /filter` (**200**, non 201), `GET/PUT/DELETE /filter/{id}`, `GET /filter/search` (PageBeanFilterDetails), `GET /filter/my`, `GET /filter/favourite`, `PUT/DELETE /filter/{id}/favourite`. Migrazione 000011 (`saved_filters.description`+`is_favourite`). Proiezione `fields` in `internal/api/v3/fields.go`. **Nota design**: stati e tipi sono tabelle per-progetto → `status`/`type` risolti via subquery per nome (come `labels`), NON via id nel `Resolver` (il `Resolver` risolve solo project/user/currentUser). UI: pagina `/jira/filters` (input JQL + list view a colonne configurabili + salva/riesegui filtri) e ricerca globale in top nav (`GlobalSearch`, testo→`text ~ "..."`). E2E `search.spec.ts` (suite 11/11 verde).

- **Round 5 — Board, Backlog, Sprint** ✅ (piano: `docs/superpowers/plans/2026-07-14-round-5-board-backlog-sprint.md`): API **Agile 1.0** (`/rest/agile/1.0/*`). Board CRUD + `configuration` (colonne dagli status del workflow), liste `backlog`/`issue`/`sprint`/`epic`; sprint `POST /sprint`, `GET/POST/PUT/DELETE /sprint/{id}` (state active→Start, closed→Complete), `GET/POST /sprint/{id}/issue`; `PUT /issue/rank`, `POST /backlog/issue`, `GET /issue/{idOrKey}` (con `fields.sprint`), `GET /epic/{idOrKey}` (read). Nuovo dominio `internal/domain/board` + estensione `internal/domain/sprint` (seq_id/originBoardId/completeDate/CreateFull/UpdateFull) + `issue.Service.Rank` (Position midpoint). Migrazione 000012 (tabella `boards` + colonne sprint). **Id interi** via seq_id (board/sprint). **Due paginazioni**: board/sprint → `values`+`isLast`; liste issue → `SearchResults` `issues`+`total`. Mapper in `internal/api/v3/agile.go`; renderer issue condiviso `handlers/render_issues.go` (fix: `nil` fields → `*all`, ripara anche una GET search R4 latente). Epic = issue di tipo Epic. UI: board dnd (`@dnd-kit`) `/jira/boards/{id}` (drag→transizione stato via `POST /issue/{key}/transitions`) e backlog `/jira/boards/{id}/backlog` (sprint create/start/complete). E2E `board.spec.ts` (suite 13/13 verde).

- **Round 6 — Workflow** ✅ (piano: `docs/superpowers/plans/2026-07-14-round-6-workflow.md`): transizioni issue conformi v3 — `GET /rest/api/3/issue/{id}/transitions` (`Transitions`/`IssueTransition` con `to.statusCategory`) e `POST` nella shape Jira `{transition:{id}}` + estensione `{status_id}` (board), **204**. `GET /statuscategory` + `/statuscategory/{idOrKey}` (`CategoryFor` esportato). **Regole base transizione**: validator `require_assignee` (400 se manca assignee) + post-function `set_resolution` (setta/azzera la resolution su categoria `done`; risolve "Done" da `ResolutionIDByName`). Editing workflow per-progetto (rotte custom `/project/{key}/workflow/*`): statuses CRUD + transizioni nome/regole/list/delete + reorder. Migrazione 000013 (`workflow_transitions.name`+`require_assignee`+`set_resolution`). Nuovi mapper `internal/api/v3/transitions.go`; `issue.Service.SetResolution`/`ResolutionIDByName`; `workflow.Service.GetTransitionByID`/`GetAvailableTransitions`/`UpdateTransition`/`ReorderStatuses`. Seed: resolution "Done". UI: tab **Workflow** nelle impostazioni progetto (`WorkflowEditor`: stati add/remove + categoria/colore; transizioni con badge regole). E2E `workflow.spec.ts` (suite 14/14 verde).

- **Round 7 — Viste & Report** ✅ (piano: `docs/superpowers/plans/2026-07-14-round-7-viste-report.md`): il backend report/dashboard esisteva già ma leggeva lo storico dal `field_name` sbagliato. **Fix correttezza**: burndown/velocity/burnup e CFD ora leggono `issue_history.field_name='status'` (non `'status_id'`), e la CFD fa join sullo **stato storico** (`ws.id = ih.new_value`) invece che sul corrente. Nuovi report: `GET /project/{key}/reports/pie?field=status|priority|assignee|type` e `.../reports/created-vs-resolved?days=N`. Test del report service (prima assenti): burndown/velocity/summary/CFD/pie/created-vs-resolved. Frontend greenfield: **grafici SVG dependency-free** (`components/charts/` Line/Bar/Pie/StackedArea — niente recharts, robusti su React 19), client `reports`+`dashboards`, pagina `/jira/projects/{key}/reports`, tab **Summary** nelle impostazioni, pagina **Dashboards** (`/jira/dashboards` — prima link morto) con gadget tipizzati (assigned_to_me/activity_stream). Seed: dashboard demo. E2E `reports.spec.ts` (suite 16/16 verde). **Nota**: i report sono estensioni custom (non nel contratto v3), la conformità v3 riguarda le dashboard/gadget.

Gap report attuale: **103 endpoint path-match conformi** su ~500 del contratto (la conformità reale è garantita dai contract test in `internal/contract/`).

## Prossimo: Round 8 — Utenti & permessi (DA PIANIFICARE)

Dalla roadmap: gruppi, ruoli progetto, permission scheme, profilo utente, notifiche in-app + email (worker Redis), preferenze notifica. Il piano NON è ancora scritto — creare con `superpowers:writing-plans`, poi eseguire con `superpowers:subagent-driven-development`.

Contesto utile: esiste già `internal/domain/user` (`User{ID,Username,Email,DisplayName}`), auth Basic+API token (Round 0), `project.ProjectMember`/`Invite` (Round 1), e un `notifSvc` usato da comment/issue services (notifiche "commented"/"assigned"/"status changed"). Endpoint v3 rilevanti: `/rest/api/3/group`, `/group/member`, `/role`, `/project/{key}/role`, `/permissionscheme`, `/mypermissions`, `/user` (già parziale), `/notification`. Valutare worker/coda per le email (Redis già citato in roadmap).

Nota: **allegati** ancora rinviati (richiedono storage file).

## Follow-up aperti (non bloccanti)

- Reporter impostato alla creazione issue (il domain `issue.Service.Create` non ha parametro reporter).
- Harness contract: valutare `Options.IncludeResponseStatus=true` per far fallire su status non documentati.
- Rimuovere dead code `WorkflowHandler.ListStatuses` (rotta `/status` ora su `refH.Statuses`).
- Esporre `favourite` v3 e ricablare lo star dei progetti (rimosso in R1).
- Filtri `/project/search` (typeKey/orderBy) e paginazione lista progetti (>50).
- Login/register handler → formato errore v3 `{errorMessages,errors}` (ora `{error}`).
- Workflow/stati di default sui progetti seedati (le issue DEMO non hanno status esplicito).
- Editor ADF ricco (TipTap) per description/commenti; createmeta/editmeta arricchiti.
- Screenshot di riferimento da Jira reale (richiede estensione Claude in Chrome connessa).
- Rinominare il progetto (via da "open-jira"/"Open Jira": trademark) prima della pubblicazione (Round 10).
- **(R4)** Filtri: nessun controllo di ownership su GET/PUT/DELETE `/filter/{id}` (qualsiasi utente autenticato può modificarli) — gap preesistente, aggiungere check owner/condivisione.
- **(R4)** `PUT /filter/{id}/owner` (change-owner) rimosso col vecchio blocco rotte: reintrodurre se serve (non nel set minimo del contratto usato).
- **(R4)** `POST /jql/parse` e funzioni JQL avanzate (`now()`, date relative, `sprint in openSprints()`); `sharePermissions`/`editPermissions` reali sui filtri (ora array vuoti).
- **(R4)** `renderIssues` fa N+1 lookup per issue (`buildIssueInput`): ottimizzare con fetch batch per pagine grandi.
- **(R4)** Rendering `fields.issuelinks` dentro la issue (link creati ma non esposti nel payload issue) — task follow-up già avviato.
- **(R5)** Ranking: ora `Position float64` midpoint — passare a LexoRank stringa per robustezza con molti reinserimenti.
- **(R5)** Epic in sola lettura: aggiungere `POST /epic/{id}` (update name/summary/done) e `PUT /epic/{id}/rank`; `epic.done` ora sempre false (calcolare da statusCategory).
- **(R5)** Board legata a un progetto: supportare board basate su filtro JQL puro; `POST /board/{id}/issue` (rank su board).
- **(R5)** Consolidare/retirare le vecchie rotte custom `/rest/api/3/project/{key}/board|sprints` + `BoardHandler` custom (ora parallele all'API agile).
- **(R5)** Drag&drop backlog↔sprint nella UI (ora solo bottoni); `project.Service.Create` non crea workflow di default (compensato nel seed) — spostare nel dominio.
- **(R6)** Workflow CRUD bulk v3 (`/workflows/create`, `/workflows`, shape `statusReference` + arrays conditions/validators/actions); `/workflowscheme` (mappa workflow→tipi issue).
- **(R6)** **Condizioni** di transizione che filtrano `isAvailable` in GET (ora sempre true); rule-engine generico oltre ai due flag base (`require_assignee`/`set_resolution`); transizioni `isGlobal`/`isInitial`; screen sulle transizioni (`hasScreen`/fields).
- **(R6)** `WorkflowHandler.ListTransitions` può emettere `null` invece di `[]` a zero transizioni (l'editor usa `GetWorkflow`, non impattato); normalizzare a slice vuoto.
- **(R7)** **Timeline/Gantt** e **Calendar** (issue per due date) — rinviati dal Round 7.
- **(R7)** Loggare i cambi `sprint_id` nello storico (burndown con issue aggiunte/rimosse a metà sprint); gadget dashboard configurabili (report-gadget) con `moduleKey`/`uri` conformi v3; export report (CSV/PDF); `GetCreatedVsResolved` usa il giorno-calendario UTC (label vs wall-clock locale può sfasare di poche ore); instradare i vecchi handler report/dashboard attraverso `v3.WriteJSON`.
- **(R7)** Rendering `fields.resolution` sulle issue aggiunto (commit `20a02a3`): `buildIssueInput` ora risolve `resolution_id` (gap preesistente reso visibile dalla post-function R6).
- **(R7)** ~~CFD piatta~~ **RISOLTO** (commit `38aa8a2`): `GetCFD` ora fa il replay degli eventi e conta le issue per categoria "as of" ogni giorno (vera cumulata); test `TestCFD_CumulativeShape`. Follow-up perf: il replay è O(giorni×issue) in memoria — ottimizzare per progetti grandi.

## Come far ripartire il lavoro

1. Apri Claude Code nella cartella del repo, sul branch `feat/frontend-next`.
2. Prompt suggerito: _"Leggi docs/superpowers/STATE.md. Pianifica ed esegui il Round 8 (Utenti & permessi) con lo stesso metodo dei round precedenti (writing-plans → subagent-driven-development, gate a tre livelli)."_
3. Comandi di verifica utili: `go build ./... && go vet ./... && go test ./...`; `go run ./cmd/gapreport`; per la UI `cd frontend-next && npm run build` e Playwright.
4. Seed/avvio demo: `APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=./dev.db go run ./cmd/seed` poi `... go run ./cmd/server` (utente demo `admin@example.com` / `admin-demo-123`).
