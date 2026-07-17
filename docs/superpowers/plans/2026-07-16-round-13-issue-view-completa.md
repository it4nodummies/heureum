# Round 13 — Issue view completa Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Portare la vista dettaglio issue di Heureum a parità funzionale con Jira Cloud: sottotask, collegamenti tra issue, allegati, time tracking/worklog, storico modifiche, editing assignee e un modale di creazione ricco.

**Architecture:** La maggior parte dei domini backend esiste già (issue links, attachments, worklog, history, GetChildren). L'audit ha però rilevato che diverse **rotte di lista per-issue non sono cablate** e alcune **shape v3 mancano** (`fields.issuelinks`/`subtasks` non esposti, allegati in snake_case grezzo, worklog che non aggiorna `TimeSpent`, nessuna scrittura estimate). Il round quindi NON è solo UI: ogni sotto-feature ha un piccolo task backend (rotta di lista + shape v3 conforme + eventuale fix) seguito dal cablaggio client + sezione UI in `IssueView.tsx` + E2E. Le sezioni sono indipendenti tra loro (condividono solo il file `IssueView.tsx`).

**Tech Stack:** Go 1.25 (net/http ServeMux, GORM), `internal/api/v3` mapping layer, `internal/contract` harness; Next.js 16 App Router + React 19 + TanStack Query + Tailwind (`#0052cc` accent), Playwright. ADF helpers in `frontend-next/components/issues/adf.tsx`.

## Global Constraints

- **Enforcement (Round 11):** ogni NUOVA rotta di lettura per-issue va gatata con `chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), handler)`; ogni rotta mutante con `chk.Enforce`/in-handler `chk.RequireProject`. Non aggiungere rotte non gatate.
- **Gate a tre livelli** dopo ogni task: `go build ./... && go vet ./... && go test ./...`; per il frontend `cd frontend-next && npx tsc --noEmit && npm run build && npx playwright test`; `go run ./cmd/gapreport` senza drift inatteso (le nuove rotte custom sono estensioni → il conteggio "extra" può salire, committare il gap-report rigenerato).
- **v3 shape:** i timestamp usano `v3.JiraTime`; gli utenti `v3.JiraUser`/`v3.User`; le liste per-issue custom usano `v3.WriteJSON` con una shape esplicita (NON serializzare struct di dominio grezze).
- **Frontend patterns:** riusare `AdfRenderer`/`adfToText`/`textToAdf` da `components/issues/adf.tsx`; ogni fetch via TanStack Query con invalidazione della query `["issue", issueKey]` (o della sotto-query specifica) dopo una mutazione; stile coerente col resto di `IssueView.tsx`.
- **Enforcement modello dati:** l'issue key è l'identificatore pubblico; le rotte per-issue accettano `{issueIdOrKey}` (numerico seq-id o key), risolto con lo stesso pattern di `resolveIssue`.
- **E2E:** ogni sezione UI ha almeno un test Playwright end-to-end (login helper esistente `admin@example.com`/`admin-demo-123`, apre `/app/browse/DEMO-1` o crea una issue). Pulire `test-results/`/`playwright-report/` prima del commit.
- **Non-goal del round:** rich text editor WYSIWYG e @mentions (→ round successivo), email notifiche, versioni/componenti. Le descrizioni restano textarea↔ADF come oggi.

---

## Struttura dei file

**Backend (nuove rotte/handler, riuso domini esistenti):**
- `internal/api/handlers/issue_handler.go` — nuovo handler `Subtasks` (lista figli); estende `Update` per `fields.timetracking`.
- `internal/api/handlers/issuelink_handler.go` — nuovo handler `ListForIssue`.
- `internal/api/handlers/attachment_handler.go` — nuovo handler `ListForIssue`.
- `internal/api/v3/issue.go` — shape `SubtaskRef`; `TimeTracking` write-back (remaining).
- `internal/api/v3/collab.go` — shape lista issue-link per-issue (già ha `IssueLinkV3`/`JiraLinkType`).
- `internal/api/v3/attachment.go` (nuovo) — shape v3 allegato (`id, filename, size, mimeType, author, created, content`).
- `internal/domain/issue/worklog_service.go` — `Add` incrementa `Issue.TimeSpent`.
- `internal/domain/issue/service.go` — `Update` accetta stima originale/rimanente.
- `internal/api/router.go` — registrazione nuove rotte (gatate).
- `internal/contract/*_test.go` — test per subtasks/links/attachments/worklog/changelog end-to-end.

**Frontend:**
- `frontend-next/lib/api.ts` — client `issues.subtasks`, `issues.create` (parent/priority/assignee/description), `issues.update` (timetracking), nuovi oggetti `issueLinks`, `attachments`, `worklogs`, `issues.changelog`, `users.assignableSearch`; tipi relativi.
- `frontend-next/components/issues/IssueView.tsx` — sezioni Subtasks, Linked work items, Attachments, Time tracking; assignee editabile; tab History nell'Activity.
- `frontend-next/components/issues/CreateIssueModal.tsx` — campi ricchi.
- `frontend-next/components/common/UserPicker.tsx` (nuovo) — autocomplete utenti assegnabili.
- `frontend-next/components/issues/subtasks/…`, `links/…`, `attachments/…`, `worklog/…` — sotto-componenti (opzionali, se `IssueView.tsx` cresce troppo, estrarre).
- `frontend-next/e2e/issue-detail.spec.ts` (nuovo) — E2E delle nuove sezioni.
- `cmd/seed/main.go` — dati demo (subtask, link, allegato, worklog) su DEMO.

---

## FASE 1 — Sottotask

### Task 1: Backend — rotta lista sottotask
**Files:**
- Modify: `internal/api/handlers/issue_handler.go`, `internal/api/router.go`
- Test: `internal/contract/subtasks_test.go`

**Interfaces:**
- Consumes: `issue.Service.GetChildren(parentID string) ([]issue.Issue, error)` (esistente, `service.go:268`); il renderer issue v3 esistente usato da `IssueHandler.List` (leggere come costruisce `[]v3.JiraIssue`, es. `renderIssues`/`buildIssueInput`).
- Produces: `GET /rest/api/3/issue/{issueIdOrKey}/subtasks` → `{ "values": [ <v3 issue> ], "total": N }` (riusa la stessa shape issue di `GET /issue/{key}`).

- [ ] **Step 1: Test (contract) che fallisce**

In `internal/contract/subtasks_test.go` (usare gli helper reali: `newTestServer`, `registerAndLogin`, `createProjectViaAPI`, `createIssueViaAPI`, `doJSON`, `decodeBody`):
```go
func TestSubtasks_ListChildren(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, tok, "SUB", "Subtask Proj")
	parent := createIssueViaAPI(t, srv, tok, "SUB", "Parent story")
	// crea un figlio con fields.parent.key = parent
	resp := doJSON(t, srv, http.MethodPost, tok, "/rest/api/3/issue", map[string]any{
		"fields": map[string]any{
			"project":   map[string]any{"key": "SUB"},
			"summary":   "Child subtask",
			"issuetype": map[string]any{"name": "Subtask"},
			"parent":    map[string]any{"key": parent},
		},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create subtask: %d", resp.StatusCode)
	}
	resp = doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/issue/"+parent+"/subtasks", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list subtasks: %d", resp.StatusCode)
	}
	body, _ := decodeBody(t, resp)
	vals, _ := body["values"].([]any)
	if len(vals) != 1 {
		t.Fatalf("atteso 1 subtask, ottenuti %d", len(vals))
	}
}
```

- [ ] **Step 2: Eseguire (fallisce)**

Run: `go test ./internal/contract/ -run TestSubtasks -v` → FAIL (404, rotta assente).

- [ ] **Step 3: Handler**

In `issue_handler.go`, nuovo metodo (leggere `resolveIssue` e il renderer usato da `List` per replicare la costruzione dei v3 issue):
```go
// Subtasks: GET /rest/api/3/issue/{issueIdOrKey}/subtasks
func (h *IssueHandler) Subtasks(w http.ResponseWriter, r *http.Request) {
	iss, err := h.resolveIssue(r.PathValue("issueIdOrKey"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	children, err := h.svc.GetChildren(iss.ID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list subtasks"}, nil)
		return
	}
	out := make([]v3.JiraIssue, 0, len(children))
	for i := range children {
		out = append(out, h.renderIssue(&children[i])) // riusa il renderer esistente; leggere il nome reale del metodo/funzione che List usa
	}
	v3.WriteJSON(w, http.StatusOK, map[string]any{"values": out, "total": len(out)})
}
```
> **Nota implementatore:** VERIFICARE il nome reale della funzione/metodo che `IssueHandler.List` usa per trasformare un `issue.Issue` in `v3.JiraIssue` (es. `buildIssueInput` + `v3.JiraIssue(...)`), e riusarla esattamente — non reinventare il mapping (deve rendere `fields.status/type/assignee/summary/…`).

- [ ] **Step 4: Rotta gatata**

In `router.go`, accanto alle altre rotte issue GET:
```go
mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/subtasks", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(issueH.Subtasks))))
```

- [ ] **Step 5: Eseguire (passa) + gate**

Run: `go test ./internal/contract/ -run TestSubtasks -v && go build ./... && go vet ./...` → PASS/OK.

- [ ] **Step 6: Commit**
```bash
git add internal/api/ internal/contract/subtasks_test.go
git commit -m "feat(issue): list subtasks endpoint (GET /issue/{key}/subtasks)"
```

### Task 2: Frontend — sezione Sottotask + create figlio
**Files:**
- Modify: `frontend-next/lib/api.ts`, `frontend-next/components/issues/IssueView.tsx`
- Test: `frontend-next/e2e/issue-detail.spec.ts` (nuovo)

**Interfaces:**
- Consumes: `GET /issue/{key}/subtasks` (Task 1); `POST /issue` con `fields.parent.key`.
- Produces: `issues.subtasks(key) → { values: Issue[]; total }`; `issues.create({..., parentKey?, issueTypeName})`.

- [ ] **Step 1: Client**

In `api.ts`, estendere `issues`:
```ts
subtasks: (idOrKey: string) => apiFetch<{ values: Issue[]; total: number }>(`/rest/api/3/issue/${idOrKey}/subtasks`),
```
e in `issues.create`, aggiungere `parentKey?: string` al payload → `fields.parent = { key: parentKey }` quando presente.

- [ ] **Step 2: Sezione UI**

In `IssueView.tsx`, tra `DevelopmentPanel` (`:252`) e `Comments` (`:254`), inserire una sezione **Subtasks**:
- `useQuery(["issue", issueKey, "subtasks"], () => issues.subtasks(issueKey))`.
- Header "Subtasks" + contatore progresso `X di Y completati` (Y = totale, X = quelli con `fields.status.statusCategory` in categoria done — usare `f.status?.statusCategory?.key === "done"`; verificare la chiave reale nel tipo `StatusRef`).
- Lista: per ogni subtask → icona tipo, key (Link a `/app/browse/{key}`), titolo, dropdown stato inline (riusa la logica transizione già presente in `SearchResults`/`IssueView`), avatar assignee.
- Input inline "Add subtask": textbox → invio crea via `useMutation(() => issues.create({ projectKey, summary, issueTypeName: "Subtask", parentKey: issueKey }))` → `invalidateQueries(["issue", issueKey, "subtasks"])`. `projectKey` deriva da `issue.fields.project?.key` (fallback: prefisso della key).
- Empty state: bottone/campo "Add subtask".

- [ ] **Step 3: E2E**

In `e2e/issue-detail.spec.ts`: login → crea una issue (via topbar Create) → apri il suo dettaglio → aggiungi un subtask via il campo inline → asserisci che compare nella lista Subtasks e che il contatore è "0 di 1".

- [ ] **Step 4: Gate frontend + commit**
```bash
cd frontend-next && npx tsc --noEmit && npm run build && npx playwright test e2e/issue-detail.spec.ts --reporter=line; cd ..
git add frontend-next/lib/api.ts frontend-next/components/issues/IssueView.tsx frontend-next/e2e/issue-detail.spec.ts
git commit -m "feat(frontend): subtasks section on issue view (list + inline create + progress)"
```

---

## FASE 2 — Collegamenti tra issue (Linked work items)

### Task 3: Backend — lista link per-issue con shape v3
**Files:**
- Modify: `internal/api/handlers/issuelink_handler.go`, `internal/api/router.go`, `internal/api/v3/collab.go`
- Test: `internal/contract/issuelinks_test.go`

**Interfaces:**
- Consumes: `issue.Service.ListLinks(issueID string) ([]issue.IssueLink, error)` (esistente, `service.go:238`), `GetByKey`, `v3.JiraLinkType(internal, baseURL)` (`collab.go:81`).
- Produces: `GET /rest/api/3/issue/{issueIdOrKey}/issuelinks` → `{ "issuelinks": [ { id, type:{name,inward,outward}, inwardIssue?:{key,summary,status}, outwardIssue?:{key,summary,status} } ] }`.

- [ ] **Step 1: Test (contract) che fallisce**

`internal/contract/issuelinks_test.go`: alice crea progetto "LNK", due issue A e B; `POST /issueLink {type:{name:"Blocks"}, outwardIssue:{key:A}, inwardIssue:{key:B}}` → 201; `GET /issue/A/issuelinks` → 200 con un link il cui `type.name` è "Blocks" e `inwardIssue.key == B`.

- [ ] **Step 2: Eseguire (fallisce)** → 404.

- [ ] **Step 3: Handler + shape**

In `collab.go` aggiungere (se non esiste già una shape per-issue) un tipo:
```go
type IssueLinkForIssue struct {
	ID           string        `json:"id"`
	Type         LinkTypeRef   `json:"type"`
	InwardIssue  *LinkedIssue  `json:"inwardIssue,omitempty"`
	OutwardIssue *LinkedIssue  `json:"outwardIssue,omitempty"`
}
type LinkedIssue struct {
	Key     string      `json:"key"`
	Fields  LinkedFields `json:"fields"`
}
type LinkedFields struct {
	Summary string      `json:"summary"`
	Status  *StatusRef  `json:"status,omitempty"` // riusare lo StatusRef v3 esistente
}
```
> **Nota:** VERIFICARE i tipi realmente presenti in `collab.go` (`LinkTypeRef`, `IssueLinkV3`) e riusarli; non duplicare.

In `issuelink_handler.go`, metodo `ListForIssue`: risolve l'issue, `ListLinks(iss.ID)`, per ogni link determina l'altro capo (source/target), lo carica (`GetByID`/`DB().First`), popola inward/outward secondo la direzione del `LinkType`, e mappa il tipo con `v3.JiraLinkType`.

- [ ] **Step 4: Rotta gatata**
```go
mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/issuelinks", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(issueLinkH.ListForIssue))))
```

- [ ] **Step 5: Eseguire (passa) + gate + commit**
```bash
git add internal/api/ internal/contract/issuelinks_test.go
git commit -m "feat(issue): per-issue linked items endpoint with v3 shape"
```

### Task 4: Frontend — sezione Linked work items
**Files:**
- Modify: `frontend-next/lib/api.ts`, `frontend-next/components/issues/IssueView.tsx`
- Test: `frontend-next/e2e/issue-detail.spec.ts`

**Interfaces:**
- Consumes: `GET /issue/{key}/issuelinks`; `POST /issueLink`; `DELETE /issueLink/{id}`.
- Produces: client `issueLinks.list(key)`, `issueLinks.create({typeName, inwardKey, outwardKey})`, `issueLinks.delete(id)`.

- [ ] **Step 1: Client** — nuovo oggetto `issueLinks` in `api.ts` con i 3 metodi + tipi.

- [ ] **Step 2: Sezione UI** in `IssueView.tsx` ("Linked work items"):
  - Lista raggruppata per `type` (etichetta con la direzione corretta: usa `inward`/`outward` label del tipo); ogni riga: key (Link), stato (badge), summary, bottone rimuovi (`issueLinks.delete`).
  - "Add linked work item": dropdown tipo relazione (i 3 tipi supportati dal backend: **Blocks / Relates / Duplicate** — con le rispettive label inward/outward), campo target con autocomplete che cerca issue (riusa `/search/jql` con `text ~ "..."` o key match — leggere il client search esistente; se non c'è un metodo comodo, usare `search.jql`), submit → `issueLinks.create`.
  - Invalidazione `["issue", issueKey, "links"]` dopo create/delete.

- [ ] **Step 3: E2E** — crea due issue, dalla prima aggiungi un link "blocks" alla seconda, asserisci che compare nella sezione, poi rimuovilo.

- [ ] **Step 4: Gate + commit**
```bash
git commit -am "feat(frontend): linked work items section (add/list/remove)"
```

---

## FASE 3 — Allegati

### Task 5: Backend — lista allegati + shape v3
**Files:**
- Create: `internal/api/v3/attachment.go`
- Modify: `internal/api/handlers/attachment_handler.go`, `internal/api/router.go`
- Test: `internal/contract/attachments_test.go`

**Interfaces:**
- Consumes: `issue.AttachmentService.GetAttachments(issueID) ([]issue.IssueAttachment, error)` (`attachment_service.go:74`), `GetAttachment`.
- Produces: `GET /rest/api/3/issue/{issueIdOrKey}/attachments` → `[ { id, filename, size, mimeType, created, author?, content } ]` dove `content = /rest/api/3/attachment/content/{id}`. Anche `Upload`/`Get` devono restituire questa stessa shape v3 (non più lo struct grezzo).

- [ ] **Step 1: Shape v3** — `internal/api/v3/attachment.go`:
```go
package v3
type Attachment struct {
	ID       string   `json:"id"`
	Filename string   `json:"filename"`
	Size     int64    `json:"size"`
	MimeType string   `json:"mimeType"`
	Created  JiraTime `json:"created"`
	Content  string   `json:"content"` // URL relativo download
}
func AttachmentFrom(id, filename string, size int64, mimeType string, created time.Time) Attachment {
	return Attachment{ID: id, Filename: filename, Size: size, MimeType: mimeType, Created: JiraTime(created), Content: "/rest/api/3/attachment/content/" + id}
}
```
> **Nota:** il model non ha `mimeType`; inferirlo da estensione con `mime.TypeByExtension(filepath.Ext(filename))` (fallback `application/octet-stream`). `author` è opzionale (aggiungere se il caricamento è agevole risolvere l'uploader; altrimenti ometterlo — documentato).

- [ ] **Step 2: Test (contract)** che fallisce: upload multipart di un file su una issue → 201 con `filename`+`size`+`content`; `GET /issue/{key}/attachments` → lista con 1 elemento; il campo `content` punta a `/rest/api/3/attachment/content/{id}`.

- [ ] **Step 3: Handler** — nuovo `ListForIssue` (risolvi issue → `GetAttachments` → `[]v3.Attachment`); aggiornare `Upload`/`Get` per restituire `v3.AttachmentFrom(...)` invece del domain struct.

- [ ] **Step 4: Rotta gatata**
```go
mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/attachments", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(attachmentH.ListForIssue))))
```

- [ ] **Step 5: Gate + commit**
```bash
git commit -am "feat(issue): list attachments endpoint + v3 attachment shape"
```

### Task 6: Frontend — sezione Allegati
**Files:** `frontend-next/lib/api.ts`, `frontend-next/components/issues/IssueView.tsx`, `frontend-next/e2e/issue-detail.spec.ts`

**Interfaces:**
- Produces: `attachments.list(key)`, `attachments.upload(key, file)` (multipart — usare `FormData`, NON `JSON.stringify`; il campo si chiama `file`), `attachments.delete(id)`.

- [ ] **Step 1: Client** — attenzione: `apiFetch` probabilmente forza `Content-Type: application/json`; l'upload multipart deve NON impostare quel header (lascia che il browser metta il boundary). Se `apiFetch` non lo consente, fare una `fetch` dedicata con il token (leggere come `apiFetch` prende il token).
- [ ] **Step 2: Sezione UI** "Attachments": area drag&drop + input file → upload con indicatore; griglia allegati (thumbnail per `image/*` via il `content` URL, icona file generica altrimenti) con nome, dimensione leggibile, data, download (link a `content`), elimina.
- [ ] **Step 3: E2E** — upload di un file di test (`page.setInputFiles`), asserisci che compare in lista, poi elimina.
- [ ] **Step 4: Gate + commit**
```bash
git commit -am "feat(frontend): attachments section (upload/list/download/delete)"
```

---

## FASE 4 — Time tracking / Worklog

### Task 7: Backend — worklog aggiorna TimeSpent + scrittura estimate
**Files:**
- Modify: `internal/domain/issue/worklog_service.go`, `internal/domain/issue/service.go`, `internal/api/handlers/issue_handler.go`
- Test: `internal/domain/issue/worklog_service_test.go`, `internal/contract/worklog_test.go`

**Interfaces:**
- Consumes: `WorklogService.Add(issueID, authorID, commentJSON string, seconds int)` (`worklog_service.go:31`).
- Produces: dopo `Add`, `Issue.TimeSpent += seconds`; `PUT /issue/{key}` accetta `fields.timetracking.originalEstimateSeconds` e `remainingEstimateSeconds`, mappati su `Issue.OriginalEstimate`/`RemainingEstimate`. `fields.timetracking.remainingEstimateSeconds` viene emesso in lettura.

- [ ] **Step 1: Test (unit) che fallisce** — `worklog_service_test.go`: crea issue con `TimeSpent=0`, `Add(...,3600)`, ricarica issue → `TimeSpent==3600`.
- [ ] **Step 2: Implementare** — in `WorklogService.Add`, dopo aver inserito il worklog, `db.Model(&Issue{}).Where("id=?",issueID).UpdateColumn("time_spent", gorm.Expr("time_spent + ?", seconds))` (o read-modify-write in transazione).
- [ ] **Step 3: Estimate write** — aggiungere colonna `RemainingEstimate int` al model issue **se assente** (verificare `model.go:43-44`; se manca, migrazione `000016_issue_remaining_estimate`). Estendere il body `PUT /issue` con:
```go
TimeTracking *struct {
	OriginalEstimateSeconds  *int `json:"originalEstimateSeconds"`
	RemainingEstimateSeconds *int `json:"remainingEstimateSeconds"`
} `json:"timetracking"`
```
e in `service.Update` aggiungere i parametri (o un metodo dedicato `SetTimeTracking(key, orig, remaining)` per non allargare la firma di Update — preferire il metodo dedicato per ridurre il ripple sui chiamanti). Il mapper v3 `TimeTracking` deve emettere `remainingEstimateSeconds` da `Issue.RemainingEstimate`.
- [ ] **Step 4: Test contract** — `POST /issue/{key}/worklog {timeSpentSeconds:3600}` poi `GET /issue/{key}` → `fields.timetracking.timeSpentSeconds == 3600`; `PUT /issue/{key} {fields:{timetracking:{originalEstimateSeconds:28800}}}` → GET riflette 28800.
- [ ] **Step 5: Gate + commit**
```bash
git commit -am "fix(worklog): logging work updates issue timeSpent; support estimate write via PUT /issue"
```

### Task 8: Frontend — blocco Time tracking + Log work
**Files:** `frontend-next/lib/api.ts`, `frontend-next/components/issues/IssueView.tsx`, `frontend-next/e2e/issue-detail.spec.ts`

**Interfaces:**
- Produces: `worklogs.add(key, {timeSpentSeconds, comment?, started?})`, `worklogs.list(key)`, `worklogs.delete(key, id)`; helper `parseJiraDuration(str)→seconds` e `formatSeconds(sec)→"2h 30m"` (client-side, speculari a `parseJiraDuration` backend); `issues.update` timetracking.

- [ ] **Step 1: Client + helper durata** (parser `Xw Yd Zh Wm` ↔ secondi: `w=5*8h, d=8h, h=3600, m=60`, coerente col backend `worklog_handler.go`).
- [ ] **Step 2: UI** — nel pannello Details (`<aside>`) di `IssueView.tsx`, blocco **Time tracking**: barra `timeSpent/originalEstimate` + testo, o "No time logged"; campi **Original/Remaining estimate** editabili (in Edit-mode, formato Jira) → `issues.update({timetracking})`; bottone **Log work** → dialog (tempo speso obbligatorio, data, descrizione) → `worklogs.add`; lista worklog (autore, tempo formattato, data, elimina).
- [ ] **Step 3: E2E** — apri issue, "Log work" 2h, salva, asserisci che il blocco mostra "2h logged" e il worklog compare in lista.
- [ ] **Step 4: Gate + commit**
```bash
git commit -am "feat(frontend): time tracking + log work UI on issue view"
```

---

## FASE 5 — Storico (History)

### Task 9: Frontend — tab History nell'Activity
**Files:** `frontend-next/lib/api.ts`, `frontend-next/components/issues/IssueView.tsx` (o `Comments.tsx`), `frontend-next/e2e/issue-detail.spec.ts`

**Interfaces:**
- Consumes: `GET /issue/{key}/changelog` → `{ values: [ { id, author?, created, items:[{field, fromString, toString}] } ], isLast }` (esistente, `router.go:202`).
- Produces: client `issues.changelog(key)`; barra tab **Comments | History** nell'area Activity.

- [ ] **Step 1: Client** — `issues.changelog(idOrKey) → { values: Changelog[] }` con tipo `Changelog`/`ChangeItem`.
- [ ] **Step 2: UI** — nell'area Activity (attualmente solo Comments in `Comments.tsx`) aggiungere una barra tab **Comments | History**. History = elenco cronologico: per ogni entry, "{author?.displayName ?? 'System'} · {field} da «{fromString}» a «{toString}» · {tempo relativo}". Riusare `AdfRenderer` non serve (testo semplice).
- [ ] **Step 3: E2E** — apri una issue, cambia lo stato (o priorità) via la UI, apri il tab History, asserisci che compare una riga con il cambiamento di quel campo.
- [ ] **Step 4: Gate + commit**
```bash
git commit -am "feat(frontend): activity History tab (per-issue changelog)"
```

> **Nota (gap dichiarato):** il backend logga `ActorID=""` (l'autore risulta assente). In R13 la History mostra "System" come autore. L'**attribuzione dell'autore** (threading dell'uid utente attraverso `issue.Service.Create/Update` fino a `logHistory`) è un fix trasversale invasivo (tocca molte firme/chiamanti) → **follow-up dedicato**, non in questo round.

---

## FASE 6 — Editing assignee + create ricco

### Task 10: Frontend — UserPicker, assignee editabile, CreateIssueModal ricco
**Files:**
- Create: `frontend-next/components/common/UserPicker.tsx`
- Modify: `frontend-next/lib/api.ts`, `frontend-next/components/issues/IssueView.tsx`, `frontend-next/components/issues/CreateIssueModal.tsx`
- Test: `frontend-next/e2e/issue-detail.spec.ts`

**Interfaces:**
- Consumes: `GET /user/assignable/search?project={key}&query={q}` → `[]v3.User` (esistente, membership-gated dal R12); `PUT /issue {fields:{assignee:{accountId}}}` (esistente).
- Produces: `users.assignableSearch(projectKey, query) → JiraUserRef[]`; componente `UserPicker({ projectKey, value, onChange })`.

- [ ] **Step 1: Client** — `users.assignableSearch(projectKey, query)`.
- [ ] **Step 2: UserPicker** — componente autocomplete: input → debounced query → dropdown risultati (avatar + displayName) → onChange(accountId). Include opzione "Unassigned" e "Assign to me".
- [ ] **Step 3: Assignee editabile in IssueView** — nel pannello Details, in Edit-mode l'Assignee diventa un `UserPicker` (projectKey da `issue.fields.project?.key`); salva via `issues.update({assignee:{accountId}})`; rimuovere il commento "read-only, out of scope" (`IssueView.tsx:24-25,191`).
- [ ] **Step 4: CreateIssueModal ricco** — aggiungere campi: description (textarea→ADF via `textToAdf`), priority (`meta.priorities()`), assignee (`UserPicker`), e parent (solo quando `issueTypeName === "Subtask"`, o sempre opzionale) → passati a `issues.create`. Aggiungere checkbox **"Create another"** (a submit, crea e resetta il form invece di chiudere).
- [ ] **Step 5: E2E** — apri issue, entra in Edit, cambia assignee con il picker (seleziona un utente seedato), salva, asserisci che l'assignee mostrato è aggiornato; + crea una issue dal modale con priority+assignee valorizzati.
- [ ] **Step 6: Gate + commit**
```bash
git commit -am "feat(frontend): user picker, editable assignee, richer create modal"
```

---

## FASE 7 — Chiusura

### Task 11: Seed demo + gate finale + STATE/CHANGELOG
**Files:** `cmd/seed/main.go`, `docs/superpowers/STATE.md`, `CHANGELOG.md`, `docs/contracts/gap-report.md`

- [ ] **Step 1: Seed** — su DEMO/DEMO-1 aggiungere (idempotente): un subtask figlio, un issue-link tra due issue, un worklog, un allegato demo (creare un piccolo file). Così le nuove sezioni sono popolate nella demo/E2E.
- [ ] **Step 2: Gate a tre livelli completo**
```bash
go build ./... && echo BUILD_OK && go vet ./... && echo VET_OK && go test ./... 2>&1 | grep -vE '^ok|no test files'; echo GO_DONE
go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md && rm -f seed gapreport
lsof -ti:8080 | xargs kill 2>/dev/null; lsof -ti:3000 | xargs kill 2>/dev/null; sleep 1
cd frontend-next && npx tsc --noEmit && echo TSC_OK && npm run build 2>&1 | tail -3 && npx playwright test --reporter=line 2>&1 | tail -8; cd ..
```
Expected: tutto verde, gap report aggiornato (nuove rotte come "extra"), E2E verdi.
- [ ] **Step 3: Docs** — `CHANGELOG.md` voce `[Unreleased]`/prossima minor: issue view completa (subtask, link, allegati, time tracking, history, assignee/create). `STATE.md`: Round 13 completato, elenco cablaggi; prossimo Round 14 (Timeline/Calendar/Burnup/CSV) dal requirements v2.
- [ ] **Step 4: Commit**
```bash
git add cmd/seed/main.go docs/ CHANGELOG.md
git commit -m "docs: complete issue view (Round 13); seed demo subtask/link/attachment/worklog"
```

---

## Note di chiusura round

- **Follow-up generati:** (1) **attribuzione autore nello storico** (threading uid → logHistory, invasivo); (2) link types oltre i 3 supportati + endpoint `/issueLinkType`; (3) `author` sugli allegati (risoluzione uploader nella shape v3); (4) rich text editor + @mentions (round successivo); (5) `RemainingEstimate` auto-decrement al log-work (ora manuale).
- **Rischi:** l'upload multipart nel frontend richiede di bypassare il `Content-Type: application/json` di `apiFetch` — verificare presto (Task 6 Step 1). Il fix worklog→TimeSpent cambia dati esistenti solo in avanti (nessuna migrazione dati). Le nuove rotte GET per-issue sono già coperte dall'enforcement `EnforceNotFound` (nessun buco letture).
- Il round chiude solo con i tre livelli verdi + E2E delle 6 sezioni.

---

## Self-Review (svolta in fase di scrittura)

**1. Copertura spec (requirements v2 §A.1 + §B.4):** Subtask → T1/T2 ✅; Linked items → T3/T4 ✅; Allegati → T5/T6 ✅; Worklog/time → T7/T8 ✅; History → T9 ✅ (autore = follow-up dichiarato); Assignee edit + create ricco → T10 ✅. Burnup/CSV/Timeline/Calendar NON in R13 (sono R14 nel v2) — corretto.

**2. Placeholder scan:** i punti "VERIFICARE il nome reale" (renderer issue v3, tipi collab.go, colonna RemainingEstimate, comportamento apiFetch multipart) sono verifiche mirate su codice esistente con file indicato — non placeholder di logica. Ogni task backend ha codice concreto per la parte non ovvia (handler, shape) + un contract test con corpo reale.

**3. Consistenza tipi/rotte:** rotte per-issue tutte `{issueIdOrKey}` + `chk.ByIssueParam("issueIdOrKey")` + `EnforceNotFound`. Client: `issues.subtasks/changelog`, `issueLinks.*`, `attachments.*`, `worklogs.*`, `users.assignableSearch` — nomi usati coerentemente tra Task backend (produce) e Task frontend (consume). `parentKey` in `issues.create` coerente tra T2 e T10. `v3.Attachment.content` = `/rest/api/3/attachment/content/{id}` coerente tra T5 (produce) e T6 (consume). Gate three-level richiamato dai Global Constraints in ogni Task di chiusura fase.
