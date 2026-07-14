package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

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
	db.AutoMigrate(&user.User{}, &project.Project{}, &project.ProjectMember{}, &project.Invite{}, &workflow.Workflow{}, &workflow.WorkflowStatus{}, &workflow.WorkflowTransition{})
	return db
}

func TestProjectCreateHandler(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	h := NewProjectHandler(svc, workflow.NewService(db), nil, "http://localhost:8080")

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
	h := NewProjectHandler(svc, workflow.NewService(db), nil, "http://localhost:8080")

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
	h := NewProjectHandler(svc, workflow.NewService(db), nil, "http://localhost:8080")

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
	h := NewProjectHandler(svc, workflow.NewService(db), nil, "http://localhost:8080")

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
	h := NewProjectHandler(svc, workflow.NewService(db), nil, "http://localhost:8080")

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
	h := NewProjectHandler(svc, workflow.NewService(db), nil, "http://localhost:8080")

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
	h := NewProjectHandler(svc, workflow.NewService(db), nil, "http://localhost:8080")

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
