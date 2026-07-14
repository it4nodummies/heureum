# Round 5 — Board, Backlog, Sprint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dare a open-jira le board Agile (Scrum/Kanban), il backlog e gli sprint con API drop-in compatibile con la Jira Agile REST API 1.0 (`/rest/agile/1.0/*`), più UI board drag&drop e backlog con gestione sprint.

**Architecture:** Nuova superficie API sotto `/rest/agile/1.0/*` costruita ex-novo, che RIUSA il dominio `sprint` esistente (esteso) e aggiunge un dominio `board`. Gli id pubblici Agile sono interi (pattern `seq_id`, come progetti/issue). Le board sono legate a un progetto+tipo; le issue di board/backlog/sprint sono le issue del progetto (backlog = `sprint_id IS NULL`). Gli epic sono issue di tipo "Epic" (nessuna tabella nuova). Il ranking riusa la colonna `Position float64` con inserimento midpoint. Le liste board/sprint usano la paginazione `values`+`isLast` (`v3.WritePage`), le liste issue usano lo shape `SearchResults` (`issues`+`total`) riusando il renderer del Round 4. Frontend: board a colonne (drag→cambio stato + rank) e backlog (sprint collassabili, start/complete, sposta issue) con `dnd-kit`.

**Tech Stack:** Go 1.25 (net/http ServeMux, GORM, golang-migrate, SQLite in test), package esistenti `internal/api/v3`, dominio `internal/domain/{sprint,issue,project,workflow}`, harness `internal/contract`. Frontend Next.js 16 + React 19 + TanStack Query + Tailwind + **@dnd-kit/core** + Playwright.

---

## Contesto per l'implementatore (leggere una volta)

**Contratto ufficiale** in `docs/contracts/jira-agile-1.0.json`. Molte operazioni dichiarano solo una risposta `default` (nessun `content` 200 esplicito): il validatore contract è quindi lenient sul body per quelle, ma le forme canoniche sono queste (proprietà ESATTE):

- **Board**: `id:int`, `self:string`, `name:string`, `type:string`, `location:object` (BoardLocationBean). **BoardCreateBean**: `name:string`, `type:string`, `filterId:int`, `location:object` (NIENTE projectKeyOrId — noi accettiamo anche una scorciatoia, vedi sotto). **BoardLocationBean**: `projectId:int`, `projectKey:string`, `projectName:string`, `projectTypeKey:string`, `displayName`, `name`.
- **BoardConfigBean**: `id:int`, `name:string`, `self:string`, `type:string`, `location:object`, `columnConfig:object` (`{columns:[{name, statuses:[{id}]}], constraintType}`).
- **SprintBean** (anche body POST/PUT `/sprint/{id}`): `id:int`, `self:string`, `state:string`, `name:string`, `startDate:string`, `endDate:string`, `completeDate:string`, `createdDate:string`, `originBoardId:int`, `goal:string`.
- **SprintCreateBean** (POST `/sprint`): `name:string`, `startDate:string`, `endDate:string`, `originBoardId:int`, `goal:string`.
- **IssueRankRequestBean** (PUT `/issue/rank`, e i body di move su board/sprint con boardId): `issues:[]string`, `rankBeforeIssue:string`, `rankAfterIssue:string`, `rankCustomFieldId:int`.
- **Move to sprint** (POST `/sprint/{id}/issue`): `{issues:[]string, rankBeforeIssue, rankAfterIssue, rankCustomFieldId}`. **Move to backlog** (POST `/backlog/issue`): `{issues:[]string}`.
- **Paginazione (DUE shape distinti):** liste **board/sprint/epic** → `startAt/maxResults/total/isLast/values`; liste **issue** (backlog, board issue, sprint issue) → `SearchResults` = `startAt/maxResults/total/issues` (NIENTE `isLast`).

**Codice ESISTENTE da riusare (verificato):**
- `internal/domain/sprint/model.go`: `Sprint{ID string(uuid PK), ProjectID, Name, Goal, State(active/closed/future), StartDate *time.Time, EndDate *time.Time, CreatedAt, UpdatedAt}`. `State` const `StateActive/StateClosed/StateFuture`.
- `internal/domain/sprint/service.go`: `NewService(db)`, `Create(projectID, name, goal string)(*Sprint,error)`, `GetByID(id)`, `Update(id, name, goal)`, `Start(sprintID)`, `Complete(sprintID, moveOpenToBacklog bool)`, `ListByProject(projectID)`, `GetActive(projectID)`, `AddIssue(sprintID, issueID)` (setta `issues.sprint_id`), `RemoveIssue(issueID)` (azzera sprint_id), `DB()`.
- `internal/domain/issue/model.go`: `Issue{ID, ProjectID, Key, Title, StatusID *string, SprintID *string(sprint_id), ParentID *string(parent_id — link epic), Position float64(position, default 0), SeqID int64, ...}`. `issue.Service.ListByProject(projectID, opts...)` ordina `position ASC`; opzioni `WithNotArchived()`, `WithStatus`, `WithAssignee`, ecc. `issue.Service.DB()`, `GetByKey(key)`, `GetBySeqID(n int64)`, `GetLabels(issueID)`.
- `internal/domain/project/model.go`: `Project{ID, Key, Name, Type project.Type(scrum/kanban/business), SeqID, ...}`; `project.Service.GetByKey`, `GetBySeqID`. `project.ProjectTypeKeyForType(t)`.
- `internal/domain/workflow`: `workflow.Service.GetWorkflow(projectID)(*Workflow,error)`; `Workflow.Statuses []WorkflowStatus{ID, Name, Position, Category}`. (Uso: colonne board.)
- `internal/api/v3`: `WriteJSON(w,status,v)`, `WriteError(w,status,[]string,map[string]string)`, `WritePage[T](w,status, Page[T]{StartAt,MaxResults,Total,Values})` (emette `startAt/maxResults/total/isLast/values`), `ParsePagination(r,def,cap)`, `JiraTime(t time.Time) string` (RFC3339 offset `:`, zero→""), `JiraIssue(in IssueInput) IssueBean`, `IssueInput`, `SearchResults{Issues []map[string]any, StartAt, MaxResults, Total, WarningMessages}` (dal Round 4), `ParseFieldsFromList([]string) Fields`, `ProjectIssue(bean, Fields)`.
- `internal/api/handlers/search_handler.go`: `SearchHandler.renderIssues(issues []issue.Issue, fields []string) ([]map[string]any, error)` (`search_handler.go:68`) — costruisce gli IssueBean via `issueH.buildIssueInput`. **Task 5 lo estrae in una funzione condivisa `renderIssueList(issueH *IssueHandler, issues []issue.Issue, fields []string)` riusata dagli handler agile.**
- `internal/api/handlers/issue_handler.go`: `IssueHandler.buildIssueInput(iss *issue.Issue) v3.IssueInput` (`issue_handler.go:53`).
- `internal/api/handlers/board_handler.go`: `BoardHandler` custom (rotte `/rest/api/3/project/{key}/board`, `/issues/rank`) — **NON toccare**; le rotte agile sono parallele. La logica di rank midpoint qui è il riferimento per `issue.Service.Rank`.

**Router (`internal/api/router.go`):** `mux := http.NewServeMux()`; `authMw := middleware.Auth(...)`. Pattern: `mux.Handle("GET /rest/agile/1.0/<path>", authMw(http.HandlerFunc(h.M)))`. Handler costruiti verso l'inizio (dove ci sono `issueH`, `projectSvc`, `workflowSvc`, `sprintSvc` se già presente — verificare; lo sprint service è già costruito per le rotte custom). `cfg.BaseURL` per i self link. `middleware.UserIDFromContext(r.Context()) string`.

**Migrazioni:** ultima `000011_filter_fields`. Prossima **`000012`**.

**Harness contract (`internal/contract/`):** `MustLoad(t, specPath)`, `newTestServer(t)(*httptest.Server, *auth.Service)`, `registerAndLogin(t, authSvc)`, `createProjectViaAPI(t, srv, jwt, key, name)`, `createIssueViaAPI(t, srv, jwt, projectKey, summary) string`, `Validator.ValidateResponse(method, path, status, header, bodyReader)`. Lo spec agile va caricato con `MustLoad(t, "../../docs/contracts/jira-agile-1.0.json")`. Vedi `internal/contract/search_test.go` (Round 4) e `harness_test.go`/`harness.go` per le firme reali; **l'implementatore deve leggerli e adattare i nomi** (potrebbe servire un secondo validator per lo spec agile).

**Scelte di scope (per contenere il round — esplicite):**
- Board legata a **un progetto** (campo `project_id`) e opzionalmente a un `filter_id`; le issue di board/backlog/sprint sono quelle del progetto (backlog = `sprint_id IS NULL`, escluse archiviate). Board filtro-JQL puro → follow-up.
- `originBoardId` sullo sprint = `seq_id` della board. Creando uno sprint via `POST /sprint` con `originBoardId`, si risolve board→project e si crea lo sprint sotto quel progetto.
- **Epic**: solo **lettura** (`GET /board/{id}/epic` lista issue di tipo Epic; `GET /epic/{idOrKey}` dettaglio). `POST /epic/{id}` (update) e epic rank → follow-up.
- **Ranking**: `Position float64` midpoint (come il board_handler esistente). LexoRank stringa → follow-up.
- `rankCustomFieldId` nei body è accettato e ignorato (non abbiamo custom field di rank).

---

## Struttura dei file

**Migrazioni:** `migrations/000012_boards.up.sql` / `.down.sql`.

**Dominio:**
- `internal/domain/board/model.go` — `Board{ID, SeqID, Name, Type, ProjectID, FilterID *string, CreatedAt}`.
- `internal/domain/board/service.go` — `Service` con Create/GetBySeqID/GetByID/ListByProject/List/Delete (assegna seq_id da max+1).
- `internal/domain/sprint/model.go` — estendere `Sprint` con `SeqID int64`, `OriginBoardID *int64`, `CompleteDate *time.Time`.
- `internal/domain/sprint/service.go` — estendere: `Create` con date+originBoardID+seq_id; `GetBySeqID`; `UpdateFull` (name/goal/state/date); `Complete` setta CompleteDate.
- `internal/domain/issue/service.go` — aggiungere `Rank(issueIDs []string, beforeID, afterID *string) error`.

**Layer v3/agile:**
- `internal/api/v3/agile.go` — mapper: `AgileBoard`, `BoardLocation`, `BoardConfig`, `AgileSprint`, `AgileTime` (riusa JiraTime), + helper.

**Handler (package `handlers`):**
- `internal/api/handlers/render_issues.go` — funzione condivisa `renderIssueList(issueH *IssueHandler, issues []issue.Issue, fields []string) ([]map[string]any, error)` (estratta da SearchHandler.renderIssues).
- `internal/api/handlers/agile_board_handler.go` — `AgileBoardHandler`: board CRUD + configuration + liste (backlog/issue/sprint/epic).
- `internal/api/handlers/agile_sprint_handler.go` — `AgileSprintHandler`: sprint CRUD + sprint issue list/move + rank + backlog move + agile issue get + epic get.

**Router:** `internal/api/router.go` — blocco `/rest/agile/1.0/*`.

**Seed:** `cmd/seed/main.go` — una board demo + uno sprint demo.

**Frontend:**
- `frontend-next/package.json` — dep `@dnd-kit/core` + `@dnd-kit/sortable`.
- `frontend-next/lib/api.ts` — tipi + `boards`, `sprints`, `agileIssues` client.
- `frontend-next/components/board/BoardColumns.tsx` — board a colonne con dnd.
- `frontend-next/app/jira/boards/[boardId]/page.tsx` — pagina board.
- `frontend-next/app/jira/boards/[boardId]/backlog/page.tsx` — pagina backlog.
- `frontend-next/e2e/board.spec.ts` — E2E.

---

### Task 1: Migrazione 000012 — boards + campi sprint

**Files:**
- Create: `migrations/000012_boards.up.sql`
- Create: `migrations/000012_boards.down.sql`

- [ ] **Step 1: Scrivere la migrazione up**

`migrations/000012_boards.up.sql`:

```sql
CREATE TABLE boards (
    id TEXT PRIMARY KEY,
    seq_id INTEGER UNIQUE,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'scrum',
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    filter_id TEXT REFERENCES saved_filters(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE sprints ADD COLUMN seq_id INTEGER;
ALTER TABLE sprints ADD COLUMN origin_board_id INTEGER;
ALTER TABLE sprints ADD COLUMN complete_date TIMESTAMP;
```

- [ ] **Step 2: Scrivere la migrazione down**

`migrations/000012_boards.down.sql`:

```sql
ALTER TABLE sprints DROP COLUMN complete_date;
ALTER TABLE sprints DROP COLUMN origin_board_id;
ALTER TABLE sprints DROP COLUMN seq_id;
DROP TABLE IF EXISTS boards;
```

- [ ] **Step 3: Verificare l'applicazione a pulito**

Run: `rm -f /tmp/mig12.db && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/mig12.db go run ./cmd/seed && rm -f /tmp/mig12.db`
Expected: `seed complete`, exit 0 (tutte le 12 migrazioni applicate).

- [ ] **Step 4: Commit**

```bash
git add migrations/000012_boards.up.sql migrations/000012_boards.down.sql
git commit -m "feat(migrations): boards table and sprint agile columns"
```

---

### Task 2: Dominio board

**Files:**
- Create: `internal/domain/board/model.go`
- Create: `internal/domain/board/service.go`
- Test: `internal/domain/board/service_test.go`

- [ ] **Step 1: Scrivere il modello**

`internal/domain/board/model.go`:

```go
package board

import "time"

// Board è una board Agile legata a un progetto. SeqID è l'id pubblico intero
// esposto dall'API agile (l'UUID resta PK interna, come per progetti/issue).
type Board struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	SeqID     int64     `gorm:"column:seq_id;uniqueIndex" json:"seq_id"`
	Name      string    `gorm:"type:text;not null" json:"name"`
	Type      string    `gorm:"type:text;not null;default:'scrum'" json:"type"`
	ProjectID string    `gorm:"type:text;not null;index" json:"project_id"`
	FilterID  *string   `gorm:"type:text" json:"filter_id,omitempty"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}
```

- [ ] **Step 2: Scrivere i test**

`internal/domain/board/service_test.go`:

```go
package board

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&Board{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCreate_AssignsSeqID(t *testing.T) {
	svc := NewService(newDB(t))
	b1, err := svc.Create("proj-1", "Board A", "scrum", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if b1.SeqID != 1 {
		t.Errorf("primo seq_id atteso 1, got %d", b1.SeqID)
	}
	b2, _ := svc.Create("proj-1", "Board B", "kanban", nil)
	if b2.SeqID != 2 {
		t.Errorf("secondo seq_id atteso 2, got %d", b2.SeqID)
	}
}

func TestGetBySeqID(t *testing.T) {
	svc := NewService(newDB(t))
	b, _ := svc.Create("proj-1", "Board A", "scrum", nil)
	got, err := svc.GetBySeqID(b.SeqID)
	if err != nil {
		t.Fatalf("GetBySeqID: %v", err)
	}
	if got.ID != b.ID {
		t.Errorf("id mismatch")
	}
	if _, err := svc.GetBySeqID(999); err == nil {
		t.Error("atteso errore per seq_id inesistente")
	}
}

func TestListByProject(t *testing.T) {
	svc := NewService(newDB(t))
	svc.Create("proj-1", "A", "scrum", nil)
	svc.Create("proj-2", "B", "scrum", nil)
	svc.Create("proj-1", "C", "kanban", nil)
	list, err := svc.ListByProject("proj-1")
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("attese 2 board per proj-1, got %d", len(list))
	}
}

func TestDelete(t *testing.T) {
	svc := NewService(newDB(t))
	b, _ := svc.Create("proj-1", "A", "scrum", nil)
	if err := svc.Delete(b.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.GetBySeqID(b.SeqID); err == nil {
		t.Error("board dovrebbe essere eliminata")
	}
}
```

- [ ] **Step 3: Eseguire i test (falliscono)**

Run: `go test ./internal/domain/board/ -v`
Expected: FAIL con "undefined: NewService".

- [ ] **Step 4: Scrivere il service**

`internal/domain/board/service.go`:

```go
package board

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) DB() *gorm.DB { return s.db }

// Create crea una board assegnando il prossimo seq_id (max+1, da 1).
func (s *Service) Create(projectID, name, boardType string, filterID *string) (*Board, error) {
	var maxSeq int64
	if err := s.db.Model(&Board{}).Select("COALESCE(MAX(seq_id), 0)").Scan(&maxSeq).Error; err != nil {
		return nil, err
	}
	b := &Board{
		ID:        uuid.NewString(),
		SeqID:     maxSeq + 1,
		Name:      name,
		Type:      boardType,
		ProjectID: projectID,
		FilterID:  filterID,
	}
	if err := s.db.Create(b).Error; err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Service) GetBySeqID(seqID int64) (*Board, error) {
	var b Board
	if err := s.db.Where("seq_id = ?", seqID).First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Service) GetByID(id string) (*Board, error) {
	var b Board
	if err := s.db.Where("id = ?", id).First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Service) ListByProject(projectID string) ([]Board, error) {
	var boards []Board
	if err := s.db.Where("project_id = ?", projectID).Order("seq_id ASC").Find(&boards).Error; err != nil {
		return nil, err
	}
	return boards, nil
}

// List restituisce tutte le board con paginazione offset, più il totale.
func (s *Service) List(offset, limit int) ([]Board, int, error) {
	var total int64
	if err := s.db.Model(&Board{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var boards []Board
	q := s.db.Order("seq_id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Find(&boards).Error; err != nil {
		return nil, 0, err
	}
	return boards, int(total), nil
}

func (s *Service) Delete(id string) error {
	return s.db.Where("id = ?", id).Delete(&Board{}).Error
}
```

- [ ] **Step 5: Eseguire i test (passano)**

Run: `go test ./internal/domain/board/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/board/
git commit -m "feat(board): board domain model and service with seq_id"
```

---

### Task 3: Estensione dominio sprint (seq_id, date, originBoard, completeDate)

**Files:**
- Modify: `internal/domain/sprint/model.go`
- Modify: `internal/domain/sprint/service.go`
- Test: `internal/domain/sprint/service_test.go` (creare se non esiste, altrimenti aggiungere)

- [ ] **Step 1: Estendere il modello**

In `internal/domain/sprint/model.go`, aggiungere al `Sprint` (dopo `EndDate`):

```go
	SeqID         int64      `gorm:"column:seq_id;uniqueIndex" json:"seq_id"`
	OriginBoardID *int64     `gorm:"column:origin_board_id" json:"origin_board_id,omitempty"`
	CompleteDate  *time.Time `gorm:"column:complete_date" json:"complete_date,omitempty"`
```

- [ ] **Step 2: Scrivere i test**

`internal/domain/sprint/service_test.go` (se il file esiste già, aggiungere questi test):

```go
package sprint

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&Sprint{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCreateFull_AssignsSeqIDAndFields(t *testing.T) {
	svc := NewService(newDB(t))
	boardID := int64(7)
	start := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 28, 17, 0, 0, 0, time.UTC)
	sp, err := svc.CreateFull("proj-1", "Sprint 1", "ship it", &boardID, &start, &end)
	if err != nil {
		t.Fatalf("CreateFull: %v", err)
	}
	if sp.SeqID != 1 {
		t.Errorf("seq_id atteso 1, got %d", sp.SeqID)
	}
	if sp.OriginBoardID == nil || *sp.OriginBoardID != 7 {
		t.Errorf("originBoardID errato: %v", sp.OriginBoardID)
	}
	if sp.StartDate == nil || !sp.StartDate.Equal(start) {
		t.Errorf("startDate errata")
	}
	if sp.State != StateFuture {
		t.Errorf("stato iniziale atteso future, got %s", sp.State)
	}
}

func TestGetBySeqID(t *testing.T) {
	svc := NewService(newDB(t))
	sp, _ := svc.CreateFull("proj-1", "S1", "", nil, nil, nil)
	got, err := svc.GetBySeqID(sp.SeqID)
	if err != nil {
		t.Fatalf("GetBySeqID: %v", err)
	}
	if got.ID != sp.ID {
		t.Error("id mismatch")
	}
}

func TestComplete_SetsCompleteDate(t *testing.T) {
	svc := NewService(newDB(t))
	sp, _ := svc.CreateFull("proj-1", "S1", "", nil, nil, nil)
	svc.Start(sp.ID)
	done, err := svc.Complete(sp.ID, false)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if done.State != StateClosed {
		t.Errorf("stato atteso closed, got %s", done.State)
	}
	if done.CompleteDate == nil {
		t.Error("completeDate deve essere valorizzata dopo Complete")
	}
}
```

- [ ] **Step 3: Eseguire i test (falliscono)**

Run: `go test ./internal/domain/sprint/ -run 'TestCreateFull|TestGetBySeqID|TestComplete_SetsCompleteDate' -v`
Expected: FAIL con "undefined: CreateFull" / "GetBySeqID".

- [ ] **Step 4: Estendere il service**

In `internal/domain/sprint/service.go`:

(a) aggiungere `CreateFull` (mantenere `Create` esistente per retro-compatibilità, ma farlo delegare):

```go
// CreateFull crea uno sprint con tutti i campi agili, assegnando il seq_id.
func (s *Service) CreateFull(projectID, name, goal string, originBoardID *int64, start, end *time.Time) (*Sprint, error) {
	var maxSeq int64
	if err := s.db.Model(&Sprint{}).Select("COALESCE(MAX(seq_id), 0)").Scan(&maxSeq).Error; err != nil {
		return nil, err
	}
	sp := &Sprint{
		ID:            uuid.NewString(),
		ProjectID:     projectID,
		Name:          name,
		Goal:          goal,
		State:         StateFuture,
		SeqID:         maxSeq + 1,
		OriginBoardID: originBoardID,
		StartDate:     start,
		EndDate:       end,
	}
	if err := s.db.Create(sp).Error; err != nil {
		return nil, err
	}
	return sp, nil
}
```

Assicurarsi che `Create(projectID, name, goal string)` esistente assegni comunque un seq_id: farlo delegare a `CreateFull(projectID, name, goal, nil, nil, nil)`. Aggiungere l'import `"github.com/google/uuid"` se non presente (il vecchio `Create` probabilmente lo usa già).

(b) aggiungere `GetBySeqID`:

```go
func (s *Service) GetBySeqID(seqID int64) (*Sprint, error) {
	var sp Sprint
	if err := s.db.Where("seq_id = ?", seqID).First(&sp).Error; err != nil {
		return nil, err
	}
	return &sp, nil
}
```

(c) in `Complete`, prima di salvare lo stato closed, impostare `CompleteDate`: leggere lo sprint, settare `now := time.Now(); sp.CompleteDate = &now; sp.State = StateClosed` e salvare (mantenere la logica `moveOpenToBacklog` esistente). Se `Complete` usa un `Update` mirato, aggiungere `complete_date` agli update: `updates["complete_date"] = time.Now()`.

(d) aggiungere `UpdateFull` per l'update agile completo:

```go
// UpdateFull aggiorna i campi modificabili di uno sprint (name/goal/state/date).
// I puntatori nil lasciano il campo invariato.
func (s *Service) UpdateFull(id string, name, goal, state *string, start, end *time.Time) (*Sprint, error) {
	var sp Sprint
	if err := s.db.Where("id = ?", id).First(&sp).Error; err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if name != nil {
		updates["name"] = *name
	}
	if goal != nil {
		updates["goal"] = *goal
	}
	if state != nil {
		updates["state"] = *state
		if *state == string(StateClosed) {
			updates["complete_date"] = time.Now()
		}
	}
	if start != nil {
		updates["start_date"] = *start
	}
	if end != nil {
		updates["end_date"] = *end
	}
	if len(updates) > 0 {
		if err := s.db.Model(&sp).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.GetByID(id)
}
```

- [ ] **Step 5: Eseguire i test (passano)**

Run: `go test ./internal/domain/sprint/ -v`
Expected: PASS (nuovi + eventuali esistenti).

- [ ] **Step 6: Commit**

```bash
git add internal/domain/sprint/
git commit -m "feat(sprint): agile fields (seq_id, dates, originBoard, completeDate)"
```

---

### Task 4: Ranking issue nel dominio

**Files:**
- Modify: `internal/domain/issue/service.go`
- Test: `internal/domain/issue/rank_test.go`

- [ ] **Step 1: Scrivere i test**

`internal/domain/issue/rank_test.go`:

```go
package issue

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func rankDB(t *testing.T) *gorm.DB {
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

var rankSeqCounter int64 // seq_id univoco per i test (len(id) collide su id di pari lunghezza)

func mk(t *testing.T, db *gorm.DB, id string, pos float64) {
	t.Helper()
	rankSeqCounter++
	if err := db.Create(&Issue{ID: id, ProjectID: "p", Key: id, Title: id, Position: pos, SeqID: rankSeqCounter}).Error; err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
}

func pos(t *testing.T, db *gorm.DB, id string) float64 {
	t.Helper()
	var iss Issue
	db.First(&iss, "id = ?", id)
	return iss.Position
}

func TestRank_Between(t *testing.T) {
	db := rankDB(t)
	mk(t, db, "a", 100)
	mk(t, db, "b", 200)
	mk(t, db, "x", 0)
	svc := NewService(db)
	after, before := "a", "b"
	if err := svc.Rank([]string{"x"}, &before, &after); err != nil {
		t.Fatalf("Rank: %v", err)
	}
	p := pos(t, db, "x")
	if p <= 100 || p >= 200 {
		t.Errorf("posizione di x deve stare tra 100 e 200, got %v", p)
	}
}

func TestRank_AfterOnly(t *testing.T) {
	db := rankDB(t)
	mk(t, db, "a", 100)
	mk(t, db, "x", 0)
	svc := NewService(db)
	after := "a"
	if err := svc.Rank([]string{"x"}, nil, &after); err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if pos(t, db, "x") <= 100 {
		t.Errorf("x deve stare dopo a (pos>100), got %v", pos(t, db, "x"))
	}
}

func TestRank_EndWhenNoNeighbors(t *testing.T) {
	db := rankDB(t)
	mk(t, db, "a", 100)
	mk(t, db, "x", 0)
	svc := NewService(db)
	if err := svc.Rank([]string{"x"}, nil, nil); err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if pos(t, db, "x") <= 100 {
		t.Errorf("x deve finire in coda (pos>max=100), got %v", pos(t, db, "x"))
	}
}
```

- [ ] **Step 2: Eseguire i test (falliscono)**

Run: `go test ./internal/domain/issue/ -run TestRank -v`
Expected: FAIL con "svc.Rank undefined".

- [ ] **Step 3: Implementare Rank**

Aggiungere in `internal/domain/issue/service.go`:

```go
// Rank riordina le issue indicate posizionandole tra afterID (posizione minore)
// e beforeID (posizione maggiore), usando la colonna Position (float, midpoint).
// Se manca un vicino, inserisce in coda/testa con passo fisso. afterID/beforeID
// sono id interni delle issue di riferimento (già risolti dal chiamante).
func (s *Service) Rank(issueIDs []string, beforeID, afterID *string) error {
	if len(issueIDs) == 0 {
		return nil
	}
	var lo, hi float64
	hasLo, hasHi := false, false
	if afterID != nil {
		var a Issue
		if err := s.db.First(&a, "id = ?", *afterID).Error; err != nil {
			return err
		}
		lo, hasLo = a.Position, true
	}
	if beforeID != nil {
		var b Issue
		if err := s.db.First(&b, "id = ?", *beforeID).Error; err != nil {
			return err
		}
		hi, hasHi = b.Position, true
	}
	n := float64(len(issueIDs))
	var base, step float64
	switch {
	case hasLo && hasHi:
		base = lo
		step = (hi - lo) / (n + 1)
	case hasLo:
		base = lo
		step = 1000
	case hasHi:
		base = hi - 1000*(n+1)
		step = 1000
	default:
		var maxPos float64
		if err := s.db.Model(&Issue{}).Select("COALESCE(MAX(position), 0)").Scan(&maxPos).Error; err != nil {
			return err
		}
		base = maxPos
		step = 1000
	}
	for i, id := range issueIDs {
		p := base + step*float64(i+1)
		if err := s.db.Model(&Issue{}).Where("id = ?", id).Update("position", p).Error; err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Eseguire i test (passano)**

Run: `go test ./internal/domain/issue/ -run TestRank -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/issue/service.go internal/domain/issue/rank_test.go
git commit -m "feat(issue): Rank for agile issue reordering (position midpoint)"
```

---

### Task 5: Renderer issue condiviso + mapper agile

**Files:**
- Create: `internal/api/handlers/render_issues.go`
- Modify: `internal/api/handlers/search_handler.go` (far delegare renderIssues)
- Create: `internal/api/v3/agile.go`
- Test: `internal/api/v3/agile_test.go`

- [ ] **Step 1: Estrarre il renderer condiviso**

`internal/api/handlers/render_issues.go`:

```go
package handlers

import (
	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/issue"
)

// renderIssueList costruisce gli IssueBean (con proiezione dei fields) per una
// lista di issue di dominio, riusando IssueHandler.buildIssueInput. Condiviso tra
// la ricerca (Round 4) e gli endpoint agile (Round 5).
func renderIssueList(issueH *IssueHandler, issues []issue.Issue, fields []string) ([]map[string]any, error) {
	f := v3.ParseFieldsFromList(fields)
	out := make([]map[string]any, 0, len(issues))
	for i := range issues {
		bean := v3.JiraIssue(issueH.buildIssueInput(&issues[i]))
		m, err := v3.ProjectIssue(bean, f)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}
```

In `internal/api/handlers/search_handler.go`, sostituire il corpo di `renderIssues` con una delega (mantenendo la firma del metodo):

```go
func (h *SearchHandler) renderIssues(issues []issue.Issue, fields []string) ([]map[string]any, error) {
	return renderIssueList(h.issueH, issues, fields)
}
```

- [ ] **Step 2: Scrivere i test dei mapper agile**

`internal/api/v3/agile_test.go`:

```go
package v3

import (
	"testing"
	"time"
)

func TestAgileBoard_Shape(t *testing.T) {
	b := AgileBoard(BoardInput{
		SeqID: 3, Name: "Scrum Board", Type: "scrum",
		ProjectID: 10000, ProjectKey: "DEMO", ProjectName: "Demo", ProjectTypeKey: "software",
		BaseURL: "http://x",
	})
	if b.ID != 3 || b.Name != "Scrum Board" || b.Type != "scrum" {
		t.Errorf("campi base errati: %+v", b)
	}
	if b.Self == "" {
		t.Error("self mancante")
	}
	if b.Location == nil || b.Location.ProjectKey != "DEMO" || b.Location.ProjectID != 10000 {
		t.Errorf("location errata: %+v", b.Location)
	}
}

func TestAgileSprint_Shape(t *testing.T) {
	start := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	board := int64(3)
	sp := AgileSprint(SprintInput{
		SeqID: 5, Name: "Sprint 1", State: "active", Goal: "ship",
		OriginBoardID: &board, StartDate: &start, BaseURL: "http://x",
	})
	if sp.ID != 5 || sp.State != "active" || sp.Goal != "ship" {
		t.Errorf("campi base errati: %+v", sp)
	}
	if sp.OriginBoardID != 3 {
		t.Errorf("originBoardId atteso 3, got %d", sp.OriginBoardID)
	}
	if sp.StartDate == "" {
		t.Error("startDate deve essere formattata (non vuota)")
	}
	if sp.Self == "" {
		t.Error("self mancante")
	}
}

func TestAgileSprint_OmitsEmptyDates(t *testing.T) {
	sp := AgileSprint(SprintInput{SeqID: 1, Name: "S", State: "future", BaseURL: "http://x"})
	if sp.StartDate != "" || sp.EndDate != "" || sp.CompleteDate != "" {
		t.Error("date non impostate devono restare stringhe vuote (omitempty)")
	}
}
```

- [ ] **Step 3: Eseguire i test (falliscono)**

Run: `go test ./internal/api/v3/ -run 'TestAgileBoard|TestAgileSprint' -v`
Expected: FAIL con "undefined: AgileBoard".

- [ ] **Step 4: Implementare i mapper**

`internal/api/v3/agile.go`:

```go
package v3

import (
	"fmt"
	"time"
)

// --- Board ---

type BoardLocation struct {
	ProjectID      int64  `json:"projectId"`
	ProjectKey     string `json:"projectKey"`
	ProjectName    string `json:"projectName"`
	ProjectTypeKey string `json:"projectTypeKey"`
	DisplayName    string `json:"displayName"`
	Name           string `json:"name"`
}

type Board struct {
	ID       int64          `json:"id"`
	Self     string         `json:"self"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Location *BoardLocation `json:"location,omitempty"`
}

type BoardInput struct {
	SeqID          int64
	Name           string
	Type           string
	ProjectID      int64
	ProjectKey     string
	ProjectName    string
	ProjectTypeKey string
	BaseURL        string
}

func AgileBoard(in BoardInput) Board {
	return Board{
		ID:   in.SeqID,
		Self: fmt.Sprintf("%s/rest/agile/1.0/board/%d", in.BaseURL, in.SeqID),
		Name: in.Name,
		Type: in.Type,
		Location: &BoardLocation{
			ProjectID:      in.ProjectID,
			ProjectKey:     in.ProjectKey,
			ProjectName:    in.ProjectName,
			ProjectTypeKey: in.ProjectTypeKey,
			DisplayName:    fmt.Sprintf("%s (%s)", in.ProjectName, in.ProjectKey),
			Name:           in.ProjectName,
		},
	}
}

// --- BoardConfiguration ---

type BoardColumnStatus struct {
	ID string `json:"id"`
}

type BoardColumnConfig struct {
	Name     string              `json:"name"`
	Statuses []BoardColumnStatus `json:"statuses"`
}

type BoardConfig struct {
	ID           int64  `json:"id"`
	Self         string `json:"self"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	ColumnConfig struct {
		Columns        []BoardColumnConfig `json:"columns"`
		ConstraintType string              `json:"constraintType"`
	} `json:"columnConfig"`
}

// --- Sprint ---

type Sprint struct {
	ID            int64  `json:"id"`
	Self          string `json:"self"`
	State         string `json:"state"`
	Name          string `json:"name"`
	StartDate     string `json:"startDate,omitempty"`
	EndDate       string `json:"endDate,omitempty"`
	CompleteDate  string `json:"completeDate,omitempty"`
	OriginBoardID int64  `json:"originBoardId,omitempty"`
	Goal          string `json:"goal,omitempty"`
}

type SprintInput struct {
	SeqID         int64
	Name          string
	State         string
	Goal          string
	OriginBoardID *int64
	StartDate     *time.Time
	EndDate       *time.Time
	CompleteDate  *time.Time
	BaseURL       string
}

func AgileSprint(in SprintInput) Sprint {
	sp := Sprint{
		ID:    in.SeqID,
		Self:  fmt.Sprintf("%s/rest/agile/1.0/sprint/%d", in.BaseURL, in.SeqID),
		State: in.State,
		Name:  in.Name,
		Goal:  in.Goal,
	}
	if in.OriginBoardID != nil {
		sp.OriginBoardID = *in.OriginBoardID
	}
	if in.StartDate != nil {
		sp.StartDate = JiraTime(*in.StartDate)
	}
	if in.EndDate != nil {
		sp.EndDate = JiraTime(*in.EndDate)
	}
	if in.CompleteDate != nil {
		sp.CompleteDate = JiraTime(*in.CompleteDate)
	}
	return sp
}
```

- [ ] **Step 5: Eseguire i test (passano)**

Run: `go test ./internal/api/v3/ -run 'TestAgileBoard|TestAgileSprint' -v` poi `go build ./internal/api/handlers/`
Expected: PASS; handlers compila (renderIssues delega a renderIssueList).

- [ ] **Step 6: Commit**

```bash
git add internal/api/handlers/render_issues.go internal/api/handlers/search_handler.go internal/api/v3/agile.go internal/api/v3/agile_test.go
git commit -m "feat(agile): board/sprint v3 mappers and shared issue renderer"
```

---

### Task 6: Handler board — CRUD + configuration

**Files:**
- Create: `internal/api/handlers/agile_board_handler.go`

- [ ] **Step 1: Scrivere l'handler board (CRUD + configuration)**

`internal/api/handlers/agile_board_handler.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/board"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/sprint"
	"github.com/open-jira/open-jira/internal/domain/workflow"
)

type AgileBoardHandler struct {
	boardSvc    *board.Service
	projectSvc  *project.Service
	issueSvc    *issue.Service
	sprintSvc   *sprint.Service
	workflowSvc *workflow.Service
	issueH      *IssueHandler
	baseURL     string
}

func NewAgileBoardHandler(boardSvc *board.Service, projectSvc *project.Service, issueSvc *issue.Service, sprintSvc *sprint.Service, workflowSvc *workflow.Service, issueH *IssueHandler, baseURL string) *AgileBoardHandler {
	return &AgileBoardHandler{boardSvc: boardSvc, projectSvc: projectSvc, issueSvc: issueSvc, sprintSvc: sprintSvc, workflowSvc: workflowSvc, issueH: issueH, baseURL: baseURL}
}

// boardInputFor costruisce il BoardInput risolvendo il progetto della board.
func (h *AgileBoardHandler) boardInputFor(b *board.Board) v3.BoardInput {
	in := v3.BoardInput{SeqID: b.SeqID, Name: b.Name, Type: b.Type, BaseURL: h.baseURL}
	if p, err := h.projectSvc.GetByID(b.ProjectID); err == nil {
		in.ProjectID = p.SeqID
		in.ProjectKey = p.Key
		in.ProjectName = p.Name
		in.ProjectTypeKey = string(project.ProjectTypeKeyForType(p.Type))
	}
	return in
}

// resolveBoard trova la board dal path param boardId (intero seq_id).
func (h *AgileBoardHandler) resolveBoard(r *http.Request) *board.Board {
	n, err := strconv.ParseInt(r.PathValue("boardId"), 10, 64)
	if err != nil {
		return nil
	}
	b, err := h.boardSvc.GetBySeqID(n)
	if err != nil {
		return nil
	}
	return b
}

func (h *AgileBoardHandler) List(w http.ResponseWriter, r *http.Request) {
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	boards, total, err := h.boardSvc.List(startAt, maxResults)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list boards"}, nil)
		return
	}
	values := make([]v3.Board, 0, len(boards))
	for i := range boards {
		values = append(values, v3.AgileBoard(h.boardInputFor(&boards[i])))
	}
	v3.WritePage(w, http.StatusOK, v3.Page[v3.Board]{StartAt: startAt, MaxResults: maxResults, Total: total, Values: values})
}

func (h *AgileBoardHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		Type           string `json:"type"`
		ProjectKeyOrID string `json:"projectKeyOrId"` // scorciatoia nostra
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if req.Name == "" || req.ProjectKeyOrID == "" {
		v3.WriteError(w, http.StatusBadRequest, []string{"name and projectKeyOrId are required"}, nil)
		return
	}
	p, err := h.projectSvc.GetByKey(req.ProjectKeyOrID)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	if req.Type == "" {
		req.Type = "scrum"
	}
	b, err := h.boardSvc.Create(p.ID, req.Name, req.Type, nil)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to create board"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, v3.AgileBoard(h.boardInputFor(b)))
}

func (h *AgileBoardHandler) Get(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.AgileBoard(h.boardInputFor(b)))
}

func (h *AgileBoardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	if err := h.boardSvc.Delete(b.ID); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete board"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AgileBoardHandler) Configuration(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	cfg := v3.BoardConfig{
		ID:   b.SeqID,
		Self: h.baseURL + "/rest/agile/1.0/board/" + strconv.FormatInt(b.SeqID, 10) + "/configuration",
		Name: b.Name,
		Type: b.Type,
	}
	cfg.ColumnConfig.ConstraintType = "none"
	if wf, err := h.workflowSvc.GetWorkflow(b.ProjectID); err == nil {
		sort.Slice(wf.Statuses, func(i, j int) bool { return wf.Statuses[i].Position < wf.Statuses[j].Position })
		for _, st := range wf.Statuses {
			cfg.ColumnConfig.Columns = append(cfg.ColumnConfig.Columns, v3.BoardColumnConfig{
				Name:     st.Name,
				Statuses: []v3.BoardColumnStatus{{ID: st.ID}},
			})
		}
	}
	v3.WriteJSON(w, http.StatusOK, cfg)
}
```

> **Nota implementatore:** verificare le firme reali: `project.Service.GetByID(id)` e `GetByKey(keyOrID)` (una potrebbe risolvere id-o-key: vedi `internal/domain/project/service.go`); `project.ProjectTypeKeyForType(t) ...` (tipo di ritorno — potrebbe essere string già). `workflow.Service.GetWorkflow(projectID)`. `v3.Page[T]`/`v3.WritePage` generics: la chiamata `v3.WritePage(w, status, v3.Page[v3.Board]{...})` deve combaciare con la firma reale (dal Round 4). Adeguare se `WritePage` richiede tipi diversi.

- [ ] **Step 2: Verificare la compilazione**

Run: `go build ./internal/api/handlers/`
Expected: compila (il router non è ancora cablato — normale, ma il package handlers deve compilare).

- [ ] **Step 3: Commit**

```bash
git add internal/api/handlers/agile_board_handler.go
git commit -m "feat(agile): board CRUD and configuration handlers"
```

---

### Task 7: Handler board — liste issue (backlog/issue/sprint/epic)

**Files:**
- Modify: `internal/api/handlers/agile_board_handler.go` (aggiungere metodi)

- [ ] **Step 1: Aggiungere i metodi di listing**

Aggiungere in `internal/api/handlers/agile_board_handler.go`:

```go
// writeIssuePage scrive una lista di issue nello shape SearchResults (issues+total).
func (h *AgileBoardHandler) writeIssuePage(w http.ResponseWriter, issues []issue.Issue, startAt, maxResults, total int) {
	items, err := renderIssueList(h.issueH, issues, nil)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"render error"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.SearchResults{Issues: items, StartAt: startAt, MaxResults: maxResults, Total: total})
}

// page applica la paginazione offset a uno slice di issue.
func page(issues []issue.Issue, startAt, maxResults int) []issue.Issue {
	if startAt > len(issues) {
		return []issue.Issue{}
	}
	end := startAt + maxResults
	if end > len(issues) {
		end = len(issues)
	}
	return issues[startAt:end]
}

// BoardIssues: tutte le issue del progetto della board.
func (h *AgileBoardHandler) BoardIssues(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	all, err := h.issueSvc.ListByProject(b.ProjectID, issue.WithNotArchived())
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list issues"}, nil)
		return
	}
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	h.writeIssuePage(w, page(all, startAt, maxResults), startAt, maxResults, len(all))
}

// Backlog: issue del progetto senza sprint (sprint_id IS NULL).
func (h *AgileBoardHandler) Backlog(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	all, err := h.issueSvc.ListByProject(b.ProjectID, issue.WithNotArchived())
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list issues"}, nil)
		return
	}
	var backlog []issue.Issue
	for _, iss := range all {
		if iss.SprintID == nil {
			backlog = append(backlog, iss)
		}
	}
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	h.writeIssuePage(w, page(backlog, startAt, maxResults), startAt, maxResults, len(backlog))
}

// BoardSprints: sprint del progetto della board (shape values+isLast).
func (h *AgileBoardHandler) BoardSprints(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	sprints, err := h.sprintSvc.ListByProject(b.ProjectID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list sprints"}, nil)
		return
	}
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	values := make([]v3.Sprint, 0, len(sprints))
	for i := range sprints {
		values = append(values, sprintToV3(&sprints[i], h.baseURL))
	}
	v3.WritePage(w, http.StatusOK, v3.Page[v3.Sprint]{StartAt: startAt, MaxResults: maxResults, Total: len(values), Values: values})
}

// BoardEpics: issue di tipo Epic nel progetto (shape values+isLast, minimal).
func (h *AgileBoardHandler) BoardEpics(w http.ResponseWriter, r *http.Request) {
	b := h.resolveBoard(r)
	if b == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"board not found"}, nil)
		return
	}
	all, err := h.issueSvc.ListByProject(b.ProjectID, issue.WithNotArchived())
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list issues"}, nil)
		return
	}
	epics := make([]map[string]any, 0)
	for i := range all {
		if h.issueH.isEpic(&all[i]) {
			epics = append(epics, map[string]any{
				"id":      all[i].SeqID,
				"key":     all[i].Key,
				"name":    all[i].Title,
				"summary": all[i].Title,
				"done":    false,
			})
		}
	}
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	lo := startAt
	if lo > len(epics) {
		lo = len(epics)
	}
	hi := lo + maxResults
	if hi > len(epics) {
		hi = len(epics)
	}
	v3.WriteJSON(w, http.StatusOK, map[string]any{
		"startAt": startAt, "maxResults": maxResults, "total": len(epics),
		"isLast": hi >= len(epics), "values": epics[lo:hi],
	})
}
```

Aggiungere il helper `sprintToV3` (usato anche in Task 8) in `agile_board_handler.go`:

```go
// sprintToV3 mappa uno sprint di dominio nella Sprint agile.
func sprintToV3(sp *sprint.Sprint, baseURL string) v3.Sprint {
	return v3.AgileSprint(v3.SprintInput{
		SeqID: sp.SeqID, Name: sp.Name, State: string(sp.State), Goal: sp.Goal,
		OriginBoardID: sp.OriginBoardID, StartDate: sp.StartDate, EndDate: sp.EndDate,
		CompleteDate: sp.CompleteDate, BaseURL: baseURL,
	})
}
```

Aggiungere il helper `isEpic` in `internal/api/handlers/issue_handler.go` (o come metodo qui): determina se una issue è un Epic dal nome del suo IssueType.

```go
// isEpic verifica se la issue è di tipo "Epic" (case-insensitive) risolvendo il type.
func (h *IssueHandler) isEpic(iss *issue.Issue) bool {
	if iss.TypeID == nil {
		return false
	}
	var it issue.IssueType
	if h.issueSvc.DB().First(&it, "id = ?", *iss.TypeID).Error != nil {
		return false
	}
	return strings.EqualFold(it.Name, "Epic")
}
```

> **Nota implementatore:** verificare come `IssueHandler` accede al DB/type (in `issue_handler.go` `buildIssueInput` fa già lookup del type — riusare quel pattern; il campo service è probabilmente `h.issueSvc` o simile — adeguare). Aggiungere l'import `strings` se serve. Verificare `issue.IssueType{ID,Name}`.

- [ ] **Step 2: Verificare la compilazione**

Run: `go build ./internal/api/handlers/`
Expected: compila.

- [ ] **Step 3: Commit**

```bash
git add internal/api/handlers/agile_board_handler.go internal/api/handlers/issue_handler.go
git commit -m "feat(agile): board backlog/issues/sprints/epics listings"
```

---

### Task 8: Handler sprint — CRUD + issue + rank + backlog + agile issue/epic get

**Files:**
- Create: `internal/api/handlers/agile_sprint_handler.go`

- [ ] **Step 1: Scrivere l'handler**

`internal/api/handlers/agile_sprint_handler.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/board"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/sprint"
)

type AgileSprintHandler struct {
	sprintSvc *sprint.Service
	boardSvc  *board.Service
	issueSvc  *issue.Service
	issueH    *IssueHandler
	baseURL   string
}

func NewAgileSprintHandler(sprintSvc *sprint.Service, boardSvc *board.Service, issueSvc *issue.Service, issueH *IssueHandler, baseURL string) *AgileSprintHandler {
	return &AgileSprintHandler{sprintSvc: sprintSvc, boardSvc: boardSvc, issueSvc: issueSvc, issueH: issueH, baseURL: baseURL}
}

func (h *AgileSprintHandler) resolveSprint(r *http.Request) *sprint.Sprint {
	n, err := strconv.ParseInt(r.PathValue("sprintId"), 10, 64)
	if err != nil {
		return nil
	}
	sp, err := h.sprintSvc.GetBySeqID(n)
	if err != nil {
		return nil
	}
	return sp
}

// parseAgileTime interpreta le date del contratto (RFC3339 / ISO8601). Vuoto → nil.
func parseAgileTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.000-07:00", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func (h *AgileSprintHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name"`
		Goal          string `json:"goal"`
		StartDate     string `json:"startDate"`
		EndDate       string `json:"endDate"`
		OriginBoardID int64  `json:"originBoardId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if req.Name == "" || req.OriginBoardID == 0 {
		v3.WriteError(w, http.StatusBadRequest, []string{"name and originBoardId are required"}, nil)
		return
	}
	b, err := h.boardSvc.GetBySeqID(req.OriginBoardID)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"originBoardId not found"}, nil)
		return
	}
	obid := req.OriginBoardID
	sp, err := h.sprintSvc.CreateFull(b.ProjectID, req.Name, req.Goal, &obid, parseAgileTime(req.StartDate), parseAgileTime(req.EndDate))
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to create sprint"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, sprintToV3(sp, h.baseURL))
}

func (h *AgileSprintHandler) Get(w http.ResponseWriter, r *http.Request) {
	sp := h.resolveSprint(r)
	if sp == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"sprint not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, sprintToV3(sp, h.baseURL))
}

// Update gestisce POST/PUT /sprint/{id}: aggiorna campi + transizioni di stato.
func (h *AgileSprintHandler) Update(w http.ResponseWriter, r *http.Request) {
	sp := h.resolveSprint(r)
	if sp == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"sprint not found"}, nil)
		return
	}
	var req struct {
		Name      *string `json:"name"`
		Goal      *string `json:"goal"`
		State     *string `json:"state"`
		StartDate *string `json:"startDate"`
		EndDate   *string `json:"endDate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	// Le transizioni active/closed passano per Start/Complete (side-effect su issue).
	if req.State != nil {
		switch *req.State {
		case "active":
			if _, err := h.sprintSvc.Start(sp.ID); err != nil {
				v3.WriteError(w, http.StatusBadRequest, []string{"cannot start sprint"}, nil)
				return
			}
		case "closed":
			if _, err := h.sprintSvc.Complete(sp.ID, true); err != nil {
				v3.WriteError(w, http.StatusBadRequest, []string{"cannot complete sprint"}, nil)
				return
			}
		}
	}
	var start, end *time.Time
	if req.StartDate != nil {
		start = parseAgileTime(*req.StartDate)
	}
	if req.EndDate != nil {
		end = parseAgileTime(*req.EndDate)
	}
	// name/goal/date (lo stato è già gestito sopra: non ripassarlo a UpdateFull)
	updated, err := h.sprintSvc.UpdateFull(sp.ID, req.Name, req.Goal, nil, start, end)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to update sprint"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, sprintToV3(updated, h.baseURL))
}

func (h *AgileSprintHandler) Delete(w http.ResponseWriter, r *http.Request) {
	sp := h.resolveSprint(r)
	if sp == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"sprint not found"}, nil)
		return
	}
	if err := h.sprintSvc.DB().Where("id = ?", sp.ID).Delete(&sprint.Sprint{}).Error; err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete sprint"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SprintIssues: GET issue dello sprint (SearchResults).
func (h *AgileSprintHandler) SprintIssues(w http.ResponseWriter, r *http.Request) {
	sp := h.resolveSprint(r)
	if sp == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"sprint not found"}, nil)
		return
	}
	var issues []issue.Issue
	if err := h.issueSvc.DB().Where("sprint_id = ? AND is_archived = ?", sp.ID, false).Order("position ASC").Find(&issues).Error; err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list issues"}, nil)
		return
	}
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	items, err := renderIssueList(h.issueH, page(issues, startAt, maxResults), nil)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"render error"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.SearchResults{Issues: items, StartAt: startAt, MaxResults: maxResults, Total: len(issues)})
}

// MoveToSprint: POST /sprint/{id}/issue — sposta issue nello sprint.
func (h *AgileSprintHandler) MoveToSprint(w http.ResponseWriter, r *http.Request) {
	sp := h.resolveSprint(r)
	if sp == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"sprint not found"}, nil)
		return
	}
	var req struct {
		Issues []string `json:"issues"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	for _, key := range req.Issues {
		iss := h.resolveIssue(key)
		if iss == nil {
			continue
		}
		if err := h.sprintSvc.AddIssue(sp.ID, iss.ID); err != nil {
			v3.WriteError(w, http.StatusInternalServerError, []string{"failed to move issue"}, nil)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// resolveIssue risolve una issue da id numerico (seq_id) o key.
func (h *AgileSprintHandler) resolveIssue(idOrKey string) *issue.Issue {
	if n, err := strconv.ParseInt(idOrKey, 10, 64); err == nil {
		if iss, err := h.issueSvc.GetBySeqID(n); err == nil {
			return iss
		}
		return nil
	}
	iss, err := h.issueSvc.GetByKey(idOrKey)
	if err != nil {
		return nil
	}
	return iss
}
```

- [ ] **Step 2: Verificare la compilazione**

Run: `go build ./internal/api/handlers/`
Expected: compila.

- [ ] **Step 3: Commit**

```bash
git add internal/api/handlers/agile_sprint_handler.go
git commit -m "feat(agile): sprint CRUD, issue listing and move-to-sprint"
```

---

### Task 9: Handler rank + backlog move + agile issue/epic get

**Files:**
- Create: `internal/api/handlers/agile_misc_handler.go`

- [ ] **Step 1: Scrivere l'handler rank/backlog/issue/epic**

`internal/api/handlers/agile_misc_handler.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/sprint"
)

type AgileMiscHandler struct {
	issueSvc  *issue.Service
	sprintSvc *sprint.Service
	issueH    *IssueHandler
	baseURL   string
}

func NewAgileMiscHandler(issueSvc *issue.Service, sprintSvc *sprint.Service, issueH *IssueHandler, baseURL string) *AgileMiscHandler {
	return &AgileMiscHandler{issueSvc: issueSvc, sprintSvc: sprintSvc, issueH: issueH, baseURL: baseURL}
}

func (h *AgileMiscHandler) resolveIssueID(idOrKey string) (string, bool) {
	if n, err := strconv.ParseInt(idOrKey, 10, 64); err == nil {
		if iss, err := h.issueSvc.GetBySeqID(n); err == nil {
			return iss.ID, true
		}
		return "", false
	}
	iss, err := h.issueSvc.GetByKey(idOrKey)
	if err != nil {
		return "", false
	}
	return iss.ID, true
}

// Rank: PUT /rest/agile/1.0/issue/rank.
func (h *AgileMiscHandler) Rank(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Issues          []string `json:"issues"`
		RankBeforeIssue string   `json:"rankBeforeIssue"`
		RankAfterIssue  string   `json:"rankAfterIssue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if len(req.Issues) == 0 {
		v3.WriteError(w, http.StatusBadRequest, []string{"issues is required"}, nil)
		return
	}
	ids := make([]string, 0, len(req.Issues))
	for _, k := range req.Issues {
		if id, ok := h.resolveIssueID(k); ok {
			ids = append(ids, id)
		}
	}
	var before, after *string
	if req.RankBeforeIssue != "" {
		if id, ok := h.resolveIssueID(req.RankBeforeIssue); ok {
			before = &id
		}
	}
	if req.RankAfterIssue != "" {
		if id, ok := h.resolveIssueID(req.RankAfterIssue); ok {
			after = &id
		}
	}
	if err := h.issueSvc.Rank(ids, before, after); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to rank issues"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MoveToBacklog: POST /rest/agile/1.0/backlog/issue — rimuove le issue dallo sprint.
func (h *AgileMiscHandler) MoveToBacklog(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Issues []string `json:"issues"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	for _, k := range req.Issues {
		if id, ok := h.resolveIssueID(k); ok {
			if err := h.sprintSvc.RemoveIssue(id); err != nil {
				v3.WriteError(w, http.StatusInternalServerError, []string{"failed to move to backlog"}, nil)
				return
			}
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetIssue: GET /rest/agile/1.0/issue/{issueIdOrKey} — IssueBean con extra sprint.
func (h *AgileMiscHandler) GetIssue(w http.ResponseWriter, r *http.Request) {
	iss := h.resolveIssue(r.PathValue("issueIdOrKey"))
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	bean := v3.JiraIssue(h.issueH.buildIssueInput(iss))
	m, err := v3.ProjectIssue(bean, v3.ParseFieldsFromList(nil))
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"render error"}, nil)
		return
	}
	// arricchisce fields.sprint se la issue è in uno sprint
	if iss.SprintID != nil {
		if sp, err := h.sprintSvc.GetByID(*iss.SprintID); err == nil {
			if fields, ok := m["fields"].(map[string]any); ok {
				fields["sprint"] = sprintToV3(sp, h.baseURL)
			}
		}
	}
	v3.WriteJSON(w, http.StatusOK, m)
}

// GetEpic: GET /rest/agile/1.0/epic/{epicIdOrKey} — shape epic minimale.
func (h *AgileMiscHandler) GetEpic(w http.ResponseWriter, r *http.Request) {
	iss := h.resolveIssue(r.PathValue("epicIdOrKey"))
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"epic not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, map[string]any{
		"id":      iss.SeqID,
		"key":     iss.Key,
		"self":    h.baseURL + "/rest/agile/1.0/epic/" + iss.Key,
		"name":    iss.Title,
		"summary": iss.Title,
		"done":    strings.EqualFold(deref(iss.StatusID), ""), // placeholder: done=false salvo status risolto
	})
}

func (h *AgileMiscHandler) resolveIssue(idOrKey string) *issue.Issue {
	if n, err := strconv.ParseInt(idOrKey, 10, 64); err == nil {
		if iss, err := h.issueSvc.GetBySeqID(n); err == nil {
			return iss
		}
		return nil
	}
	iss, err := h.issueSvc.GetByKey(idOrKey)
	if err != nil {
		return nil
	}
	return iss
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
```

> **Nota implementatore:** il campo `done` dell'epic è semplificato a `false` (la valutazione reale = status category "done"); questo è coerente con lo scope "epic read-only minimal" dichiarato nel piano. Se preferisci, calcola `done` risolvendo lo statusCategory del `StatusID` via workflow. La riga `strings.EqualFold(deref(iss.StatusID), "")` produce sempre `false` se lo status è impostato — sostituiscila con `false` diretto e rimuovi l'import `strings`/`deref` se non altrimenti usati (mantieni il codice pulito: preferisci `"done": false`).

- [ ] **Step 2: Semplificare done (pulizia)**

Sostituire la riga `"done": strings.EqualFold(...)` con `"done": false` e rimuovere import/helper non più usati (`strings`, `deref`) se restano orfani. Verificare con `go vet`.

- [ ] **Step 3: Verificare la compilazione**

Run: `go build ./internal/api/handlers/ && go vet ./internal/api/handlers/`
Expected: compila, vet pulito.

- [ ] **Step 4: Commit**

```bash
git add internal/api/handlers/agile_misc_handler.go
git commit -m "feat(agile): issue rank, move-to-backlog, agile issue and epic get"
```

---

### Task 10: Router — montare /rest/agile/1.0/*

**Files:**
- Modify: `internal/api/router.go`

- [ ] **Step 1: Costruire i servizi/handler agile**

Nel blocco di costruzione handler di `internal/api/router.go` (vicino agli altri), aggiungere. VERIFICARE i nomi reali delle variabili già presenti (`issueSvc`, `projectSvc`, `workflowSvc`, `sprintSvc`, `issueH`, `db`, `cfg.BaseURL`) — lo sprint service è già costruito per le rotte custom; riusarlo.

```go
	boardSvc := board.NewService(db)
	agileBoardH := handlers.NewAgileBoardHandler(boardSvc, projectSvc, issueSvc, sprintSvc, workflowSvc, issueH, cfg.BaseURL)
	agileSprintH := handlers.NewAgileSprintHandler(sprintSvc, boardSvc, issueSvc, issueH, cfg.BaseURL)
	agileMiscH := handlers.NewAgileMiscHandler(issueSvc, sprintSvc, issueH, cfg.BaseURL)
```

Aggiungere l'import `"github.com/open-jira/open-jira/internal/domain/board"`.

- [ ] **Step 2: Registrare le rotte agile**

Aggiungere un nuovo blocco rotte (stile identico alle altre `mux.Handle(... authMw(...))`):

```go
	// --- Agile API 1.0 (Round 5) ---
	mux.Handle("GET /rest/agile/1.0/board", authMw(http.HandlerFunc(agileBoardH.List)))
	mux.Handle("POST /rest/agile/1.0/board", authMw(http.HandlerFunc(agileBoardH.Create)))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}", authMw(http.HandlerFunc(agileBoardH.Get)))
	mux.Handle("DELETE /rest/agile/1.0/board/{boardId}", authMw(http.HandlerFunc(agileBoardH.Delete)))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/configuration", authMw(http.HandlerFunc(agileBoardH.Configuration)))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/backlog", authMw(http.HandlerFunc(agileBoardH.Backlog)))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/issue", authMw(http.HandlerFunc(agileBoardH.BoardIssues)))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/sprint", authMw(http.HandlerFunc(agileBoardH.BoardSprints)))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/epic", authMw(http.HandlerFunc(agileBoardH.BoardEpics)))

	mux.Handle("POST /rest/agile/1.0/sprint", authMw(http.HandlerFunc(agileSprintH.Create)))
	mux.Handle("GET /rest/agile/1.0/sprint/{sprintId}", authMw(http.HandlerFunc(agileSprintH.Get)))
	mux.Handle("POST /rest/agile/1.0/sprint/{sprintId}", authMw(http.HandlerFunc(agileSprintH.Update)))
	mux.Handle("PUT /rest/agile/1.0/sprint/{sprintId}", authMw(http.HandlerFunc(agileSprintH.Update)))
	mux.Handle("DELETE /rest/agile/1.0/sprint/{sprintId}", authMw(http.HandlerFunc(agileSprintH.Delete)))
	mux.Handle("GET /rest/agile/1.0/sprint/{sprintId}/issue", authMw(http.HandlerFunc(agileSprintH.SprintIssues)))
	mux.Handle("POST /rest/agile/1.0/sprint/{sprintId}/issue", authMw(http.HandlerFunc(agileSprintH.MoveToSprint)))

	mux.Handle("PUT /rest/agile/1.0/issue/rank", authMw(http.HandlerFunc(agileMiscH.Rank)))
	mux.Handle("GET /rest/agile/1.0/issue/{issueIdOrKey}", authMw(http.HandlerFunc(agileMiscH.GetIssue)))
	mux.Handle("POST /rest/agile/1.0/backlog/issue", authMw(http.HandlerFunc(agileMiscH.MoveToBacklog)))
	mux.Handle("GET /rest/agile/1.0/epic/{epicIdOrKey}", authMw(http.HandlerFunc(agileMiscH.GetEpic)))
```

> **Nota ServeMux:** `PUT /rest/agile/1.0/issue/rank` (segmento letterale `rank`) e `GET /rest/agile/1.0/issue/{issueIdOrKey}` coesistono senza conflitto (metodi diversi + specificità letterale). Verificare che il build passi.

- [ ] **Step 3: Gate build/vet/test**

Run: `go build ./... && go vet ./... && go test ./... 2>&1 | grep -vE '^ok|no test files'`
Expected: build+vet ok; nessun FAIL.

- [ ] **Step 4: Smoke test manuale (opzionale ma consigliato)**

Run:
```bash
rm -f /tmp/r5.db && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/r5.db go run ./cmd/seed >/dev/null 2>&1
APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/r5.db go run ./cmd/server & SRV=$!; sleep 3
TOK=$(curl -s -X POST localhost:8080/rest/api/3/auth/login -H 'Content-Type: application/json' -d '{"email":"admin@example.com","password":"admin-demo-123"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')
curl -s -X POST localhost:8080/rest/agile/1.0/board -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -d '{"name":"Demo Board","type":"scrum","projectKeyOrId":"DEMO"}' | python3 -m json.tool
kill $SRV; rm -f /tmp/r5.db
```
Expected: JSON board con `id`, `self`, `location.projectKey="DEMO"`.

- [ ] **Step 5: Commit**

```bash
git add internal/api/router.go
git commit -m "feat(agile): mount /rest/agile/1.0 board/sprint/backlog/rank routes"
```

---

### Task 11: Contract test — Agile 1.0

**Files:**
- Create: `internal/contract/agile_test.go`

- [ ] **Step 1: Scrivere i contract test**

`internal/contract/agile_test.go` (leggere `internal/contract/search_test.go` e l'harness per le firme REALI di `newTestServer`/`registerAndLogin`/`createProjectViaAPI`/`createIssueViaAPI`/`MustLoad`/`ValidateResponse` e per gli helper di richiesta; adattare i nomi). Caricare lo spec agile con `MustLoad(t, "../../docs/contracts/jira-agile-1.0.json")`.

```go
package contract

import (
	"net/http"
	"testing"
)

func TestAgileBoardAndSprint(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, tok, "AG", "Agile Proj")
	issueKey := createIssueViaAPI(t, srv, tok, "AG", "Backlog item")

	agile := mustLoad(t, "../../docs/contracts/jira-agile-1.0.json") // adattare a MustLoad reale

	// create board
	resp := doJSON(t, srv, http.MethodPost, tok, "/rest/agile/1.0/board", map[string]any{
		"name": "AG Board", "type": "scrum", "projectKeyOrId": "AG",
	})
	assertStatus(t, resp, http.StatusCreated)
	board := decodeJSON(t, resp)
	boardID := int64(board["id"].(float64))
	if boardID == 0 {
		t.Fatal("board id mancante")
	}

	// list boards
	resp = doJSON(t, srv, http.MethodGet, tok, "/rest/agile/1.0/board", nil)
	assertStatus(t, resp, http.StatusOK)
	agile.ValidateResponse(http.MethodGet, "/rest/agile/1.0/board", http.StatusOK, resp.Header, resp.Body)

	// board backlog (SearchResults: issues+total)
	resp = doJSON(t, srv, http.MethodGet, tok, itoaPath("/rest/agile/1.0/board/", boardID, "/backlog"), nil)
	assertStatus(t, resp, http.StatusOK)
	body := decodeJSON(t, resp)
	if _, ok := body["issues"]; !ok {
		t.Error("backlog deve avere il campo issues")
	}

	// board configuration
	resp = doJSON(t, srv, http.MethodGet, tok, itoaPath("/rest/agile/1.0/board/", boardID, "/configuration"), nil)
	assertStatus(t, resp, http.StatusOK)

	// create sprint on the board
	resp = doJSON(t, srv, http.MethodPost, tok, "/rest/agile/1.0/sprint", map[string]any{
		"name": "Sprint 1", "originBoardId": boardID, "goal": "ship",
	})
	assertStatus(t, resp, http.StatusCreated)
	sp := decodeJSON(t, resp)
	sprintID := int64(sp["id"].(float64))

	// move issue to sprint
	resp = doJSON(t, srv, http.MethodPost, tok, itoaPath("/rest/agile/1.0/sprint/", sprintID, "/issue"), map[string]any{
		"issues": []string{issueKey},
	})
	assertStatus(t, resp, http.StatusNoContent)

	// sprint issues now contains it
	resp = doJSON(t, srv, http.MethodGet, tok, itoaPath("/rest/agile/1.0/sprint/", sprintID, "/issue"), nil)
	assertStatus(t, resp, http.StatusOK)
	sissues := decodeJSON(t, resp)
	arr, _ := sissues["issues"].([]any)
	if len(arr) != 1 {
		t.Errorf("sprint deve contenere 1 issue, got %d", len(arr))
	}

	// move back to backlog
	resp = doJSON(t, srv, http.MethodPost, tok, "/rest/agile/1.0/backlog/issue", map[string]any{
		"issues": []string{issueKey},
	})
	assertStatus(t, resp, http.StatusNoContent)

	// rank
	resp = doJSON(t, srv, http.MethodPut, tok, "/rest/agile/1.0/issue/rank", map[string]any{
		"issues": []string{issueKey},
	})
	assertStatus(t, resp, http.StatusNoContent)

	// start then complete sprint
	resp = doJSON(t, srv, http.MethodPost, tok, itoaPath("/rest/agile/1.0/sprint/", sprintID, ""), map[string]any{"state": "active"})
	assertStatus(t, resp, http.StatusOK)
	resp = doJSON(t, srv, http.MethodPost, tok, itoaPath("/rest/agile/1.0/sprint/", sprintID, ""), map[string]any{"state": "closed"})
	assertStatus(t, resp, http.StatusOK)
	closed := decodeJSON(t, resp)
	if closed["state"] != "closed" {
		t.Errorf("sprint deve essere closed, got %v", closed["state"])
	}
}
```

> **Nota implementatore:** gli helper `doJSON`/`assertStatus`/`decodeJSON`/`itoaPath`/`mustLoad` sono PLACEHOLDER — mappare a quelli reali dell'harness (dal Round 4 esiste `doJSON`/`assertStatus`/`decodeJSON`; `itoaPath` è banale: `fmt.Sprintf("%s%d%s", a, id, b)` — definirlo localmente se serve). Per la validazione OpenAPI dello spec agile, molte operazioni hanno solo risposta `default` → `ValidateResponse` potrebbe essere lenient; chiama comunque `ValidateResponse` sulle GET con schema (board list, se lo spec lo definisce) e per le altre asserisci gli status + campi chiave. Se `ValidateResponse` con un secondo spec richiede un secondo `Validator`, caricalo con `MustLoad` e usalo.

- [ ] **Step 2: Eseguire i contract test**

Run: `go test ./internal/contract/ -run TestAgile -v`
Expected: PASS. Se una validazione fallisce, correggere il mapper/handler.

- [ ] **Step 3: Suite completa**

Run: `go test ./...`
Expected: verde.

- [ ] **Step 4: Commit**

```bash
git add internal/contract/agile_test.go
git commit -m "test(contract): agile board, sprint, backlog, rank conformance"
```

---

### Task 12: Frontend — dnd-kit + client API agile

**Files:**
- Modify: `frontend-next/package.json` (dep)
- Modify: `frontend-next/lib/api.ts`

- [ ] **Step 1: Installare dnd-kit**

Run: `cd frontend-next && npm install @dnd-kit/core@^6 @dnd-kit/sortable@^8`
Expected: aggiunge le dipendenze a package.json + package-lock.json.

- [ ] **Step 2: Aggiungere il client agile**

In `frontend-next/lib/api.ts`, aggiungere (usando il wrapper reale `apiFetch<T>` e la convenzione degli altri oggetti):

```ts
export interface AgileBoard {
  id: number;
  name: string;
  type: string;
  location?: { projectKey: string; projectName: string };
}

export interface AgileSprint {
  id: number;
  name: string;
  state: "future" | "active" | "closed";
  goal?: string;
  startDate?: string;
  endDate?: string;
}

// Le liste issue agili usano lo shape SearchResults (issues+total).
export interface AgileIssueList {
  issues: SearchIssue[];
  total: number;
}

export const boards = {
  list: () => apiFetch<{ values: AgileBoard[] }>("/rest/agile/1.0/board"),
  create: (name: string, projectKeyOrId: string, type = "scrum") =>
    apiFetch<AgileBoard>("/rest/agile/1.0/board", {
      method: "POST",
      body: JSON.stringify({ name, projectKeyOrId, type }),
    }),
  get: (boardId: number) => apiFetch<AgileBoard>(`/rest/agile/1.0/board/${boardId}`),
  issues: (boardId: number) => apiFetch<AgileIssueList>(`/rest/agile/1.0/board/${boardId}/issue`),
  backlog: (boardId: number) => apiFetch<AgileIssueList>(`/rest/agile/1.0/board/${boardId}/backlog`),
  sprints: (boardId: number) => apiFetch<{ values: AgileSprint[] }>(`/rest/agile/1.0/board/${boardId}/sprint`),
  configuration: (boardId: number) =>
    apiFetch<{ columnConfig: { columns: { name: string; statuses: { id: string }[] }[] } }>(
      `/rest/agile/1.0/board/${boardId}/configuration`,
    ),
};

export const sprints = {
  create: (name: string, originBoardId: number, goal?: string) =>
    apiFetch<AgileSprint>("/rest/agile/1.0/sprint", {
      method: "POST",
      body: JSON.stringify({ name, originBoardId, goal }),
    }),
  issues: (sprintId: number) => apiFetch<AgileIssueList>(`/rest/agile/1.0/sprint/${sprintId}/issue`),
  setState: (sprintId: number, state: "active" | "closed") =>
    apiFetch<AgileSprint>(`/rest/agile/1.0/sprint/${sprintId}`, {
      method: "POST",
      body: JSON.stringify({ state }),
    }),
  moveIssues: (sprintId: number, issues: string[]) =>
    apiFetch<void>(`/rest/agile/1.0/sprint/${sprintId}/issue`, {
      method: "POST",
      body: JSON.stringify({ issues }),
    }),
};

export const agileIssues = {
  moveToBacklog: (issues: string[]) =>
    apiFetch<void>("/rest/agile/1.0/backlog/issue", { method: "POST", body: JSON.stringify({ issues }) }),
  rank: (issues: string[], rankBeforeIssue?: string, rankAfterIssue?: string) =>
    apiFetch<void>("/rest/agile/1.0/issue/rank", {
      method: "PUT",
      body: JSON.stringify({ issues, rankBeforeIssue, rankAfterIssue }),
    }),
};
```

> **Nota:** `SearchIssue` esiste già da Round 4. Verificare il nome reale del wrapper (`apiFetch`) e la convenzione.

- [ ] **Step 3: Type-check + build**

Run: `cd frontend-next && npx tsc --noEmit && npm run build`
Expected: nessun errore.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/package.json frontend-next/package-lock.json frontend-next/lib/api.ts
git commit -m "feat(frontend): dnd-kit dependency and agile API client"
```

---

### Task 13: Frontend — pagina board a colonne (drag&drop)

**Files:**
- Create: `frontend-next/components/board/BoardColumns.tsx`
- Create: `frontend-next/app/jira/boards/[boardId]/page.tsx`

- [ ] **Step 1: Componente colonne board con dnd**

`frontend-next/components/board/BoardColumns.tsx`:

```tsx
"use client";

import { DndContext, DragEndEvent, useDraggable, useDroppable } from "@dnd-kit/core";
import type { SearchIssue } from "@/lib/api";

interface Column {
  id: string;
  name: string;
}

// Card issue trascinabile.
function IssueCard({ issue }: { issue: SearchIssue }) {
  const { attributes, listeners, setNodeRef, transform } = useDraggable({ id: issue.key });
  const style = transform ? { transform: `translate(${transform.x}px, ${transform.y}px)` } : undefined;
  return (
    <div
      ref={setNodeRef}
      style={style}
      {...listeners}
      {...attributes}
      className="mb-2 cursor-grab rounded border border-slate-200 bg-white p-2 text-sm shadow-sm"
      data-testid={`card-${issue.key}`}
    >
      <div className="font-mono text-xs text-slate-500">{issue.key}</div>
      <div className="text-[#1a1f36]">{issue.fields.summary}</div>
    </div>
  );
}

// Colonna droppabile.
function ColumnBox({ col, issues }: { col: Column; issues: SearchIssue[] }) {
  const { setNodeRef, isOver } = useDroppable({ id: col.id });
  return (
    <div
      ref={setNodeRef}
      className={`w-64 shrink-0 rounded bg-slate-100 p-2 ${isOver ? "ring-2 ring-[#0052cc]" : ""}`}
      data-testid={`column-${col.name}`}
    >
      <h3 className="mb-2 text-xs font-semibold uppercase text-slate-500">
        {col.name} <span className="text-slate-400">({issues.length})</span>
      </h3>
      {issues.map((iss) => (
        <IssueCard key={iss.key} issue={iss} />
      ))}
    </div>
  );
}

export function BoardColumns({
  columns,
  issuesByStatus,
  onMove,
}: {
  columns: Column[];
  issuesByStatus: Record<string, SearchIssue[]>;
  onMove: (issueKey: string, toStatusId: string) => void;
}) {
  const handleDragEnd = (e: DragEndEvent) => {
    const issueKey = String(e.active.id);
    const toStatusId = e.over ? String(e.over.id) : null;
    if (toStatusId) onMove(issueKey, toStatusId);
  };
  return (
    <DndContext onDragEnd={handleDragEnd}>
      <div className="flex gap-3 overflow-x-auto p-2">
        {columns.map((col) => (
          <ColumnBox key={col.id} col={col} issues={issuesByStatus[col.id] ?? []} />
        ))}
      </div>
    </DndContext>
  );
}
```

- [ ] **Step 2: Pagina board**

`frontend-next/app/jira/boards/[boardId]/page.tsx`:

```tsx
"use client";

import { use, useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { boards, issues as issuesApi, type SearchIssue } from "@/lib/api";
import { BoardColumns } from "@/components/board/BoardColumns";

export default function BoardPage({ params }: { params: Promise<{ boardId: string }> }) {
  const { boardId } = use(params);
  const id = Number(boardId);
  const qc = useQueryClient();

  const board = useQuery({ queryKey: ["board", id], queryFn: () => boards.get(id) });
  const config = useQuery({ queryKey: ["board", id, "config"], queryFn: () => boards.configuration(id) });
  const issueList = useQuery({ queryKey: ["board", id, "issues"], queryFn: () => boards.issues(id) });

  const columns = useMemo(
    () => (config.data?.columnConfig.columns ?? []).map((c) => ({ id: c.statuses[0]?.id ?? c.name, name: c.name })),
    [config.data],
  );

  const issuesByStatus = useMemo(() => {
    const map: Record<string, SearchIssue[]> = {};
    for (const iss of issueList.data?.issues ?? []) {
      const sid = iss.fields.status?.name ?? "";
      // la colonna è per status id; mappiamo per id via config
      const col = (config.data?.columnConfig.columns ?? []).find((c) => c.name === iss.fields.status?.name);
      const key = col?.statuses[0]?.id ?? sid;
      (map[key] ??= []).push(iss);
    }
    return map;
  }, [issueList.data, config.data]);

  const move = useMutation({
    mutationFn: ({ issueKey, toStatusId }: { issueKey: string; toStatusId: string }) =>
      issuesApi.update(issueKey, { statusId: toStatusId }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["board", id, "issues"] }),
  });

  return (
    <div className="p-4">
      <div className="mb-3 flex items-center gap-3">
        <h1 className="text-xl font-semibold text-[#1a1f36]">{board.data?.name ?? "Board"}</h1>
        <a href={`/jira/boards/${id}/backlog`} className="text-sm text-[#0052cc] hover:underline">
          Backlog
        </a>
      </div>
      {columns.length > 0 && (
        <BoardColumns
          columns={columns}
          issuesByStatus={issuesByStatus}
          onMove={(issueKey, toStatusId) => move.mutate({ issueKey, toStatusId })}
        />
      )}
    </div>
  );
}
```

> **Nota implementatore:** verificare la firma reale di `issues.update` in `lib/api.ts` (dal Round 2): come si aggiorna lo status di una issue (`update(key, {statusId})` o un endpoint transizione). Se l'update status richiede un `transition` invece di `statusId`, adeguare la mutation (o usare `PUT /rest/api/3/issue/{key}` con `fields.status`). L'obiettivo E2E è che trascinando una card tra colonne lo status cambi. Se manca un modo semplice di settare lo status, usare l'endpoint esistente usato dalla vista issue per l'edit inline.

- [ ] **Step 3: Type-check + build**

Run: `cd frontend-next && npx tsc --noEmit && npm run build`
Expected: build ok.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/components/board/BoardColumns.tsx frontend-next/app/jira/boards/
git commit -m "feat(frontend): agile board with drag-and-drop columns"
```

---

### Task 14: Frontend — pagina backlog (sprint + move)

**Files:**
- Create: `frontend-next/app/jira/boards/[boardId]/backlog/page.tsx`

- [ ] **Step 1: Pagina backlog**

`frontend-next/app/jira/boards/[boardId]/backlog/page.tsx`:

```tsx
"use client";

import { use, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { boards, sprints, agileIssues, type AgileSprint, type SearchIssue } from "@/lib/api";

function IssueRow({ issue }: { issue: SearchIssue }) {
  return (
    <div className="flex items-center gap-2 border-b border-slate-100 py-1 text-sm" data-testid={`row-${issue.key}`}>
      <span className="font-mono text-xs text-slate-500">{issue.key}</span>
      <span className="text-[#1a1f36]">{issue.fields.summary}</span>
    </div>
  );
}

export default function BacklogPage({ params }: { params: Promise<{ boardId: string }> }) {
  const { boardId } = use(params);
  const id = Number(boardId);
  const qc = useQueryClient();
  const [newSprint, setNewSprint] = useState("");

  const backlog = useQuery({ queryKey: ["board", id, "backlog"], queryFn: () => boards.backlog(id) });
  const sprintList = useQuery({ queryKey: ["board", id, "sprints"], queryFn: () => boards.sprints(id) });

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ["board", id, "backlog"] });
    qc.invalidateQueries({ queryKey: ["board", id, "sprints"] });
  };

  const createSprint = useMutation({
    mutationFn: (name: string) => sprints.create(name, id),
    onSuccess: () => {
      setNewSprint("");
      invalidate();
    },
  });
  const setState = useMutation({
    mutationFn: ({ sprintId, state }: { sprintId: number; state: "active" | "closed" }) =>
      sprints.setState(sprintId, state),
    onSuccess: invalidate,
  });

  return (
    <div className="mx-auto max-w-3xl p-4">
      <div className="mb-3 flex items-center gap-3">
        <h1 className="text-xl font-semibold text-[#1a1f36]">Backlog</h1>
        <a href={`/jira/boards/${id}`} className="text-sm text-[#0052cc] hover:underline">
          Board
        </a>
      </div>

      {/* sprint */}
      {sprintList.data?.values.map((sp: AgileSprint) => (
        <SprintSection key={sp.id} boardId={id} sprint={sp} onState={(state) => setState.mutate({ sprintId: sp.id, state })} />
      ))}

      {/* crea sprint */}
      <div className="my-3 flex gap-2">
        <input
          aria-label="New sprint name"
          value={newSprint}
          onChange={(e) => setNewSprint(e.target.value)}
          placeholder="Sprint name"
          className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm"
        />
        <button
          onClick={() => newSprint && createSprint.mutate(newSprint)}
          className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60"
          disabled={createSprint.isPending}
        >
          Create sprint
        </button>
      </div>

      {/* backlog */}
      {/* div, non heading: evita la collisione strict-mode con l'<h1>Backlog</h1> nell'E2E */}
      <div className="mb-1 mt-4 text-sm font-semibold text-slate-500">
        Backlog ({backlog.data?.issues.length ?? 0})
      </div>
      <div>
        {backlog.data?.issues.map((iss) => <IssueRow key={iss.key} issue={iss} />)}
        {backlog.data && backlog.data.issues.length === 0 && (
          <p className="py-2 text-sm text-slate-400">Backlog is empty</p>
        )}
      </div>
    </div>
  );
}

function SprintSection({
  boardId,
  sprint,
  onState,
}: {
  boardId: number;
  sprint: AgileSprint;
  onState: (state: "active" | "closed") => void;
}) {
  const issues = useQuery({ queryKey: ["sprint", sprint.id, "issues"], queryFn: () => sprints.issues(sprint.id) });
  return (
    <div className="mb-3 rounded border border-slate-200 p-2" data-testid={`sprint-${sprint.id}`}>
      <div className="mb-1 flex items-center justify-between">
        <span className="text-sm font-semibold text-[#1a1f36]">
          {sprint.name} <span className="text-xs font-normal text-slate-400">({sprint.state})</span>
        </span>
        <span className="flex gap-2">
          {sprint.state === "future" && (
            <button onClick={() => onState("active")} className="rounded border px-2 py-0.5 text-xs">
              Start sprint
            </button>
          )}
          {sprint.state === "active" && (
            <button onClick={() => onState("closed")} className="rounded border px-2 py-0.5 text-xs">
              Complete sprint
            </button>
          )}
        </span>
      </div>
      {issues.data?.issues.map((iss) => <IssueRow key={iss.key} issue={iss} />)}
    </div>
  );
}
```

> **Nota:** questa versione usa i pulsanti (create/start/complete) e mostra backlog+sprint; il move issue via drag è coperto dall'API `sprints.moveIssues`/`agileIssues.moveToBacklog` (usati anche dall'E2E via UI se aggiungi bottoni "→ sprint"; per semplicità e robustezza E2E, i bottoni di stato bastano a dimostrare il flusso). Se vuoi il drag anche qui, riusa `DndContext` come nel board. YAGNI: implementa il drag backlog↔sprint solo se il tempo lo consente; i bottoni + le mutation coprono il requisito funzionale del round.

- [ ] **Step 2: Type-check + build**

Run: `cd frontend-next && npx tsc --noEmit && npm run build`
Expected: build ok.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/app/jira/boards/
git commit -m "feat(frontend): backlog page with sprint create/start/complete"
```

---

### Task 15: E2E — board e backlog

**Files:**
- Modify: `cmd/seed/main.go` (seed board demo — anticipato qui perché l'E2E ne ha bisogno; il seed sprint resta in Task 16)
- Create: `frontend-next/e2e/board.spec.ts`

- [ ] **Step 1: Seed board demo (idempotente)**

In `cmd/seed/main.go`, dopo il seed del filtro demo, aggiungere (stile idempotente):

```go
	boardSvc := board.NewService(s.DB)
	var existingB board.Board
	err = s.DB.Where("project_id = ? AND name = ?", demo.ID, "DEMO board").First(&existingB).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if _, err := boardSvc.Create(demo.ID, "DEMO board", "scrum", nil); err != nil {
			log.Fatalf("seed board: %v", err)
		}
		fmt.Println("created demo board")
	} else if err != nil {
		log.Fatalf("check demo board: %v", err)
	}
```

(Import `board "github.com/open-jira/open-jira/internal/domain/board"`. Verificare il nome della variabile del progetto DEMO nel seed — è `demo` dal contesto Round 3.) La board demo avrà `seq_id=1`.

- [ ] **Step 2: E2E**

`frontend-next/e2e/board.spec.ts` (riusare l'helper `login()` reale da `search.spec.ts`/`collaboration.spec.ts`):

```ts
import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/jira/);
}

test("board shows columns with seeded issues", async ({ page }) => {
  await login(page);
  await page.goto("/jira/boards/1");
  // la board DEMO ha colonne dagli status del workflow; almeno una colonna visibile
  await expect(page.getByRole("heading", { name: /DEMO board/i })).toBeVisible();
  await expect(page.locator('[data-testid^="column-"]').first()).toBeVisible();
  // almeno una card issue del progetto DEMO
  await expect(page.locator('[data-testid^="card-DEMO-"]').first()).toBeVisible();
});

test("backlog page lists sprints controls and creates a sprint", async ({ page }) => {
  await login(page);
  await page.goto("/jira/boards/1/backlog");
  await expect(page.getByRole("heading", { name: /Backlog/i })).toBeVisible();
  await page.getByLabel("New sprint name").fill("E2E Sprint");
  await page.getByRole("button", { name: "Create sprint" }).click();
  await expect(page.getByText("E2E Sprint")).toBeVisible();
  // lo sprint appena creato è "future" → mostra il bottone Start
  await expect(page.getByRole("button", { name: "Start sprint" }).first()).toBeVisible();
});
```

- [ ] **Step 3: Eseguire l'E2E**

Run: `cd frontend-next && npx playwright test e2e/board.spec.ts --reporter=line`
Expected: 2 test PASS (il webServer riavvia il backend seedato — la board demo `seq_id=1` esiste).

- [ ] **Step 4: Suite completa (nessuna regressione)**

Run: `cd frontend-next && npx playwright test --reporter=line`
Expected: tutti verdi (login, projects, issues, collaboration, search, board). Pulire `test-results/`/`playwright-report/` e i processi su 8080/3000.

- [ ] **Step 5: Commit**

```bash
git add cmd/seed/main.go frontend-next/e2e/board.spec.ts
git commit -m "test(e2e): agile board columns and backlog sprint creation"
```

---

### Task 16: Seed sprint demo + gap report

**Files:**
- Modify: `cmd/seed/main.go`
- Modify: `docs/contracts/gap-report.md` (rigenerato)

- [ ] **Step 1: Seed sprint demo (idempotente)**

In `cmd/seed/main.go`, dopo il seed della board, aggiungere uno sprint demo idempotente legato alla board demo (seq_id 1):

```go
	sprintSvc := sprint.NewService(s.DB)
	var existingS sprint.Sprint
	err = s.DB.Where("project_id = ? AND name = ?", demo.ID, "Sprint 1").First(&existingS).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		boardSeq := int64(1)
		if _, err := sprintSvc.CreateFull(demo.ID, "Sprint 1", "Primo sprint demo", &boardSeq, nil, nil); err != nil {
			log.Fatalf("seed sprint: %v", err)
		}
		fmt.Println("created demo sprint")
	} else if err != nil {
		log.Fatalf("check demo sprint: %v", err)
	}
```

(Import `sprint "github.com/open-jira/open-jira/internal/domain/sprint"` se non presente.)

- [ ] **Step 2: Verificare il seed idempotente**

Run: `rm -f /tmp/seed16.db && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/seed16.db go run ./cmd/seed && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/seed16.db go run ./cmd/seed && rm -f /tmp/seed16.db`
Expected: prima run stampa "created demo board"/"created demo sprint"; seconda run NON li ricrea; entrambe exit 0.

- [ ] **Step 3: Rigenerare il gap report**

Run: `go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md`
Expected: aggiornato; i nuovi endpoint agile (`/rest/agile/1.0/board*`, `/sprint*`, `/backlog/issue`, `/issue/rank`, `/issue/{id}`, `/epic/{id}`) compaiono. Riportare il conteggio conformi old→new.

- [ ] **Step 4: Commit**

```bash
git add cmd/seed/main.go docs/contracts/gap-report.md
git commit -m "feat(seed): demo sprint; regenerate gap report for Round 5"
```

---

### Task 17: Gate finale + STATE.md → Round 6

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
Expected: BUILD_OK, VET_OK, nessun FAIL Go, frontend build ok, tutti gli E2E verdi.

- [ ] **Step 2: Gap report senza drift**

Run: `go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md`
Expected: nessun drift inatteso.

- [ ] **Step 3: Aggiornare STATE.md**

In `docs/superpowers/STATE.md`:
- aggiungere alla sezione "Round completati" la riga del **Round 5 — Board, Backlog, Sprint** (API agile 1.0: board CRUD+configuration, sprint CRUD+start/complete, backlog/board/sprint issue listing, move to sprint/backlog, issue rank, epic read; pattern seq_id per board/sprint; UI board dnd + backlog);
- cambiare "Prossimo" in **Round 6 — Workflow** (stati custom con categorie, transizioni con condizioni/validator/post-function base, workflow per progetto; UI editor workflow in settings);
- aggiornare il conteggio gap report e la data;
- aggiungere ai follow-up: LexoRank stringa (ora Position float), epic update/rank (`POST /epic/{id}`), board filtro-JQL puro, consolidare le vecchie rotte custom `/rest/api/3/project/{key}/board|sprints`, drag backlog↔sprint nella UI.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/STATE.md docs/contracts/gap-report.md
git commit -m "docs: mark Round 5 (Boards) complete, Round 6 (Workflow) next"
```

---

## Note di chiusura round

- **Follow-up:** LexoRank stringa (ranking robusto vs float); epic update/rank (`POST /epic/{id}`, `PUT /epic/{id}/rank`); board basate su filtro JQL (non solo progetto); `board/{id}/issue` POST (rank su board); consolidamento/retiro delle vecchie rotte custom `/rest/api/3/project/{key}/board|sprints` e del BoardHandler custom; drag&drop backlog↔sprint nella UI; websocket board update per l'agile board.
- **Rischi noti:** le date agili in ingresso sono parse-best-effort (RFC3339/ISO/date); `Position` float può perdere precisione con molti reinserimenti (mitigato da LexoRank in follow-up); `originBoardId` non è vincolato univoco per progetto (più board per progetto → lo sprint punta a una board arbitraria se create via project). Il round chiude solo con i tre livelli verdi.

---

## Self-Review (svolta in fase di scrittura)

**1. Copertura spec (roadmap Round 5 + contratto agile):**
- Board CRUD + configuration → Task 6 (List/Create/Get/Delete/Configuration). ✅
- Backlog / board issues / board sprints / board epics → Task 7. ✅
- Sprint CRUD + start/complete + issue list + move-to-sprint → Task 8. ✅
- Issue rank + move-to-backlog + agile issue get + epic get → Task 9. ✅
- Ranking (LexoRank-like) → Task 4 (Position midpoint; LexoRank stringa esplicitamente follow-up). ✅
- Router agile → Task 10; contract → Task 11. ✅
- UI board dnd + backlog + sprint start/complete → Task 13/14; E2E → Task 15. ✅
- seq_id interi (board/sprint) → Task 1/2/3. ✅

**2. Placeholder scan:** codice completo in ogni task. Le "Note implementatore" indicano verifiche puntuali su firme reali (project.Service.GetByID/GetByKey, WritePage generics, issues.update per lo status, apiFetch, helper harness) con i comandi per risolverle — legittime, non placeholder di logica. Lo scope epic-read e ranking-float sono dichiarati esplicitamente, non lasciati impliciti.

**3. Consistenza tipi:** `board.Service.Create(projectID, name, type string, filterID *string)` e `GetBySeqID`/`ListByProject`/`List`/`Delete` usati coerentemente (T2/T6/T7/T10). `sprint.Service.CreateFull(projectID, name, goal string, originBoardID *int64, start, end *time.Time)`, `GetBySeqID`, `UpdateFull`, `Complete` (T3/T7/T8). `issue.Service.Rank(issueIDs []string, before, after *string)` (T4/T9). `v3.AgileBoard(BoardInput)`, `v3.AgileSprint(SprintInput)`, `v3.Board`, `v3.Sprint`, `v3.BoardConfig`, `v3.Page[T]`/`WritePage`, `v3.SearchResults` (T5/T6/T7/T8). `renderIssueList(issueH, issues, fields)` condiviso (T5/T7/T8/T9). Helper `sprintToV3`, `page`, `resolveIssue` coerenti. Frontend `boards`/`sprints`/`agileIssues` client (T12) usati in T13/T14/T15. Chiavi paginazione: board/sprint list → `values`; issue list → `issues` — rispettato dappertutto.
