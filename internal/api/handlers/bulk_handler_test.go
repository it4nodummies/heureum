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
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

// setupBulkTestDB migra le tabelle necessarie al percorso bulk (utenti,
// progetti+membri, workflow, issue+label) su un DB SQLite in memoria.
func setupBulkTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	db.AutoMigrate(
		&user.User{},
		&project.Project{}, &project.ProjectMember{}, &project.Invite{},
		&workflow.Workflow{}, &workflow.WorkflowStatus{}, &workflow.WorkflowTransition{},
		&issue.Issue{}, &issue.IssueType{}, &issue.Label{}, &issue.IssueLabel{}, &issue.IssueHistory{},
	)
	return db
}

// newBulkTestHandler costruisce un IssueHandler con un Checker reale. L'utente
// admin è admin globale, così RequireProject cortocircuita a nil (bypass ruolo)
// e i risolutori figli non sono necessari — passiamo nil per boards/sprints/
// autos/cfs, non toccati da RequireProject/isGlobalAdmin.
func newBulkTestHandler(t *testing.T, db *gorm.DB, adminUID string) *IssueHandler {
	t.Helper()
	issueSvc := issue.NewService(db)
	userSvc := user.NewService(db)
	projSvc := project.NewService(db, &user.User{ID: adminUID})
	chk := authz.New(userSvc, projSvc, issueSvc, nil, nil, nil, nil)
	return NewIssueHandler(issueSvc, projSvc, nil, chk, "http://localhost:8080")
}

func bulkPost(t *testing.T, h *IssueHandler, uid string, body map[string]any) (*httptest.ResponseRecorder, []bulkResult) {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/rest/api/3/issues/bulk", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uid))
	w := httptest.NewRecorder()
	h.BulkUpdate(w, req)
	var resp struct {
		Results []bulkResult `json:"results"`
	}
	if w.Code == http.StatusOK {
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode bulk response: %v", err)
		}
	}
	return w, resp.Results
}

// TestBulkUpdate_PriorityAllOK verifica che un bulk di priorità applichi la
// modifica a tutte le issue (entrambe ok:true) e che il reload rifletta la
// nuova priorità.
func TestBulkUpdate_PriorityAllOK(t *testing.T) {
	db := setupBulkTestDB(t)
	adminUID := uuid.New().String()
	db.Create(&user.User{ID: adminUID, Email: "admin@example.com", Username: adminUID, DisplayName: "Admin", IsActive: true, IsAdmin: true})

	h := newBulkTestHandler(t, db, adminUID)
	svc := h.svc

	p := &project.Project{ID: uuid.New().String(), Key: "DEMO", Name: "Demo", SeqID: 10000}
	db.Create(p)
	i1, err := svc.Create("DEMO", p.ID, "One", "d", issue.PriorityMedium, nil, nil)
	if err != nil {
		t.Fatalf("create i1: %v", err)
	}
	i2, err := svc.Create("DEMO", p.ID, "Two", "d", issue.PriorityMedium, nil, nil)
	if err != nil {
		t.Fatalf("create i2: %v", err)
	}

	w, results := bulkPost(t, h, adminUID, map[string]any{
		"keys":   []string{i1.Key, i2.Key},
		"fields": map[string]any{"priority": map[string]any{"id": "1"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %+v", len(results), results)
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("expected ok:true for %s, got error %q", r.Key, r.Error)
		}
	}
	for _, k := range []string{i1.Key, i2.Key} {
		reloaded, err := svc.GetByKey(k)
		if err != nil {
			t.Fatalf("reload %s: %v", k, err)
		}
		if reloaded.Priority != issue.PriorityHighest {
			t.Errorf("issue %s priority = %q, want highest", k, reloaded.Priority)
		}
	}
}

// TestBulkUpdate_PartialFailure verifica che una chiave inesistente venga
// riportata come ok:false con un error non vuoto, mentre la chiave valida
// dello stesso batch riesce (fallimento parziale, non 403/errore globale).
func TestBulkUpdate_PartialFailure(t *testing.T) {
	db := setupBulkTestDB(t)
	adminUID := uuid.New().String()
	db.Create(&user.User{ID: adminUID, Email: "admin@example.com", Username: adminUID, DisplayName: "Admin", IsActive: true, IsAdmin: true})

	h := newBulkTestHandler(t, db, adminUID)
	svc := h.svc

	p := &project.Project{ID: uuid.New().String(), Key: "DEMO", Name: "Demo", SeqID: 10000}
	db.Create(p)
	i1, err := svc.Create("DEMO", p.ID, "One", "d", issue.PriorityMedium, nil, nil)
	if err != nil {
		t.Fatalf("create i1: %v", err)
	}

	w, results := bulkPost(t, h, adminUID, map[string]any{
		"keys":   []string{i1.Key, "NOPE-999"},
		"fields": map[string]any{"priority": map[string]any{"id": "2"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %+v", len(results), results)
	}
	byKey := map[string]bulkResult{}
	for _, r := range results {
		byKey[r.Key] = r
	}
	if got := byKey[i1.Key]; !got.OK {
		t.Errorf("valid key %s: expected ok:true, got error %q", i1.Key, got.Error)
	}
	if got := byKey["NOPE-999"]; got.OK || got.Error == "" {
		t.Errorf("bogus key: expected ok:false with non-empty error, got %+v", got)
	}
	// La chiave valida deve aver comunque applicato la modifica.
	reloaded, _ := svc.GetByKey(i1.Key)
	if reloaded.Priority != issue.PriorityHigh {
		t.Errorf("issue %s priority = %q, want high", i1.Key, reloaded.Priority)
	}
}
