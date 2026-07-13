package search

import (
	"testing"

	"github.com/google/uuid"
	"github.com/open-jira/open-jira/internal/domain/issue"
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
	if err := db.AutoMigrate(&issue.Issue{}, &issue.Label{}, &issue.IssueLabel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedIssue(t *testing.T, db *gorm.DB, projectID, title, statusID string, seq int64) *issue.Issue {
	t.Helper()
	iss := &issue.Issue{ID: uuid.NewString(), ProjectID: projectID, Key: "DEMO-" + title[:1], Title: title, StatusID: &statusID, SeqID: seq}
	if err := db.Create(iss).Error; err != nil {
		t.Fatalf("create issue: %v", err)
	}
	return iss
}

func TestSearch_ByProject(t *testing.T) {
	db := newDB(t)
	seedIssue(t, db, "proj-1", "Alpha", "st-todo", 1)
	seedIssue(t, db, "proj-2", "Beta", "st-todo", 2)

	svc := NewService(db)
	r := &staticResolver{project: map[string]string{"DEMO": "proj-1"}}
	res, err := svc.Search(`project = DEMO`, r, 0, 50)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 1 || len(res.Issues) != 1 || res.Issues[0].Title != "Alpha" {
		t.Errorf("risultato errato: total=%d issues=%d", res.Total, len(res.Issues))
	}
}

func TestSearch_EmptyJQLReturnsAll(t *testing.T) {
	db := newDB(t)
	seedIssue(t, db, "proj-1", "Alpha", "st-todo", 1)
	seedIssue(t, db, "proj-1", "Beta", "st-todo", 2)
	svc := NewService(db)
	res, err := svc.Search(``, &staticResolver{}, 0, 50)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 2 {
		t.Errorf("attese 2 issue, %d", res.Total)
	}
}

func TestSearch_Pagination(t *testing.T) {
	db := newDB(t)
	for i := int64(1); i <= 5; i++ {
		seedIssue(t, db, "proj-1", string(rune('A'+i)), "st-todo", i)
	}
	svc := NewService(db)
	res, err := svc.Search(``, &staticResolver{}, 2, 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 5 {
		t.Errorf("total atteso 5, %d", res.Total)
	}
	if len(res.Issues) != 2 {
		t.Errorf("page size atteso 2, %d", len(res.Issues))
	}
}

func TestSearch_InvalidJQL(t *testing.T) {
	svc := NewService(newDB(t))
	if _, err := svc.Search(`project =`, &staticResolver{}, 0, 50); err == nil {
		t.Error("attesa err per JQL invalida")
	}
}

func TestSearch_CountOnly(t *testing.T) {
	db := newDB(t)
	seedIssue(t, db, "proj-1", "Alpha", "st-todo", 1)
	seedIssue(t, db, "proj-1", "Beta", "st-todo", 2)
	svc := NewService(db)
	res, err := svc.Search(``, &staticResolver{}, 0, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 2 {
		t.Errorf("total atteso 2, %d", res.Total)
	}
	if len(res.Issues) != 0 {
		t.Errorf("count-only non deve restituire righe, ne ha %d", len(res.Issues))
	}
}

// TestSearch_StatusByName_SpansProjects verifica il fix per il bug per cui
// status/type venivano risolti a un singolo id "First()", scelto arbitrariamente
// tra i progetti. Due progetti hanno ciascuno una propria riga workflow_statuses
// chiamata "To Do" (id diversi): status = "To Do" deve abbracciare entrambi.
func TestSearch_StatusByName_SpansProjects(t *testing.T) {
	db := newDB(t)
	if err := db.AutoMigrate(&workflow.WorkflowStatus{}); err != nil {
		t.Fatalf("migrate workflow_statuses: %v", err)
	}

	st1 := &workflow.WorkflowStatus{ID: "st-todo-proj1", WorkflowID: "wf-1", Name: "To Do"}
	st2 := &workflow.WorkflowStatus{ID: "st-todo-proj2", WorkflowID: "wf-2", Name: "To Do"}
	if err := db.Create(st1).Error; err != nil {
		t.Fatalf("create status 1: %v", err)
	}
	if err := db.Create(st2).Error; err != nil {
		t.Fatalf("create status 2: %v", err)
	}

	seedIssue(t, db, "proj-1", "Alpha", st1.ID, 1)
	seedIssue(t, db, "proj-2", "Beta", st2.ID, 2)

	svc := NewService(db)
	res, err := svc.Search(`status = "To Do"`, &staticResolver{}, 0, 50)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 2 || len(res.Issues) != 2 {
		t.Errorf("atteso status name-based su entrambi i progetti: total=%d issues=%d", res.Total, len(res.Issues))
	}
}

// staticResolver implementa jql.Resolver per i test.
type staticResolver struct {
	project map[string]string
}

func (s *staticResolver) ProjectID(k string) (string, bool) { id, ok := s.project[k]; return id, ok }
func (s *staticResolver) UserID(string) (string, bool)      { return "", false }
func (s *staticResolver) CurrentUserID() string             { return "user-me" }
