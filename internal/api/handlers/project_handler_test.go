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

	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/user"
)

func setupProjectTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	db.AutoMigrate(&user.User{}, &project.Project{}, &project.ProjectMember{}, &project.Invite{})
	return db
}

func TestProjectCreateHandler(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	h := NewProjectHandler(svc)

	body := map[string]string{"name": "Test", "key": "TE", "description": "desc", "type": "scrum"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var p project.Project
	json.NewDecoder(w.Body).Decode(&p)
	if p.Key != "TE" {
		t.Errorf("Key = %s", p.Key)
	}
}

func TestProjectGetHandler(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	svc.Create("Get Test", "GET", "desc", project.TypeScrum)
	h := NewProjectHandler(svc)

	req := httptest.NewRequest("GET", "/api/v1/projects/GET", nil)
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
	h := NewProjectHandler(svc)

	req := httptest.NewRequest("GET", "/api/v1/projects/NOPE", nil)
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
	h := NewProjectHandler(svc)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var projects []project.Project
	json.NewDecoder(w.Body).Decode(&projects)
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}
}

func TestProjectDeleteHandler(t *testing.T) {
	db := setupProjectTestDB(t)
	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	svc.Create("Del", "DEL", "desc", project.TypeScrum)
	h := NewProjectHandler(svc)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/DEL", nil)
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
	h := NewProjectHandler(svc)

	body := map[string]string{"name": "New Name", "description": "new desc"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PATCH", "/api/v1/projects/OLD", bytes.NewReader(b))
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
	h := NewProjectHandler(svc)

	body := map[string]string{"email": "invited@test.com", "role": "member"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/projects/INV/invites", bytes.NewReader(b))
	req.SetPathValue("key", "INV")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Invite(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	_ = p
}
