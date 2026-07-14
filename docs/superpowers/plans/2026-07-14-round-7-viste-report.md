# Round 7 — Viste & Report Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dare a open-jira i report agili (Burndown, Velocity, CFD, torta per campo, created-vs-resolved), il Summary di progetto e le Dashboard con gadget, con UI a grafici — correggendo prima il bug di sorgente dati nello storico che oggi rende burndown/CFD inaffidabili.

**Architecture:** Il backend di report/dashboard **esiste già** (`internal/domain/report`, `internal/domain/dashboard`, relativi handler e rotte). Round 7: (1) **corregge** le query sullo storico issue (`issue_history`) che cercano `field_name='status_id'`/`'sprint_id'` mentre `logHistory` scrive `'status'` — così burndown/CFD leggono i veri cambi di stato, con la CFD che fa join sullo stato **storico** (`ih.new_value`) e non su quello corrente; (2) **aggiunge** i report torta-per-campo e created-vs-resolved; (3) aggiunge **test** (oggi assenti). Il frontend è greenfield: grafici SVG dependency-free (robusti su React 19, nessuna dipendenza pesante), client API, pagina Report di progetto, sezione Summary, pagina Dashboards con gadget. **Timeline/Gantt e Calendar sono rinviati a follow-up.**

**Tech Stack:** Go 1.25 (net/http, GORM, SQLite in test), dominio `internal/domain/{report,dashboard,issue,sprint}`, `internal/api/v3`, harness `internal/contract`. Frontend Next.js 16 + React 19 + TanStack Query + Tailwind + **grafici SVG inline (no libreria esterna)** + Playwright.

---

## Contesto per l'implementatore (leggere una volta)

**Il bug centrale (sorgente dati storico):**
- `issue.Service.logHistory(issueID, actorID, field, oldVal, newVal string)` (`internal/domain/issue/service.go:303`) scrive nella tabella `issue_history` questi `field_name`: `"created"` (new_value = issue key), `"title"`, `"description"`, `"priority"`, `"assignee"`, `"status"` (old/new = **status ID**, service.go:130), `"story_points"` (old/new = numero).
- `internal/domain/report/service.go` interroga invece `field_name IN ('story_points','status_id','sprint_id')` (burndown, `:88`), `IN ('story_points','status_id')` (burnup, `:273`), `IN ('status_id','created')` (CFD, `:339`), e negli switch usa `case "status_id"` (`:128`, `:307`). **Nessuno matcha `'status'`**, e `'sprint_id'` non viene MAI loggato. Risultato: burndown/burnup/CFD non vedono i cambi di stato.
- Inoltre la CFD (`:338-340`) fa `LEFT JOIN workflow_statuses ws ON i.status_id = ws.id` — cioè sullo stato **corrente** dell'issue, non su quello storico dell'evento. Va corretta a join sullo `new_value` dell'evento `status`.

**Modelli/servizi dati (verificati):**
- `issue.IssueHistory` (`internal/domain/issue/model.go:84-94`, tabella `issue_history`): `ID`, `IssueID`, `ActorID *string`, `FieldName`, `OldValue`, `NewValue`, `CreatedAt`. `issue.Service.GetHistory(issueID)`; `issue.Service.DB() *gorm.DB`.
- `issue.Issue` (model.go:26-56): `StatusID *string`, `ResolutionID *string`, `Priority` (enum `highest/high/medium/low/lowest`), `TypeID *string`, `AssigneeID *string`, `SprintID *string`, `StoryPoints int`, `StartDate`/`DueDate *time.Time`, `CreatedAt`/`UpdatedAt`, `ProjectID`, `Key`, `IsArchived`, `SeqID`.
- `sprint.Sprint`: `StartDate`/`EndDate`/`CompleteDate *time.Time`, `State` (`active/closed/future`), `SeqID`, `ProjectID`, `Name`, `Goal`.
- `workflow.WorkflowStatus{ID, Name, Category(todo/inprogress/done), Position}`; `workflow.Service.GetWorkflow(projectID)`.

**Backend report/dashboard esistente (RIUSARE, correggere):**
- `internal/domain/report/service.go`: `NewService(db)`, `GetBurndownData(sprintID)(BurndownData,...)` con `BurndownData{Labels []string, Ideal []int, Actual []int}`, `GetBurnupData(sprintID)`, `GetVelocity(projectID)(VelocityData{Sprints []SprintVelocity{SprintID,SprintName,Completed,TotalPlanned}})`, `GetCFD(projectID)(CFDData{Categories []string, Dates []string, Data map[string][]int})`, `GetProjectSummary(projectID)(ProjectSummary{IssueCountByStatus map[string]int, CreatedLast7Days, UpdatedLast7Days, CompletedLast7Days int64, ActiveSprint *sprint.Sprint})`. **VERIFICARE le firme reali leggendo il file prima di modificarle.**
- `internal/api/handlers/report_handler.go`: `Burndown`/`Velocity`/`Burnup`/`CFD`/`Summary`; rotte `GET /rest/api/3/project/{key}/reports/{burndown,velocity,burnup,cfd}` (`router.go:282-285`), `GET /rest/api/3/project/{key}/summary` (`:286`). Gli handler risolvono il progetto per `{key}` e leggono `?sprintId=` dove serve.
- `internal/domain/dashboard/service.go` + `internal/api/handlers/dashboard_handler.go`: dashboard/gadget CRUD già montati (`router.go:288-303`), inclusi gli endpoint v3-shaped `/rest/api/3/dashboard[...]` e `/dashboard/{id}/gadget`. Widget computati per tipo (`assigned_to_me`, `activity_stream`).

**Piattaforma v3:** `v3.WriteJSON(w, status, v)`, `v3.WriteError(w, status, []string, map[string]string)`. Gli handler report/dashboard esistenti fanno `json.NewEncoder(w).Encode(...)` a mano — i NUOVI endpoint usino `v3.WriteJSON`/`v3.WriteError`.

**Frontend (greenfield per questa area):**
- `apiFetch<T>(path, opts)` wrapper reale (Bearer, 401→login, parse errori v3, 204→undefined); `buildQuery(params)`; export raggruppati (`projects`/`issues`/`search`/`filters`/`boards`/`sprints`/`workflow`). **Nessun** client `reports`/`dashboards` ancora.
- **Nessuna libreria grafici** in package.json (React 19.2 + Next 16.2). Round 7 usa **componenti SVG inline dependency-free** (niente recharts: evita rischi di compatibilità React 19 e bundle pesante).
- Le pagine `/jira/dashboards` e `/jira/plans` sono **link morti** nella sidebar (`Sidebar.tsx`) — nessuna pagina esiste. Round 7 crea la pagina Dashboards.
- Pattern pagina (`app/jira/filters/page.tsx`): `"use client"`, `useQuery`/`useMutation`, `@/lib/api`, Tailwind con hex (`text-[#1a1f36]`, primario `bg-[#0052cc]`), layout `mx-auto max-w-5xl p-6`. Le pagine con param dinamico usano `const { key } = use(params)`.

**Migrazioni:** ultima `000013`. Le tabelle `dashboards`/`dashboard_widgets`/`issue_history` **esistono già** (000001) → Round 7 **non serve nuova migrazione** salvo imprevisti.

**Harness contract:** come nei round precedenti (`newTestServer`, `registerAndLogin`, `createProjectViaAPI`, `createIssueViaAPI`, `MustLoad`, `doJSON`, `decodeBody`). I report sono **estensioni custom** (non nel contratto v3) → i loro test verificano status+shape+comportamento, NON `ValidateResponse`. Le dashboard hanno endpoint v3-shaped: per quelli si può usare `ValidateResponse` contro `jira-platform-v3.json` se lo shape combacia (verificare; altrimenti solo comportamento).

**Scope escluso (follow-up, dichiarato):** Timeline/Gantt; Calendar (issue per due date); gadget configurabili avanzati; report export (CSV/PDF); scelta libreria grafici professionale.

---

## Struttura dei file

**Backend:**
- `internal/domain/report/service.go` — correggere query storico (T1/T2); aggiungere `GetPieByField` (T3), `GetCreatedVsResolved` (T4).
- `internal/domain/report/service_test.go` — nuovo (T1/T2/T3/T4).
- `internal/api/handlers/report_handler.go` — aggiungere handler `Pie`, `CreatedVsResolved` (T3/T4).
- `internal/api/router.go` — 2 nuove rotte report (T3/T4).

**Frontend:**
- `frontend-next/components/charts/` — `LineChart.tsx`, `BarChart.tsx`, `StackedAreaChart.tsx`, `PieChart.tsx` (T5).
- `frontend-next/lib/api.ts` — `reports` + `dashboards` client (T6).
- `frontend-next/app/jira/projects/[key]/reports/page.tsx` — pagina report (T7).
- `frontend-next/components/projects/ProjectSummary.tsx` + tab in `ProjectSettings`? no → sezione nella pagina progetto (T8).
- `frontend-next/app/jira/dashboards/page.tsx` + `frontend-next/app/jira/dashboards/[id]/page.tsx` — dashboards (T9).
- `frontend-next/e2e/reports.spec.ts` (T10).

**Seed:** `cmd/seed/main.go` — dashboard demo + widget (T11).

---

### Task 1: Fix storico burndown/velocity + test report

**Files:**
- Modify: `internal/domain/report/service.go`
- Test: `internal/domain/report/service_test.go`

- [ ] **Step 1: Leggere il codice reale**

Leggere `internal/domain/report/service.go` per intero: firme esatte di `GetBurndownData`, `GetBurnupData`, `GetVelocity`, `GetProjectSummary`, i tipi (`BurndownData`, `VelocityData`, `SprintVelocity`, `ProjectSummary`), e le query/switch che usano `field_name` / `case "status_id"`.

- [ ] **Step 2: Scrivere i test (definiscono il comportamento corretto)**

`internal/domain/report/service_test.go`:

```go
package report

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/sprint"
	"github.com/open-jira/open-jira/internal/domain/workflow"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&issue.Issue{}, &issue.IssueHistory{}, &sprint.Sprint{}, &workflow.Workflow{}, &workflow.WorkflowStatus{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// seedWorkflow crea uno workflow con TO DO(todo)/DONE(done) e ritorna gli id.
func seedWorkflow(t *testing.T, db *gorm.DB, projectID string) (todoID, doneID string) {
	t.Helper()
	wf := &workflow.Workflow{ID: uuid.NewString(), ProjectID: projectID, Name: "WF"}
	db.Create(wf)
	todo := &workflow.WorkflowStatus{ID: uuid.NewString(), WorkflowID: wf.ID, Name: "TO DO", Category: workflow.CategoryTodo, Position: 0}
	done := &workflow.WorkflowStatus{ID: uuid.NewString(), WorkflowID: wf.ID, Name: "DONE", Category: workflow.CategoryDone, Position: 1}
	db.Create(todo)
	db.Create(done)
	return todo.ID, done.ID
}

func TestBurndown_ReadsStatusHistory(t *testing.T) {
	db := newDB(t)
	todoID, doneID := seedWorkflow(t, db, "proj-1")
	start := time.Now().AddDate(0, 0, -3)
	end := time.Now().AddDate(0, 0, 3)
	sp := &sprint.Sprint{ID: uuid.NewString(), ProjectID: "proj-1", Name: "S1", State: sprint.StateActive, StartDate: &start, EndDate: &end, SeqID: 1}
	db.Create(sp)
	// una issue da 5 punti, nello sprint, passata a DONE ieri
	iss := &issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "x", SeqID: 1, StoryPoints: 5, SprintID: &sp.ID, StatusID: &doneID}
	db.Create(iss)
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: iss.ID, FieldName: "story_points", OldValue: "0", NewValue: "5", CreatedAt: start})
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: iss.ID, FieldName: "status", OldValue: todoID, NewValue: doneID, CreatedAt: time.Now().AddDate(0, 0, -1)})

	svc := NewService(db)
	data, err := svc.GetBurndownData(sp.ID)
	if err != nil {
		t.Fatalf("GetBurndownData: %v", err)
	}
	if len(data.Actual) == 0 {
		t.Fatal("Actual vuoto")
	}
	// dopo il passaggio a DONE il lavoro rimanente cala: l'ultimo valore < primo
	if data.Actual[len(data.Actual)-1] >= data.Actual[0] {
		t.Errorf("il burndown deve scendere dopo il completamento: %v", data.Actual)
	}
}

func TestVelocity_ClosedSprints(t *testing.T) {
	db := newDB(t)
	_, doneID := seedWorkflow(t, db, "proj-1")
	cd := time.Now().AddDate(0, 0, -1)
	sp := &sprint.Sprint{ID: uuid.NewString(), ProjectID: "proj-1", Name: "S1", State: sprint.StateClosed, CompleteDate: &cd, SeqID: 1}
	db.Create(sp)
	iss := &issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "x", SeqID: 1, StoryPoints: 8, SprintID: &sp.ID, StatusID: &doneID}
	db.Create(iss)

	svc := NewService(db)
	v, err := svc.GetVelocity("proj-1")
	if err != nil {
		t.Fatalf("GetVelocity: %v", err)
	}
	if len(v.Sprints) != 1 {
		t.Fatalf("atteso 1 sprint chiuso, %d", len(v.Sprints))
	}
	if v.Sprints[0].Completed != 8 {
		t.Errorf("completed atteso 8, got %d", v.Sprints[0].Completed)
	}
}

func TestProjectSummary_Counts(t *testing.T) {
	db := newDB(t)
	todoID, doneID := seedWorkflow(t, db, "proj-1")
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "a", SeqID: 1, StatusID: &todoID})
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-2", Title: "b", SeqID: 2, StatusID: &doneID})

	svc := NewService(db)
	sum, err := svc.GetProjectSummary("proj-1")
	if err != nil {
		t.Fatalf("GetProjectSummary: %v", err)
	}
	total := 0
	for _, n := range sum.IssueCountByStatus {
		total += n
	}
	if total != 2 {
		t.Errorf("attese 2 issue nei conteggi per stato, got %d (%v)", total, sum.IssueCountByStatus)
	}
}
```

> **Nota implementatore:** adeguare i test alle firme/tipi REALI letti allo Step 1 (nomi campi di `BurndownData`/`VelocityData`/`ProjectSummary`). Se un getter ha una firma diversa (es. ritorna `(*BurndownData, error)`), adeguare. L'intento è: burndown/velocity/summary leggono i dati realmente loggati.

- [ ] **Step 3: Eseguire i test (falliscono per il bug)**

Run: `go test ./internal/domain/report/ -run 'TestBurndown_ReadsStatusHistory|TestVelocity|TestProjectSummary' -v`
Expected: `TestBurndown_ReadsStatusHistory` FALLISCE (le query cercano `status_id`, lo storico ha `status`) — dimostra il bug.

- [ ] **Step 4: Correggere le query nello storico**

In `internal/domain/report/service.go`:
- Sostituire OGNI occorrenza di `'status_id'` con `'status'` nelle clausole `field_name IN (...)` (burndown `:88`, burnup `:273`) e nei `case "status_id":` degli switch (`:128`, `:307`) → `case "status":`.
- Rimuovere `'sprint_id'` dalle liste `field_name IN (...)` e il relativo `case "sprint_id":` (non viene mai loggato — codice morto). NB: nel `case "status"` il `NewValue` è l'**id** dello stato di destinazione: la logica esistente che confronta lo stato per determinare "done" deve usare `NewValue` come status id (già così se prima usava il valore dell'evento). Verificare che il confronto "è uno stato done?" risolva `NewValue` → categoria via `workflow_statuses` (query o mappa) — se il codice attuale confrontava un id, ora funziona perché `NewValue` è l'id giusto.

- [ ] **Step 5: Eseguire i test (passano)**

Run: `go test ./internal/domain/report/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/report/service.go internal/domain/report/service_test.go
git commit -m "fix(report): read status history by real field name 'status'; add report tests"
```

---

### Task 2: CFD — join sullo stato storico + test

**Files:**
- Modify: `internal/domain/report/service.go` (`GetCFD`)
- Test: `internal/domain/report/service_test.go` (aggiungere)

- [ ] **Step 1: Scrivere il test CFD**

Aggiungere a `internal/domain/report/service_test.go`:

```go
func TestCFD_ReadsHistoricalStatus(t *testing.T) {
	db := newDB(t)
	todoID, doneID := seedWorkflow(t, db, "proj-1")
	iss := &issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "x", SeqID: 1, StatusID: &doneID}
	db.Create(iss)
	// evento 'created' (entra in todo) e 'status' → done
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: iss.ID, FieldName: "created", OldValue: "", NewValue: "P-1", CreatedAt: time.Now().AddDate(0, 0, -2)})
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: iss.ID, FieldName: "status", OldValue: todoID, NewValue: doneID, CreatedAt: time.Now().AddDate(0, 0, -1)})

	svc := NewService(db)
	cfd, err := svc.GetCFD("proj-1")
	if err != nil {
		t.Fatalf("GetCFD: %v", err)
	}
	if len(cfd.Dates) == 0 {
		t.Fatal("CFD senza date")
	}
	// deve esistere un conteggio non-zero per la categoria 'done' (dallo status storico)
	done, ok := cfd.Data["done"]
	if !ok {
		t.Fatalf("categoria 'done' assente nel CFD: chiavi %v", keysOf(cfd.Data))
	}
	sum := 0
	for _, n := range done {
		sum += n
	}
	if sum == 0 {
		t.Errorf("la categoria done deve avere conteggi > 0 dallo status storico")
	}
}

func keysOf(m map[string][]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
```

> **Nota:** adeguare i nomi di campo di `CFDData` (`Categories`/`Dates`/`Data`) e le chiavi della mappa (categoria interna `todo`/`inprogress`/`done`) a quelle reali. Se `GetCFD` chiavizza per NOME categoria diverso, adeguare l'asserzione.

- [ ] **Step 2: Eseguire il test (fallisce)**

Run: `go test ./internal/domain/report/ -run TestCFD -v`
Expected: FAIL (la query CFD cerca `'status_id'` e fa join sullo stato corrente).

- [ ] **Step 3: Correggere GetCFD**

In `internal/domain/report/service.go` `GetCFD` (query intorno a `:331-340`): la query deve (a) filtrare `ih.field_name IN ('status','created')`; (b) per le righe `status`, ricavare la categoria dallo stato **storico** `ih.new_value` (non `i.status_id`), cioè `LEFT JOIN workflow_statuses ws ON ws.id = ih.new_value`; (c) per le righe `created`, contare l'ingresso nella categoria iniziale (todo). Esempio di query corretta (adeguare al dialetto/colonne reali):

```sql
SELECT
  DATE(ih.created_at) AS date,
  CASE WHEN ih.field_name = 'created' THEN 'todo'
       ELSE COALESCE(ws.category, 'inprogress') END AS category,
  COUNT(*) AS cnt
FROM issue_history ih
JOIN issues i ON i.id = ih.issue_id
LEFT JOIN workflow_statuses ws ON ws.id = ih.new_value
WHERE i.project_id = ? AND i.is_archived = FALSE
  AND ih.field_name IN ('status','created')
GROUP BY DATE(ih.created_at), category
ORDER BY date ASC
```

Mantenere la struttura di uscita `CFDData{Categories, Dates, Data}` invariata (la logica di post-processing che riempie `Data[cat][i]` resta; cambia solo la sorgente).

- [ ] **Step 4: Eseguire i test (passano)**

Run: `go test ./internal/domain/report/ -v`
Expected: PASS (tutti, incl. Task 1).

- [ ] **Step 5: Commit**

```bash
git add internal/domain/report/service.go internal/domain/report/service_test.go
git commit -m "fix(report): CFD joins historical status (ih.new_value) and reads 'status' events"
```

---

### Task 3: Report torta per campo

**Files:**
- Modify: `internal/domain/report/service.go`
- Modify: `internal/api/handlers/report_handler.go`
- Modify: `internal/api/router.go`
- Test: `internal/domain/report/service_test.go`

- [ ] **Step 1: Scrivere il test**

Aggiungere a `internal/domain/report/service_test.go`:

```go
func TestPieByField_Status(t *testing.T) {
	db := newDB(t)
	todoID, doneID := seedWorkflow(t, db, "proj-1")
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "a", SeqID: 1, StatusID: &todoID})
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-2", Title: "b", SeqID: 2, StatusID: &doneID})
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-3", Title: "c", SeqID: 3, StatusID: &doneID})

	svc := NewService(db)
	slices, err := svc.GetPieByField("proj-1", "status")
	if err != nil {
		t.Fatalf("GetPieByField: %v", err)
	}
	byLabel := map[string]int{}
	for _, s := range slices {
		byLabel[s.Label] = s.Count
	}
	if byLabel["TO DO"] != 1 || byLabel["DONE"] != 2 {
		t.Errorf("conteggi torta errati: %v", byLabel)
	}
}

func TestPieByField_Priority(t *testing.T) {
	db := newDB(t)
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "a", SeqID: 1, Priority: issue.PriorityHigh})
	db.Create(&issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-2", Title: "b", SeqID: 2, Priority: issue.PriorityHigh})
	svc := NewService(db)
	slices, err := svc.GetPieByField("proj-1", "priority")
	if err != nil {
		t.Fatalf("GetPieByField: %v", err)
	}
	if len(slices) != 1 || slices[0].Label != "high" || slices[0].Count != 2 {
		t.Errorf("torta priority errata: %+v", slices)
	}
}

func TestPieByField_Invalid(t *testing.T) {
	svc := NewService(newDB(t))
	if _, err := svc.GetPieByField("proj-1", "bogus"); err == nil {
		t.Error("atteso errore per campo non supportato")
	}
}
```

- [ ] **Step 2: Eseguire (falliscono)**

Run: `go test ./internal/domain/report/ -run TestPieByField -v`
Expected: FAIL con "GetPieByField undefined".

- [ ] **Step 3: Implementare GetPieByField**

Aggiungere in `internal/domain/report/service.go`:

```go
// PieSlice è una fetta della torta: etichetta + conteggio.
type PieSlice struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// GetPieByField aggrega le issue non archiviate del progetto per il campo dato.
// Campi supportati: status, priority, assignee, type.
func (s *Service) GetPieByField(projectID, field string) ([]PieSlice, error) {
	var q string
	switch field {
	case "status":
		q = `SELECT COALESCE(ws.name, 'No status') AS label, COUNT(*) AS count
		     FROM issues i LEFT JOIN workflow_statuses ws ON ws.id = i.status_id
		     WHERE i.project_id = ? AND i.is_archived = FALSE
		     GROUP BY label ORDER BY count DESC`
	case "priority":
		q = `SELECT priority AS label, COUNT(*) AS count
		     FROM issues WHERE project_id = ? AND is_archived = FALSE
		     GROUP BY priority ORDER BY count DESC`
	case "assignee":
		q = `SELECT COALESCE(u.display_name, 'Unassigned') AS label, COUNT(*) AS count
		     FROM issues i LEFT JOIN users u ON u.id = i.assignee_id
		     WHERE i.project_id = ? AND i.is_archived = FALSE
		     GROUP BY label ORDER BY count DESC`
	case "type":
		q = `SELECT COALESCE(it.name, 'No type') AS label, COUNT(*) AS count
		     FROM issues i LEFT JOIN issue_types it ON it.id = i.type_id
		     WHERE i.project_id = ? AND i.is_archived = FALSE
		     GROUP BY label ORDER BY count DESC`
	default:
		return nil, fmt.Errorf("unsupported field: %s", field)
	}
	var slices []PieSlice
	if err := s.db.Raw(q, projectID).Scan(&slices).Error; err != nil {
		return nil, err
	}
	return slices, nil
}
```

> **Nota implementatore:** verificare i nomi tabella/colonna reali: `workflow_statuses(name,id)`, `users(display_name,id)` (confermare `display_name`), `issue_types(name,id)` (confermare il nome tabella — potrebbe essere `issue_types`). Aggiungere l'import `fmt` se assente. `issue.PriorityHigh` = `"high"`.

- [ ] **Step 4: Handler + rotta**

In `internal/api/handlers/report_handler.go` aggiungere (usando `v3.WriteJSON`/`v3.WriteError`):

```go
// Pie: GET /rest/api/3/project/{key}/reports/pie?field=status|priority|assignee|type
func (h *ReportHandler) Pie(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	field := r.URL.Query().Get("field")
	if field == "" {
		field = "status"
	}
	slices, err := h.svc.GetPieByField(p.ID, field)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{err.Error()}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, slices)
}
```

In `internal/api/router.go`, vicino alle altre rotte report:

```go
	mux.Handle("GET /rest/api/3/project/{key}/reports/pie", authMw(http.HandlerFunc(reportH.Pie)))
```

> **Nota:** verificare il nome reale della variabile handler report in router.go (`reportH`?) e i campi di `ReportHandler` (`svc`, `projectSvc`) leggendo `report_handler.go`. Importare `v3` se non presente.

- [ ] **Step 5: Eseguire i test + build**

Run: `go test ./internal/domain/report/ -run TestPieByField -v && go build ./...`
Expected: PASS + build OK.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/report/service.go internal/api/handlers/report_handler.go internal/api/router.go internal/domain/report/service_test.go
git commit -m "feat(report): pie-by-field aggregation endpoint"
```

---

### Task 4: Report created-vs-resolved

**Files:**
- Modify: `internal/domain/report/service.go`
- Modify: `internal/api/handlers/report_handler.go`
- Modify: `internal/api/router.go`
- Test: `internal/domain/report/service_test.go`

- [ ] **Step 1: Scrivere il test**

Aggiungere a `internal/domain/report/service_test.go`:

```go
func TestCreatedVsResolved(t *testing.T) {
	db := newDB(t)
	_, doneID := seedWorkflow(t, db, "proj-1")
	now := time.Now()
	// una issue creata 2 giorni fa
	c := &issue.Issue{ID: uuid.NewString(), ProjectID: "proj-1", Key: "P-1", Title: "a", SeqID: 1, StatusID: &doneID, CreatedAt: now.AddDate(0, 0, -2)}
	db.Create(c)
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: c.ID, FieldName: "created", OldValue: "", NewValue: "P-1", CreatedAt: now.AddDate(0, 0, -2)})
	// risolta (status → done) ieri
	db.Create(&issue.IssueHistory{ID: uuid.NewString(), IssueID: c.ID, FieldName: "status", OldValue: "x", NewValue: doneID, CreatedAt: now.AddDate(0, 0, -1)})

	svc := NewService(db)
	data, err := svc.GetCreatedVsResolved("proj-1", 7)
	if err != nil {
		t.Fatalf("GetCreatedVsResolved: %v", err)
	}
	if len(data.Dates) != 7 {
		t.Fatalf("attesi 7 giorni, %d", len(data.Dates))
	}
	sumC, sumR := 0, 0
	for i := range data.Dates {
		sumC += data.Created[i]
		sumR += data.Resolved[i]
	}
	if sumC != 1 {
		t.Errorf("created totali attesi 1, %d", sumC)
	}
	if sumR != 1 {
		t.Errorf("resolved totali attesi 1, %d", sumR)
	}
}
```

- [ ] **Step 2: Eseguire (fallisce)**

Run: `go test ./internal/domain/report/ -run TestCreatedVsResolved -v`
Expected: FAIL con "GetCreatedVsResolved undefined".

- [ ] **Step 3: Implementare**

Aggiungere in `internal/domain/report/service.go`:

```go
// CreatedVsResolvedData: serie giornaliere di issue create vs risolte.
type CreatedVsResolvedData struct {
	Dates    []string `json:"dates"`
	Created  []int    `json:"created"`
	Resolved []int    `json:"resolved"`
}

// GetCreatedVsResolved conta, per gli ultimi `days` giorni, le issue create
// (evento 'created') e risolte (transizione a uno stato categoria 'done').
func (s *Service) GetCreatedVsResolved(projectID string, days int) (CreatedVsResolvedData, error) {
	if days <= 0 {
		days = 30
	}
	out := CreatedVsResolvedData{Dates: make([]string, days), Created: make([]int, days), Resolved: make([]int, days)}
	// indicizza le date (oggi-days+1 .. oggi)
	base := time.Now().Truncate(24 * time.Hour).AddDate(0, 0, -(days - 1))
	idx := map[string]int{}
	for i := 0; i < days; i++ {
		d := base.AddDate(0, 0, i).Format("2006-01-02")
		out.Dates[i] = d
		idx[d] = i
	}
	// created: eventi 'created'
	type row struct {
		Date string
		Cnt  int
	}
	var created []row
	s.db.Raw(`SELECT DATE(ih.created_at) AS date, COUNT(*) AS cnt
	          FROM issue_history ih JOIN issues i ON i.id = ih.issue_id
	          WHERE i.project_id = ? AND ih.field_name = 'created' AND DATE(ih.created_at) >= ?
	          GROUP BY DATE(ih.created_at)`, projectID, base.Format("2006-01-02")).Scan(&created)
	for _, r := range created {
		if i, ok := idx[r.Date]; ok {
			out.Created[i] = r.Cnt
		}
	}
	// resolved: transizioni 'status' verso uno stato categoria 'done'
	var resolved []row
	s.db.Raw(`SELECT DATE(ih.created_at) AS date, COUNT(*) AS cnt
	          FROM issue_history ih
	          JOIN issues i ON i.id = ih.issue_id
	          JOIN workflow_statuses ws ON ws.id = ih.new_value
	          WHERE i.project_id = ? AND ih.field_name = 'status' AND ws.category = 'done' AND DATE(ih.created_at) >= ?
	          GROUP BY DATE(ih.created_at)`, projectID, base.Format("2006-01-02")).Scan(&resolved)
	for _, r := range resolved {
		if i, ok := idx[r.Date]; ok {
			out.Resolved[i] = r.Cnt
		}
	}
	return out, nil
}
```

> **Nota:** verificare che `DATE(...)` e il confronto stringa data funzionino su SQLite e Postgres (entrambi supportano `DATE()`; per Postgres `DATE(col) >= 'YYYY-MM-DD'` funziona). Aggiungere import `time` se assente.

- [ ] **Step 4: Handler + rotta**

In `report_handler.go`:

```go
// CreatedVsResolved: GET /rest/api/3/project/{key}/reports/created-vs-resolved?days=30
func (h *ReportHandler) CreatedVsResolved(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}
	data, err := h.svc.GetCreatedVsResolved(p.ID, days)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to compute report"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, data)
}
```

In `router.go`:

```go
	mux.Handle("GET /rest/api/3/project/{key}/reports/created-vs-resolved", authMw(http.HandlerFunc(reportH.CreatedVsResolved)))
```

> **Nota:** import `strconv` se assente.

- [ ] **Step 5: Test + build**

Run: `go test ./internal/domain/report/ -v && go build ./... && go vet ./...`
Expected: PASS + build/vet OK.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/report/service.go internal/api/handlers/report_handler.go internal/api/router.go internal/domain/report/service_test.go
git commit -m "feat(report): created-vs-resolved daily series endpoint"
```

---

### Task 5: Frontend — primitive grafici SVG

**Files:**
- Create: `frontend-next/components/charts/LineChart.tsx`
- Create: `frontend-next/components/charts/BarChart.tsx`
- Create: `frontend-next/components/charts/PieChart.tsx`
- Create: `frontend-next/components/charts/StackedAreaChart.tsx`

- [ ] **Step 1: LineChart (multi-serie)**

`frontend-next/components/charts/LineChart.tsx`:

```tsx
"use client";

interface Series {
  name: string;
  color: string;
  values: number[];
}

// LineChart: grafico a linee dependency-free (SVG). labels sull'asse X, una o più serie.
export function LineChart({ labels, series, height = 220 }: { labels: string[]; series: Series[]; height?: number }) {
  const w = 640;
  const pad = 32;
  const max = Math.max(1, ...series.flatMap((s) => s.values));
  const n = Math.max(1, labels.length - 1);
  const x = (i: number) => pad + (i * (w - 2 * pad)) / n;
  const y = (v: number) => height - pad - (v / max) * (height - 2 * pad);
  return (
    <svg viewBox={`0 0 ${w} ${height}`} className="w-full" role="img" data-testid="line-chart">
      <line x1={pad} y1={height - pad} x2={w - pad} y2={height - pad} stroke="#d1d5db" />
      <line x1={pad} y1={pad} x2={pad} y2={height - pad} stroke="#d1d5db" />
      {series.map((s) => (
        <polyline
          key={s.name}
          fill="none"
          stroke={s.color}
          strokeWidth={2}
          points={s.values.map((v, i) => `${x(i)},${y(v)}`).join(" ")}
        />
      ))}
      {series.map((s, si) => (
        <g key={s.name}>
          <rect x={pad + si * 120} y={8} width={10} height={10} fill={s.color} />
          <text x={pad + si * 120 + 14} y={17} fontSize={11} fill="#6b7280">{s.name}</text>
        </g>
      ))}
    </svg>
  );
}
```

- [ ] **Step 2: BarChart**

`frontend-next/components/charts/BarChart.tsx`:

```tsx
"use client";

interface Bar {
  label: string;
  value: number;
  color?: string;
}

export function BarChart({ bars, height = 220 }: { bars: Bar[]; height?: number }) {
  const w = 640;
  const pad = 32;
  const max = Math.max(1, ...bars.map((b) => b.value));
  const bw = bars.length ? (w - 2 * pad) / bars.length : 0;
  return (
    <svg viewBox={`0 0 ${w} ${height}`} className="w-full" role="img" data-testid="bar-chart">
      <line x1={pad} y1={height - pad} x2={w - pad} y2={height - pad} stroke="#d1d5db" />
      {bars.map((b, i) => {
        const h = (b.value / max) * (height - 2 * pad);
        return (
          <g key={b.label}>
            <rect x={pad + i * bw + 6} y={height - pad - h} width={bw - 12} height={h} fill={b.color ?? "#0052cc"} />
            <text x={pad + i * bw + bw / 2} y={height - pad + 14} fontSize={10} textAnchor="middle" fill="#6b7280">{b.label}</text>
            <text x={pad + i * bw + bw / 2} y={height - pad - h - 4} fontSize={10} textAnchor="middle" fill="#1a1f36">{b.value}</text>
          </g>
        );
      })}
    </svg>
  );
}
```

- [ ] **Step 3: PieChart**

`frontend-next/components/charts/PieChart.tsx`:

```tsx
"use client";

interface Slice {
  label: string;
  count: number;
}

const PALETTE = ["#0052cc", "#00875a", "#de350b", "#ff991f", "#6554c0", "#00b8d9", "#8993a4"];

export function PieChart({ slices }: { slices: Slice[] }) {
  const total = slices.reduce((a, s) => a + s.count, 0) || 1;
  const cx = 90, cy = 90, r = 80;
  let acc = 0;
  const arcs = slices.map((s, i) => {
    const start = (acc / total) * 2 * Math.PI;
    acc += s.count;
    const end = (acc / total) * 2 * Math.PI;
    const large = end - start > Math.PI ? 1 : 0;
    const x1 = cx + r * Math.sin(start), y1 = cy - r * Math.cos(start);
    const x2 = cx + r * Math.sin(end), y2 = cy - r * Math.cos(end);
    return { d: `M${cx},${cy} L${x1},${y1} A${r},${r} 0 ${large} 1 ${x2},${y2} Z`, color: PALETTE[i % PALETTE.length], s };
  });
  return (
    <div className="flex items-center gap-6" data-testid="pie-chart">
      <svg viewBox="0 0 180 180" className="h-44 w-44">
        {arcs.map((a) => <path key={a.s.label} d={a.d} fill={a.color} />)}
      </svg>
      <ul className="text-sm">
        {arcs.map((a) => (
          <li key={a.s.label} className="flex items-center gap-2">
            <span className="inline-block h-3 w-3 rounded" style={{ backgroundColor: a.color }} />
            <span className="text-[#1a1f36]">{a.s.label}</span>
            <span className="text-slate-400">{a.s.count}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
```

- [ ] **Step 4: StackedAreaChart (CFD)**

`frontend-next/components/charts/StackedAreaChart.tsx`:

```tsx
"use client";

// StackedAreaChart per la CFD: date sull'asse X, categorie impilate.
export function StackedAreaChart({
  dates,
  categories,
  data,
  height = 240,
}: {
  dates: string[];
  categories: string[];
  data: Record<string, number[]>;
  height?: number;
}) {
  const w = 640;
  const pad = 32;
  const colors: Record<string, string> = { todo: "#8993a4", inprogress: "#0052cc", done: "#00875a" };
  const n = Math.max(1, dates.length - 1);
  const x = (i: number) => pad + (i * (w - 2 * pad)) / n;
  // cumulativo per punto
  const cum = dates.map((_, i) => categories.reduce((a, c) => a + (data[c]?.[i] ?? 0), 0));
  const max = Math.max(1, ...cum);
  const y = (v: number) => height - pad - (v / max) * (height - 2 * pad);
  let below = dates.map(() => 0);
  return (
    <svg viewBox={`0 0 ${w} ${height}`} className="w-full" role="img" data-testid="cfd-chart">
      {categories.map((c) => {
        const top = dates.map((_, i) => below[i] + (data[c]?.[i] ?? 0));
        const area =
          dates.map((_, i) => `${x(i)},${y(top[i])}`).join(" ") +
          " " +
          dates.map((_, i) => `${x(dates.length - 1 - i)},${y(below[dates.length - 1 - i])}`).join(" ");
        below = top;
        return <polygon key={c} points={area} fill={colors[c] ?? "#c1c7d0"} fillOpacity={0.85} />;
      })}
      {categories.map((c, ci) => (
        <g key={c}>
          <rect x={pad + ci * 110} y={8} width={10} height={10} fill={colors[c] ?? "#c1c7d0"} />
          <text x={pad + ci * 110 + 14} y={17} fontSize={11} fill="#6b7280">{c}</text>
        </g>
      ))}
    </svg>
  );
}
```

- [ ] **Step 5: Type-check + build**

Run: `cd frontend-next && npx tsc --noEmit && npm run build`
Expected: build OK (i componenti non sono ancora usati — Next può fare tree-shaking; se `npm run build` fallisce per componenti non importati, non è un errore — verificare che compilino via tsc).

- [ ] **Step 6: Commit**

```bash
git add frontend-next/components/charts/
git commit -m "feat(frontend): dependency-free SVG chart primitives (line/bar/pie/stacked-area)"
```

---

### Task 6: Frontend — client reports + dashboards

**Files:**
- Modify: `frontend-next/lib/api.ts`

- [ ] **Step 1: Aggiungere i tipi e i client**

In `frontend-next/lib/api.ts`:

```ts
export interface BurndownData { labels: string[]; ideal: number[]; actual: number[] }
export interface VelocityData { sprints: { sprint_id: string; sprint_name: string; completed: number; total_planned: number }[] }
export interface CFDData { categories: string[]; dates: string[]; data: Record<string, number[]> }
export interface PieSlice { label: string; count: number }
export interface CreatedVsResolvedData { dates: string[]; created: number[]; resolved: number[] }
export interface ProjectSummary {
  issue_count_by_status: Record<string, number>;
  created_last_7_days: number;
  updated_last_7_days: number;
  completed_last_7_days: number;
  active_sprint?: { id: string; name: string } | null;
}

export const reports = {
  burndown: (key: string, sprintId: string) => apiFetch<BurndownData>(`/rest/api/3/project/${key}/reports/burndown?sprintId=${sprintId}`),
  velocity: (key: string) => apiFetch<VelocityData>(`/rest/api/3/project/${key}/reports/velocity`),
  cfd: (key: string) => apiFetch<CFDData>(`/rest/api/3/project/${key}/reports/cfd`),
  pie: (key: string, field: string) => apiFetch<PieSlice[]>(`/rest/api/3/project/${key}/reports/pie?field=${field}`),
  createdVsResolved: (key: string, days = 30) => apiFetch<CreatedVsResolvedData>(`/rest/api/3/project/${key}/reports/created-vs-resolved?days=${days}`),
  summary: (key: string) => apiFetch<ProjectSummary>(`/rest/api/3/project/${key}/summary`),
};

export interface Dashboard { id: string; name: string; is_public?: boolean }
export interface DashboardWidget { id: string; widget_type: string; config_json?: string; data?: unknown }

export const dashboards = {
  list: () => apiFetch<Dashboard[]>("/rest/api/3/dashboards"),
  get: (id: string) => apiFetch<Dashboard & { widgets?: DashboardWidget[] }>(`/rest/api/3/dashboards/${id}`),
  create: (name: string) => apiFetch<Dashboard>("/rest/api/3/dashboards", { method: "POST", body: JSON.stringify({ name }) }),
};
```

> **Nota implementatore (CRITICO):** i JSON key REALI dipendono dai tipi Go esistenti. LEGGERE `internal/domain/report/service.go` (tag json di `BurndownData`/`VelocityData`/`SprintVelocity`/`CFDData`/`ProjectSummary`) e `internal/domain/dashboard/service.go` + `dashboard_handler.go` (shape di list/get/create e dei widget) e ADEGUARE i tipi TS + i path esatti (es. la lista dashboard è `/rest/api/3/dashboards` custom o `/dashboard/search`? — verificare in router.go quale ritorna la lista semplice). Confermare `apiFetch`.

- [ ] **Step 2: Type-check**

Run: `cd frontend-next && npx tsc --noEmit`
Expected: nessun errore.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/lib/api.ts
git commit -m "feat(frontend): reports and dashboards API client"
```

---

### Task 7: Frontend — pagina Report di progetto

**Files:**
- Create: `frontend-next/app/jira/projects/[key]/reports/page.tsx`

- [ ] **Step 1: Pagina report**

`frontend-next/app/jira/projects/[key]/reports/page.tsx`:

```tsx
"use client";

import { use, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { reports, boards } from "@/lib/api";
import { LineChart } from "@/components/charts/LineChart";
import { BarChart } from "@/components/charts/BarChart";
import { PieChart } from "@/components/charts/PieChart";
import { StackedAreaChart } from "@/components/charts/StackedAreaChart";

function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mb-6 rounded border border-slate-200 bg-white p-4">
      <h2 className="mb-3 text-sm font-semibold text-[#1a1f36]">{title}</h2>
      {children}
    </section>
  );
}

export default function ReportsPage({ params }: { params: Promise<{ key: string }> }) {
  const { key } = use(params);
  const [pieField, setPieField] = useState("status");

  // sprint attivo/primo per il burndown: prendiamo gli sprint della board 1 del progetto
  const sprints = useQuery({ queryKey: ["reports", key, "sprints"], queryFn: () => boards.sprints(1) });
  const sprintId = sprints.data?.values[0]?.id;

  const burndown = useQuery({ queryKey: ["reports", key, "burndown", sprintId], queryFn: () => reports.burndown(key, String(sprintId)), enabled: !!sprintId });
  const velocity = useQuery({ queryKey: ["reports", key, "velocity"], queryFn: () => reports.velocity(key) });
  const cfd = useQuery({ queryKey: ["reports", key, "cfd"], queryFn: () => reports.cfd(key) });
  const pie = useQuery({ queryKey: ["reports", key, "pie", pieField], queryFn: () => reports.pie(key, pieField) });
  const cvr = useQuery({ queryKey: ["reports", key, "cvr"], queryFn: () => reports.createdVsResolved(key, 14) });

  return (
    <div className="mx-auto max-w-3xl p-6">
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36]">Reports — {key}</h1>

      <Card title="Burndown">
        {burndown.data ? (
          <LineChart
            labels={burndown.data.labels}
            series={[
              { name: "Ideal", color: "#8993a4", values: burndown.data.ideal },
              { name: "Actual", color: "#0052cc", values: burndown.data.actual },
            ]}
          />
        ) : <p className="text-sm text-slate-400">No active sprint</p>}
      </Card>

      <Card title="Velocity">
        {velocity.data && velocity.data.sprints.length > 0 ? (
          <BarChart bars={velocity.data.sprints.map((s) => ({ label: s.sprint_name, value: s.completed }))} />
        ) : <p className="text-sm text-slate-400">No completed sprints</p>}
      </Card>

      <Card title="Cumulative Flow">
        {cfd.data ? <StackedAreaChart dates={cfd.data.dates} categories={cfd.data.categories} data={cfd.data.data} /> : null}
      </Card>

      <Card title="Created vs Resolved (14d)">
        {cvr.data ? (
          <LineChart
            labels={cvr.data.dates}
            series={[
              { name: "Created", color: "#de350b", values: cvr.data.created },
              { name: "Resolved", color: "#00875a", values: cvr.data.resolved },
            ]}
          />
        ) : null}
      </Card>

      <Card title="Breakdown">
        <div className="mb-3">
          <select aria-label="Pie field" value={pieField} onChange={(e) => setPieField(e.target.value)} className="rounded border border-slate-300 px-2 py-1 text-sm">
            <option value="status">Status</option>
            <option value="priority">Priority</option>
            <option value="assignee">Assignee</option>
            <option value="type">Type</option>
          </select>
        </div>
        {pie.data ? <PieChart slices={pie.data} /> : null}
      </Card>
    </div>
  );
}
```

> **Nota implementatore:** verificare i JSON key reali (`labels`/`ideal`/`actual`, `sprints[].sprint_name`/`completed`, `categories`/`dates`/`data`) e adeguare. Il burndown usa il primo sprint della board `1` (demo): se il progetto non ha board/sprint, mostra "No active sprint". Aggiungere un link "Reports" dove sensato (es. dalla pagina progetto o board) — opzionale. Verificare che il route dinamico `[key]` legga i params con `use(params)` come le altre pagine.

- [ ] **Step 2: Type-check + build**

Run: `cd frontend-next && npx tsc --noEmit && npm run build`
Expected: build OK; route `/jira/projects/[key]/reports` generata.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/app/jira/projects/
git commit -m "feat(frontend): project reports page (burndown/velocity/cfd/pie/created-vs-resolved)"
```

---

### Task 8: Frontend — sezione Summary di progetto

**Files:**
- Create: `frontend-next/components/projects/ProjectSummary.tsx`
- Modify: la pagina progetto o settings per linkare/mostrare il Summary

- [ ] **Step 1: Componente Summary**

`frontend-next/components/projects/ProjectSummary.tsx`:

```tsx
"use client";

import { useQuery } from "@tanstack/react-query";
import { reports } from "@/lib/api";

export function ProjectSummary({ projectKey }: { projectKey: string }) {
  const summary = useQuery({ queryKey: ["summary", projectKey], queryFn: () => reports.summary(projectKey) });
  const s = summary.data;
  return (
    <div className="space-y-4" data-testid="project-summary">
      <div className="grid grid-cols-3 gap-3">
        <Stat label="Created (7d)" value={s?.created_last_7_days ?? 0} />
        <Stat label="Updated (7d)" value={s?.updated_last_7_days ?? 0} />
        <Stat label="Completed (7d)" value={s?.completed_last_7_days ?? 0} />
      </div>
      <div>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">Issues by status</h3>
        <ul className="text-sm" data-testid="summary-status-counts">
          {Object.entries(s?.issue_count_by_status ?? {}).map(([name, n]) => (
            <li key={name} className="flex justify-between border-b border-slate-100 py-1">
              <span className="text-[#1a1f36]">{name}</span>
              <span className="text-slate-500">{n}</span>
            </li>
          ))}
        </ul>
      </div>
      {s?.active_sprint && <p className="text-sm text-slate-600">Active sprint: <strong>{s.active_sprint.name}</strong></p>}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded border border-slate-200 bg-white p-3 text-center">
      <div className="text-2xl font-semibold text-[#1a1f36]">{value}</div>
      <div className="text-xs text-slate-500">{label}</div>
    </div>
  );
}
```

- [ ] **Step 2: Aggiungere un tab "Summary" a ProjectSettings (o una sezione nella pagina progetto)**

Il modo più semplice e coerente col Round 6 (che ha aggiunto il tab Workflow): aggiungere un terzo tab "Summary" in `frontend-next/components/projects/ProjectSettings.tsx` che rende `<ProjectSummary projectKey={projectKey} />`, e un link "Reports" verso `/jira/projects/{key}/reports`. Mantenere General/Workflow intatti.

```tsx
// stato tab esteso: "general" | "workflow" | "summary"
// bottone tab "Summary" + {tab === "summary" && <ProjectSummary projectKey={projectKey} />}
// e nel tab summary un link: <a href={`/jira/projects/${projectKey}/reports`} className="text-[#0052cc] hover:underline text-sm">Open reports →</a>
```

> **Nota implementatore:** leggere `ProjectSettings.tsx` (ha già `tab: "general" | "workflow"` dal Round 6) ed estendere l'union a `"summary"`, aggiungere il bottone tab e il blocco condizionale. Importare `ProjectSummary` e (se lo aggiungi) niente altro. Non rompere i tab esistenti.

- [ ] **Step 3: Type-check + build**

Run: `cd frontend-next && npx tsc --noEmit && npm run build`
Expected: build OK.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/components/projects/
git commit -m "feat(frontend): project summary tab (status counts, recent activity, active sprint)"
```

---

### Task 9: Frontend — pagina Dashboards con gadget

**Files:**
- Create: `frontend-next/app/jira/dashboards/page.tsx`
- Create: `frontend-next/app/jira/dashboards/[id]/page.tsx`

- [ ] **Step 1: Lista dashboard**

`frontend-next/app/jira/dashboards/page.tsx`:

```tsx
"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { dashboards } from "@/lib/api";

export default function DashboardsPage() {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const list = useQuery({ queryKey: ["dashboards"], queryFn: dashboards.list });
  const create = useMutation({
    mutationFn: (n: string) => dashboards.create(n),
    onSuccess: () => { setName(""); qc.invalidateQueries({ queryKey: ["dashboards"] }); },
  });
  return (
    <div className="mx-auto max-w-3xl p-6">
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36]">Dashboards</h1>
      <div className="mb-4 flex gap-2">
        <input aria-label="New dashboard name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Dashboard name" className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm" />
        <button onClick={() => name && create.mutate(name)} className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60" disabled={create.isPending}>Create</button>
      </div>
      <ul className="space-y-1" data-testid="dashboards-list">
        {list.data?.map((d) => (
          <li key={d.id}>
            <a href={`/jira/dashboards/${d.id}`} className="text-[#0052cc] hover:underline">{d.name}</a>
          </li>
        ))}
        {list.data && list.data.length === 0 && <li className="text-sm text-slate-400">No dashboards yet</li>}
      </ul>
    </div>
  );
}
```

- [ ] **Step 2: Vista dashboard con gadget**

`frontend-next/app/jira/dashboards/[id]/page.tsx`:

```tsx
"use client";

import { use } from "react";
import { useQuery } from "@tanstack/react-query";
import { dashboards } from "@/lib/api";

export default function DashboardView({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const dash = useQuery({ queryKey: ["dashboard", id], queryFn: () => dashboards.get(id) });
  const d = dash.data;
  return (
    <div className="mx-auto max-w-4xl p-6">
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36]">{d?.name ?? "Dashboard"}</h1>
      <div className="grid grid-cols-2 gap-4" data-testid="dashboard-gadgets">
        {(d?.widgets ?? []).map((wgt) => (
          <section key={wgt.id} className="rounded border border-slate-200 bg-white p-4">
            <h2 className="mb-2 text-sm font-semibold text-slate-700">{wgt.widget_type}</h2>
            <pre className="max-h-48 overflow-auto text-xs text-slate-600">{JSON.stringify(wgt.data ?? {}, null, 2)}</pre>
          </section>
        ))}
        {(d?.widgets ?? []).length === 0 && <p className="text-sm text-slate-400">No gadgets</p>}
      </div>
    </div>
  );
}
```

> **Nota implementatore (CRITICO):** verificare lo shape REALE di `dashboards.get(id)` — quali chiavi ha (widgets? gadgets?), e come i widget espongono i dati computati (`data`? `assigned_issues`?). LEGGERE `dashboard_handler.go` `Get` e `dashboard/service.go` per i tipi widget (`assigned_to_me`, `activity_stream`) e adeguare il rendering (il `<pre>` JSON è un fallback generico onesto; se i dati hanno una forma nota — es. lista issue assegnate — renderla come lista). Adeguare i tipi in lib/api.ts di conseguenza. Se la lista dashboard è a `/dashboard/search` invece di `/dashboards`, adeguare il client.

- [ ] **Step 3: Type-check + build**

Run: `cd frontend-next && npx tsc --noEmit && npm run build`
Expected: build OK; route `/jira/dashboards` e `/jira/dashboards/[id]` generate. La voce sidebar `/jira/dashboards` ora punta a una pagina reale.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/app/jira/dashboards/
git commit -m "feat(frontend): dashboards list and gadget view"
```

---

### Task 10: E2E — report e dashboard

**Files:**
- Create: `frontend-next/e2e/reports.spec.ts`

- [ ] **Step 1: Scrivere l'E2E**

`frontend-next/e2e/reports.spec.ts` (login helper reale da `board.spec.ts`):

```ts
import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/jira/);
}

test("project reports page renders charts", async ({ page }) => {
  await login(page);
  await page.goto("/jira/projects/DEMO/reports");
  await expect(page.getByRole("heading", { name: /Reports/i })).toBeVisible();
  // almeno il grafico velocity o CFD o la torta rende un SVG con testid
  await expect(page.getByTestId("pie-chart")).toBeVisible();
  // cambia il campo della torta
  await page.getByLabel("Pie field").selectOption("priority");
  await expect(page.getByTestId("pie-chart")).toBeVisible();
});

test("dashboards page lists and creates a dashboard", async ({ page }) => {
  await login(page);
  await page.goto("/jira/dashboards");
  await expect(page.getByRole("heading", { name: /Dashboards/i })).toBeVisible();
  await page.getByLabel("New dashboard name").fill("E2E Dashboard");
  await page.getByRole("button", { name: "Create" }).click();
  await expect(page.getByRole("link", { name: "E2E Dashboard" })).toBeVisible();
});
```

> **Nota:** il progetto DEMO ha issue con status (dal seed R5/R6) → la torta per status ha dati. Se `pie-chart` non rende per assenza dati, verificare che il seed dia issue con status (lo fa). Adeguare i selettori se necessario.

- [ ] **Step 2: Eseguire l'E2E**

Run: `cd frontend-next && npx playwright test e2e/reports.spec.ts --reporter=line`
Expected: 2 PASS.

- [ ] **Step 3: Suite completa**

Run: `cd frontend-next && npx playwright test --reporter=line`
Expected: tutti verdi (login, projects, issues, collaboration, search, board, workflow, reports). Pulire `test-results/`/`playwright-report/` e i processi 8080/3000.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/e2e/reports.spec.ts
git commit -m "test(e2e): reports charts render and dashboard create"
```

---

### Task 11: Seed dashboard demo + gap report

**Files:**
- Modify: `cmd/seed/main.go`
- Modify: `docs/contracts/gap-report.md` (rigenerato)

- [ ] **Step 1: Seed dashboard demo (idempotente)**

In `cmd/seed/main.go`, dopo il seed dello sprint, aggiungere una dashboard demo idempotente con un widget `assigned_to_me` (usando `dashboard.NewService(s.DB)` — VERIFICARE la firma di `CreateDashboard`/`AddWidget` leggendo `internal/domain/dashboard/service.go`). Check-esistenza per nome+owner. Stampa "created demo dashboard". Esempio (adeguare alle firme reali):

```go
	dashSvc := dashboard.NewService(s.DB)
	var existingD dashboard.Dashboard
	err = s.DB.Where("owner_id = ? AND name = ?", admin.ID, "My Dashboard").First(&existingD).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		d, cerr := dashSvc.CreateDashboard(admin.ID, "My Dashboard", false)
		if cerr != nil {
			log.Fatalf("seed dashboard: %v", cerr)
		}
		if _, werr := dashSvc.AddWidget(d.ID, "assigned_to_me", "{}", "{}"); werr != nil {
			log.Fatalf("seed widget: %v", werr)
		}
		fmt.Println("created demo dashboard")
	} else if err != nil {
		log.Fatalf("check demo dashboard: %v", err)
	}
```
(Import `dashboard "github.com/open-jira/open-jira/internal/domain/dashboard"`.)

- [ ] **Step 2: Verificare idempotenza**

Run: `rm -f /tmp/s11.db && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/s11.db go run ./cmd/seed && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/s11.db go run ./cmd/seed && rm -f /tmp/s11.db`
Expected: prima run "created demo dashboard"; seconda no; entrambe exit 0.

- [ ] **Step 3: Rigenerare gap report**

Run: `go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md`
Expected: eventuali nuove rotte report/dashboard riflesse (i report sono estensioni custom, quindi potrebbero comparire come "extra", non "matched" — riportare comunque il conteggio).

- [ ] **Step 4: Commit**

```bash
git add cmd/seed/main.go docs/contracts/gap-report.md
git commit -m "feat(seed): demo dashboard with widget; regenerate gap report for Round 7"
```

---

### Task 12: Gate finale + STATE.md → Round 8

**Files:**
- Modify: `docs/superpowers/STATE.md`

- [ ] **Step 1: Gate a tre livelli**

Run:
```bash
go build ./... && echo BUILD_OK
go vet ./... && echo VET_OK
go test ./... 2>&1 | grep -vE '^ok|no test files'
cd frontend-next && npx tsc --noEmit && npm run build && npx playwright test --reporter=line; cd ..
```
Expected: BUILD_OK, VET_OK, nessun FAIL Go, frontend build OK, tutti gli E2E verdi.

- [ ] **Step 2: Gap report senza drift**

Run: `go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md`
Expected: nessun drift inatteso.

- [ ] **Step 3: Aggiornare STATE.md**

In `docs/superpowers/STATE.md`:
- aggiungere alla sezione "Round completati" la riga del **Round 7 — Viste & Report** (fix storico report — burndown/CFD leggono `field_name='status'` con join sullo stato storico; report pie-by-field e created-vs-resolved; test report; frontend: grafici SVG dependency-free, pagina Report progetto, tab Summary, pagina Dashboards con gadget);
- cambiare "Prossimo" in **Round 8 — Utenti & permessi** (gruppi, ruoli progetto, permission scheme, profilo utente, notifiche in-app + email);
- aggiornare data e conteggio gap;
- aggiungere ai follow-up: **Timeline/Gantt** e **Calendar** (rinviati da R7); logging dei cambi `sprint_id` nello storico (per burndown con scope-change); gadget dashboard configurabili + report-gadget; export report; scelta libreria grafici professionale.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/STATE.md docs/contracts/gap-report.md
git commit -m "docs: mark Round 7 (Views & Reports) complete, Round 8 (Users & Permissions) next"
```

---

## Note di chiusura round

- **Follow-up:** Timeline/Gantt; Calendar (issue per due date); log dei cambi `sprint_id` nello storico (burndown con issue aggiunte/rimosse a metà sprint); gadget dashboard configurabili (report-gadget, filter-results-gadget) con `moduleKey`/`uri` conformi v3; export report (CSV/PDF); valutare una libreria grafici (recharts con React 19) se le primitive SVG diventano insufficienti; instradare i vecchi handler report/dashboard attraverso `v3.WriteJSON`/`WriteError` per coerenza.
- **Rischi noti:** i report sono **estensioni custom** (non nel contratto v3) — la conformità v3 riguarda solo le dashboard/gadget dove lo shape combacia; le funzioni `DATE()` nelle query sono compatibili SQLite/Postgres ma vanno verificate su MariaDB in un round di hardening. Il round chiude solo con i tre livelli verdi.

---

## Self-Review (svolta in fase di scrittura)

**1. Copertura spec (roadmap Round 7):**
- Burndown/Velocity/CFD → backend esistente **corretto** (T1/T2) + UI (T7). ✅
- Report torta (pie) e created-vs-resolved → T3/T4 (backend) + T7 (UI). ✅
- Summary progetto → endpoint esistente + T8 (UI). ✅
- Dashboard con gadget → backend esistente + T9 (UI). ✅
- Timeline/Gantt, Calendar → **esplicitamente rinviati** a follow-up (documentato). ⚠️ (scoping deliberato)
- Gate: test report (T1-T4), E2E (T10), gate (T12). ✅

**2. Placeholder scan:** codice completo in ogni task. Le "Note implementatore" segnalano le verifiche necessarie su firme/shape reali (tipi Go dei report/dashboard, json key, path lista dashboard, shape widget, tabelle `users.display_name`/`issue_types`) con i file da leggere — non placeholder di logica. Il bug dello storico è descritto con precisione (righe e stringhe esatte) e guidato da test.

**3. Consistenza tipi:** `report.Service.GetPieByField(projectID, field)([]PieSlice{Label,Count})` (T3) usato in handler T3 + client `reports.pie` (T6) + UI T7. `GetCreatedVsResolved(projectID, days)(CreatedVsResolvedData{Dates,Created,Resolved})` (T4) → client T6 → UI T7. Le primitive `LineChart{labels,series[]}`, `BarChart{bars[]}`, `PieChart{slices[]}`, `StackedAreaChart{dates,categories,data}` (T5) usate in T7. `reports`/`dashboards` client (T6) usati in T7/T8/T9/T10. Le fix T1/T2 non cambiano firme pubbliche (solo query interne) → nessun impatto a valle. I data-testid (`pie-chart`, `dashboards-list`, `project-summary`, `line-chart`, `bar-chart`, `cfd-chart`) sono usati dall'E2E T10.
