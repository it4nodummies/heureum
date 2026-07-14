# Round 6 — Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dare a open-jira workflow per progetto con stati/categorie e transizioni (con regole base: validator + post-function), transizioni issue conformi a Jira Cloud REST API v3, endpoint status-category, e un editor workflow nelle impostazioni progetto.

**Architecture:** Si estende il dominio `workflow` esistente (stati + transizioni from→to) aggiungendo alle transizioni un `Name` e due regole base editabili (`RequireAssignee` = validator, `SetResolution` = post-function). La parte drop-in v3 sono le transizioni a livello issue (`GET /issue/{id}/transitions` → `Transitions`/`IssueTransition`, `POST` nella shape Jira `{transition:{id}}` mantenendo l'estensione `{status_id}` usata dalla board) più `GET /statuscategory[/{idOrKey}]`. L'editing del workflow per-progetto resta sulle rotte custom `/rest/api/3/project/{key}/workflow/*` (estensioni). Post-function: sulle transizioni con `SetResolution` la resolution viene settata/azzerata in base alla categoria dello stato di destinazione (done). Frontend: un tab "Workflow" nella pagina impostazioni progetto.

**Tech Stack:** Go 1.25 (net/http ServeMux, GORM, golang-migrate, SQLite in test), dominio `internal/domain/{workflow,issue,project}`, layer `internal/api/v3`, harness `internal/contract`. Frontend Next.js 16 + React 19 + TanStack Query + Tailwind + Playwright.

---

## Contesto per l'implementatore (leggere una volta)

**Contratto ufficiale** in `docs/contracts/jira-platform-v3.json`. Schemi rilevanti (proprietà ESATTE):

- `GET /rest/api/3/issue/{issueIdOrKey}/transitions` → **`Transitions`** `{ expand:string, transitions:[IssueTransition] }`.
- **`IssueTransition`**: `id:string`, `name:string`, `to:StatusDetails` (readOnly), `hasScreen:bool`, `isGlobal:bool`, `isInitial:bool`, `isAvailable:bool`, `isConditional:bool`, `looped:bool`, `fields:object`, `expand:string`.
- `POST /rest/api/3/issue/{issueIdOrKey}/transitions` body **`IssueUpdateDetails`** `{ transition:IssueTransition, fields:object, update:object, historyMetadata, properties:[] }` → **204** (nessun body).
- **`StatusDetails`** (il `to` della transizione, e `GET /status`): `id:string`, `name:string`, `description:string`, `iconUrl:string`, `self:string`, `statusCategory:StatusCategory`.
- **`StatusCategory`**: `id:integer`, `key:string`, `colorName:string`, `name:string`, `self:string`. `GET /rest/api/3/statuscategory` → `[StatusCategory]`; `GET /rest/api/3/statuscategory/{idOrKey}` → `StatusCategory`.

**Codice ESISTENTE da riusare (verificato):**
- `internal/domain/workflow/model.go`: `StatusCategory` string enum con costanti `CategoryTodo="todo"`, `CategoryInProgress="inprogress"`, `CategoryDone="done"`. `Workflow{ID, ProjectID, Name, Statuses []WorkflowStatus, Transitions []WorkflowTransition, CreatedAt}`. `WorkflowStatus{ID, WorkflowID, Name, Category StatusCategory, Color, Position int}`. `WorkflowTransition{ID, WorkflowID, FromStatusID, ToStatusID}` (**solo edge from→to, nessuna regola** — Round 6 aggiunge Name/RequireAssignee/SetResolution).
- `internal/domain/workflow/service.go`: `NewService(db)`, `GetWorkflow(projectID)(*Workflow,error)` (preload Statuses+Transitions), `AddStatus(workflowID, name string, category StatusCategory, color string)(*WorkflowStatus,error)`, `UpdateStatus(statusID, name, category, color)`, `RemoveStatus(statusID)`, `AddTransition(workflowID, fromStatusID, toStatusID string)(*WorkflowTransition,error)`, `RemoveTransition(transitionID)`, `GetTransitions(workflowID)([]WorkflowTransition,error)`, `ValidateTransition(workflowID, fromStatusID, toStatusID string) error` (conta edge esatti >0), `ListAllStatuses`, `GetStatus(idOrName)`, `CreateDefaultWorkflow(projectID)` (seed TO DO/IN PROGRESS/DONE + edge todo↔inprog, inprog→done).
- `internal/domain/issue/service.go`: `Update(key string, title, descriptionJSON *string, priority *Priority, assigneeID, statusID *string, storyPoints *int)(*Issue,error)` — **NON gestisce resolution_id** (Round 6 aggiunge `SetResolution`). `GetByKey(key)`, `GetBySeqID(n)`, `DB() *gorm.DB`. `issue.Issue{StatusID *string, AssigneeID *string, ResolutionID *string(resolution_id), ProjectID, Key}`.
- `internal/api/v3/reference.go`: `StatusCategoryRef{Self, ID int, Key, ColorName, Name}`, `StatusRef{Self, ID string, Name, Description omitempty, IconURL omitempty, StatusCategory StatusCategoryRef}`, `categoryFor(internal string, baseURL string) StatusCategoryRef` (done→{3,done,green,Done}; inprogress→{4,indeterminate,yellow,In Progress}; default todo→{2,new,blue-gray,To Do}), `JiraStatus(id, name, internalCategory, baseURL string) StatusRef`.
- `internal/api/handlers/workflow_handler.go`: `WorkflowHandler{wfSvc, issueSvc, projectSvc}` (costruito router.go:54 `NewWorkflowHandler(wfSvc, issueSvc, projectSvc)`). Metodi attuali: `GetWorkflow`, `AddStatus` (body `{name,category,color}`→201), `UpdateStatus` (PATCH), `DeleteStatus`, `AddTransition` (body `{from_status_id,to_status_id}`), `TransitionIssue` (body `{status_id}` → valida via ValidateTransition, Update, ritorna issue JSON), `GetStatus`, `SearchWorkflows`, e un `ListStatuses` **dead** (non montato). `TransitionIssue` va **riscritto** (Task 6). Le rotte custom `/project/{key}/workflow/*` restano.
- `internal/api/handlers/reference_handler.go`: `ReferenceHandler{db, baseURL}` `NewReferenceHandler(db, baseURL)`; `Statuses` serve `GET /status` (de-dup per Name → `[]v3.StatusRef`, fallback default). **Nessuna rotta `/statuscategory`** (Task 5 la aggiunge).
- Router `internal/api/router.go`: `mux`, `authMw`, `cfg.BaseURL`, variabili `wfSvc`, `issueSvc`, `projectSvc`, `refH`, `issueH`. Rotta esistente `POST /rest/api/3/issue/{issueKey}/transitions` → `wfH.TransitionIssue` (router.go:209). `GET /rest/api/3/status` → `refH.Statuses`, `GET /rest/api/3/status/{idOrName}` → `wfH.GetStatus`.
- Board (Round 5) dnd: `POST /rest/api/3/issue/{key}/transitions` con `{status_id: toStatusId}` (`frontend-next/lib/api.ts` `issues.transition`, `app/jira/boards/[boardId]/page.tsx`). **Il POST riscritto DEVE continuare ad accettare `{status_id}`** o la board si rompe.

**Migrazioni:** ultima `000012_boards`. Prossima **`000013`**.

**Scelte di scope (esplicite, per contenere il round):**
- **Regole base** = un **validator** (`require_assignee`) + una **post-function** (`set_resolution`: su transizione con questo flag, se lo stato destinazione è categoria `done` setta la resolution, altrimenti la azzera). **Condizioni** (filtro `isAvailable` in GET) → follow-up: per ora tutte le transizioni dallo stato corrente sono `isAvailable:true`.
- Il workflow **CRUD conforme v3** è la shape bulk nuova (`/workflows/create`, `statusReference`, arrays conditions/validators/actions) — **fuori scope** (follow-up). L'editing per-progetto resta sulle rotte custom `/project/{key}/workflow/*`.
- Nome transizione: se vuoto in GET, si deriva `"→ {ToStatus.Name}"`.

**Harness contract (`internal/contract/`):** `MustLoad(t, "../../docs/contracts/jira-platform-v3.json")`, `newTestServer(t)(*httptest.Server, *auth.Service)`, `registerAndLogin`, `createProjectViaAPI`, `createIssueViaAPI`, `Validator.ValidateResponse(method, path, status, header, bodyReader)`, e gli helper `doJSON`/`decodeBody` (vedi `internal/contract/search_test.go`/`agile_test.go`). **NOTA:** `createProjectViaAPI` crea il progetto via HTTP → l'handler HTTP crea il workflow di default (a differenza di `project.Service.Create`). Quindi le issue create nei test hanno un workflow con stati TO DO/IN PROGRESS/DONE.

---

## Struttura dei file

**Migrazioni:** `migrations/000013_workflow_transition_rules.up.sql` / `.down.sql`.

**Dominio:**
- `internal/domain/workflow/model.go` — `WorkflowTransition` + `Name string`, `RequireAssignee bool`, `SetResolution bool`.
- `internal/domain/workflow/service.go` — `AddTransition` firma estesa (name+flags); nuovi `UpdateTransition`, `GetTransitionByID`, `GetAvailableTransitions(workflowID, fromStatusID string)([]WorkflowTransition,error)`, `ReorderStatuses(workflowID string, orderedStatusIDs []string) error`.
- `internal/domain/issue/service.go` — `SetResolution(key string, resolutionID *string) error`; `ResolutionIDByName(name string)(string,bool)`.

**Layer v3:**
- `internal/api/v3/transitions.go` — `Transitions`, `IssueTransition`, `AgileTransition`... no: `TransitionInput`, `MakeTransition(...)`. Riusa `StatusRef`/`JiraStatus`.

**Handler:**
- `internal/api/handlers/reference_handler.go` — aggiungere `StatusCategories` (GET /statuscategory) e `StatusCategoryByID` (GET /statuscategory/{idOrKey}).
- `internal/api/handlers/workflow_handler.go` — riscrivere `TransitionIssue` → `AvailableTransitions` (GET) + `DoTransition` (POST conforme); estendere `AddTransition` (name+flags), aggiungere `UpdateTransition`, `DeleteTransition`, `ListTransitions`, `ReorderStatuses`.

**Router:** `internal/api/router.go` — `GET /issue/{id}/transitions`, ridirezione POST, `/statuscategory[/{idOrKey}]`, rotte editing transizioni + reorder.

**Frontend:**
- `frontend-next/lib/api.ts` — `workflow` client (getWorkflow, add/update/delete status, reorderStatuses, add/update/delete transition) + `issues.availableTransitions(key)`.
- `frontend-next/components/workflow/WorkflowEditor.tsx` — editor stati+transizioni.
- `frontend-next/components/projects/ProjectSettings.tsx` — aggiungere tab "Workflow".
- `frontend-next/e2e/workflow.spec.ts` — E2E.

**Seed:** `cmd/seed/main.go` — dare un nome alle transizioni demo (facoltativo) + gap report.

---

### Task 1: Migrazione 000013 — regole transizione

**Files:**
- Create: `migrations/000013_workflow_transition_rules.up.sql`
- Create: `migrations/000013_workflow_transition_rules.down.sql`

- [ ] **Step 1: Migrazione up**

`migrations/000013_workflow_transition_rules.up.sql`:

```sql
ALTER TABLE workflow_transitions ADD COLUMN name TEXT DEFAULT '';
ALTER TABLE workflow_transitions ADD COLUMN require_assignee BOOLEAN DEFAULT FALSE;
ALTER TABLE workflow_transitions ADD COLUMN set_resolution BOOLEAN DEFAULT FALSE;
```

- [ ] **Step 2: Migrazione down**

`migrations/000013_workflow_transition_rules.down.sql`:

```sql
ALTER TABLE workflow_transitions DROP COLUMN set_resolution;
ALTER TABLE workflow_transitions DROP COLUMN require_assignee;
ALTER TABLE workflow_transitions DROP COLUMN name;
```

- [ ] **Step 3: Verificare a pulito**

Run: `rm -f /tmp/mig13.db && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/mig13.db go run ./cmd/seed && rm -f /tmp/mig13.db`
Expected: `seed complete`, exit 0.

- [ ] **Step 4: Commit**

```bash
git add migrations/000013_workflow_transition_rules.up.sql migrations/000013_workflow_transition_rules.down.sql
git commit -m "feat(migrations): workflow transition name and base rule flags"
```

---

### Task 2: Dominio workflow — nome transizione + regole + helper

**Files:**
- Modify: `internal/domain/workflow/model.go`
- Modify: `internal/domain/workflow/service.go`
- Test: `internal/domain/workflow/transition_test.go`

- [ ] **Step 1: Estendere il modello**

In `internal/domain/workflow/model.go`, aggiungere a `WorkflowTransition` (dopo `ToStatusID`):

```go
	Name            string `gorm:"type:text;default:''" json:"name"`
	RequireAssignee bool   `gorm:"column:require_assignee;default:false" json:"require_assignee"`
	SetResolution   bool   `gorm:"column:set_resolution;default:false" json:"set_resolution"`
```

- [ ] **Step 2: Scrivere i test**

`internal/domain/workflow/transition_test.go`:

```go
package workflow

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWFDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&Workflow{}, &WorkflowStatus{}, &WorkflowTransition{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestAddTransition_WithNameAndRules(t *testing.T) {
	db := newWFDB(t)
	svc := NewService(db)
	wf, _ := svc.CreateWorkflow("proj-1", "WF")
	todo, _ := svc.AddStatus(wf.ID, "To Do", CategoryTodo, "#111")
	done, _ := svc.AddStatus(wf.ID, "Done", CategoryDone, "#222")

	tr, err := svc.AddTransition(wf.ID, todo.ID, done.ID, "Resolve", true, true)
	if err != nil {
		t.Fatalf("AddTransition: %v", err)
	}
	if tr.Name != "Resolve" || !tr.RequireAssignee || !tr.SetResolution {
		t.Errorf("campi transizione errati: %+v", tr)
	}
}

func TestGetTransitionByID(t *testing.T) {
	db := newWFDB(t)
	svc := NewService(db)
	wf, _ := svc.CreateWorkflow("proj-1", "WF")
	a, _ := svc.AddStatus(wf.ID, "A", CategoryTodo, "#111")
	b, _ := svc.AddStatus(wf.ID, "B", CategoryInProgress, "#222")
	tr, _ := svc.AddTransition(wf.ID, a.ID, b.ID, "Go", false, false)

	got, err := svc.GetTransitionByID(tr.ID)
	if err != nil {
		t.Fatalf("GetTransitionByID: %v", err)
	}
	if got.ToStatusID != b.ID {
		t.Errorf("toStatus errato: %+v", got)
	}
	if _, err := svc.GetTransitionByID("nope"); err == nil {
		t.Error("atteso errore per id inesistente")
	}
}

func TestGetAvailableTransitions(t *testing.T) {
	db := newWFDB(t)
	svc := NewService(db)
	wf, _ := svc.CreateWorkflow("proj-1", "WF")
	a, _ := svc.AddStatus(wf.ID, "A", CategoryTodo, "#111")
	b, _ := svc.AddStatus(wf.ID, "B", CategoryInProgress, "#222")
	c, _ := svc.AddStatus(wf.ID, "C", CategoryDone, "#333")
	svc.AddTransition(wf.ID, a.ID, b.ID, "A→B", false, false)
	svc.AddTransition(wf.ID, a.ID, c.ID, "A→C", false, false)
	svc.AddTransition(wf.ID, b.ID, c.ID, "B→C", false, false)

	avail, err := svc.GetAvailableTransitions(wf.ID, a.ID)
	if err != nil {
		t.Fatalf("GetAvailableTransitions: %v", err)
	}
	if len(avail) != 2 {
		t.Errorf("attese 2 transizioni da A, got %d", len(avail))
	}
}

func TestReorderStatuses(t *testing.T) {
	db := newWFDB(t)
	svc := NewService(db)
	wf, _ := svc.CreateWorkflow("proj-1", "WF")
	a, _ := svc.AddStatus(wf.ID, "A", CategoryTodo, "#111")
	b, _ := svc.AddStatus(wf.ID, "B", CategoryInProgress, "#222")
	c, _ := svc.AddStatus(wf.ID, "C", CategoryDone, "#333")

	if err := svc.ReorderStatuses(wf.ID, []string{c.ID, a.ID, b.ID}); err != nil {
		t.Fatalf("ReorderStatuses: %v", err)
	}
	wf2, _ := svc.GetWorkflow("proj-1")
	// GetWorkflow preload ordina per position? verifichiamo le position assegnate
	posByID := map[string]int{}
	for _, st := range wf2.Statuses {
		posByID[st.ID] = st.Position
	}
	if !(posByID[c.ID] < posByID[a.ID] && posByID[a.ID] < posByID[b.ID]) {
		t.Errorf("ordine posizioni errato: %v", posByID)
	}
}
```

- [ ] **Step 3: Eseguire i test (falliscono)**

Run: `go test ./internal/domain/workflow/ -run 'TestAddTransition_WithNameAndRules|TestGetTransitionByID|TestGetAvailableTransitions|TestReorderStatuses' -v`
Expected: FAIL (firma `AddTransition` a 3 arg, metodi mancanti).

- [ ] **Step 4: Estendere il service**

In `internal/domain/workflow/service.go`:

(a) cambiare la firma di `AddTransition` (e adeguare i chiamanti interni — `CreateDefaultWorkflow` la usa con 3 arg):

```go
// AddTransition crea una transizione from→to con nome e regole base.
func (s *Service) AddTransition(workflowID, fromStatusID, toStatusID, name string, requireAssignee, setResolution bool) (*WorkflowTransition, error) {
	tr := &WorkflowTransition{
		ID:              uuid.NewString(),
		WorkflowID:      workflowID,
		FromStatusID:    fromStatusID,
		ToStatusID:      toStatusID,
		Name:            name,
		RequireAssignee: requireAssignee,
		SetResolution:   setResolution,
	}
	if err := s.db.Create(tr).Error; err != nil {
		return nil, err
	}
	return tr, nil
}
```

In `CreateDefaultWorkflow`, aggiornare le chiamate `AddTransition(wf.ID, todo.ID, inprog.ID)` → `AddTransition(wf.ID, todo.ID, inprog.ID, "Start Progress", false, false)`, `inprog→todo` → `"Stop Progress"`, `inprog→done` → `"Done"` (con `setResolution=true` sull'ultima, così la demo mostra la post-function): `AddTransition(wf.ID, inprog.ID, done.ID, "Done", false, true)`. (Adeguare i nomi delle variabili reali dello stato in CreateDefaultWorkflow.)

(b) aggiungere i metodi:

```go
// GetTransitionByID carica una singola transizione.
func (s *Service) GetTransitionByID(id string) (*WorkflowTransition, error) {
	var tr WorkflowTransition
	if err := s.db.Where("id = ?", id).First(&tr).Error; err != nil {
		return nil, err
	}
	return &tr, nil
}

// GetAvailableTransitions restituisce le transizioni uscenti da fromStatusID.
func (s *Service) GetAvailableTransitions(workflowID, fromStatusID string) ([]WorkflowTransition, error) {
	var trs []WorkflowTransition
	if err := s.db.Where("workflow_id = ? AND from_status_id = ?", workflowID, fromStatusID).Find(&trs).Error; err != nil {
		return nil, err
	}
	return trs, nil
}

// UpdateTransition aggiorna nome e regole di una transizione (puntatori nil = invariato).
func (s *Service) UpdateTransition(id string, name *string, requireAssignee, setResolution *bool) (*WorkflowTransition, error) {
	updates := map[string]any{}
	if name != nil {
		updates["name"] = *name
	}
	if requireAssignee != nil {
		updates["require_assignee"] = *requireAssignee
	}
	if setResolution != nil {
		updates["set_resolution"] = *setResolution
	}
	if len(updates) > 0 {
		if err := s.db.Model(&WorkflowTransition{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.GetTransitionByID(id)
}

// ReorderStatuses riassegna la position degli stati secondo l'ordine dato.
func (s *Service) ReorderStatuses(workflowID string, orderedStatusIDs []string) error {
	for i, id := range orderedStatusIDs {
		if err := s.db.Model(&WorkflowStatus{}).Where("workflow_id = ? AND id = ?", workflowID, id).Update("position", i).Error; err != nil {
			return err
		}
	}
	return nil
}
```

> **Nota implementatore:** verificare che `GetWorkflow` preload gli stati ordinati per `position` (se non lo fa, il test ReorderStatuses controlla comunque le position numeriche, non l'ordine dello slice — ok). Verificare l'import `uuid` (già usato altrove nel service). Cercare TUTTI i chiamanti di `AddTransition` (`grep -rn "AddTransition(" internal/ cmd/`) e adeguarli alla nuova firma — includono `CreateDefaultWorkflow` e l'handler `AddTransition` (Task 7 lo aggiorna, ma se rompe il build ora, aggiorna la chiamata handler a passare `"", false, false` temporaneamente e completala in Task 7).

- [ ] **Step 5: Eseguire i test (passano)**

Run: `go test ./internal/domain/workflow/ -v`
Expected: PASS (nuovi + esistenti).

- [ ] **Step 6: Commit**

```bash
git add internal/domain/workflow/
git commit -m "feat(workflow): transition name, base rules, available/reorder helpers"
```

---

### Task 3: Issue — setter resolution (post-function)

**Files:**
- Modify: `internal/domain/issue/service.go`
- Test: `internal/domain/issue/resolution_test.go`

- [ ] **Step 1: Scrivere i test**

`internal/domain/issue/resolution_test.go`:

```go
package issue

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func resDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&Issue{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestSetResolution(t *testing.T) {
	db := resDB(t)
	svc := NewService(db)
	iss := &Issue{ID: "i1", ProjectID: "p", Key: "P-1", Title: "x", SeqID: 1}
	db.Create(iss)

	res := "res-done"
	if err := svc.SetResolution("P-1", &res); err != nil {
		t.Fatalf("SetResolution: %v", err)
	}
	var got Issue
	db.First(&got, "id = ?", "i1")
	if got.ResolutionID == nil || *got.ResolutionID != "res-done" {
		t.Errorf("resolution non settata: %v", got.ResolutionID)
	}

	// clear
	if err := svc.SetResolution("P-1", nil); err != nil {
		t.Fatalf("SetResolution clear: %v", err)
	}
	db.First(&got, "id = ?", "i1")
	if got.ResolutionID != nil {
		t.Errorf("resolution doveva essere azzerata, got %v", *got.ResolutionID)
	}
}
```

- [ ] **Step 2: Eseguire i test (falliscono)**

Run: `go test ./internal/domain/issue/ -run TestSetResolution -v`
Expected: FAIL con "svc.SetResolution undefined".

- [ ] **Step 3: Implementare**

Aggiungere in `internal/domain/issue/service.go`:

```go
// SetResolution imposta (o azzera, se resolutionID è nil) la resolution di una issue.
func (s *Service) SetResolution(key string, resolutionID *string) error {
	iss, err := s.GetByKey(key)
	if err != nil {
		return err
	}
	// Update esplicito su colonna: nil → NULL, valore → id.
	return s.db.Model(&Issue{}).Where("id = ?", iss.ID).Update("resolution_id", resolutionID).Error
}

// ResolutionIDByName restituisce l'id della resolution con quel nome (case-insensitive).
func (s *Service) ResolutionIDByName(name string) (string, bool) {
	var row struct{ ID string }
	err := s.db.Table("resolutions").Select("id").Where("LOWER(name) = LOWER(?)", name).Limit(1).Scan(&row).Error
	if err != nil || row.ID == "" {
		return "", false
	}
	return row.ID, true
}
```

- [ ] **Step 4: Eseguire i test (passano)**

Run: `go test ./internal/domain/issue/ -run TestSetResolution -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/issue/service.go internal/domain/issue/resolution_test.go
git commit -m "feat(issue): SetResolution and ResolutionIDByName for transition post-function"
```

---

### Task 4: v3 — mapper transizioni

**Files:**
- Create: `internal/api/v3/transitions.go`
- Test: `internal/api/v3/transitions_test.go`

- [ ] **Step 1: Scrivere i test**

`internal/api/v3/transitions_test.go`:

```go
package v3

import "testing"

func TestMakeTransition_Shape(t *testing.T) {
	tr := MakeTransition(TransitionInput{
		ID: "tr-1", Name: "Done", ToID: "st-done", ToName: "Done", ToCategory: "done",
		Available: true, BaseURL: "http://x",
	})
	if tr.ID != "tr-1" || tr.Name != "Done" {
		t.Errorf("campi base errati: %+v", tr)
	}
	if tr.To.ID != "st-done" || tr.To.Name != "Done" {
		t.Errorf("to errato: %+v", tr.To)
	}
	if tr.To.StatusCategory.Key != "done" {
		t.Errorf("statusCategory errata: %+v", tr.To.StatusCategory)
	}
	if !tr.IsAvailable {
		t.Error("isAvailable atteso true")
	}
	// campi booleani conformi presenti (default false)
	if tr.HasScreen || tr.IsGlobal || tr.IsInitial || tr.IsConditional || tr.Looped {
		t.Error("flag booleani non di default")
	}
}

func TestTransitions_Wrapper(t *testing.T) {
	ts := Transitions{Transitions: []IssueTransition{}}
	if ts.Transitions == nil {
		t.Error("transitions deve essere slice non-nil (anche vuoto)")
	}
}
```

- [ ] **Step 2: Eseguire i test (falliscono)**

Run: `go test ./internal/api/v3/ -run 'TestMakeTransition|TestTransitions_Wrapper' -v`
Expected: FAIL con "undefined: MakeTransition".

- [ ] **Step 3: Implementare**

`internal/api/v3/transitions.go`:

```go
package v3

// IssueTransition è lo shape del contratto per una transizione disponibile.
type IssueTransition struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	To            StatusRef `json:"to"`
	HasScreen     bool      `json:"hasScreen"`
	IsGlobal      bool      `json:"isGlobal"`
	IsInitial     bool      `json:"isInitial"`
	IsAvailable   bool      `json:"isAvailable"`
	IsConditional bool      `json:"isConditional"`
	Looped        bool      `json:"looped"`
}

// Transitions è la risposta di GET /issue/{id}/transitions.
type Transitions struct {
	Transitions []IssueTransition `json:"transitions"`
}

// TransitionInput porta i dati per costruire una IssueTransition.
type TransitionInput struct {
	ID, Name             string
	ToID, ToName         string
	ToCategory           string // categoria interna: todo/inprogress/done
	Available            bool
	BaseURL              string
}

// MakeTransition costruisce la IssueTransition conforme (to via JiraStatus).
func MakeTransition(in TransitionInput) IssueTransition {
	return IssueTransition{
		ID:          in.ID,
		Name:        in.Name,
		To:          JiraStatus(in.ToID, in.ToName, in.ToCategory, in.BaseURL),
		IsAvailable: in.Available,
	}
}
```

- [ ] **Step 4: Eseguire i test (passano)**

Run: `go test ./internal/api/v3/ -run 'TestMakeTransition|TestTransitions_Wrapper' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/v3/transitions.go internal/api/v3/transitions_test.go
git commit -m "feat(v3): issue transition mappers"
```

---

### Task 5: Status category — endpoint + rotte

**Files:**
- Modify: `internal/api/handlers/reference_handler.go`
- Modify: `internal/api/router.go`
- Test: (contract in Task 8; qui build + smoke)

- [ ] **Step 1: Aggiungere gli handler status-category**

In `internal/api/handlers/reference_handler.go` aggiungere (riusando `categoryFor` del package v3 se esportato, altrimenti costruendo le 3 categorie note):

```go
// allCategories restituisce le 3 status-category emesse dal sistema.
func (h *ReferenceHandler) allCategories() []v3.StatusCategoryRef {
	// categoryFor mappa la categoria interna → Jira. Usiamo le 3 interne note.
	return []v3.StatusCategoryRef{
		v3.CategoryFor("todo", h.baseURL),
		v3.CategoryFor("inprogress", h.baseURL),
		v3.CategoryFor("done", h.baseURL),
	}
}

// StatusCategories serve GET /rest/api/3/statuscategory.
func (h *ReferenceHandler) StatusCategories(w http.ResponseWriter, r *http.Request) {
	v3.WriteJSON(w, http.StatusOK, h.allCategories())
}

// StatusCategoryByID serve GET /rest/api/3/statuscategory/{idOrKey} (per id numerico o key).
func (h *ReferenceHandler) StatusCategoryByID(w http.ResponseWriter, r *http.Request) {
	idOrKey := r.PathValue("idOrKey")
	for _, c := range h.allCategories() {
		if idOrKey == c.Key || idOrKey == strconv.Itoa(c.ID) {
			v3.WriteJSON(w, http.StatusOK, c)
			return
		}
	}
	v3.WriteError(w, http.StatusNotFound, []string{"status category not found"}, nil)
}
```

> **Nota implementatore:** `categoryFor` in `internal/api/v3/reference.go` è **non esportato**. Esportarlo come `CategoryFor` (rinominare la funzione e adeguare i chiamanti interni al package v3, es. `JiraStatus`) — è il modo DRY. Verificare che `ReferenceHandler` importi `v3` e abbia `baseURL`. Aggiungere l'import `strconv` se serve. `v3.WriteJSON`/`v3.WriteError` esistono.

- [ ] **Step 2: Cablare le rotte**

In `internal/api/router.go`, vicino a `GET /rest/api/3/status`:

```go
	mux.Handle("GET /rest/api/3/statuscategory", authMw(http.HandlerFunc(refH.StatusCategories)))
	mux.Handle("GET /rest/api/3/statuscategory/{idOrKey}", authMw(http.HandlerFunc(refH.StatusCategoryByID)))
```

- [ ] **Step 3: Build + smoke**

Run: `go build ./... && go vet ./internal/api/...`
Expected: compila, vet pulito.

- [ ] **Step 4: Commit**

```bash
git add internal/api/handlers/reference_handler.go internal/api/v3/reference.go internal/api/router.go
git commit -m "feat(api): statuscategory endpoints (list + by id/key)"
```

---

### Task 6: Transizioni issue conformi — GET disponibili + POST

**Files:**
- Modify: `internal/api/handlers/workflow_handler.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Riscrivere il transition handler**

In `internal/api/handlers/workflow_handler.go`, SOSTITUIRE `TransitionIssue` con due metodi `AvailableTransitions` (GET) e `DoTransition` (POST). Rimuovere il vecchio `TransitionIssue`.

```go
// resolveIssueForTransition trova la issue e il suo workflow.
func (h *WorkflowHandler) issueAndWorkflow(issueKey string) (*issue.Issue, *workflow.Workflow, error) {
	iss, err := h.issueSvc.GetByKey(issueKey)
	if err != nil {
		return nil, nil, err
	}
	wf, err := h.wfSvc.GetWorkflow(iss.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	return iss, wf, nil
}

// statusByID cerca uno stato nel workflow.
func statusByID(wf *workflow.Workflow, id string) *workflow.WorkflowStatus {
	for i := range wf.Statuses {
		if wf.Statuses[i].ID == id {
			return &wf.Statuses[i]
		}
	}
	return nil
}

// AvailableTransitions gestisce GET /rest/api/3/issue/{issueKey}/transitions.
func (h *WorkflowHandler) AvailableTransitions(w http.ResponseWriter, r *http.Request) {
	iss, wf, err := h.issueAndWorkflow(r.PathValue("issueKey"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue or workflow not found"}, nil)
		return
	}
	from := ""
	if iss.StatusID != nil {
		from = *iss.StatusID
	}
	trs, err := h.wfSvc.GetAvailableTransitions(wf.ID, from)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list transitions"}, nil)
		return
	}
	out := make([]v3.IssueTransition, 0, len(trs))
	for _, tr := range trs {
		to := statusByID(wf, tr.ToStatusID)
		if to == nil {
			continue
		}
		name := tr.Name
		if name == "" {
			name = "→ " + to.Name
		}
		out = append(out, v3.MakeTransition(v3.TransitionInput{
			ID: tr.ID, Name: name, ToID: to.ID, ToName: to.Name,
			ToCategory: string(to.Category), Available: true, BaseURL: h.baseURL,
		}))
	}
	v3.WriteJSON(w, http.StatusOK, v3.Transitions{Transitions: out})
}

// DoTransition gestisce POST /rest/api/3/issue/{issueKey}/transitions.
// Accetta lo shape Jira {transition:{id}} e l'estensione {status_id} (usata dalla board).
func (h *WorkflowHandler) DoTransition(w http.ResponseWriter, r *http.Request) {
	iss, wf, err := h.issueAndWorkflow(r.PathValue("issueKey"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue or workflow not found"}, nil)
		return
	}
	var req struct {
		Transition struct {
			ID string `json:"id"`
		} `json:"transition"`
		StatusID string `json:"status_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	from := ""
	if iss.StatusID != nil {
		from = *iss.StatusID
	}

	// Risolvi la transizione target.
	var tr *workflow.WorkflowTransition
	switch {
	case req.Transition.ID != "":
		t, err := h.wfSvc.GetTransitionByID(req.Transition.ID)
		if err != nil || t.WorkflowID != wf.ID || t.FromStatusID != from {
			v3.WriteError(w, http.StatusBadRequest, []string{"invalid transition"}, nil)
			return
		}
		tr = t
	case req.StatusID != "":
		avail, _ := h.wfSvc.GetAvailableTransitions(wf.ID, from)
		for i := range avail {
			if avail[i].ToStatusID == req.StatusID {
				tr = &avail[i]
				break
			}
		}
		if tr == nil {
			v3.WriteError(w, http.StatusBadRequest, []string{"invalid transition"}, nil)
			return
		}
	default:
		v3.WriteError(w, http.StatusBadRequest, []string{"transition.id or status_id is required"}, nil)
		return
	}

	// Validator: require_assignee.
	if tr.RequireAssignee && (iss.AssigneeID == nil || *iss.AssigneeID == "") {
		v3.WriteError(w, http.StatusBadRequest, []string{"assignee is required for this transition"}, nil)
		return
	}

	// Applica il cambio di stato.
	if _, err := h.issueSvc.Update(iss.Key, nil, nil, nil, nil, &tr.ToStatusID, nil); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to update issue"}, nil)
		return
	}

	// Post-function: set_resolution in base alla categoria dello stato destinazione.
	if tr.SetResolution {
		toStatus := statusByID(wf, tr.ToStatusID)
		if toStatus != nil && toStatus.Category == workflow.CategoryDone {
			if resID, ok := h.issueSvc.ResolutionIDByName("Done"); ok {
				_ = h.issueSvc.SetResolution(iss.Key, &resID)
			}
		} else {
			_ = h.issueSvc.SetResolution(iss.Key, nil)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
```

> **Note implementatore:**
> - `WorkflowHandler` non ha attualmente un campo `baseURL`. AGGIUNGERLO: estendere lo struct con `baseURL string` e il costruttore `NewWorkflowHandler(wfSvc, issueSvc, projectSvc, baseURL)`; aggiornare la chiamata in `router.go`. (Serve per i self-link degli stati nelle transizioni.)
> - Aggiungere gli import `v3`, `workflow`, `issue` se non presenti (l'handler già usa `issue`/`workflow`).
> - Rimuovere il vecchio `TransitionIssue`. La board invia `{status_id}` → gestito dal ramo `req.StatusID`. La board legge solo il success (non il body), quindi il passaggio da 200+JSON a 204 è ok.

- [ ] **Step 2: Cablare le rotte (GET nuovo, POST ripuntato)**

In `internal/api/router.go`, dove c'è `POST /rest/api/3/issue/{issueKey}/transitions` → `wfH.TransitionIssue`, sostituire e aggiungere il GET:

```go
	mux.Handle("GET /rest/api/3/issue/{issueKey}/transitions", authMw(http.HandlerFunc(wfH.AvailableTransitions)))
	mux.Handle("POST /rest/api/3/issue/{issueKey}/transitions", authMw(http.HandlerFunc(wfH.DoTransition)))
```

- [ ] **Step 3: Build + vet + test**

Run: `go build ./... && go vet ./... && go test ./... 2>&1 | grep -vE '^ok|no test'`
Expected: verde (nessun FAIL).

- [ ] **Step 4: Smoke test (board compat + conformità)**

Run:
```bash
rm -f /tmp/r6.db && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/r6.db go run ./cmd/seed >/dev/null 2>&1
APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/r6.db go run ./cmd/server & SRV=$!; sleep 3
TOK=$(curl -s -X POST localhost:8080/rest/api/3/auth/login -H 'Content-Type: application/json' -d '{"email":"admin@example.com","password":"admin-demo-123"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')
echo "GET transitions DEMO-1:"; curl -s localhost:8080/rest/api/3/issue/DEMO-1/transitions -H "Authorization: Bearer $TOK" | python3 -m json.tool
kill $SRV; rm -f /tmp/r6.db
```
Expected: `{"transitions":[{"id":...,"name":...,"to":{"id":...,"statusCategory":{...}},...}]}`.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/workflow_handler.go internal/api/router.go
git commit -m "feat(api): conformant GET/POST issue transitions with base rules"
```

---

### Task 7: Editing workflow — transizioni (nome/regole/list/delete) + reorder stati

**Files:**
- Modify: `internal/api/handlers/workflow_handler.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Aggiornare AddTransition + aggiungere ListTransitions/UpdateTransition/DeleteTransition/ReorderStatuses**

In `internal/api/handlers/workflow_handler.go`:

(a) `AddTransition` handler — estendere il body con nome e regole e passare alla nuova firma del service:

```go
func (h *WorkflowHandler) AddTransition(w http.ResponseWriter, r *http.Request) {
	wf, err := h.wfSvc.GetWorkflow(r.PathValue("key")) // vedi nota: risolve per project key
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"workflow not found"}, nil)
		return
	}
	var req struct {
		FromStatusID    string `json:"from_status_id"`
		ToStatusID      string `json:"to_status_id"`
		Name            string `json:"name"`
		RequireAssignee bool   `json:"require_assignee"`
		SetResolution   bool   `json:"set_resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	tr, err := h.wfSvc.AddTransition(wf.ID, req.FromStatusID, req.ToStatusID, req.Name, req.RequireAssignee, req.SetResolution)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to add transition"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, tr)
}
```

(b) aggiungere:

```go
// ListTransitions: GET /project/{key}/workflow/transitions.
func (h *WorkflowHandler) ListTransitions(w http.ResponseWriter, r *http.Request) {
	wf, err := h.wfSvc.GetWorkflow(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"workflow not found"}, nil)
		return
	}
	trs, err := h.wfSvc.GetTransitions(wf.ID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list transitions"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, trs)
}

// UpdateTransition: PATCH /project/{key}/workflow/transitions/{id}.
func (h *WorkflowHandler) UpdateTransition(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            *string `json:"name"`
		RequireAssignee *bool   `json:"require_assignee"`
		SetResolution   *bool   `json:"set_resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	tr, err := h.wfSvc.UpdateTransition(r.PathValue("id"), req.Name, req.RequireAssignee, req.SetResolution)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"transition not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, tr)
}

// DeleteTransition: DELETE /project/{key}/workflow/transitions/{id}.
func (h *WorkflowHandler) DeleteTransition(w http.ResponseWriter, r *http.Request) {
	if err := h.wfSvc.RemoveTransition(r.PathValue("id")); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete transition"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReorderStatuses: PUT /project/{key}/workflow/statuses/order  body {status_ids:[...]}.
func (h *WorkflowHandler) ReorderStatuses(w http.ResponseWriter, r *http.Request) {
	wf, err := h.wfSvc.GetWorkflow(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"workflow not found"}, nil)
		return
	}
	var req struct {
		StatusIDs []string `json:"status_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if err := h.wfSvc.ReorderStatuses(wf.ID, req.StatusIDs); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to reorder"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

> **Nota implementatore:** VERIFICARE come l'`AddTransition` handler ESISTENTE risolve il workflow: probabilmente `GetWorkflow` prende un projectID, non una project key. Leggere l'handler esistente `AddStatus`/`GetWorkflow` in workflow_handler.go per il pattern reale (potrebbe risolvere il progetto via `projectSvc.GetByKey(r.PathValue("key"))` poi `wfSvc.GetWorkflow(p.ID)`). USARE lo STESSO pattern in tutti i nuovi metodi (non `GetWorkflow(key)` diretto se la firma è `GetWorkflow(projectID)`). Adeguare il codice sopra di conseguenza (aggiungere `p, _ := h.projectSvc.GetByKey(key); wf, _ := h.wfSvc.GetWorkflow(p.ID)`).

- [ ] **Step 2: Cablare le rotte**

In `internal/api/router.go`, vicino alle rotte `/project/{key}/workflow/*` esistenti:

```go
	mux.Handle("GET /rest/api/3/project/{key}/workflow/transitions", authMw(http.HandlerFunc(wfH.ListTransitions)))
	mux.Handle("PATCH /rest/api/3/project/{key}/workflow/transitions/{id}", authMw(http.HandlerFunc(wfH.UpdateTransition)))
	mux.Handle("DELETE /rest/api/3/project/{key}/workflow/transitions/{id}", authMw(http.HandlerFunc(wfH.DeleteTransition)))
	mux.Handle("PUT /rest/api/3/project/{key}/workflow/statuses/order", authMw(http.HandlerFunc(wfH.ReorderStatuses)))
```
(La `POST /project/{key}/workflow/transitions` → `wfH.AddTransition` esiste già.)

- [ ] **Step 3: Build + vet**

Run: `go build ./... && go vet ./...`
Expected: verde.

- [ ] **Step 4: Commit**

```bash
git add internal/api/handlers/workflow_handler.go internal/api/router.go
git commit -m "feat(api): workflow transition editing (name/rules/list/delete) and status reorder"
```

---

### Task 8: Contract test — transizioni + statuscategory

**Files:**
- Create: `internal/contract/workflow_test.go`

- [ ] **Step 1: Scrivere i contract test**

`internal/contract/workflow_test.go` (usare gli helper reali dell'harness come in `agile_test.go`/`search_test.go`; adeguare i nomi):

```go
package contract

import (
	"net/http"
	"testing"
)

func TestStatusCategory_Conformant(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")

	resp := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/statuscategory", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("statuscategory status %d", resp.StatusCode)
	}
	v.ValidateResponse(http.MethodGet, "/rest/api/3/statuscategory", http.StatusOK, resp.Header, resp.Body)

	// by key
	resp2 := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/statuscategory/done", nil)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("statuscategory/done status %d", resp2.StatusCode)
	}
	v.ValidateResponse(http.MethodGet, "/rest/api/3/statuscategory/{idOrKey}", http.StatusOK, resp2.Header, resp2.Body)
}

func TestIssueTransitions_Conformant(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, tok, "WF", "Workflow Proj")
	key := createIssueViaAPI(t, srv, tok, "WF", "Transition me")
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")

	// GET available transitions
	resp := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/issue/"+key+"/transitions", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET transitions status %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	v.ValidateResponse(http.MethodGet, "/rest/api/3/issue/{issueIdOrKey}/transitions", http.StatusOK, resp.Header, bodyReader(body))
	trs, _ := body["transitions"].([]any)
	if len(trs) == 0 {
		t.Fatal("attese transizioni disponibili dallo stato iniziale")
	}
	first := trs[0].(map[string]any)
	trID, _ := first["id"].(string)
	if trID == "" {
		t.Fatal("transizione senza id")
	}
	if _, ok := first["to"].(map[string]any); !ok {
		t.Error("transizione senza campo to")
	}

	// POST do transition (shape Jira {transition:{id}})
	resp2 := doJSON(t, srv, http.MethodPost, tok, "/rest/api/3/issue/"+key+"/transitions", map[string]any{
		"transition": map[string]any{"id": trID},
	})
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("POST transition status %d (atteso 204)", resp2.StatusCode)
	}

	// la issue ora è nello stato di destinazione: le transizioni disponibili sono cambiate
	resp3 := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/issue/"+key+"/transitions", nil)
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("GET transitions #2 status %d", resp3.StatusCode)
	}
}
```

> **Nota implementatore:** adeguare gli helper: `decodeBody`/`doJSON` esistono in `search_test.go`; se serve ri-passare il body a `ValidateResponse` dopo averlo decodificato, usare l'helper reale (potrebbe esserci `bodyReader` o si rilegge la response — vedi come `agile_test.go` valida DOPO aver letto il body; se `ValidateResponse` consuma `resp.Body`, validare PRIMA di `decodeBody`, oppure bufferizzare). Guardare il pattern esatto in `agile_test.go` e replicarlo (probabilmente: validare direttamente su `resp` senza pre-decodificare, poi una seconda richiesta per le asserzioni, oppure un helper che bufferizza). Mantenere le asserzioni comportamentali (id transizione, campo `to`, 204 sul POST, cambio di stato).

- [ ] **Step 2: Eseguire i contract test**

Run: `go test ./internal/contract/ -run 'TestStatusCategory|TestIssueTransitions' -v`
Expected: PASS. Se la validazione OpenAPI fallisce, correggere il mapper (`omitempty`, campi mancanti).

- [ ] **Step 3: Suite completa**

Run: `go test ./...`
Expected: verde (incluse le suite dei round precedenti — la board dnd usa lo stesso POST transitions).

- [ ] **Step 4: Commit**

```bash
git add internal/contract/workflow_test.go
git commit -m "test(contract): issue transitions and statuscategory conformance"
```

---

### Task 9: Frontend — client workflow

**Files:**
- Modify: `frontend-next/lib/api.ts`

- [ ] **Step 1: Aggiungere tipi e client**

In `frontend-next/lib/api.ts`:

```ts
export interface WorkflowStatus {
  id: string;
  name: string;
  category: "todo" | "inprogress" | "done";
  color: string;
  position: number;
}

export interface WorkflowTransition {
  id: string;
  from_status_id: string;
  to_status_id: string;
  name: string;
  require_assignee: boolean;
  set_resolution: boolean;
}

export interface Workflow {
  id: string;
  name: string;
  statuses: WorkflowStatus[];
  transitions: WorkflowTransition[];
}

export interface AvailableTransition {
  id: string;
  name: string;
  to: { id: string; name: string; statusCategory: { key: string; name: string } };
}

export const workflow = {
  get: (projectKey: string) => apiFetch<Workflow>(`/rest/api/3/project/${projectKey}/workflow`),
  addStatus: (projectKey: string, name: string, category: string, color: string) =>
    apiFetch<WorkflowStatus>(`/rest/api/3/project/${projectKey}/workflow/statuses`, {
      method: "POST",
      body: JSON.stringify({ name, category, color }),
    }),
  updateStatus: (projectKey: string, id: string, patch: { name?: string; category?: string; color?: string }) =>
    apiFetch<WorkflowStatus>(`/rest/api/3/project/${projectKey}/workflow/statuses/${id}`, {
      method: "PATCH",
      body: JSON.stringify(patch),
    }),
  deleteStatus: (projectKey: string, id: string) =>
    apiFetch<void>(`/rest/api/3/project/${projectKey}/workflow/statuses/${id}`, { method: "DELETE" }),
  reorderStatuses: (projectKey: string, statusIds: string[]) =>
    apiFetch<void>(`/rest/api/3/project/${projectKey}/workflow/statuses/order`, {
      method: "PUT",
      body: JSON.stringify({ status_ids: statusIds }),
    }),
  addTransition: (projectKey: string, t: { from_status_id: string; to_status_id: string; name: string; require_assignee?: boolean; set_resolution?: boolean }) =>
    apiFetch<WorkflowTransition>(`/rest/api/3/project/${projectKey}/workflow/transitions`, {
      method: "POST",
      body: JSON.stringify(t),
    }),
  updateTransition: (projectKey: string, id: string, patch: { name?: string; require_assignee?: boolean; set_resolution?: boolean }) =>
    apiFetch<WorkflowTransition>(`/rest/api/3/project/${projectKey}/workflow/transitions/${id}`, {
      method: "PATCH",
      body: JSON.stringify(patch),
    }),
  deleteTransition: (projectKey: string, id: string) =>
    apiFetch<void>(`/rest/api/3/project/${projectKey}/workflow/transitions/${id}`, { method: "DELETE" }),
};
```

Aggiungere anche a `issues` il getter delle transizioni disponibili:

```ts
  availableTransitions: (idOrKey: string) =>
    apiFetch<{ transitions: AvailableTransition[] }>(`/rest/api/3/issue/${idOrKey}/transitions`),
```

> **Nota:** confermare il wrapper reale `apiFetch` e lo stile degli altri oggetti. `GET /project/{key}/workflow` ritorna la shape di dominio del workflow (con `statuses`/`transitions`). Se l'handler `GetWorkflow` attuale ritorna 204/shape diversa, verificare e adeguare il tipo `Workflow` (leggere la risposta reale).

- [ ] **Step 2: Type-check**

Run: `cd frontend-next && npx tsc --noEmit`
Expected: nessun errore.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/lib/api.ts
git commit -m "feat(frontend): workflow client and available-transitions"
```

---

### Task 10: Frontend — editor workflow in impostazioni progetto

**Files:**
- Create: `frontend-next/components/workflow/WorkflowEditor.tsx`
- Modify: `frontend-next/components/projects/ProjectSettings.tsx`

- [ ] **Step 1: Componente editor**

`frontend-next/components/workflow/WorkflowEditor.tsx`:

```tsx
"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { workflow, type WorkflowStatus } from "@/lib/api";

const CATEGORIES = [
  { value: "todo", label: "To Do" },
  { value: "inprogress", label: "In Progress" },
  { value: "done", label: "Done" },
] as const;

export function WorkflowEditor({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const [newStatus, setNewStatus] = useState("");
  const [newCat, setNewCat] = useState("todo");

  const wf = useQuery({ queryKey: ["workflow", projectKey], queryFn: () => workflow.get(projectKey) });
  const invalidate = () => qc.invalidateQueries({ queryKey: ["workflow", projectKey] });

  const addStatus = useMutation({
    mutationFn: () => workflow.addStatus(projectKey, newStatus, newCat, "#6B7280"),
    onSuccess: () => {
      setNewStatus("");
      invalidate();
    },
  });
  const delStatus = useMutation({
    mutationFn: (id: string) => workflow.deleteStatus(projectKey, id),
    onSuccess: invalidate,
  });

  const statuses = wf.data?.statuses ?? [];
  const nameByID = (id: string) => statuses.find((s) => s.id === id)?.name ?? id;

  return (
    <div className="space-y-6">
      <section>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">Statuses</h3>
        <ul className="space-y-1" data-testid="workflow-statuses">
          {statuses.map((s: WorkflowStatus) => (
            <li key={s.id} className="flex items-center gap-2 text-sm" data-testid={`status-${s.name}`}>
              <span className="inline-block h-3 w-3 rounded" style={{ backgroundColor: s.color }} />
              <span className="text-[#1a1f36]">{s.name}</span>
              <span className="text-xs text-slate-400">({s.category})</span>
              <button
                onClick={() => delStatus.mutate(s.id)}
                className="ml-auto text-xs text-red-600 hover:underline"
                aria-label={`Delete status ${s.name}`}
              >
                Remove
              </button>
            </li>
          ))}
        </ul>
        <div className="mt-2 flex gap-2">
          <input
            aria-label="New status name"
            value={newStatus}
            onChange={(e) => setNewStatus(e.target.value)}
            placeholder="Status name"
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          />
          <select aria-label="New status category" value={newCat} onChange={(e) => setNewCat(e.target.value)} className="rounded border border-slate-300 px-2 py-1 text-sm">
            {CATEGORIES.map((c) => (
              <option key={c.value} value={c.value}>{c.label}</option>
            ))}
          </select>
          <button
            onClick={() => newStatus && addStatus.mutate()}
            className="rounded bg-[#0052cc] px-3 py-1 text-sm text-white disabled:opacity-60"
            disabled={addStatus.isPending}
          >
            Add status
          </button>
        </div>
      </section>

      <section>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">Transitions</h3>
        <ul className="space-y-1 text-sm" data-testid="workflow-transitions">
          {(wf.data?.transitions ?? []).map((t) => (
            <li key={t.id} className="flex items-center gap-2">
              <span className="text-[#1a1f36]">{t.name || `${nameByID(t.from_status_id)} → ${nameByID(t.to_status_id)}`}</span>
              <span className="text-xs text-slate-400">
                {nameByID(t.from_status_id)} → {nameByID(t.to_status_id)}
                {t.require_assignee ? " · requires assignee" : ""}
                {t.set_resolution ? " · sets resolution" : ""}
              </span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
```

- [ ] **Step 2: Aggiungere il tab Workflow nelle impostazioni**

In `frontend-next/components/projects/ProjectSettings.tsx`: aggiungere una tab-bar semplice (stato `tab: "general" | "workflow"`) e renderizzare `<WorkflowEditor projectKey={projectKey} />` quando `tab==="workflow"`. Mantenere il contenuto esistente (name/description/archive) sotto `tab==="general"`. Esempio minimale di header tab:

```tsx
// in cima al render, con: const [tab, setTab] = useState<"general" | "workflow">("general");
<div className="mb-4 flex gap-4 border-b">
  <button onClick={() => setTab("general")} className={tab === "general" ? "border-b-2 border-[#0052cc] pb-2 text-sm font-medium" : "pb-2 text-sm text-slate-500"}>General</button>
  <button onClick={() => setTab("workflow")} className={tab === "workflow" ? "border-b-2 border-[#0052cc] pb-2 text-sm font-medium" : "pb-2 text-sm text-slate-500"}>Workflow</button>
</div>
```
e sotto: `{tab === "workflow" && <WorkflowEditor projectKey={projectKey} />}` avvolgendo il contenuto esistente in `{tab === "general" && (<>...</>)}`.

> **Nota implementatore:** leggere `ProjectSettings.tsx` per la prop reale (`projectKey`) e lo stile; importare `WorkflowEditor` con l'alias `@/components/workflow/WorkflowEditor`. Non rompere il comportamento esistente name/description/archive.

- [ ] **Step 3: Type-check + build**

Run: `cd frontend-next && npx tsc --noEmit && npm run build`
Expected: build OK.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/components/workflow/WorkflowEditor.tsx frontend-next/components/projects/ProjectSettings.tsx
git commit -m "feat(frontend): workflow editor tab in project settings"
```

---

### Task 11: E2E — editor workflow

**Files:**
- Create: `frontend-next/e2e/workflow.spec.ts`

- [ ] **Step 1: Scrivere l'E2E**

`frontend-next/e2e/workflow.spec.ts` (riusare l'helper `login()` reale come in `board.spec.ts`):

```ts
import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/jira/);
}

test("workflow editor shows seeded statuses and adds a new one", async ({ page }) => {
  await login(page);
  await page.goto("/jira/projects/DEMO/settings");
  await page.getByRole("button", { name: "Workflow" }).click();

  // gli stati di default seedati sono visibili
  await expect(page.getByTestId("workflow-statuses")).toBeVisible();
  await expect(page.getByTestId("status-TO DO")).toBeVisible();

  // aggiungi uno stato
  await page.getByLabel("New status name").fill("Review");
  await page.getByLabel("New status category").selectOption("inprogress");
  await page.getByRole("button", { name: "Add status" }).click();
  await expect(page.getByTestId("status-Review")).toBeVisible();
});
```

> **Nota:** verificare il path reale delle impostazioni progetto (`/jira/projects/DEMO/settings` — confermare da `frontend-next/app/jira/projects/[key]/settings/page.tsx`). Verificare i nomi esatti degli stati seedati (`TO DO`/`IN PROGRESS`/`DONE` da `CreateDefaultWorkflow`). Adeguare il selettore `status-TO DO` al nome reale (case).

- [ ] **Step 2: Eseguire l'E2E**

Run: `cd frontend-next && npx playwright test e2e/workflow.spec.ts --reporter=line`
Expected: PASS.

- [ ] **Step 3: Suite completa**

Run: `cd frontend-next && npx playwright test --reporter=line`
Expected: tutti verdi (login, projects, issues, collaboration, search, board, workflow). Pulire `test-results/`/`playwright-report/` e i processi su 8080/3000.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/e2e/workflow.spec.ts
git commit -m "test(e2e): workflow editor shows statuses and adds one"
```

---

### Task 12: Seed nomi transizione demo + gap report

**Files:**
- Modify: `cmd/seed/main.go` (opzionale — solo se i nomi transizione non sono già seedati da CreateDefaultWorkflow)
- Modify: `docs/contracts/gap-report.md` (rigenerato)

- [ ] **Step 1: Verificare i nomi transizione nel default workflow**

Le transizioni di default ora hanno un nome (Task 2 le ha aggiornate in `CreateDefaultWorkflow`: "Start Progress"/"Stop Progress"/"Done" con `set_resolution=true` su "Done"). Nessuna modifica al seed necessaria SE `CreateDefaultWorkflow` è la fonte (lo è, sia via handler HTTP di creazione progetto sia via il seed R5). Verificare con uno smoke: `GET /issue/DEMO-1/transitions` mostra nomi non vuoti.

Se invece si vuole un dato demo esplicito (una issue assegnata che dimostra `require_assignee`), aggiungerlo idempotente in `cmd/seed/main.go` — altrimenti saltare questo step.

- [ ] **Step 2: Rigenerare il gap report**

Run: `go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md`
Expected: aggiornato; nuovi endpoint conformi: `GET /issue/{id}/transitions`, `GET /statuscategory`, `GET /statuscategory/{idOrKey}`. Riportare il conteggio old→new.

- [ ] **Step 3: Commit**

```bash
git add cmd/seed/main.go docs/contracts/gap-report.md
git commit -m "feat(seed): named default transitions; regenerate gap report for Round 6"
```
(Se `cmd/seed/main.go` non è cambiato, committare solo `docs/contracts/gap-report.md`.)

---

### Task 13: Gate finale + STATE.md → Round 7

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
- aggiungere alla sezione "Round completati" la riga del **Round 6 — Workflow** (transizioni issue conformi `GET/POST /issue/{id}/transitions` con regole base validator `require_assignee` + post-function `set_resolution`; `/statuscategory[/{idOrKey}]`; editing workflow per-progetto — statuses CRUD + transizioni nome/regole/list/delete + reorder; UI editor workflow nel tab impostazioni; transizione ritorna 204);
- cambiare "Prossimo" in **Round 7 — Viste & Report** (Timeline/Gantt, Calendar, Summary progetto, Dashboard con gadget, report Burndown/Velocity/CFD/created-vs-resolved);
- aggiornare il conteggio gap report e la data;
- aggiungere ai follow-up: workflow CRUD bulk v3 (`/workflows/create`, `statusReference`); **condizioni** di transizione (filtro `isAvailable`); rule engine generico (oltre a require_assignee/set_resolution); workflow scheme (`/workflowscheme`) per mappare più workflow ai tipi issue; transizioni globali/iniziali.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/STATE.md docs/contracts/gap-report.md
git commit -m "docs: mark Round 6 (Workflow) complete, Round 7 (Views & Reports) next"
```

---

## Note di chiusura round

- **Follow-up:** workflow CRUD bulk v3 (`/workflows/create`/`/workflows` con `statusReference`, conditions/validators/actions arrays); **condizioni** di transizione che filtrano `isAvailable` in GET (ora sempre true); rule engine generico configurabile (oltre ai due flag base); `/workflowscheme` (mappa workflow→tipi issue per progetto); transizioni `isGlobal`/`isInitial`; screen sulle transizioni (`hasScreen`/fields); spostare `CreateDefaultWorkflow` dentro `project.Service.Create` (ora solo l'handler HTTP + seed lo creano).
- **Rischi noti:** il POST transitions ora ritorna 204 (prima 200+issue) — verificato che la board legge solo il success; la post-function `set_resolution` dipende dall'esistenza di una resolution "Done" (se assente, non setta nulla — comportamento accettabile). Il round chiude solo con i tre livelli verdi.

---

## Self-Review (svolta in fase di scrittura)

**1. Copertura spec (roadmap Round 6 + contratto):**
- "Stati custom con categorie" → già presenti (workflow domain); editing via Task 7 + UI Task 10. ✅
- "Transizioni con condizioni/validator/post-function base" → Task 2 (Name + RequireAssignee validator + SetResolution post-function), applicate in Task 6 (DoTransition). Condizioni esplicitamente rinviate. ✅ (base)
- "Workflow per progetto" → editing su rotte `/project/{key}/workflow/*` (Task 7). ✅
- Transizioni issue conformi v3 → Task 4 (mapper) + Task 6 (GET/POST). ✅
- `/statuscategory` → Task 5. ✅
- "UI: editor workflow in settings" → Task 10; "colonne board mappate su stati" → già dal Round 5. ✅
- Gate: contract Task 8, E2E Task 11, gate Task 13. ✅

**2. Placeholder scan:** codice completo in ogni task. Le "Note implementatore" indicano verifiche puntuali su firme reali (come `GetWorkflow` risolve project key vs id, `categoryFor` da esportare, `apiFetch`, helper harness, shape reale di `GET /project/{key}/workflow`, path settings, nomi status seedati) con comandi per risolverle — non placeholder di logica. Scope condizioni/rule-engine dichiarato esplicitamente.

**3. Consistenza tipi:** `workflow.WorkflowTransition{...,Name,RequireAssignee,SetResolution}`; `AddTransition(workflowID, from, to, name string, requireAssignee, setResolution bool)`; `GetTransitionByID`/`GetAvailableTransitions(workflowID, fromStatusID)`/`UpdateTransition(id, name *string, requireAssignee, setResolution *bool)`/`ReorderStatuses(workflowID, []string)` (Task 2) usati in Task 6/7. `issue.Service.SetResolution(key, *string)`/`ResolutionIDByName(name)` (Task 3) usati in Task 6. `v3.MakeTransition(TransitionInput)`/`Transitions`/`IssueTransition`/`StatusRef` (Task 4) usati in Task 6. `v3.CategoryFor` (esportato in Task 5) usato in reference_handler. Handler `WorkflowHandler` + campo `baseURL` (Task 6) usato in Task 6/7. Frontend `workflow` client + `issues.availableTransitions` (Task 9) usati in Task 10/11. La board (Round 5) continua a usare `POST .../transitions {status_id}` — gestito dal ramo `req.StatusID` di DoTransition.
