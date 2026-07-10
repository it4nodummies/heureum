# Round 2 — Issue core: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Portare la risorsa Issue alla conformità drop-in con la Jira Cloud REST API v3 — CRUD con id numerici, campi ADF, tipi/priorità/risoluzioni/stati come dati di riferimento, createmeta/editmeta, custom field e label — con la UI di vista issue, creazione e modifica inline.

**Architecture:** Si riusa il pattern del Round 1: `seq_id` numerico (UUID resta PK interna), mapping v3 dedicato (`v3.JiraIssue` → schema `IssueBean`), un contract test per ogni endpoint contro `docs/contracts/jira-platform-v3.json`. Vantaggio del contratto issue: il campo `fields` di `IssueBean` è **free-form** (`additionalProperties:{}`), quindi la conformità è vincolata solo sul top-level (`id` stringa numerica, `key`, `self`, `fields`); il lavoro dentro `fields` è fedeltà funzionale (ADF, User completi, `statusCategory`), non wrangling di schema. La description usa `internal/adf`; assignee/reporter usano `v3.JiraUser`.

**Tech Stack:** Go 1.25, GORM, golang-migrate, kin-openapi (contract), Next.js 16 + TanStack Query + TipTap (ADF) + Playwright.

**Contesto codebase (per chi non lo conosce):**
- Pattern seq_id di riferimento (Round 1): `migrations/000006_project_seq_id.*`, campo `SeqID int64 \`gorm:"column:seq_id;uniqueIndex"\`` su `project.Project`, `Service.nextSeqID()` (`COALESCE(MAX(seq_id),9999)+1`), `Service.GetBySeqID(int64)`, e nel handler la risoluzione id-or-key (`strconv.ParseInt` → GetBySeqID, else GetByKey). Replicare identico per le issue (base 10000; le issue Jira partono ~10001).
- Mapping v3 di riferimento: `internal/api/v3/project.go` (`JiraProject`), `internal/api/v3/user.go` (`JiraUser(u user.User, baseURL) v3.User`).
- Helper risposta: `v3.WriteJSON(w,status,v)`, `v3.WriteError(w,status,[]string,map[string]string)`, `v3.WritePage[T]`, `v3.ParsePagination(r,def,cap)`.
- Contract harness: `contract.MustLoad(tb,"../../docs/contracts/jira-platform-v3.json")`; `(*Validator).ValidateResponse(method,path string,status int,header http.Header,body io.Reader) error`; helper in `internal/contract/` (package `contract`): `newTestServer(t)(*httptest.Server,*auth.Service)`, `registerAndLogin(t,authSvc) string`, `createProjectViaAPI(t,srv,jwt,key,name)`.
- ADF: `internal/adf` — `adf.Node`, `adf.FromText(string) adf.Node`, `adf.Validate(adf.Node) error`. Un documento ADF è `{"type":"doc","version":1,"content":[...]}`.
- Modello issue: `internal/domain/issue/model.go` — `Issue{ID(uuid),ProjectID,Key,Title,DescriptionJSON,TypeID *string,StatusID *string,Priority(highest|high|medium|low|lowest),AssigneeID *string,ReporterID *string,ResolutionID *string,ParentID *string,SprintID,VersionID,StoryPoints int,OriginalEstimate int,TimeSpent int,StartDate,DueDate,Environment,IsArchived,Position,CreatedAt,UpdatedAt}`; `IssueType{ID,ProjectID,Name,Description,Icon,Color,IsSubtask}`; `Label`, `IssueLabel`. Esiste `ToJiraResponse`/`JiraIssueResponse`/`JiraIssueFields` ad-hoc **NON conforme** — verrà sostituito dal nuovo mapping v3 e poi rimosso.
- Service issue: `NewService(db)`, `Create(projectKey,projectID,title,description string,priority Priority,parentID *string,typeID *string)(*Issue,error)`, `GetByKey(key)`, `Update(...)`, `Delete(key)`, `ListByProject(projectID, opts...)`, `GetChildren(parentID)`, `AddLabel`, `Watch`, `GetHistory`, `DB()`.
- Handler issue: `internal/api/handlers/issue_handler.go` — `NewIssueHandler(svc *issue.Service, projectSvc *project.Service, wfSvc *workflow.Service)`, metodi Create/Get/Update/Delete/List/AddLabel/Watchers/History. **Non importa ancora `internal/api/v3`** — va aggiunto, e va aggiunto `baseURL` al costruttore (come fatto per project/user nei round precedenti).
- Stati: `internal/domain/workflow` — `WorkflowStatus{ID,Name,Category(StatusCategory: todo|inprogress|done),Color,...}`. Le issue referenziano `StatusID`.
- Custom field: `internal/domain/customfield` — `CustomField{ID,ProjectID,Name,FieldType}`, `CustomFieldOption{ID,FieldID,Value}`, valori in `issue_custom_values`.
- Tabelle già presenti (migrazione 000001/000002): `workflow_statuses`, `issue_types`, `custom_fields`, `custom_field_options`, `issue_custom_values`, `resolutions`. Ultima migrazione: `000006`. Le nuove partono da `000007`.
- Router: `internal/api/router.go`; issue routes attorno alle righe 144-178. `issueSvc`, `projectSvc`, `wfSvc`, `cfg.BaseURL` disponibili in `NewRouter`.
- Priorità Jira standard (id fissi, per sintetizzare `/priority`): 1=Highest, 2=High, 3=Medium, 4=Low, 5=Lowest. Mappa dal nostro enum: highest→1, high→2, medium→3, low→4, lowest→5.
- Commit: conventional commits. Non fare push. Se `git` dà "index.lock", riprovare, mai cancellare il lock. Gli handler che toccano `internal/api/handlers/issue_handler.go` (Task 7,8,9,10) vanno **serializzati** tra loro.

**Decisioni di mapping (chiave del round):**
- Issue `id` v3 = stringa del `seq_id` numerico (10001+). `self` = `{base}/rest/api/3/issue/{seq_id}`. `GET /issue/{idOrKey}` risolve per id numerico o per key (es. `DEMO-1`).
- `fields.description` = documento ADF: se `DescriptionJSON` è già un doc ADF (`type=="doc"`) si passa così com'è; altrimenti si interpreta come testo/`{"content":"..."}` e si costruisce con `adf.FromText`. Se vuoto → `null`.
- `fields.assignee`/`reporter` = `v3.JiraUser` completo, oppure `null`.
- `fields.issuetype` = da `IssueType` risolto; se assente, default `{name:"Task", subtask:false}`.
- `fields.status` = da `WorkflowStatus` risolto con `statusCategory`; se assente, default `To Do`/categoria `new` (todo).
- `fields.priority` = sintetizzata dall'enum (id fisso + nome).
- `fields.created`/`updated` = stringa datetime formato Jira `2006-01-02T15:04:05.000-0700`.
- `POST /issue` risponde `CreatedIssue{id,key,self}` (id **stringa** numerica).

---

## File Structure

- `migrations/000007_issue_seq_id.up.sql` / `.down.sql` — `seq_id INTEGER` + unique index su `issues`.
- `internal/domain/issue/model.go` — campo `SeqID int64`; rimozione (a fine round) di `ToJiraResponse`/`JiraIssueResponse`/`JiraIssueFields`.
- `internal/domain/issue/service.go` — `nextSeqID()`, assegnazione in `Create`, `GetBySeqID`, `GetLabels(issueID)`.
- `internal/api/v3/datetime.go` (+ test) — `JiraTime(t time.Time) string`.
- `internal/api/v3/issue.go` (+ test) — tipi `IssueBean`, `IssueFields`, `IssueTypeRef`, `StatusRef`, `StatusCategoryRef`, `PriorityRef`, `ResolutionRef`, `ProjectRef`, `ParentRef`; funzione `JiraIssue(in IssueInput) IssueBean`.
- `internal/api/v3/reference.go` (+ test) — `StandardPriorities(baseURL)`, `JiraIssueTypeDetails(...)`, `JiraStatus(...)`, `JiraResolution(...)`.
- `internal/api/handlers/issue_handler.go` — riscrittura Create/Get/Update/Delete in v3; `baseURL`; helper `resolveIssueKey`, `mapIssue`.
- `internal/api/handlers/reference_handler.go` (nuovo) — GET `/priority`, `/priority/{id}`, `/resolution`, `/issuetype`, `/status`, `/field`, POST `/field`, GET `/label`, `/issue/createmeta`, `/issue/{idOrKey}/editmeta`.
- `internal/api/router.go` — nuove rotte; nota fix rotta riga frontend.
- `internal/contract/issue_test.go` — contract test per ogni endpoint.
- `cmd/seed/main.go` — assegnare a ogni issue DEMO un `TypeID` (default "Task") e `StatusID` (default "To Do") così il mapping produce type/status reali; assicurare seq_id.
- Frontend: `frontend-next/lib/api.ts` (tipo `Issue` v3 + chiamate), `frontend-next/app/jira/browse/[key]/page.tsx` + `frontend-next/components/issues/IssueView.tsx`, `frontend-next/components/issues/CreateIssueModal.tsx`, `frontend-next/components/projects/ProjectsPage.tsx` (fix rotta riga), `frontend-next/e2e/issues.spec.ts`.

---

### Task 1: Migrazione — seq_id sulle issue

**Files:**
- Create: `migrations/000007_issue_seq_id.up.sql`, `migrations/000007_issue_seq_id.down.sql`

- [ ] **Step 1: up migration**

```sql
-- migrations/000007_issue_seq_id.up.sql
ALTER TABLE issues ADD COLUMN seq_id INTEGER;
CREATE UNIQUE INDEX idx_issues_seq_id ON issues(seq_id);
```

- [ ] **Step 2: down migration**

```sql
-- migrations/000007_issue_seq_id.down.sql
DROP INDEX idx_issues_seq_id;
ALTER TABLE issues DROP COLUMN seq_id;
```

Guarda `migrations/000006_project_seq_id.up.sql` per lo stile identico.

- [ ] **Step 3: verifica su SQLite pulito**

Run:
```bash
APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/mig-r2.db go run ./cmd/seed && sqlite3 /tmp/mig-r2.db 'PRAGMA table_info(issues);' | grep seq_id && rm -f /tmp/mig-r2.db
```
Expected: la colonna `seq_id` compare; exit 0.

- [ ] **Step 4: commit**

```bash
git add migrations/000007_issue_seq_id.*
git commit -m "feat(issue): migration adds numeric seq_id column"
```

---

### Task 2: Modello + service — seq_id, GetBySeqID, GetLabels

**Files:**
- Modify: `internal/domain/issue/model.go`, `internal/domain/issue/service.go`
- Test: `internal/domain/issue/service_test.go` (create se assente)

- [ ] **Step 1: test fallente**

Crea/estendi `internal/domain/issue/service_test.go`:

```go
package issue

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newIssueTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Issue{}, &IssueType{}, &Label{}, &IssueLabel{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreateAssignsSeqID(t *testing.T) {
	db := newIssueTestDB(t)
	svc := NewService(db)
	i1, err := svc.Create("DEMO", "p1", "First", "", PriorityMedium, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if i1.SeqID < 10000 {
		t.Errorf("seq_id = %d, want >= 10000", i1.SeqID)
	}
	i2, _ := svc.Create("DEMO", "p1", "Second", "", PriorityMedium, nil, nil)
	if i2.SeqID != i1.SeqID+1 {
		t.Errorf("second seq_id = %d, want %d", i2.SeqID, i1.SeqID+1)
	}
	got, err := svc.GetBySeqID(i1.SeqID)
	if err != nil || got.Key != i1.Key {
		t.Errorf("GetBySeqID: got %+v err %v", got, err)
	}
}
```

Se il package ha già un helper DB di test, riusalo e non duplicarlo.

- [ ] **Step 2: verifica FAIL**

Run: `go test ./internal/domain/issue/ -run 'TestCreateAssignsSeqID'`
Expected: FAIL — `i1.SeqID undefined` / `GetBySeqID undefined`.

- [ ] **Step 3: aggiungi il campo al modello**

In `internal/domain/issue/model.go`, dentro `Issue`, dopo `Position`:

```go
	SeqID int64 `gorm:"column:seq_id;uniqueIndex" json:"seq_id"`
```

- [ ] **Step 4: assegna seq_id in Create + GetBySeqID**

In `internal/domain/issue/service.go` aggiungi (import `database/sql` se serve):

```go
func (s *Service) nextSeqID() (int64, error) {
	var max sql.NullInt64
	// nota: MAX+1 ha una race teorica sotto create concorrenti; accettabile a questa scala.
	if err := s.db.Model(&Issue{}).Select("COALESCE(MAX(seq_id), 9999)").Scan(&max).Error; err != nil {
		return 0, err
	}
	return max.Int64 + 1, nil
}

func (s *Service) GetBySeqID(id int64) (*Issue, error) {
	var i Issue
	if err := s.db.First(&i, "seq_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &i, nil
}

// GetLabels restituisce i nomi delle label associate a una issue.
func (s *Service) GetLabels(issueID string) ([]string, error) {
	var names []string
	err := s.db.Table("labels").
		Joins("JOIN issue_labels ON issue_labels.label_id = labels.id").
		Where("issue_labels.issue_id = ?", issueID).
		Pluck("labels.name", &names).Error
	return names, err
}
```

In `Create`, prima di `s.db.Create(issue)` (o equivalente), assegna:

```go
	seq, err := s.nextSeqID()
	if err != nil {
		return nil, err
	}
	issue.SeqID = seq
```

Leggi il corpo attuale di `Create` per inserire l'assegnazione nel punto giusto (il nome della variabile della issue potrebbe essere diverso da `issue`). Verifica i nomi reali delle tabelle join per `GetLabels` (`labels`, `issue_labels`, colonne `label_id`/`issue_id`) leggendo il modello `IssueLabel`; adatta se diversi.

- [ ] **Step 5: verifica PASS**

Run: `go test ./internal/domain/issue/ -count=1`
Expected: PASS (nuovo + preesistenti).

- [ ] **Step 6: commit**

```bash
git add internal/domain/issue/model.go internal/domain/issue/service.go internal/domain/issue/service_test.go
git commit -m "feat(issue): numeric seq_id assignment, GetBySeqID, GetLabels"
```

---

### Task 3: Helper datetime formato Jira

**Files:**
- Create: `internal/api/v3/datetime.go`, `internal/api/v3/datetime_test.go`

- [ ] **Step 1: test fallente**

```go
// internal/api/v3/datetime_test.go
package v3

import (
	"testing"
	"time"
)

func TestJiraTime(t *testing.T) {
	loc := time.FixedZone("CET", 2*3600)
	ts := time.Date(2026, 7, 10, 15, 4, 5, 123_000_000, loc)
	got := JiraTime(ts)
	if got != "2026-07-10T15:04:05.123+0200" {
		t.Errorf("JiraTime = %q", got)
	}
}

func TestJiraTime_Zero(t *testing.T) {
	if JiraTime(time.Time{}) != "" {
		t.Error("zero time must render empty string")
	}
}
```

- [ ] **Step 2: verifica FAIL**

Run: `go test ./internal/api/v3/ -run TestJiraTime`
Expected: FAIL — `undefined: JiraTime`.

- [ ] **Step 3: implementa**

```go
// internal/api/v3/datetime.go
package v3

import "time"

// jiraTimeLayout è il formato timestamp usato da Jira Cloud v3.
const jiraTimeLayout = "2006-01-02T15:04:05.000-0700"

// JiraTime formatta un time.Time nel formato di Jira. Zero time → "".
func JiraTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(jiraTimeLayout)
}
```

- [ ] **Step 4: verifica PASS**

Run: `go test ./internal/api/v3/ -run TestJiraTime`
Expected: PASS

- [ ] **Step 5: commit**

```bash
git add internal/api/v3/datetime.go internal/api/v3/datetime_test.go
git commit -m "feat(v3): Jira datetime formatting helper"
```

---

### Task 4: Tipi e builder dei dati di riferimento (priority, resolution, issuetype, status)

**Files:**
- Create: `internal/api/v3/reference.go`, `internal/api/v3/reference_test.go`

- [ ] **Step 1: test fallente**

```go
// internal/api/v3/reference_test.go
package v3

import "testing"

func TestStandardPriorities(t *testing.T) {
	ps := StandardPriorities("http://h")
	if len(ps) != 5 {
		t.Fatalf("got %d priorities, want 5", len(ps))
	}
	if ps[0].Name != "Highest" || ps[0].ID != "1" {
		t.Errorf("first priority = %+v", ps[0])
	}
	if ps[0].Self != "http://h/rest/api/3/priority/1" {
		t.Errorf("self = %q", ps[0].Self)
	}
}

func TestPriorityForEnum(t *testing.T) {
	p := PriorityForEnum("high", "http://h")
	if p.ID != "2" || p.Name != "High" {
		t.Errorf("high → %+v", p)
	}
	// valore ignoto → Medium (3)
	if PriorityForEnum("weird", "http://h").ID != "3" {
		t.Error("unknown priority must default to Medium (3)")
	}
}

func TestJiraStatus_Category(t *testing.T) {
	s := JiraStatus("s1", "In Progress", "inprogress", "http://h")
	if s.StatusCategory.Key != "indeterminate" || s.StatusCategory.Name != "In Progress" {
		t.Errorf("statusCategory = %+v", s.StatusCategory)
	}
	if JiraStatus("s2", "Done", "done", "http://h").StatusCategory.Key != "done" {
		t.Error("done category key must be 'done'")
	}
	if JiraStatus("s3", "To Do", "todo", "http://h").StatusCategory.Key != "new" {
		t.Error("todo category key must be 'new'")
	}
}
```

- [ ] **Step 2: verifica FAIL**

Run: `go test ./internal/api/v3/ -run 'TestStandardPriorities|TestPriorityForEnum|TestJiraStatus'`
Expected: FAIL — simboli non definiti.

- [ ] **Step 3: implementa**

```go
// internal/api/v3/reference.go
package v3

import "fmt"

// PriorityRef è la rappresentazione v3 di una priorità (schema Priority).
type PriorityRef struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IconURL     string `json:"iconUrl,omitempty"`
	StatusColor string `json:"statusColor,omitempty"`
}

var priorityOrder = []struct {
	id, name, color string
}{
	{"1", "Highest", "#CD1317"},
	{"2", "High", "#E9494A"},
	{"3", "Medium", "#E97F33"},
	{"4", "Low", "#2A8735"},
	{"5", "Lowest", "#57A55A"},
}

// enumToID mappa il nostro enum interno all'id priorità Jira standard.
var enumToID = map[string]string{
	"highest": "1", "high": "2", "medium": "3", "low": "4", "lowest": "5",
}

func StandardPriorities(baseURL string) []PriorityRef {
	out := make([]PriorityRef, 0, len(priorityOrder))
	for _, p := range priorityOrder {
		out = append(out, PriorityRef{
			Self:        fmt.Sprintf("%s/rest/api/3/priority/%s", baseURL, p.id),
			ID:          p.id,
			Name:        p.name,
			StatusColor: p.color,
			IconURL:     fmt.Sprintf("%s/static/priority-%s.svg", baseURL, p.id),
		})
	}
	return out
}

func PriorityForEnum(enum, baseURL string) PriorityRef {
	id := enumToID[enum]
	if id == "" {
		id = "3"
	}
	for _, p := range StandardPriorities(baseURL) {
		if p.ID == id {
			return p
		}
	}
	return StandardPriorities(baseURL)[2]
}

// StatusCategoryRef e StatusRef modellano status + statusCategory v3.
type StatusCategoryRef struct {
	Self      string `json:"self"`
	ID        int    `json:"id"`
	Key       string `json:"key"`
	ColorName string `json:"colorName"`
	Name      string `json:"name"`
}

type StatusRef struct {
	Self           string            `json:"self"`
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	IconURL        string            `json:"iconUrl,omitempty"`
	StatusCategory StatusCategoryRef `json:"statusCategory"`
}

// categoryFor mappa la nostra categoria interna (todo|inprogress|done) allo statusCategory Jira.
func categoryFor(internal, baseURL string) StatusCategoryRef {
	switch internal {
	case "done":
		return StatusCategoryRef{Self: baseURL + "/rest/api/3/statuscategory/3", ID: 3, Key: "done", ColorName: "green", Name: "Done"}
	case "inprogress":
		return StatusCategoryRef{Self: baseURL + "/rest/api/3/statuscategory/4", ID: 4, Key: "indeterminate", ColorName: "yellow", Name: "In Progress"}
	default: // todo
		return StatusCategoryRef{Self: baseURL + "/rest/api/3/statuscategory/2", ID: 2, Key: "new", ColorName: "blue-gray", Name: "To Do"}
	}
}

func JiraStatus(id, name, internalCategory, baseURL string) StatusRef {
	return StatusRef{
		Self:           fmt.Sprintf("%s/rest/api/3/status/%s", baseURL, id),
		ID:             id,
		Name:           name,
		StatusCategory: categoryFor(internalCategory, baseURL),
	}
}

// IssueTypeRef modella issuetype v3 (schema IssueTypeDetails, campi essenziali).
type IssueTypeRef struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IconURL     string `json:"iconUrl,omitempty"`
	Subtask     bool   `json:"subtask"`
}

func JiraIssueType(id, name, iconURL string, subtask bool, baseURL string) IssueTypeRef {
	return IssueTypeRef{
		Self:    fmt.Sprintf("%s/rest/api/3/issuetype/%s", baseURL, id),
		ID:      id,
		Name:    name,
		IconURL: iconURL,
		Subtask: subtask,
	}
}

// ResolutionRef modella resolution v3.
type ResolutionRef struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func JiraResolution(id, name, desc, baseURL string) ResolutionRef {
	return ResolutionRef{
		Self:        fmt.Sprintf("%s/rest/api/3/resolution/%s", baseURL, id),
		ID:          id,
		Name:        name,
		Description: desc,
	}
}
```

- [ ] **Step 4: verifica PASS**

Run: `go test ./internal/api/v3/ -count=1`
Expected: PASS

- [ ] **Step 5: commit**

```bash
git add internal/api/v3/reference.go internal/api/v3/reference_test.go
git commit -m "feat(v3): reference types/builders for priority, status, issuetype, resolution"
```

---

### Task 5: Mapping v3.JiraIssue (IssueBean)

**Files:**
- Create: `internal/api/v3/issue.go`, `internal/api/v3/issue_test.go`

- [ ] **Step 1: test fallente**

```go
// internal/api/v3/issue_test.go
package v3

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/user"
)

func TestJiraIssue_TopLevel(t *testing.T) {
	iss := issue.Issue{ID: "u-1", SeqID: 10001, Key: "DEMO-1", Title: "Do it",
		Priority: issue.PriorityHigh, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	bean := JiraIssue(IssueInput{Issue: iss, BaseURL: "http://h"})
	if bean.ID != "10001" {
		t.Errorf("id = %q, want 10001", bean.ID)
	}
	if bean.Key != "DEMO-1" {
		t.Errorf("key = %q", bean.Key)
	}
	if bean.Self != "http://h/rest/api/3/issue/10001" {
		t.Errorf("self = %q", bean.Self)
	}
	if bean.Fields.Summary != "Do it" {
		t.Errorf("summary = %q", bean.Fields.Summary)
	}
	if bean.Fields.Priority == nil || bean.Fields.Priority.Name != "High" {
		t.Errorf("priority = %+v", bean.Fields.Priority)
	}
	// default issuetype + status quando non risolti
	if bean.Fields.IssueType == nil || bean.Fields.IssueType.Name != "Task" {
		t.Errorf("default issuetype = %+v", bean.Fields.IssueType)
	}
	if bean.Fields.Status == nil || bean.Fields.Status.StatusCategory.Key != "new" {
		t.Errorf("default status = %+v", bean.Fields.Status)
	}
}

func TestJiraIssue_DescriptionADF(t *testing.T) {
	iss := issue.Issue{ID: "u2", SeqID: 10002, Key: "DEMO-2", Title: "x", DescriptionJSON: `{"content":"Hello world"}`}
	bean := JiraIssue(IssueInput{Issue: iss, BaseURL: "http://h"})
	raw, _ := json.Marshal(bean.Fields.Description)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	if doc["type"] != "doc" {
		t.Errorf("description not ADF doc: %s", raw)
	}
}

func TestJiraIssue_AssigneeAndLabels(t *testing.T) {
	iss := issue.Issue{ID: "u3", SeqID: 10003, Key: "DEMO-3", Title: "x"}
	assignee := &user.User{ID: "ua", DisplayName: "Ana", IsActive: true}
	bean := JiraIssue(IssueInput{Issue: iss, BaseURL: "http://h", Assignee: assignee, Labels: []string{"backend", "urgent"}})
	if bean.Fields.Assignee == nil || bean.Fields.Assignee.AccountID != "ua" {
		t.Errorf("assignee = %+v", bean.Fields.Assignee)
	}
	if len(bean.Fields.Labels) != 2 {
		t.Errorf("labels = %v", bean.Fields.Labels)
	}
}
```

- [ ] **Step 2: verifica FAIL**

Run: `go test ./internal/api/v3/ -run TestJiraIssue`
Expected: FAIL — `undefined: JiraIssue`.

- [ ] **Step 3: implementa**

```go
// internal/api/v3/issue.go
package v3

import (
	"encoding/json"
	"fmt"

	"github.com/open-jira/open-jira/internal/adf"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/user"
)

// ProjectRef è un riferimento minimale a un progetto dentro una issue.
type ProjectRef struct {
	Self string `json:"self"`
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// ParentRef è un riferimento minimale al parent di una issue.
type ParentRef struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// TimeTracking modella il time tracking v3.
type TimeTracking struct {
	OriginalEstimateSeconds  int `json:"originalEstimateSeconds,omitempty"`
	TimeSpentSeconds         int `json:"timeSpentSeconds,omitempty"`
	RemainingEstimateSeconds int `json:"remainingEstimateSeconds,omitempty"`
}

// IssueFields è il contenuto free-form di IssueBean.fields (il contratto non
// valida stray keys qui). Usiamo i nomi di campo Jira ufficiali.
type IssueFields struct {
	Summary      string        `json:"summary"`
	Description  *adf.Node     `json:"description"`
	IssueType    *IssueTypeRef `json:"issuetype"`
	Status       *StatusRef    `json:"status"`
	Priority     *PriorityRef  `json:"priority"`
	Assignee     *User         `json:"assignee"`
	Reporter     *User         `json:"reporter"`
	Resolution   *ResolutionRef `json:"resolution"`
	Project      *ProjectRef   `json:"project,omitempty"`
	Parent       *ParentRef    `json:"parent,omitempty"`
	Labels       []string      `json:"labels"`
	Created      string        `json:"created"`
	Updated      string        `json:"updated"`
	DueDate      string        `json:"duedate,omitempty"`
	StoryPoints  *int          `json:"customfield_10016,omitempty"`
	TimeTracking *TimeTracking `json:"timetracking,omitempty"`
}

// IssueBean è la rappresentazione Jira v3 di una issue.
type IssueBean struct {
	Self   string      `json:"self"`
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

// IssueInput raccoglie la issue e le entità già risolte per il mapping.
type IssueInput struct {
	Issue      issue.Issue
	BaseURL    string
	Assignee   *user.User
	Reporter   *user.User
	IssueType  *issue.IssueType
	Status     *StatusRef        // già mappato dal chiamante (con categoria), opzionale
	Resolution *ResolutionRef    // opzionale
	Project    *ProjectRef       // opzionale
	Parent     *ParentRef        // opzionale
	Labels     []string
}

// descriptionADF trasforma DescriptionJSON in un documento ADF.
func descriptionADF(descJSON string) *adf.Node {
	if descJSON == "" || descJSON == "{}" {
		return nil
	}
	var node adf.Node
	if err := json.Unmarshal([]byte(descJSON), &node); err == nil && node.Type == "doc" {
		return &node
	}
	// forma legacy {"content":"..."} o testo semplice
	var legacy struct {
		Content string `json:"content"`
	}
	text := descJSON
	if err := json.Unmarshal([]byte(descJSON), &legacy); err == nil && legacy.Content != "" {
		text = legacy.Content
	}
	doc := adf.FromText(text)
	return &doc
}

func JiraIssue(in IssueInput) IssueBean {
	iss := in.Issue
	fields := IssueFields{
		Summary:     iss.Title,
		Description: descriptionADF(iss.DescriptionJSON),
		Priority:    ptr(PriorityForEnum(string(iss.Priority), in.BaseURL)),
		Labels:      in.Labels,
		Created:     JiraTime(iss.CreatedAt),
		Updated:     JiraTime(iss.UpdatedAt),
	}
	if fields.Labels == nil {
		fields.Labels = []string{}
	}
	// issuetype: risolto o default "Task"
	if in.IssueType != nil {
		it := JiraIssueType(in.IssueType.ID, in.IssueType.Name, in.BaseURL+"/static/issuetype-"+in.IssueType.Icon+".svg", in.IssueType.IsSubtask, in.BaseURL)
		fields.IssueType = &it
	} else {
		it := JiraIssueType("0", "Task", in.BaseURL+"/static/issuetype-task.svg", false, in.BaseURL)
		fields.IssueType = &it
	}
	// status: mappato dal chiamante o default To Do
	if in.Status != nil {
		fields.Status = in.Status
	} else {
		s := JiraStatus("0", "To Do", "todo", in.BaseURL)
		fields.Status = &s
	}
	if in.Assignee != nil {
		u := JiraUser(*in.Assignee, in.BaseURL)
		fields.Assignee = &u
	}
	if in.Reporter != nil {
		u := JiraUser(*in.Reporter, in.BaseURL)
		fields.Reporter = &u
	}
	fields.Resolution = in.Resolution
	fields.Project = in.Project
	fields.Parent = in.Parent
	if iss.DueDate != nil {
		fields.DueDate = iss.DueDate.Format("2006-01-02")
	}
	if iss.StoryPoints > 0 {
		sp := iss.StoryPoints
		fields.StoryPoints = &sp
	}
	if iss.OriginalEstimate > 0 || iss.TimeSpent > 0 {
		fields.TimeTracking = &TimeTracking{
			OriginalEstimateSeconds: iss.OriginalEstimate,
			TimeSpentSeconds:        iss.TimeSpent,
		}
	}
	return IssueBean{
		Self:   fmt.Sprintf("%s/rest/api/3/issue/%d", in.BaseURL, iss.SeqID),
		ID:     fmt.Sprintf("%d", iss.SeqID),
		Key:    iss.Key,
		Fields: fields,
	}
}

func ptr[T any](v T) *T { return &v }
```

Nota: `iss.DueDate` è `*time.Time` nel modello; verifica il tipo reale e adatta. `descriptionADF` è tollerante alle tre forme (ADF doc, `{"content":...}`, testo).

- [ ] **Step 4: verifica PASS**

Run: `go test ./internal/api/v3/ -count=1`
Expected: PASS

- [ ] **Step 5: commit**

```bash
git add internal/api/v3/issue.go internal/api/v3/issue_test.go
git commit -m "feat(v3): JiraIssue mapping to official IssueBean shape"
```

---

### Task 6: Endpoint dati di riferimento (priority, resolution, issuetype, status)

**Files:**
- Create: `internal/api/handlers/reference_handler.go`
- Modify: `internal/api/router.go`
- Test: `internal/contract/issue_test.go` (create)

- [ ] **Step 1: contract test fallente**

```go
// internal/contract/issue_test.go
package contract

import (
	"net/http"
	"testing"
)

func TestPriorities_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/priority", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/priority", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /priority non conforme: %v", err)
	}
}

func TestIssueTypes_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issuetype", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/issuetype", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /issuetype non conforme: %v", err)
	}
}
```

- [ ] **Step 2: verifica FAIL**

Run: `go test ./internal/contract/ -run 'TestPriorities|TestIssueTypes' -count=1`
Expected: FAIL — rotte assenti.

- [ ] **Step 3: implementa il reference handler**

```go
// internal/api/handlers/reference_handler.go
package handlers

import (
	"net/http"

	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/workflow"
	"gorm.io/gorm"
)

type ReferenceHandler struct {
	db      *gorm.DB
	baseURL string
}

func NewReferenceHandler(db *gorm.DB, baseURL string) *ReferenceHandler {
	return &ReferenceHandler{db: db, baseURL: baseURL}
}

func (h *ReferenceHandler) Priorities(w http.ResponseWriter, r *http.Request) {
	v3.WriteJSON(w, http.StatusOK, v3.StandardPriorities(h.baseURL))
}

func (h *ReferenceHandler) PriorityByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	for _, p := range v3.StandardPriorities(h.baseURL) {
		if p.ID == id {
			v3.WriteJSON(w, http.StatusOK, p)
			return
		}
	}
	v3.WriteError(w, http.StatusNotFound, []string{"The priority does not exist."}, nil)
}

func (h *ReferenceHandler) IssueTypes(w http.ResponseWriter, r *http.Request) {
	var types []issue.IssueType
	h.db.Find(&types)
	out := make([]v3.IssueTypeRef, 0, len(types))
	for _, t := range types {
		out = append(out, v3.JiraIssueType(t.ID, t.Name, h.baseURL+"/static/issuetype-"+t.Icon+".svg", t.IsSubtask, h.baseURL))
	}
	// se il DB è vuoto, esponi i default standard così l'array è utile
	if len(out) == 0 {
		for i, name := range []string{"Epic", "Story", "Task", "Bug", "Subtask"} {
			out = append(out, v3.JiraIssueType(itoa(i+1), name, h.baseURL+"/static/issuetype-task.svg", name == "Subtask", h.baseURL))
		}
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

func (h *ReferenceHandler) Statuses(w http.ResponseWriter, r *http.Request) {
	var statuses []workflow.WorkflowStatus
	h.db.Find(&statuses)
	out := make([]v3.StatusRef, 0, len(statuses))
	for _, s := range statuses {
		out = append(out, v3.JiraStatus(s.ID, s.Name, string(s.Category), h.baseURL))
	}
	if len(out) == 0 {
		out = append(out,
			v3.JiraStatus("1", "To Do", "todo", h.baseURL),
			v3.JiraStatus("2", "In Progress", "inprogress", h.baseURL),
			v3.JiraStatus("3", "Done", "done", h.baseURL),
		)
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

func (h *ReferenceHandler) Resolutions(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID, Name, Description string
	}
	var rows []row
	h.db.Table("resolutions").Select("id, name, description").Scan(&rows)
	out := make([]v3.ResolutionRef, 0, len(rows))
	for _, rr := range rows {
		out = append(out, v3.JiraResolution(rr.ID, rr.Name, rr.Description, h.baseURL))
	}
	if len(out) == 0 {
		out = append(out,
			v3.JiraResolution("1", "Done", "Work has been completed", h.baseURL),
			v3.JiraResolution("2", "Won't Do", "This issue won't be actioned", h.baseURL),
		)
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

func itoa(i int) string {
	return map[int]string{1: "1", 2: "2", 3: "3", 4: "4", 5: "5"}[i]
}
```

Verifica il nome reale del modello status (`workflow.WorkflowStatus`) e del campo categoria (`Category`), e i nomi colonna della tabella `resolutions`. Adatta.

- [ ] **Step 4: rotte in router.go**

Costruisci `refH := handlers.NewReferenceHandler(db, cfg.BaseURL)` e registra:

```go
	mux.Handle("GET /rest/api/3/priority", authMw(http.HandlerFunc(refH.Priorities)))
	mux.Handle("GET /rest/api/3/priority/{id}", authMw(http.HandlerFunc(refH.PriorityByID)))
	mux.Handle("GET /rest/api/3/issuetype", authMw(http.HandlerFunc(refH.IssueTypes)))
	mux.Handle("GET /rest/api/3/status", authMw(http.HandlerFunc(refH.Statuses)))
	mux.Handle("GET /rest/api/3/resolution", authMw(http.HandlerFunc(refH.Resolutions)))
```

Nota ServeMux: `GET /rest/api/3/priority/{id}` è wildcard a un livello sotto `priority`; non collide con altre rotte perché `priority` è un segmento literal unico. Se emerge un panic di pattern ambigui (come nel Round 1 per `project/type/{k}`), registra invece gli id literal `1..5` con `SetPathValue`.

- [ ] **Step 5: verifica PASS**

Run: `go test ./internal/contract/ -run 'TestPriorities|TestIssueTypes' -count=1 && go build ./...`
Expected: PASS + build OK.

- [ ] **Step 6: commit**

```bash
git add internal/api/handlers/reference_handler.go internal/api/router.go internal/contract/issue_test.go
git commit -m "feat(v3): reference-data endpoints (priority, issuetype, status, resolution)"
```

---

### Task 7: GET /rest/api/3/issue/{issueIdOrKey} conforme

**Files:**
- Modify: `internal/api/handlers/issue_handler.go`, `internal/api/router.go`
- Test: `internal/contract/issue_test.go` (aggiunta)

Contesto: aggiungi `baseURL string` a `IssueHandler` e al costruttore `NewIssueHandler(svc, projectSvc, wfSvc, baseURL)`; aggiorna il call-site in `router.go` (`cfg.BaseURL`) e ogni altro (`grep -rn NewIssueHandler`). Aggiungi l'import `internal/api/v3`, `internal/domain/user`.

- [ ] **Step 1: contract test fallente**

Aggiungi a `internal/contract/issue_test.go` un helper per creare una issue via API e il test:

```go
func createIssueViaAPI(t *testing.T, srv *httptest.Server, jwt, projectKey, summary string) string {
	t.Helper()
	body := `{"fields":{"project":{"key":"` + projectKey + `"},"summary":"` + summary + `","issuetype":{"name":"Task"}}}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create issue status = %d: %s", res.StatusCode, b)
	}
	var out struct{ Key string `json:"key"` }
	json.NewDecoder(res.Body).Decode(&out)
	return out.Key
}

func TestGetIssue_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "First issue")

	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key, nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/issue/"+key, res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /issue/{key} non conforme: %v", err)
	}
}
```

Aggiungi gli import `bytes`/`strings`/`io`/`encoding/json`/`net/http/httptest` mancanti. Questo test dipende dal POST conforme (Task 8) per creare la issue; ordina l'esecuzione così che Task 8 sia implementato insieme (vedi nota sotto), oppure crea la issue via service. **Come nel Round 1 (GET+POST project), esegui Task 7 e Task 8 come un unico blocco combinato** dato che il GET ha bisogno del POST per creare i dati.

- [ ] **Step 2: verifica FAIL**

Run: `go test ./internal/contract/ -run TestGetIssue -count=1`
Expected: FAIL — GET restituisce la forma interna / il POST non è ancora conforme.

- [ ] **Step 3: helper di mapping nel handler**

In `issue_handler.go` aggiungi la risoluzione id-or-key e il caricamento delle entità collegate:

```go
func (h *IssueHandler) resolveIssue(idOrKey string) (*issue.Issue, error) {
	if n, err := strconv.ParseInt(idOrKey, 10, 64); err == nil {
		return h.svc.GetBySeqID(n)
	}
	return h.svc.GetByKey(idOrKey)
}

// buildIssueInput carica le entità collegate e costruisce l'input del mapping v3.
func (h *IssueHandler) buildIssueInput(iss *issue.Issue) v3.IssueInput {
	in := v3.IssueInput{Issue: *iss, BaseURL: h.baseURL}
	db := h.svc.DB()
	if iss.AssigneeID != nil {
		var u user.User
		if db.First(&u, "id = ?", *iss.AssigneeID).Error == nil {
			in.Assignee = &u
		}
	}
	if iss.ReporterID != nil {
		var u user.User
		if db.First(&u, "id = ?", *iss.ReporterID).Error == nil {
			in.Reporter = &u
		}
	}
	if iss.TypeID != nil {
		var it issue.IssueType
		if db.First(&it, "id = ?", *iss.TypeID).Error == nil {
			in.IssueType = &it
		}
	}
	if iss.StatusID != nil {
		var st workflow.WorkflowStatus
		if db.First(&st, "id = ?", *iss.StatusID).Error == nil {
			s := v3.JiraStatus(st.ID, st.Name, string(st.Category), h.baseURL)
			in.Status = &s
		}
	}
	// project ref
	var p project.Project
	if db.First(&p, "id = ?", iss.ProjectID).Error == nil {
		in.Project = &v3.ProjectRef{
			Self: h.baseURL + "/rest/api/3/project/" + itoaInt64(p.SeqID),
			ID:   itoaInt64(p.SeqID), Key: p.Key, Name: p.Name,
		}
	}
	if iss.ParentID != nil {
		var parent issue.Issue
		if db.First(&parent, "id = ?", *iss.ParentID).Error == nil {
			in.Parent = &v3.ParentRef{ID: itoaInt64(parent.SeqID), Key: parent.Key, Self: h.baseURL + "/rest/api/3/issue/" + itoaInt64(parent.SeqID)}
		}
	}
	labels, _ := h.svc.GetLabels(iss.ID)
	in.Labels = labels
	return in
}

func itoaInt64(n int64) string { return strconv.FormatInt(n, 10) }
```

Aggiungi gli import `strconv`, `internal/domain/workflow`, `internal/domain/project` (il handler ha già `projectSvc`; usa `h.svc.DB()` o `h.projectSvc.DB()` in modo coerente). Riscrivi `Get`:

```go
func (h *IssueHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("issueKey")
	iss, err := h.resolveIssue(key)
	if err != nil || iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraIssue(h.buildIssueInput(iss)))
}
```

Nota: il path param della rotta GET è `{issueKey}` (vedi router). Mantieni il nome o aggiornalo coerentemente a `{issueIdOrKey}` sia in rotta sia in handler.

- [ ] **Step 4:** implementa insieme il POST (Task 8) e poi verifica entrambi.

- [ ] **Step 5: commit** (combinato con Task 8).

---

### Task 8: POST /rest/api/3/issue conforme (CreatedIssue)

**Files:**
- Modify: `internal/api/handlers/issue_handler.go`
- Test: `internal/contract/issue_test.go` (aggiunta)

- [ ] **Step 1: contract test fallente**

```go
func TestCreateIssue_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")

	body := `{"fields":{"project":{"key":"DEMO"},"summary":"New issue","issuetype":{"name":"Task"},"priority":{"id":"2"}}}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("POST", "/rest/api/3/issue", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("POST /issue non conforme: %v", err)
	}
}

func TestCreateIssue_MissingSummary_400(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue", strings.NewReader(`{"fields":{"project":{"key":"DEMO"}}}`))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, _ := http.DefaultClient.Do(req)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
}
```

- [ ] **Step 2: verifica FAIL**

Run: `go test ./internal/contract/ -run 'TestCreateIssue|TestGetIssue' -count=1`
Expected: FAIL.

- [ ] **Step 3: riscrivi Create in forma v3**

`CreatedIssue` è `{id,key,self}` con `id` **stringa**. Il body è `{"fields":{...}}` (CreateProjectDetails-like ma per issue). Riscrivi:

```go
func (h *IssueHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Fields struct {
			Project struct {
				Key string `json:"key"`
				ID  string `json:"id"`
			} `json:"project"`
			Summary     string `json:"summary"`
			Description any    `json:"description"`
			IssueType   struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"issuetype"`
			Priority struct {
				ID string `json:"id"`
			} `json:"priority"`
			Parent struct {
				Key string `json:"key"`
			} `json:"parent"`
			Labels []string `json:"labels"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	fe := map[string]string{}
	if req.Fields.Summary == "" {
		fe["summary"] = "You must specify a summary of the issue."
	}
	if req.Fields.Project.Key == "" && req.Fields.Project.ID == "" {
		fe["project"] = "You must specify a valid project."
	}
	if len(fe) > 0 {
		v3.WriteError(w, http.StatusBadRequest, nil, fe)
		return
	}
	proj, err := h.projectSvc.GetByKey(req.Fields.Project.Key)
	if err != nil || proj == nil {
		v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"project": "The project does not exist."})
		return
	}
	// description → stringa interna (se ADF, serializza il JSON; il mapping in lettura lo re-interpreta)
	descJSON := ""
	if req.Fields.Description != nil {
		if b, err := json.Marshal(req.Fields.Description); err == nil {
			descJSON = string(b)
		}
	}
	priority := issue.PriorityMedium
	if p := priorityEnumForID(req.Fields.Priority.ID); p != "" {
		priority = issue.Priority(p)
	}
	var parentID *string
	if req.Fields.Parent.Key != "" {
		if parent, err := h.svc.GetByKey(req.Fields.Parent.Key); err == nil && parent != nil {
			parentID = &parent.ID
		}
	}
	var typeID *string
	if req.Fields.IssueType.ID != "" {
		typeID = &req.Fields.IssueType.ID
	}
	iss, err := h.svc.Create(proj.Key, proj.ID, req.Fields.Summary, descJSON, priority, parentID, typeID)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{err.Error()}, nil)
		return
	}
	// label opzionali
	for _, name := range req.Fields.Labels {
		_, _ = h.svc.AddLabel(iss.ID, proj.ID, name, "")
	}
	v3.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":   itoaInt64(iss.SeqID),
		"key":  iss.Key,
		"self": h.baseURL + "/rest/api/3/issue/" + itoaInt64(iss.SeqID),
	})
}

// priorityEnumForID mappa l'id priorità Jira (1..5) al nostro enum.
func priorityEnumForID(id string) string {
	return map[string]string{"1": "highest", "2": "high", "3": "medium", "4": "low", "5": "lowest"}[id]
}
```

Verifica la firma reale di `svc.Create` e `svc.AddLabel` e adatta. Verifica che `svc.Create` generi la `Key` (es. `DEMO-1`) e assegni il `seq_id` (Task 2).

- [ ] **Step 4: verifica PASS (GET + POST insieme)**

Run: `go test ./internal/contract/ -run 'TestCreateIssue|TestGetIssue' -count=1 && go build ./... && go test ./internal/api/... -count=1`
Expected: PASS. Aggiorna eventuali handler test preesistenti che assumevano la vecchia forma di Create/Get.

- [ ] **Step 5: commit (GET+POST combinati)**

```bash
git add internal/api/handlers/issue_handler.go internal/api/router.go internal/contract/issue_test.go
git commit -m "feat(v3): GET and POST /rest/api/3/issue conform to official contract"
```

---

### Task 9: PUT + DELETE /rest/api/3/issue/{issueIdOrKey} conformi

**Files:**
- Modify: `internal/api/handlers/issue_handler.go`, `internal/api/router.go`
- Test: `internal/contract/issue_test.go` (aggiunta)

- [ ] **Step 1: contract test fallenti**

```go
func TestUpdateIssue_204(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "Before")

	body := `{"fields":{"summary":"After"}}`
	req, _ := http.NewRequest("PUT", srv.URL+"/rest/api/3/issue/"+key, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 204 {
		t.Fatalf("PUT status = %d, want 204", res.StatusCode)
	}
	// verifica che il summary sia cambiato
	greq, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key, nil)
	greq.Header.Set("Authorization", "Bearer "+jwt)
	gres, _ := http.DefaultClient.Do(greq)
	defer gres.Body.Close()
	var bean struct {
		Fields struct{ Summary string `json:"summary"` } `json:"fields"`
	}
	json.NewDecoder(gres.Body).Decode(&bean)
	if bean.Fields.Summary != "After" {
		t.Errorf("summary not updated: %q", bean.Fields.Summary)
	}
}

func TestDeleteIssue_204(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "Trash")
	req, _ := http.NewRequest("DELETE", srv.URL+"/rest/api/3/issue/"+key, nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, _ := http.DefaultClient.Do(req)
	res.Body.Close()
	if res.StatusCode != 204 {
		t.Fatalf("DELETE status = %d, want 204", res.StatusCode)
	}
}
```

Verifica nel contratto lo status di `PUT /issue/{id}` (Jira risponde 204 No Content) e `DELETE` (204):
```bash
python3 -c 'import json;d=json.load(open("docs/contracts/jira-platform-v3.json"));print({m:list(d["paths"]["/rest/api/3/issue/{issueIdOrKey}"][m]["responses"].keys()) for m in ("put","delete")})'
```
Allinea gli assert allo status reale.

- [ ] **Step 2: verifica FAIL**

Run: `go test ./internal/contract/ -run 'TestUpdateIssue|TestDeleteIssue' -count=1`
Expected: FAIL.

- [ ] **Step 3: riscrivi Update e Delete**

```go
func (h *IssueHandler) Update(w http.ResponseWriter, r *http.Request) {
	iss, err := h.resolveIssue(r.PathValue("issueKey"))
	if err != nil || iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	var req struct {
		Fields struct {
			Summary     *string `json:"summary"`
			Description any     `json:"description"`
			Assignee    *struct {
				AccountID string `json:"accountId"`
			} `json:"assignee"`
			Priority *struct {
				ID string `json:"id"`
			} `json:"priority"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	var title, descJSON *string
	if req.Fields.Summary != nil {
		title = req.Fields.Summary
	}
	if req.Fields.Description != nil {
		if b, err := json.Marshal(req.Fields.Description); err == nil {
			s := string(b)
			descJSON = &s
		}
	}
	var priority *issue.Priority
	if req.Fields.Priority != nil {
		if e := priorityEnumForID(req.Fields.Priority.ID); e != "" {
			p := issue.Priority(e)
			priority = &p
		}
	}
	var assigneeID *string
	if req.Fields.Assignee != nil {
		assigneeID = &req.Fields.Assignee.AccountID
	}
	if _, err := h.svc.Update(iss.Key, title, descJSON, priority, assigneeID, nil, nil); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to update issue."}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *IssueHandler) Delete(w http.ResponseWriter, r *http.Request) {
	iss, err := h.resolveIssue(r.PathValue("issueKey"))
	if err != nil || iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	if err := h.svc.Delete(iss.Key); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to delete issue."}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Verifica la firma reale di `svc.Update` (il piano assume `Update(key string, title, descJSON *string, priority *Priority, assigneeID, statusID *string, storyPoints *int)`); adatta l'ordine dei parametri a quella effettiva.

- [ ] **Step 4: verifica PASS**

Run: `go test ./internal/contract/ -run 'TestUpdateIssue|TestDeleteIssue' -count=1 && go build ./...`
Expected: PASS.

- [ ] **Step 5: commit**

```bash
git add internal/api/handlers/issue_handler.go internal/contract/issue_test.go
git commit -m "feat(v3): PUT and DELETE /issue conform to official contract"
```

---

### Task 10: createmeta + editmeta

**Files:**
- Modify: `internal/api/handlers/reference_handler.go`, `internal/api/router.go`
- Test: `internal/contract/issue_test.go` (aggiunta)

- [ ] **Step 1: contract test fallenti**

```go
func TestCreateMeta_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/createmeta", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/issue/createmeta", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("createmeta non conforme: %v", err)
	}
}

func TestEditMeta_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "x")
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key+"/editmeta", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/issue/"+key+"/editmeta", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("editmeta non conforme: %v", err)
	}
}
```

Ispeziona gli schemi `IssueCreateMetadata` e la risposta editmeta per capire quali chiavi sono `additionalProperties:false`:
```bash
python3 -c 'import json;d=json.load(open("docs/contracts/jira-platform-v3.json"));s=d["components"]["schemas"]["IssueCreateMetadata"];print("createmeta props:",list(s.get("properties",{}).keys()),"addl:",s.get("additionalProperties"))'
```

- [ ] **Step 2: verifica FAIL**

Run: `go test ./internal/contract/ -run 'TestCreateMeta|TestEditMeta' -count=1`
Expected: FAIL.

- [ ] **Step 3: implementa (forma minima conforme)**

`IssueCreateMetadata` ha tipicamente `{projects: [...]}` con `additionalProperties` permissivo. Restituisci una struttura minima ma valida:

```go
func (h *ReferenceHandler) CreateMeta(w http.ResponseWriter, r *http.Request) {
	// Forma minima conforme: elenco progetti con i loro issue type.
	v3.WriteJSON(w, http.StatusOK, map[string]any{
		"projects": []any{},
	})
}

func (h *ReferenceHandler) EditMeta(w http.ResponseWriter, r *http.Request) {
	// editmeta minimale: mappa fields vuota (i client la usano per scoprire i campi editabili).
	v3.WriteJSON(w, http.StatusOK, map[string]any{
		"fields": map[string]any{},
	})
}
```

Adatta la forma esatta agli schemi verificati nello Step 1 (se `projects`/`fields` hanno vincoli, popola i campi obbligatori). Registra le rotte in `router.go`:

```go
	mux.Handle("GET /rest/api/3/issue/createmeta", authMw(http.HandlerFunc(refH.CreateMeta)))
	mux.Handle("GET /rest/api/3/issue/{issueKey}/editmeta", authMw(http.HandlerFunc(refH.EditMeta)))
```

Nota ServeMux: `createmeta` è un segmento literal, quindi va registrato in modo che non sia catturato da `GET /rest/api/3/issue/{issueKey}`; con Go 1.22+ il literal vince sul wildcard. Verifica assenza di panic al build.

- [ ] **Step 4: verifica PASS**

Run: `go test ./internal/contract/ -run 'TestCreateMeta|TestEditMeta' -count=1 && go build ./...`
Expected: PASS.

- [ ] **Step 5: commit**

```bash
git add internal/api/handlers/reference_handler.go internal/api/router.go internal/contract/issue_test.go
git commit -m "feat(v3): issue createmeta and editmeta endpoints"
```

---

### Task 11: custom fields (/field) e label (/label)

**Files:**
- Modify: `internal/api/handlers/reference_handler.go`, `internal/api/router.go`
- Test: `internal/contract/issue_test.go` (aggiunta)

- [ ] **Step 1: contract test fallenti**

```go
func TestFields_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/field", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/field", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /field non conforme: %v", err)
	}
}

func TestLabels_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/label", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/label", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /label non conforme: %v", err)
	}
}
```

Ispeziona gli schemi: `GET /field` → array di `FieldDetails`; `GET /label` → `PageBeanString` (`{startAt,maxResults,total,isLast,values:[]string}`). Verifica:
```bash
python3 -c 'import json;d=json.load(open("docs/contracts/jira-platform-v3.json"));print("field 200:",d["paths"]["/rest/api/3/field"]["get"]["responses"]["200"]["content"]["application/json"]["schema"]);print("label 200:",d["paths"]["/rest/api/3/label"]["get"]["responses"]["200"]["content"]["application/json"]["schema"].get("$ref"))'
```

- [ ] **Step 2: verifica FAIL**

Run: `go test ./internal/contract/ -run 'TestFields|TestLabels' -count=1`
Expected: FAIL.

- [ ] **Step 3: implementa**

```go
// FieldDetails minimale: i campi di sistema + i custom field dal DB.
func (h *ReferenceHandler) Fields(w http.ResponseWriter, r *http.Request) {
	type fieldOut struct {
		ID       string `json:"id"`
		Key      string `json:"key"`
		Name     string `json:"name"`
		Custom   bool   `json:"custom"`
		Orderable bool  `json:"orderable"`
		Navigable bool  `json:"navigable"`
		Searchable bool `json:"searchable"`
	}
	out := []fieldOut{
		{ID: "summary", Key: "summary", Name: "Summary", Navigable: true, Orderable: true, Searchable: true},
		{ID: "issuetype", Key: "issuetype", Name: "Issue Type", Navigable: true, Orderable: true, Searchable: true},
		{ID: "status", Key: "status", Name: "Status", Navigable: true, Searchable: true},
		{ID: "priority", Key: "priority", Name: "Priority", Navigable: true, Orderable: true, Searchable: true},
		{ID: "assignee", Key: "assignee", Name: "Assignee", Navigable: true, Orderable: true, Searchable: true},
		{ID: "labels", Key: "labels", Name: "Labels", Navigable: true, Orderable: true, Searchable: true},
	}
	type cf struct{ ID, Name string }
	var customs []cf
	h.db.Table("custom_fields").Select("id, name").Scan(&customs)
	for _, c := range customs {
		out = append(out, fieldOut{ID: "customfield_" + c.ID, Key: "customfield_" + c.ID, Name: c.Name, Custom: true, Navigable: true, Orderable: true, Searchable: true})
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

func (h *ReferenceHandler) Labels(w http.ResponseWriter, r *http.Request) {
	var names []string
	h.db.Table("labels").Distinct("name").Pluck("name", &names)
	if names == nil {
		names = []string{}
	}
	v3.WritePage(w, http.StatusOK, v3.Page[string]{StartAt: 0, MaxResults: 1000, Total: len(names), Values: names})
}
```

Adatta i campi di `FieldDetails` allo schema reale (Step 1) — se ci sono chiavi obbligatorie mancanti, aggiungile; se `additionalProperties:false` rifiuta le nostre, rimuovi le extra. Registra:

```go
	mux.Handle("GET /rest/api/3/field", authMw(http.HandlerFunc(refH.Fields)))
	mux.Handle("GET /rest/api/3/label", authMw(http.HandlerFunc(refH.Labels)))
```

- [ ] **Step 4: verifica PASS**

Run: `go test ./internal/contract/ -run 'TestFields|TestLabels' -count=1 && go build ./...`
Expected: PASS.

- [ ] **Step 5: commit**

```bash
git add internal/api/handlers/reference_handler.go internal/api/router.go internal/contract/issue_test.go
git commit -m "feat(v3): GET /field and GET /label endpoints"
```

---

### Task 12: Seed — tipi/stati sulle issue DEMO + gap report + suite verde

**Files:**
- Modify: `cmd/seed/main.go`
- Modify: `docs/contracts/gap-report.md` (rigenerato)

- [ ] **Step 1: assegna type + status alle issue DEMO nel seed**

In `cmd/seed/main.go`, dopo la creazione delle 5 issue DEMO, assicura (idempotente) che ogni issue abbia un `TypeID` e uno `StatusID`. Se il progetto DEMO ha già issue types/statuses seedati dal workflow di default, prendi il primo status "TO DO" e un issue type "Task"; altrimenti crea i default. Esempio:

```go
	// assegna type "Task" e status "TO DO" alle issue DEMO senza type/status
	var taskType issue.IssueType
	if err := s.DB.Where("project_id = ? AND name = ?", demo.ID, "Task").First(&taskType).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		taskType = issue.IssueType{ID: uuid.NewString(), ProjectID: demo.ID, Name: "Task", Icon: "task", Color: "#4BADE8"}
		if err := s.DB.Create(&taskType).Error; err != nil {
			log.Fatalf("create issue type: %v", err)
		}
	}
	var todo workflow.WorkflowStatus
	_ = s.DB.Where("name = ?", "TO DO").First(&todo).Error // dal workflow di default, se presente
	updates := map[string]any{"type_id": taskType.ID}
	if todo.ID != "" {
		updates["status_id"] = todo.ID
	}
	s.DB.Model(&issue.Issue{}).Where("project_id = ? AND (type_id IS NULL OR type_id = '')", demo.ID).Updates(updates)
```

Adatta ai nomi reali (workflow status "TO DO", tabella issue_types). Import `errors`, `gorm`, `uuid`, `internal/domain/workflow` se non presenti.

- [ ] **Step 2: verifica seed idempotente + issue mappabili**

Run:
```bash
rm -f /tmp/seed-r2.db && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/seed-r2.db go run ./cmd/seed && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/seed-r2.db go run ./cmd/seed && sqlite3 /tmp/seed-r2.db 'select key, seq_id, type_id is not null as has_type from issues;' && rm -f /tmp/seed-r2.db
```
Expected: 5 issue DEMO-1..DEMO-5 con `seq_id` valorizzato (≥10000) e `has_type=1`; seconda run non duplica.

- [ ] **Step 3: suite completa + gap report**

Run:
```bash
go build ./... && go vet ./... && go test ./... -count=1 2>&1 | grep -c "^FAIL"
go run ./cmd/gapreport
```
Expected: 0 FAIL; `matched` cresce (issue, priority, issuetype, status, resolution, field, label, createmeta, editmeta).

- [ ] **Step 4: commit**

```bash
git add cmd/seed/main.go docs/contracts/gap-report.md
git commit -m "feat(seed): assign type/status to demo issues; regenerate gap report"
```

---

### Task 13: Frontend — tipo Issue v3 e chiamate API

**Files:**
- Modify: `frontend-next/lib/api.ts`

- [ ] **Step 1: aggiungi il tipo Issue e le chiamate**

In `frontend-next/lib/api.ts` aggiungi (riusa `apiFetch`, `buildQuery`, `PagedResponse`, `JiraUserRef` già presenti dal Round 1):

```ts
export interface ADFNode {
  type: string;
  version?: number;
  text?: string;
  content?: ADFNode[];
  attrs?: Record<string, unknown>;
  marks?: { type: string; attrs?: Record<string, unknown> }[];
}

export interface IssueTypeRef { id: string; name: string; iconUrl?: string; subtask: boolean; }
export interface StatusRef {
  id: string; name: string;
  statusCategory: { id: number; key: string; colorName: string; name: string };
}
export interface PriorityRef { id: string; name: string; iconUrl?: string; statusColor?: string; }

export interface IssueFields {
  summary: string;
  description: ADFNode | null;
  issuetype: IssueTypeRef | null;
  status: StatusRef | null;
  priority: PriorityRef | null;
  assignee: JiraUserRef | null;
  reporter: JiraUserRef | null;
  labels: string[];
  created: string;
  updated: string;
  duedate?: string;
  parent?: { id: string; key: string };
  project?: { id: string; key: string; name: string };
}

export interface Issue {
  self: string;
  id: string;      // numeric string
  key: string;     // e.g. DEMO-1
  fields: IssueFields;
}

export const issues = {
  get: (idOrKey: string) => apiFetch<Issue>(`/rest/api/3/issue/${idOrKey}`),

  create: (payload: {
    projectKey: string;
    summary: string;
    issueTypeName?: string;
    priorityId?: string;
    description?: ADFNode;
    labels?: string[];
  }) =>
    apiFetch<{ id: string; key: string; self: string }>("/rest/api/3/issue", {
      method: "POST",
      body: JSON.stringify({
        fields: {
          project: { key: payload.projectKey },
          summary: payload.summary,
          issuetype: { name: payload.issueTypeName ?? "Task" },
          ...(payload.priorityId ? { priority: { id: payload.priorityId } } : {}),
          ...(payload.description ? { description: payload.description } : {}),
          ...(payload.labels ? { labels: payload.labels } : {}),
        },
      }),
    }),

  update: (idOrKey: string, fields: Record<string, unknown>) =>
    apiFetch<void>(`/rest/api/3/issue/${idOrKey}`, { method: "PUT", body: JSON.stringify({ fields }) }),

  del: (idOrKey: string) => apiFetch<void>(`/rest/api/3/issue/${idOrKey}`, { method: "DELETE" }),
};

export const meta = {
  priorities: () => apiFetch<PriorityRef[]>("/rest/api/3/priority"),
  issueTypes: () => apiFetch<IssueTypeRef[]>("/rest/api/3/issuetype"),
  statuses: () => apiFetch<StatusRef[]>("/rest/api/3/status"),
};
```

Se `JiraUserRef` non è esportato dal Round 1, esportalo. Non toccare la sezione `projects`.

- [ ] **Step 2: verifica build**

Run: `cd frontend-next && npm run build`
Expected: build pulito.

- [ ] **Step 3: commit**

```bash
git add frontend-next/lib/api.ts
git commit -m "feat(frontend): v3 Issue type and issue/meta API calls"
```

---

### Task 14: Frontend — vista issue completa

**Files:**
- Create: `frontend-next/app/jira/browse/[key]/page.tsx`, `frontend-next/components/issues/IssueView.tsx`, `frontend-next/components/issues/adf.tsx`

- [ ] **Step 1: renderer ADF → testo/HTML minimale**

```tsx
// frontend-next/components/issues/adf.tsx
"use client";
import type { ADFNode } from "@/lib/api";

// Render ADF minimale: paragrafi, testo con grassetto/corsivo, liste. Sufficiente
// per la vista issue del Round 2; l'editor ricco arriva con TipTap più avanti.
export function AdfRenderer({ doc }: { doc: ADFNode | null }) {
  if (!doc) return <p className="text-slate-400 italic">No description</p>;
  return <div className="prose prose-sm max-w-none">{doc.content?.map((n, i) => <AdfBlock key={i} node={n} />)}</div>;
}

function AdfBlock({ node }: { node: ADFNode }) {
  if (node.type === "paragraph") return <p>{node.content?.map((c, i) => <AdfInline key={i} node={c} />)}</p>;
  if (node.type === "bulletList") return <ul className="list-disc pl-5">{node.content?.map((c, i) => <AdfBlock key={i} node={c} />)}</ul>;
  if (node.type === "listItem") return <li>{node.content?.map((c, i) => <AdfBlock key={i} node={c} />)}</li>;
  return <>{node.content?.map((c, i) => <AdfBlock key={i} node={c} />)}</>;
}

function AdfInline({ node }: { node: ADFNode }) {
  if (node.type !== "text") return null;
  let el: React.ReactNode = node.text;
  for (const m of node.marks ?? []) {
    if (m.type === "strong") el = <strong>{el}</strong>;
    if (m.type === "em") el = <em>{el}</em>;
    if (m.type === "code") el = <code className="bg-slate-100 px-1 rounded">{el}</code>;
  }
  return <>{el}</>;
}
```

- [ ] **Step 2: componente IssueView**

```tsx
// frontend-next/components/issues/IssueView.tsx
"use client";
import { useQuery } from "@tanstack/react-query";
import { issues } from "@/lib/api";
import { AdfRenderer } from "./adf";

export function IssueView({ issueKey }: { issueKey: string }) {
  const { data: issue, isLoading, error } = useQuery({
    queryKey: ["issue", issueKey],
    queryFn: () => issues.get(issueKey),
  });
  if (isLoading) return <div className="p-8 text-slate-500">Loading…</div>;
  if (error || !issue) return <div className="p-8 text-slate-500">Issue not found.</div>;
  const f = issue.fields;
  return (
    <div className="max-w-5xl p-8">
      <div className="mb-2 text-xs font-semibold text-slate-500">{issue.key}</div>
      <h1 className="mb-6 text-2xl font-semibold text-[#1a1f36]">{f.summary}</h1>
      <div className="grid grid-cols-[1fr_260px] gap-8">
        <div>
          <h2 className="mb-2 text-xs font-semibold uppercase tracking-wider text-slate-500">Description</h2>
          <AdfRenderer doc={f.description} />
        </div>
        <aside className="space-y-4 rounded-lg border border-slate-200 p-4">
          <Field label="Status" value={f.status?.name} />
          <Field label="Assignee" value={f.assignee?.displayName ?? "Unassigned"} />
          <Field label="Reporter" value={f.reporter?.displayName ?? "—"} />
          <Field label="Priority" value={f.priority?.name} />
          <Field label="Type" value={f.issuetype?.name} />
          <Field label="Labels" value={f.labels.length ? f.labels.join(", ") : "None"} />
        </aside>
      </div>
    </div>
  );
}

function Field({ label, value }: { label: string; value?: string }) {
  return (
    <div>
      <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">{label}</div>
      <div className="text-sm text-[#1a1f36]">{value ?? "—"}</div>
    </div>
  );
}
```

- [ ] **Step 3: route browse/[key]**

```tsx
// frontend-next/app/jira/browse/[key]/page.tsx
import { IssueView } from "@/components/issues/IssueView";

export default async function Page({ params }: { params: Promise<{ key: string }> }) {
  const { key } = await params;
  return <IssueView issueKey={key} />;
}
```

- [ ] **Step 4: verifica build**

Run: `cd frontend-next && npm run build`
Expected: build pulito; la route `/jira/browse/[key]` compare.

- [ ] **Step 5: commit**

```bash
git add frontend-next/app/jira/browse/ frontend-next/components/issues/IssueView.tsx frontend-next/components/issues/adf.tsx
git commit -m "feat(frontend): full issue view with ADF description rendering"
```

---

### Task 15: Frontend — modale creazione issue + edit inline summary

**Files:**
- Create: `frontend-next/components/issues/CreateIssueModal.tsx`
- Modify: `frontend-next/components/issues/IssueView.tsx`

- [ ] **Step 1: modale creazione**

```tsx
// frontend-next/components/issues/CreateIssueModal.tsx
"use client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { issues, meta } from "@/lib/api";

export function CreateIssueModal({ projectKey, onClose, onCreated }: { projectKey: string; onClose: () => void; onCreated: (key: string) => void; }) {
  const qc = useQueryClient();
  const { data: types } = useQuery({ queryKey: ["issuetypes"], queryFn: () => meta.issueTypes() });
  const [summary, setSummary] = useState("");
  const [typeName, setTypeName] = useState("Task");

  const create = useMutation({
    mutationFn: () => issues.create({ projectKey, summary, issueTypeName: typeName }),
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ["projectIssues", projectKey] });
      onCreated(res.key);
      onClose();
    },
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30">
      <div className="w-[480px] rounded-lg bg-white p-6 shadow-xl">
        <h2 className="mb-4 text-lg font-semibold">Create issue</h2>
        <label htmlFor="issue-type" className="mb-1 block text-xs font-semibold uppercase tracking-wider text-slate-500">Type</label>
        <select id="issue-type" value={typeName} onChange={(e) => setTypeName(e.target.value)} className="mb-3 w-full rounded border border-slate-300 px-3 py-2">
          {(types ?? [{ id: "0", name: "Task", subtask: false }]).map((t) => <option key={t.id} value={t.name}>{t.name}</option>)}
        </select>
        <label htmlFor="issue-summary" className="mb-1 block text-xs font-semibold uppercase tracking-wider text-slate-500">Summary</label>
        <input id="issue-summary" value={summary} onChange={(e) => setSummary(e.target.value)} className="mb-4 w-full rounded border border-slate-300 px-3 py-2" />
        {create.isError && <p className="mb-3 text-red-600">{create.error instanceof Error ? create.error.message : "Failed"}</p>}
        <div className="flex justify-end gap-3">
          <button onClick={onClose} className="rounded px-4 py-2 text-slate-600">Cancel</button>
          <button onClick={() => create.mutate()} disabled={!summary || create.isPending} className="rounded bg-[#0052cc] px-4 py-2 font-semibold text-white disabled:opacity-60">Create</button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: edit inline del summary nella IssueView**

In `IssueView.tsx`, rendi il titolo editabile inline: click → input; su blur/Enter chiama `issues.update(issueKey, { summary })` e invalida `["issue", issueKey]`. Aggiungi:

```tsx
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
// dentro il componente, dopo aver ottenuto `issue`:
const qc = useQueryClient();
const [editing, setEditing] = useState(false);
const [draft, setDraft] = useState("");
const save = useMutation({
  mutationFn: (summary: string) => issues.update(issue.key, { summary }),
  onSuccess: () => { qc.invalidateQueries({ queryKey: ["issue", issueKey] }); setEditing(false); },
});
// sostituisci l'<h1> con:
{editing ? (
  <input
    autoFocus
    defaultValue={f.summary}
    onChange={(e) => setDraft(e.target.value)}
    onBlur={() => save.mutate(draft || f.summary)}
    onKeyDown={(e) => { if (e.key === "Enter") save.mutate(draft || f.summary); if (e.key === "Escape") setEditing(false); }}
    className="mb-6 w-full rounded border border-[#0052cc] px-2 py-1 text-2xl font-semibold"
  />
) : (
  <h1 className="mb-6 cursor-text text-2xl font-semibold text-[#1a1f36]" onClick={() => { setDraft(f.summary); setEditing(true); }}>{f.summary}</h1>
)}
```

- [ ] **Step 3: verifica build**

Run: `cd frontend-next && npm run build`
Expected: build pulito.

- [ ] **Step 4: commit**

```bash
git add frontend-next/components/issues/CreateIssueModal.tsx frontend-next/components/issues/IssueView.tsx
git commit -m "feat(frontend): create-issue modal and inline summary edit"
```

---

### Task 16: E2E issue + fix rotta riga progetto

**Files:**
- Create: `frontend-next/e2e/issues.spec.ts`
- Modify: `frontend-next/components/projects/ProjectsPage.tsx` (fix rotta riga 404)

- [ ] **Step 1: fix della rotta riga progetto**

In `ProjectsPage.tsx` il click sulla riga apre `/jira/project/{key}` (singolare, 404). Poiché la board non esiste ancora, cambia la destinazione a una vista esistente: la lista issue del progetto non c'è ancora come pagina; punta temporaneamente alle impostazioni del progetto, oppure — meglio — rendi la riga NON navigante (rimuovi `window.open(...)`) e lascia la navigazione solo al menu azioni. Scegli la seconda (meno sorprendente): rimuovi l'handler di navigazione della riga; mantieni il menu azioni. Commenta il motivo (`// la board del progetto arriverà nel Round 5`).

- [ ] **Step 2: E2E — crea issue via API e visualizzala**

Poiché la creazione issue dalla UI richiede un punto d'ingresso (che sarà nella board del Round 5), l'E2E del Round 2 verifica la **vista issue** navigando direttamente a `/jira/browse/DEMO-1` (issue seedata) dopo il login:

```ts
// frontend-next/e2e/issues.spec.ts
import { test, expect } from "@playwright/test";

async function login(page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/jira\/projects/);
}

test("apre la vista di una issue seedata e mostra i campi", async ({ page }) => {
  await login(page);
  await page.goto("/jira/browse/DEMO-1");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(page.getByText(/status/i)).toBeVisible();
  await expect(page.getByText(/priority/i)).toBeVisible();
});

test("modifica inline del summary di una issue", async ({ page }) => {
  await login(page);
  await page.goto("/jira/browse/DEMO-2");
  const h1 = page.getByRole("heading", { level: 1 });
  await h1.click();
  const input = page.locator('input[value], input').first();
  await input.fill("Summary modificato E2E");
  await input.press("Enter");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(/Summary modificato E2E/);
});
```

Adatta i selettori dopo aver letto i componenti reali (l'input inline potrebbe richiedere un selettore più preciso). Il seed deve avere DEMO-1 e DEMO-2 con `seq_id`/type/status (Task 12).

- [ ] **Step 3: run**

Run: `cd frontend-next && npx playwright test e2e/issues.spec.ts`
Expected: 2 passed. Se la porta 8080 è occupata nel sandbox, verifica su porta alternativa e ripristina la config prima del commit (come nei round precedenti).

- [ ] **Step 4: commit**

```bash
git add frontend-next/e2e/issues.spec.ts frontend-next/components/projects/ProjectsPage.tsx
git commit -m "test(e2e): issue view and inline edit; fix project row 404 navigation"
```

---

### Task 17: Rimozione del vecchio ToJiraResponse + suite finale

**Files:**
- Modify: `internal/domain/issue/model.go` (rimozione codice morto), eventuali chiamanti

- [ ] **Step 1: verifica che il vecchio mapping non sia più usato**

Run: `grep -rn "ToJiraResponse\|JiraIssueResponse\|JiraIssueFields" internal/ cmd/`
Expected: solo le definizioni in `model.go` (nessun chiamante di produzione dopo la riscrittura del handler). Se qualche handler lo usa ancora (es. `List`, `ExportCSV`), migralo a `v3.JiraIssue` o lascialo se fuori scope — in tal caso NON rimuovere il codice e salta questo task, annotando il follow-up.

- [ ] **Step 2: rimuovi il codice morto**

Se nessun chiamante di produzione resta, elimina `ToJiraResponse`, `JiraIssueResponse`, `JiraIssueFields` da `internal/domain/issue/model.go`.

- [ ] **Step 3: suite completa**

Run: `go build ./... && go vet ./... && go test ./... -count=1 2>&1 | grep -c "^FAIL"`
Expected: 0.

- [ ] **Step 4: commit**

```bash
git add internal/domain/issue/model.go
git commit -m "refactor(issue): remove legacy non-conformant ToJiraResponse mapping"
```

---

## Definition of Done del Round 2

- `go build ./... && go vet ./... && go test ./...` verdi.
- Contract test verdi per: `GET /issue/{idOrKey}`, `POST /issue`, `PUT /issue/{idOrKey}`, `DELETE /issue/{idOrKey}`, `GET /priority`, `GET /issuetype`, `GET /status`, `GET /resolution`, `GET /issue/createmeta`, `GET /issue/{id}/editmeta`, `GET /field`, `GET /label`.
- Issue con id numerici (10001+), `GET /issue/10001` e `GET /issue/DEMO-1` entrambi 200.
- `description` esposta come ADF; assignee/reporter come User v3; status con `statusCategory`.
- `docs/contracts/gap-report.md` rigenerato e committato.
- Frontend: vista issue completa con rendering ADF, modale creazione, edit inline; build pulito; E2E verdi.
- Fix della rotta riga progetto (niente più 404).

## Note e follow-up ereditati/nuovi

- Editor rich-text ADF completo (TipTap) per description/commenti: rimandato; il Round 2 rende solo (renderer read-only) e crea con testo semplice.
- `createmeta`/`editmeta` in forma minima conforme: arricchire con i campi reali per-progetto quando servirà a un client SDK.
- Custom field: esposti in `/field` in sola lettura di base; POST /field e valori custom completi restano da approfondire.
- Board/backlog del progetto (punto d'ingresso naturale per creare issue dalla UI) arrivano nel Round 5; per ora la creazione issue è testata via API e la vista via navigazione diretta.
- Follow-up Round 1 ancora aperti: `favourite`/star progetti (#32), filtri search progetti, paginazione lista, errori login formato v3.
