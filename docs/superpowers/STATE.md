# Stato del progetto — punto di ripresa

> Aggiornato: 2026-07-14 (dopo Round 5). Questo file è il punto di ingresso per riprendere lo sviluppo in una nuova sessione di Claude Code.

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

Gap report attuale: **100 endpoint path-match conformi** su ~500 del contratto (la conformità reale è garantita dai contract test in `internal/contract/`).

## Prossimo: Round 6 — Workflow (DA PIANIFICARE)

Dalla roadmap: stati custom con categorie, transizioni con condizioni/validator/post-function base, workflow per progetto. UI: editor workflow nelle impostazioni progetto, colonne board mappate sugli stati. Il piano NON è ancora scritto — creare con `superpowers:writing-plans`, poi eseguire con `superpowers:subagent-driven-development`.

Contesto utile: esiste già `internal/domain/workflow` (`Workflow`, `WorkflowStatus{ID,Name,Position,Category}`, `WorkflowTransition`, `Service` con `GetWorkflow`/`CreateDefaultWorkflow`/`AddStatus`/`UpdateStatus`/`ValidateTransition`); il `WorkflowHandler` espone già `POST /issue/{key}/transitions` (validato contro le transizioni) — usato dalla board dnd del Round 5. Da allineare al contratto v3: `/rest/api/3/workflow`, `/workflowscheme`, `/status` (già presente), transizioni con regole. **Nota**: `project.Service.Create` NON crea un workflow di default (solo l'handler HTTP lo fa) — il seed R5 lo compensa; valutare di spostare la creazione del workflow nel dominio.

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

## Come far ripartire il lavoro

1. Apri Claude Code nella cartella del repo, sul branch `feat/frontend-next`.
2. Prompt suggerito: _"Leggi docs/superpowers/STATE.md. Pianifica ed esegui il Round 6 (Workflow) con lo stesso metodo dei round precedenti (writing-plans → subagent-driven-development, gate a tre livelli)."_
3. Comandi di verifica utili: `go build ./... && go vet ./... && go test ./...`; `go run ./cmd/gapreport`; per la UI `cd frontend-next && npm run build` e Playwright.
4. Seed/avvio demo: `APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=./dev.db go run ./cmd/seed` poi `... go run ./cmd/server` (utente demo `admin@example.com` / `admin-demo-123`).
