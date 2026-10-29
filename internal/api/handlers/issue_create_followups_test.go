package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/version"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
	"github.com/it4nodummies/heureum/internal/storetest"
)

// issueCreateFixture is a real-schema fixture for POST /rest/api/3/issue:
// an admin, a project with its default workflow and one version.
type issueCreateFixture struct {
	db       *gorm.DB
	h        *IssueHandler
	adminUID string
	proj     *project.Project
	wf       *workflow.Workflow
	ver      *version.Version
}

func newIssueCreateFixture(t *testing.T) *issueCreateFixture {
	t.Helper()
	db := storetest.New(t)
	adminUID := uuid.New().String()
	if err := db.Create(&user.User{ID: adminUID, Email: "admin@example.test", Username: adminUID, DisplayName: "Admin", IsActive: true, IsAdmin: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	p := &project.Project{ID: uuid.New().String(), Key: "FUP", Name: "Follow-ups", SeqID: 10000}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	wfSvc := workflow.NewService(db)
	wf, err := wfSvc.CreateDefaultWorkflow(p.ID, false)
	if err != nil {
		t.Fatalf("default workflow: %v", err)
	}
	versionSvc := version.NewService(db)
	ver, err := versionSvc.Create(p.ID, "v1.0", "", nil, nil)
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	issueSvc := issue.NewService(db)
	projSvc := project.NewService(db, &user.User{ID: adminUID})
	chk := authz.New(user.NewService(db), projSvc, issueSvc, nil, nil, nil, nil, versionSvc)
	h := NewIssueHandler(issueSvc, projSvc, wfSvc, chk, versionSvc, "http://localhost:8080")
	return &issueCreateFixture{db: db, h: h, adminUID: adminUID, proj: p, wf: wf, ver: ver}
}

func (f *issueCreateFixture) post(t *testing.T, fields map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	fields["project"] = map[string]string{"key": f.proj.Key}
	body, _ := json.Marshal(map[string]any{"fields": fields})
	req := withUID(httptest.NewRequest(http.MethodPost, "/rest/api/3/issue", bytes.NewReader(body)), f.adminUID)
	w := httptest.NewRecorder()
	f.h.Create(w, req)
	return w
}

func (f *issueCreateFixture) issueCount(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := f.db.Raw("SELECT count(*) FROM issues WHERE project_id = ?", f.proj.ID).Scan(&n).Error; err != nil {
		t.Fatalf("count issues: %v", err)
	}
	return n
}

// TestCreateIssue_RollsBackOnFollowUpFailure: the create-time follow-ups
// (initial status, assignee, labels, fix versions) are part of the create
// transaction. When one of them fails after the issues row is inserted (here:
// the labels pivot is gone), the client gets a generic 500, no issue row
// survives, the key counter is not consumed, and nothing panics.
func TestCreateIssue_RollsBackOnFollowUpFailure(t *testing.T) {
	f := newIssueCreateFixture(t)
	if err := f.db.Exec("ALTER TABLE issue_labels RENAME TO issue_labels_gone").Error; err != nil {
		t.Fatalf("rename issue_labels: %v", err)
	}

	w := f.post(t, map[string]any{"summary": "with labels", "labels": []string{"backend"}})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500: %s", w.Code, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte("issue_labels")) {
		t.Errorf("500 body leaks the DB error: %s", w.Body.String())
	}
	if n := f.issueCount(t); n != 0 {
		t.Errorf("%d issue row(s) survived the failed create, want 0", n)
	}
	var counter int64
	f.db.Raw("SELECT issue_counter FROM projects WHERE id = ?", f.proj.ID).Scan(&counter)
	if counter != 0 {
		t.Errorf("issue_counter = %d after a rolled-back create, want 0", counter)
	}
}

// TestCreateIssue_UnknownAssigneeIs400: an accountId that is not a user is a
// client error (Jira: errors.assignee), not a silently ignored field and not
// an FK violation surfacing as 500.
func TestCreateIssue_UnknownAssigneeIs400(t *testing.T) {
	f := newIssueCreateFixture(t)
	w := f.post(t, map[string]any{
		"summary":  "bad assignee",
		"assignee": map[string]string{"accountId": "nobody-" + uuid.New().String()},
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
	var body struct {
		Errors map[string]string `json:"errors"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Errors["assignee"] == "" {
		t.Errorf("want errors.assignee in the body, got %s", w.Body.String())
	}
	if n := f.issueCount(t); n != 0 {
		t.Errorf("%d issue row(s) created with an unknown assignee, want 0", n)
	}
}

// TestCreateIssue_AppliesFollowUpsAtomically pins the happy path of the same
// code: one create sets the workflow's TO DO status, the assignee, the labels
// and the fix versions, all visible on the stored row afterwards.
func TestCreateIssue_AppliesFollowUpsAtomically(t *testing.T) {
	f := newIssueCreateFixture(t)
	w := f.post(t, map[string]any{
		"summary":     "full create",
		"assignee":    map[string]string{"accountId": f.adminUID},
		"labels":      []string{"backend", "urgent", "backend"},
		"fixVersions": []map[string]string{{"id": f.ver.ID}},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", w.Code, w.Body.String())
	}
	var created struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.Key == "" {
		t.Fatalf("create response: %v (%s)", err, w.Body.String())
	}

	var iss issue.Issue
	if err := f.db.First(&iss, "key = ?", created.Key).Error; err != nil {
		t.Fatalf("reload issue: %v", err)
	}
	var todoID string
	for _, st := range f.wf.Statuses {
		if st.Category == workflow.CategoryTodo {
			todoID = st.ID
		}
	}
	if iss.StatusID == nil || *iss.StatusID != todoID {
		t.Errorf("status_id = %v, want the workflow's TO DO status %s", iss.StatusID, todoID)
	}
	if iss.AssigneeID == nil || *iss.AssigneeID != f.adminUID {
		t.Errorf("assignee_id = %v, want %s", iss.AssigneeID, f.adminUID)
	}
	labels, err := f.h.svc.GetLabels(iss.ID)
	if err != nil || len(labels) != 2 {
		t.Errorf("labels = %v (err %v), want [backend urgent] (deduplicated)", labels, err)
	}
	var nv int64
	f.db.Raw("SELECT count(*) FROM issue_versions WHERE issue_id = ? AND version_id = ?", iss.ID, f.ver.ID).Scan(&nv)
	if nv != 1 {
		t.Errorf("issue_versions rows for the fix version = %d, want 1", nv)
	}
}

// TestCreateIssue_UnknownIssueTypeIs400: an issuetype.id that is not a type
// of the project is a client error (errors.issuetype), not an FK violation
// surfacing as 500; nothing is created and the key counter is untouched.
func TestCreateIssue_UnknownIssueTypeIs400(t *testing.T) {
	f := newIssueCreateFixture(t)
	w := f.post(t, map[string]any{
		"summary":   "bad type",
		"issuetype": map[string]string{"id": "type-" + uuid.New().String()},
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
	var body struct {
		Errors map[string]string `json:"errors"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Errors["issuetype"] == "" {
		t.Errorf("want errors.issuetype in the body, got %s", w.Body.String())
	}
	if n := f.issueCount(t); n != 0 {
		t.Errorf("%d issue row(s) created with an unknown issue type, want 0", n)
	}
	var counter int64
	f.db.Raw("SELECT issue_counter FROM projects WHERE id = ?", f.proj.ID).Scan(&counter)
	if counter != 0 {
		t.Errorf("issue_counter = %d after a rejected create, want 0", counter)
	}
}
