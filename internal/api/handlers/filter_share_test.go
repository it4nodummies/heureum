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

	"github.com/it4nodummies/heureum/internal/api/middleware"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/search"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

func setupFilterTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	db.AutoMigrate(&user.User{}, &search.SavedFilter{})
	return db
}

// TestFilterCreate_ExposesIsShared verifica che il campo is_shared inviato nel
// body venga persistito ed esposto nella risposta. Lo schema Jira Filter è
// chiuso (additionalProperties:false) e non ammette un booleano custom, quindi
// la condivisione è esposta tramite sharePermissions ({"type":"global"}); una
// GET successiva deve riflettere lo stesso stato.
func TestFilterCreate_ExposesIsShared(t *testing.T) {
	db := setupFilterTestDB(t)
	uid := uuid.New().String()
	db.Create(&user.User{ID: uid, Email: "owner@test.com", Username: uid, DisplayName: "Owner", IsActive: true})

	h := NewFilterHandler(search.NewFilterService(db), db, "http://localhost:8080", nil)

	body := map[string]any{"name": "Shared", "jql": "project = DEMO", "is_shared": true}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/rest/api/3/filter", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uid))
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var created struct {
		ID               string           `json:"id"`
		SharePermissions []map[string]any `json:"sharePermissions"`
	}
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if !hasGlobalShare(created.SharePermissions) {
		t.Errorf("expected a global sharePermission in create response, got %+v", created.SharePermissions)
	}

	// GET must reflect the shared state too.
	greq := httptest.NewRequest("GET", "/rest/api/3/filter/"+created.ID, nil)
	greq.SetPathValue("id", created.ID)
	greq = greq.WithContext(context.WithValue(greq.Context(), middleware.UserIDKey, uid))
	gw := httptest.NewRecorder()
	h.Get(gw, greq)
	if gw.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET, got %d: %s", gw.Code, gw.Body.String())
	}
	var got struct {
		SharePermissions []map[string]any `json:"sharePermissions"`
	}
	json.NewDecoder(gw.Body).Decode(&got)
	if !hasGlobalShare(got.SharePermissions) {
		t.Errorf("expected a global sharePermission on GET, got %+v", got.SharePermissions)
	}
}

func hasGlobalShare(perms []map[string]any) bool {
	for _, p := range perms {
		if p["type"] == "global" {
			return true
		}
	}
	return false
}

// TestListMembers_HydratesUserInfo verifica che ListMembers restituisca i campi
// utente v3 (displayName) oltre al ruolo, non il grezzo ProjectMember.
func TestListMembers_HydratesUserInfo(t *testing.T) {
	db := setupProjectTestDB(t)
	uid := uuid.New().String()
	db.Create(&user.User{ID: uid, Email: "member@test.com", Username: uid, DisplayName: "Member One", IsActive: true})

	svc := project.NewService(db, &user.User{ID: uuid.New().String()})
	p, _ := svc.Create("Members", "MEM", "desc", project.TypeScrum)
	if err := svc.AddMember(p.ID, uid, project.RoleAdmin); err != nil {
		t.Fatalf("add member: %v", err)
	}

	h := NewProjectHandler(svc, workflow.NewService(db), nil, "http://localhost:8080")
	req := httptest.NewRequest("GET", "/rest/api/3/project/MEM/members", nil)
	req.SetPathValue("key", "MEM")
	w := httptest.NewRecorder()
	h.ListMembers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var members []struct {
		AccountID   string `json:"accountId"`
		DisplayName string `json:"displayName"`
		Role        string `json:"role"`
	}
	if err := json.NewDecoder(w.Body).Decode(&members); err != nil {
		t.Fatalf("decode members: %v", err)
	}
	found := false
	for _, m := range members {
		if m.AccountID == uid {
			found = true
			if m.DisplayName != "Member One" {
				t.Errorf("displayName = %q, want %q", m.DisplayName, "Member One")
			}
			if m.Role != "admin" {
				t.Errorf("role = %q, want admin", m.Role)
			}
		}
	}
	if !found {
		t.Errorf("member %s not found in hydrated response: %s", uid, w.Body.String())
	}
}
