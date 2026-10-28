package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/domain/board"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

func setupProjectTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	db.AutoMigrate(&user.User{}, &project.Project{}, &project.ProjectMember{}, &project.Invite{},
		&workflow.Workflow{}, &workflow.WorkflowStatus{}, &workflow.WorkflowTransition{},
		&board.Board{}, &board.BoardColumn{}, &board.BoardColumnStatus{}, &issue.IssueType{})
	// `resolutions` has no Go model; create it raw (mirrors cmd/seed and
	// resolution_test.go, which EnsureResolution writes into via db.Table).
	db.Exec(`CREATE TABLE resolutions (id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT)`)
	return db
}

func TestProjectCreateHandler(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	h := NewProjectHandler(svc, workflow.NewService(db), board.NewService(db), issue.NewService(db), nil, "http://localhost:8080")

	body := map[string]string{"name": "Test", "key": "TE", "description": "desc", "type": "scrum"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/rest/api/3/project", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	// Create now returns the Jira v3 "ProjectIdentifiers" shape: {id, key, self}.
	var identifiers struct {
		ID   int64  `json:"id"`
		Key  string `json:"key"`
		Self string `json:"self"`
	}
	json.NewDecoder(w.Body).Decode(&identifiers)
	if identifiers.Key != "TE" {
		t.Errorf("Key = %s", identifiers.Key)
	}
	if identifiers.Self == "" {
		t.Errorf("Self should not be empty")
	}
	if identifiers.ID == 0 {
		t.Errorf("ID should not be zero")
	}
}

func TestProjectGetHandler(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	svc.Create("Get Test", "GET", "desc", project.TypeScrum)
	h := NewProjectHandler(svc, workflow.NewService(db), board.NewService(db), issue.NewService(db), nil, "http://localhost:8080")

	req := httptest.NewRequest("GET", "/rest/api/3/project/GET", nil)
	req.SetPathValue("key", "GET")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestProjectGetNotFoundHandler(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	h := NewProjectHandler(svc, workflow.NewService(db), board.NewService(db), issue.NewService(db), nil, "http://localhost:8080")

	req := httptest.NewRequest("GET", "/rest/api/3/project/NOPE", nil)
	req.SetPathValue("key", "NOPE")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestProjectListHandler(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	svc.Create("P1", "P1", "desc", project.TypeScrum)
	h := NewProjectHandler(svc, workflow.NewService(db), board.NewService(db), issue.NewService(db), nil, "http://localhost:8080")

	req := httptest.NewRequest("GET", "/rest/api/3/project", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Total  int                       `json:"total"`
		Values []project.ProjectWithLead `json:"values"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Values) != 1 {
		t.Errorf("expected 1 project, got %d", len(resp.Values))
	}
}

func TestProjectDeleteHandler(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	svc.Create("Del", "DEL", "desc", project.TypeScrum)
	h := NewProjectHandler(svc, workflow.NewService(db), board.NewService(db), issue.NewService(db), nil, "http://localhost:8080")

	req := httptest.NewRequest("DELETE", "/rest/api/3/project/DEL", nil)
	req.SetPathValue("key", "DEL")
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestProjectUpdateHandler(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	svc.Create("Old", "OLD", "old desc", project.TypeScrum)
	h := NewProjectHandler(svc, workflow.NewService(db), board.NewService(db), issue.NewService(db), nil, "http://localhost:8080")

	body := map[string]string{"name": "New Name", "description": "new desc"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/rest/api/3/project/OLD", bytes.NewReader(b))
	req.SetPathValue("key", "OLD")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var p project.Project
	json.NewDecoder(w.Body).Decode(&p)
	if p.Name != "New Name" {
		t.Errorf("Name = %s", p.Name)
	}
}

func TestProjectInviteHandler(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	p, _ := svc.Create("Inv", "INV", "desc", project.TypeScrum)
	h := NewProjectHandler(svc, workflow.NewService(db), board.NewService(db), issue.NewService(db), nil, "http://localhost:8080")

	body := map[string]string{"email": "invited@test.com", "role": "member"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/rest/api/3/project/INV/invites", bytes.NewReader(b))
	req.SetPathValue("key", "INV")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Invite(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	_ = p
}

// postProject drives the Create handler with the given key + template and
// returns the recorder, so scaffold-oriented tests stay terse.
func postProject(t *testing.T, h *ProjectHandler, key, template string) *httptest.ResponseRecorder {
	t.Helper()
	body := map[string]string{"name": key + " proj", "key": key, "projectTemplateKey": template}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/rest/api/3/project", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	return w
}

func TestCreate_ScaffoldsBoardTypesResolution(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	boardSvc := board.NewService(db)
	issueSvc := issue.NewService(db)
	h := NewProjectHandler(svc, workflow.NewService(db), boardSvc, issueSvc, nil, "http://localhost:8080")

	if w := postProject(t, h, "SCRUM", "com.pyxis.greenhopper.jira:gh-scrum-template"); w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	p, err := svc.GetByKey("SCRUM")
	if err != nil {
		t.Fatalf("project not found: %v", err)
	}

	boards, err := boardSvc.ListByProject(p.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(boards) != 1 {
		t.Fatalf("board count = %d, want 1", len(boards))
	}
	if boards[0].Type != "scrum" {
		t.Errorf("board type = %q, want scrum", boards[0].Type)
	}

	var typeCount int64
	db.Model(&issue.IssueType{}).Where("project_id = ?", p.ID).Count(&typeCount)
	if typeCount != 5 {
		t.Errorf("issue type count = %d, want 5", typeCount)
	}

	if _, ok := issueSvc.ResolutionIDByName("Done"); !ok {
		t.Error("global \"Done\" resolution was not created")
	}

	var wfCount int64
	db.Model(&workflow.Workflow{}).Where("project_id = ?", p.ID).Count(&wfCount)
	if wfCount != 1 {
		t.Errorf("workflow count = %d, want 1", wfCount)
	}
}

func TestCreate_TemplateDiffers(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	boardSvc := board.NewService(db)
	h := NewProjectHandler(svc, workflow.NewService(db), boardSvc, issue.NewService(db), nil, "http://localhost:8080")

	// kanban template → kanban board.
	if w := postProject(t, h, "KAN", "com.pyxis.greenhopper.jira:gh-kanban-template"); w.Code != http.StatusCreated {
		t.Fatalf("kanban: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	kb, _ := svc.GetByKey("KAN")
	kboards, _ := boardSvc.ListByProject(kb.ID)
	if len(kboards) != 1 || kboards[0].Type != "kanban" {
		t.Fatalf("kanban board = %+v, want a single kanban board", kboards)
	}

	// business template → kanban board + issue-type set exactly {Task,Bug,Subtask}.
	if w := postProject(t, h, "BIZ", "com.atlassian.jira-core-project-templates:jira-core-simplified-process-control"); w.Code != http.StatusCreated {
		t.Fatalf("business: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	bz, _ := svc.GetByKey("BIZ")
	bboards, _ := boardSvc.ListByProject(bz.ID)
	if len(bboards) != 1 || bboards[0].Type != "kanban" {
		t.Fatalf("business board = %+v, want a single kanban board", bboards)
	}
	var types []issue.IssueType
	db.Where("project_id = ?", bz.ID).Find(&types)
	got := map[string]bool{}
	for _, it := range types {
		got[it.Name] = true
	}
	if len(got) != 3 || !got["Task"] || !got["Bug"] || !got["Subtask"] {
		t.Errorf("business issue types = %v, want exactly {Task,Bug,Subtask}", got)
	}
	if got["Epic"] || got["Story"] {
		t.Errorf("business must not have Epic/Story, got %v", got)
	}
}

// TestProjectCreate_DuplicateKeyIsSanitized400: a second project with the
// same key is a client error on the "key" field — never the driver's UNIQUE
// constraint text.
func TestProjectCreate_DuplicateKeyIsSanitized400(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	h := NewProjectHandler(svc, workflow.NewService(db), board.NewService(db), issue.NewService(db), nil, "http://localhost:8080")

	if w := postProject(t, h, "DUP", "com.pyxis.greenhopper.jira:gh-scrum-template"); w.Code != http.StatusCreated {
		t.Fatalf("first create: %d %s", w.Code, w.Body.String())
	}
	w := postProject(t, h, "DUP", "com.pyxis.greenhopper.jira:gh-scrum-template")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate key: status = %d, want 400, body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, w.Body.String())
	}
	if resp.Errors["key"] == "" {
		t.Errorf("want a field error on key, got %s", w.Body.String())
	}
	for _, leak := range []string{"UNIQUE", "constraint", "projects."} {
		if strings.Contains(w.Body.String(), leak) {
			t.Errorf("400 body leaks %q: %s", leak, w.Body.String())
		}
	}
}

// TestProjectCreate_InternalErrorIsGeneric500: an unexpected DB failure while
// creating the projects row is a generic 500 without driver text.
func TestProjectCreate_InternalErrorIsGeneric500(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	h := NewProjectHandler(svc, workflow.NewService(db), board.NewService(db), issue.NewService(db), nil, "http://localhost:8080")
	if err := db.Exec("DROP TABLE projects").Error; err != nil {
		t.Fatal(err)
	}

	w := postProject(t, h, "GONE", "com.pyxis.greenhopper.jira:gh-scrum-template")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500, body: %s", w.Code, w.Body.String())
	}
	for _, leak := range []string{"no such table", "projects", "SQL", "sqlite"} {
		if strings.Contains(w.Body.String(), leak) {
			t.Errorf("500 body leaks %q: %s", leak, w.Body.String())
		}
	}
}

func TestCreate_ScaffoldFailureRollsBack(t *testing.T) {
	// Main DB has the full schema, so the project, its default workflow (+statuses
	// +transitions) and issue types really get created during scaffolding. The
	// board service, however, points at a separate DB with no `boards` table, so
	// the LAST scaffold step (board.Create) fails — after workflow + issue types
	// already exist. Rollback must then remove the ENTIRE object graph, not just
	// the projects row, regardless of FK enforcement (off under the SQLite driver
	// on this branch).
	db := setupProjectTestDB(t)

	brokenBoardDB, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open broken board db: %v", err)
	}
	// Intentionally NOT migrating board.Board here, so board.Create fails.

	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	h := NewProjectHandler(svc, workflow.NewService(db), board.NewService(brokenBoardDB), issue.NewService(db), nil, "http://localhost:8080")

	w := postProject(t, h, "DOOM", "com.pyxis.greenhopper.jira:gh-scrum-template")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	// The project row must be gone.
	if _, err := svc.GetByKey("DOOM"); err == nil {
		t.Fatalf("project row still present after scaffold failure — rollback did not delete it")
	}
	// This DB is exclusive to this one failed create, so after rollback every
	// scaffold-produced child table must hold ZERO rows — no dangling project_id.
	assertEmpty := func(table string) {
		var n int64
		if err := db.Table(table).Count(&n).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("%s still has %d orphaned row(s) after rollback, want 0", table, n)
		}
	}
	assertEmpty("workflows")
	assertEmpty("workflow_statuses")
	assertEmpty("workflow_transitions")
	assertEmpty("boards")
	assertEmpty("issue_types")
	assertEmpty("projects")
}
