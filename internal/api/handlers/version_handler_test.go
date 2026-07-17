package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/version"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

func setupVersionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	db.AutoMigrate(
		&user.User{},
		&project.Project{}, &project.ProjectMember{}, &project.Invite{},
		&workflow.WorkflowStatus{},
		&issue.Issue{},
		&version.Version{}, &version.IssueVersion{},
	)
	return db
}

func newVersionTestHandler(t *testing.T, db *gorm.DB, adminUID string) *VersionHandler {
	t.Helper()
	versionSvc := version.NewService(db)
	userSvc := user.NewService(db)
	projSvc := project.NewService(db, &user.User{ID: adminUID})
	issueSvc := issue.NewService(db)
	chk := authz.New(userSvc, projSvc, issueSvc, nil, nil, nil, nil, versionSvc)
	return NewVersionHandler(versionSvc, projSvc, chk, "http://localhost:8080")
}

func withUID(req *http.Request, uid string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uid))
}

// TestVersionHandler_Lifecycle: create -> get -> list -> update(released) -> delete
// esercitati direttamente sull'handler.
func TestVersionHandler_Lifecycle(t *testing.T) {
	db := setupVersionTestDB(t)
	adminUID := uuid.New().String()
	db.Create(&user.User{ID: adminUID, Email: "admin@example.com", Username: adminUID, DisplayName: "Admin", IsActive: true, IsAdmin: true})

	h := newVersionTestHandler(t, db, adminUID)
	p := &project.Project{ID: uuid.New().String(), Key: "DEMO", Name: "Demo", SeqID: 10000}
	db.Create(p)

	// Create
	body, _ := json.Marshal(map[string]any{
		"name": "v1.0", "description": "first", "startDate": "2026-01-01", "releaseDate": "2026-06-30", "project": "DEMO",
	})
	req := withUID(httptest.NewRequest("POST", "/rest/api/3/version", bytes.NewReader(body)), adminUID)
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", w.Code, w.Body.String())
	}
	var created v3.JiraVersion
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.ID == "" || created.ProjectID != 10000 || created.StartDate != "2026-01-01" || created.Released {
		t.Fatalf("unexpected created version: %+v", created)
	}

	// Get
	greq := httptest.NewRequest("GET", "/rest/api/3/version/"+created.ID, nil)
	greq.SetPathValue("id", created.ID)
	gw := httptest.NewRecorder()
	h.Get(gw, greq)
	if gw.Code != http.StatusOK {
		t.Fatalf("get status = %d", gw.Code)
	}

	// List
	lreq := httptest.NewRequest("GET", "/rest/api/3/project/DEMO/versions", nil)
	lreq.SetPathValue("key", "DEMO")
	lw := httptest.NewRecorder()
	h.List(lw, lreq)
	var list []v3.JiraVersion
	json.Unmarshal(lw.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}

	// Update released=true
	ureq := httptest.NewRequest("PUT", "/rest/api/3/version/"+created.ID, bytes.NewReader([]byte(`{"released":true}`)))
	ureq.SetPathValue("id", created.ID)
	uw := httptest.NewRecorder()
	h.Update(uw, ureq)
	if uw.Code != http.StatusOK {
		t.Fatalf("update status = %d", uw.Code)
	}
	var updated v3.JiraVersion
	json.Unmarshal(uw.Body.Bytes(), &updated)
	if !updated.Released {
		t.Errorf("released = %v, want true", updated.Released)
	}

	// Delete
	dreq := httptest.NewRequest("DELETE", "/rest/api/3/version/"+created.ID, nil)
	dreq.SetPathValue("id", created.ID)
	dw := httptest.NewRecorder()
	h.Delete(dw, dreq)
	if dw.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", dw.Code)
	}
}

// TestVersionHandler_CreateNonAdmin_403 verifica che un utente non membro del
// progetto non possa creare una version (autorizzazione in-handler).
func TestVersionHandler_CreateNonAdmin_403(t *testing.T) {
	db := setupVersionTestDB(t)
	adminUID := uuid.New().String()
	db.Create(&user.User{ID: adminUID, Email: "admin@example.com", Username: adminUID, DisplayName: "Admin", IsActive: true, IsAdmin: true})
	bobUID := uuid.New().String()
	db.Create(&user.User{ID: bobUID, Email: "bob@example.com", Username: bobUID, DisplayName: "Bob", IsActive: true})

	h := newVersionTestHandler(t, db, adminUID)
	p := &project.Project{ID: uuid.New().String(), Key: "DEMO", Name: "Demo", SeqID: 10000}
	db.Create(p)

	body, _ := json.Marshal(map[string]any{"name": "nope", "project": "DEMO"})
	req := withUID(httptest.NewRequest("POST", "/rest/api/3/version", bytes.NewReader(body)), bobUID)
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin create status = %d, want 403", w.Code)
	}
}
