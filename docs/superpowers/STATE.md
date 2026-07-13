# Stato del progetto — punto di ripresa

> Aggiornato: 2026-07-11 (dopo Round 3). Questo file è il punto di ingresso per riprendere lo sviluppo in una nuova sessione di Claude Code.

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

Gap report attuale: **78 endpoint path-match conformi** su ~500 del contratto.

## Prossimo: Round 4 — Ricerca & JQL (DA PIANIFICARE)

Dalla roadmap: motore di ricerca issue con JQL (parser + valutatore), `GET/POST /search`, `/search/jql`, filtri salvati (`/filter`, `/filter/{id}`, `/filter/favourite`), autocomplete JQL (`/jql/autocompletedata`), sfondo per board/backlog. Il piano NON è ancora scritto — va creato con `superpowers:writing-plans` come per i round precedenti, poi eseguito con `superpowers:subagent-driven-development`.

Endpoint v3 rilevanti da allineare (verificare in `docs/contracts/jira-platform-v3.json`): `POST /search` (deprecato ma diffuso), `GET/POST /search/jql`, `POST /search/approximate-count`, `/filter`, `/filter/{id}`, `/filter/favourite`, `/filter/my`, `/jql/autocompletedata`, `/jql/parse`.

Riusare: `internal/api/v3` (WriteJSON/WriteError/WritePage, JiraIssue, ParsePagination), pattern `seq_id`, harness `internal/contract`. La ricerca deve restituire pagine di issue nello stesso shape della `GET /issue` (fields/expand). Attenzione: JQL è la feature più grande della roadmap — valutare un parser dedicato in un package `internal/jql`.

Nota: **allegati** rinviati dal Round 3 (richiedono storage file) — riprenderli in un round dedicato o insieme alla ricerca se serve.

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

## Come far ripartire il lavoro

1. Apri Claude Code nella cartella del repo, sul branch `feat/frontend-next`.
2. Prompt suggerito: _"Leggi docs/superpowers/STATE.md. Pianifica ed esegui il Round 3 (Collaborazione) con lo stesso metodo dei round precedenti (writing-plans → subagent-driven-development, gate a tre livelli)."_
3. Comandi di verifica utili: `go build ./... && go vet ./... && go test ./...`; `go run ./cmd/gapreport`; per la UI `cd frontend-next && npm run build` e Playwright.
4. Seed/avvio demo: `APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=./dev.db go run ./cmd/seed` poi `... go run ./cmd/server` (utente demo `admin@example.com` / `admin-demo-123`).
