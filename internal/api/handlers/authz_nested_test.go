package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/domain/group"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/sprint"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/webhook"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

// nestedFixture: two projects A and B, a caller who is admin of A but only a
// viewer of B, one issue per project, and a ServeMux wired exactly like
// router.go for the nested-child routes under test (parent enforced via
// chk.Enforce on {key} / {issueIdOrKey}). The tests send B's child id under A's
// path and expect 404 with B's row untouched: the authz layer only checks the
// parent, so the handler/service must bind the child to it.
type nestedFixture struct {
	db         *gorm.DB
	mux        *http.ServeMux
	chk        *authz.Checker
	issueSvc   *issue.Service
	commentSvc *issue.CommentService
	worklogSvc *issue.WorklogService
	webhookSvc *webhook.Service
	wfSvc      *workflow.Service
	sprintSvc  *sprint.Service

	adminA  string // admin of A, viewer of B
	memberB string // member of B (EditIssues, not AdministerProjects)
	authorB string // member of B, author of B's comment
	adminB  string // admin of B

	projA, projB   *project.Project
	issueA, issueB *issue.Issue
}

func setupNestedFixture(t *testing.T) *nestedFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&user.User{},
		&project.Project{}, &project.ProjectMember{}, &project.ProjectTeam{}, &group.GroupMember{},
		&workflow.Workflow{}, &workflow.WorkflowStatus{}, &workflow.WorkflowTransition{},
		&issue.Issue{}, &issue.IssueType{}, &issue.IssueHistory{}, &issue.Comment{}, &issue.Worklog{},
		&webhook.Webhook{},
		&sprint.Sprint{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	newUser := func(name string) string {
		id := uuid.New().String()
		if err := db.Create(&user.User{ID: id, Email: name + "@example.com", Username: id, DisplayName: name, IsActive: true}).Error; err != nil {
			t.Fatalf("create user %s: %v", name, err)
		}
		return id
	}
	f := &nestedFixture{db: db}
	f.adminA = newUser("adminA")
	f.memberB = newUser("memberB")
	f.authorB = newUser("authorB")
	f.adminB = newUser("adminB")

	userSvc := user.NewService(db)
	projSvc := project.NewService(db, nil)
	f.issueSvc = issue.NewService(db)
	f.commentSvc = issue.NewCommentService(db)
	f.worklogSvc = issue.NewWorklogService(db)
	f.webhookSvc = webhook.NewService(db)
	f.wfSvc = workflow.NewService(db)
	f.sprintSvc = sprint.NewService(db)
	f.chk = authz.New(userSvc, projSvc, f.issueSvc, nil, f.sprintSvc, nil, nil, nil)

	f.projA = &project.Project{ID: uuid.New().String(), Key: "AAA", Name: "A", SeqID: 10000}
	f.projB = &project.Project{ID: uuid.New().String(), Key: "BBB", Name: "B", SeqID: 10001}
	for _, p := range []*project.Project{f.projA, f.projB} {
		if err := db.Create(p).Error; err != nil {
			t.Fatalf("create project %s: %v", p.Key, err)
		}
		if _, err := f.wfSvc.CreateDefaultWorkflow(p.ID, false); err != nil {
			t.Fatalf("default workflow %s: %v", p.Key, err)
		}
	}
	for _, m := range []struct {
		proj *project.Project
		uid  string
		role project.MemberRole
	}{
		{f.projA, f.adminA, project.RoleAdmin},
		{f.projB, f.adminA, project.RoleViewer},
		{f.projB, f.memberB, project.RoleMember},
		{f.projB, f.authorB, project.RoleMember},
		{f.projB, f.adminB, project.RoleAdmin},
	} {
		if err := projSvc.AddMember(m.proj.ID, m.uid, m.role); err != nil {
			t.Fatalf("add member: %v", err)
		}
	}
	if f.issueA, err = f.issueSvc.Create("AAA", f.projA.ID, "issue A", "", issue.PriorityMedium, nil, nil); err != nil {
		t.Fatalf("create issue A: %v", err)
	}
	if f.issueB, err = f.issueSvc.Create("BBB", f.projB.ID, "issue B", "", issue.PriorityMedium, nil, nil); err != nil {
		t.Fatalf("create issue B: %v", err)
	}

	const baseURL = "http://localhost:8080"
	commentH := NewCommentHandler(f.commentSvc, f.issueSvc, f.chk, baseURL)
	worklogH := NewWorklogHandler(f.worklogSvc, f.issueSvc, baseURL)
	webhookH := NewWebhookHandler(f.webhookSvc, projSvc)
	wfH := NewWorkflowHandler(f.wfSvc, f.issueSvc, projSvc, baseURL)
	sprintH := NewSprintHandler(f.sprintSvc, projSvc)
	chk := f.chk

	// Same wiring as internal/api/router.go for the routes under test (the
	// auth middleware is replaced by withUID on the request).
	mux := http.NewServeMux()
	mux.Handle("DELETE /rest/api/3/project/{key}/webhooks/{id}", chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(webhookH.Delete)))
	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/comment/{id}", chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(commentH.Get)))
	mux.Handle("PUT /rest/api/3/issue/{issueIdOrKey}/comment/{id}", chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(commentH.Update)))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/comment/{id}", chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(commentH.Delete)))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/worklog/{id}", chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(worklogH.Delete)))
	mux.Handle("PATCH /rest/api/3/project/{key}/workflow/statuses/{id}", chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(wfH.UpdateStatus)))
	mux.Handle("DELETE /rest/api/3/project/{key}/workflow/statuses/{id}", chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(wfH.DeleteStatus)))
	mux.Handle("PATCH /rest/api/3/project/{key}/workflow/transitions/{id}", chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(wfH.UpdateTransition)))
	mux.Handle("DELETE /rest/api/3/project/{key}/workflow/transitions/{id}", chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(wfH.DeleteTransition)))
	mux.Handle("GET /rest/api/3/project/{key}/sprints/{id}", chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(sprintH.Get)))
	mux.Handle("PATCH /rest/api/3/project/{key}/sprints/{id}", chk.Enforce(permission.ManageSprints, chk.ByKey, http.HandlerFunc(sprintH.Update)))
	mux.Handle("POST /rest/api/3/project/{key}/sprints/{id}/start", chk.Enforce(permission.ManageSprints, chk.ByKey, http.HandlerFunc(sprintH.Start)))
	mux.Handle("POST /rest/api/3/project/{key}/sprints/{id}/complete", chk.Enforce(permission.ManageSprints, chk.ByKey, http.HandlerFunc(sprintH.Complete)))
	f.mux = mux
	return f
}

// do sends method/path with an optional JSON body as uid through the mux.
func (f *nestedFixture) do(t *testing.T, uid, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd *bytes.Reader
	if body != "" {
		rd = bytes.NewReader([]byte(body))
	} else {
		rd = bytes.NewReader(nil)
	}
	req := withUID(httptest.NewRequest(method, path, rd), uid)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.mux.ServeHTTP(w, req)
	return w
}

func (f *nestedFixture) expect(t *testing.T, what string, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("%s: status = %d, want %d: %s", what, w.Code, want, strings.TrimSpace(w.Body.String()))
	}
}

const adfBody = `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"%s"}]}]}`

func adf(text string) string { return strings.Replace(adfBody, "%s", text, 1) }

// seedCommentB creates a comment on issue B authored by authorB.
func (f *nestedFixture) seedCommentB(t *testing.T) *issue.Comment {
	t.Helper()
	c, err := f.commentSvc.AddComment(f.issueB.ID, f.authorB, adf("original"))
	if err != nil {
		t.Fatalf("add comment: %v", err)
	}
	return c
}

func (f *nestedFixture) reloadComment(t *testing.T, id string) issue.Comment {
	t.Helper()
	var c issue.Comment
	if err := f.db.First(&c, "id = ?", id).Error; err != nil {
		t.Fatalf("reload comment: %v", err)
	}
	return c
}

// TestNested_CommentUpdateDelete_OtherProjectChildIs404: admin of A, viewer of
// B, sends B's comment id under A's issue → 404 for GET/PUT/DELETE, B's comment
// unchanged (body and is_deleted).
func TestNested_CommentUpdateDelete_OtherProjectChildIs404(t *testing.T) {
	f := setupNestedFixture(t)
	cB := f.seedCommentB(t)
	base := "/rest/api/3/issue/" + f.issueA.Key + "/comment/" + cB.ID

	f.expect(t, "GET other-project comment", f.do(t, f.adminA, http.MethodGet, base, ""), http.StatusNotFound)
	f.expect(t, "PUT other-project comment", f.do(t, f.adminA, http.MethodPut, base, `{"body":`+adf("tampered")+`}`), http.StatusNotFound)
	f.expect(t, "DELETE other-project comment", f.do(t, f.adminA, http.MethodDelete, base, ""), http.StatusNotFound)

	got := f.reloadComment(t, cB.ID)
	if got.BodyJSON != adf("original") {
		t.Errorf("comment body changed across project boundary: %s", got.BodyJSON)
	}
	if got.IsDeleted {
		t.Errorf("comment soft-deleted across project boundary")
	}
}

// TestNested_CommentUpdate_NonAuthorNonAdminIs403or404: a member of B who is
// neither the comment's author nor a project admin may read the comment
// (BrowseProjects) but may not edit or delete it → 403 (not 404: the comment
// is visible to them, so hiding its existence buys nothing; 403 is also what
// Jira returns). The author may edit it (200) and a project admin may delete
// it (204).
func TestNested_CommentUpdate_NonAuthorNonAdminIs403or404(t *testing.T) {
	f := setupNestedFixture(t)
	cB := f.seedCommentB(t)
	base := "/rest/api/3/issue/" + f.issueB.Key + "/comment/" + cB.ID

	f.expect(t, "PUT as non-author member", f.do(t, f.memberB, http.MethodPut, base, `{"body":`+adf("tampered")+`}`), http.StatusForbidden)
	f.expect(t, "DELETE as non-author member", f.do(t, f.memberB, http.MethodDelete, base, ""), http.StatusForbidden)
	got := f.reloadComment(t, cB.ID)
	if got.BodyJSON != adf("original") || got.IsDeleted {
		t.Errorf("comment changed by non-author non-admin: body=%s deleted=%v", got.BodyJSON, got.IsDeleted)
	}

	// The author can still edit their own comment. (The handler re-marshals
	// the decoded ADF, so keys may be reordered: compare on the text.)
	f.expect(t, "PUT as author", f.do(t, f.authorB, http.MethodPut, base, `{"body":`+adf("edited by author")+`}`), http.StatusOK)
	if got := f.reloadComment(t, cB.ID); !strings.Contains(got.BodyJSON, "edited by author") {
		t.Errorf("author edit not applied: %s", got.BodyJSON)
	}
	// A project admin can moderate (delete) someone else's comment.
	f.expect(t, "DELETE as project admin", f.do(t, f.adminB, http.MethodDelete, base, ""), http.StatusNoContent)
	if got := f.reloadComment(t, cB.ID); !got.IsDeleted {
		t.Errorf("project admin delete not applied")
	}
}

// TestNested_WorklogDelete_OtherProjectChildIs404: B's worklog id under A's
// issue → 404, worklog row still present.
func TestNested_WorklogDelete_OtherProjectChildIs404(t *testing.T) {
	f := setupNestedFixture(t)
	wl, err := f.worklogSvc.Add(f.issueB.ID, f.authorB, "", 3600)
	if err != nil {
		t.Fatalf("add worklog: %v", err)
	}

	w := f.do(t, f.adminA, http.MethodDelete, "/rest/api/3/issue/"+f.issueA.Key+"/worklog/"+wl.ID, "")
	f.expect(t, "DELETE other-project worklog", w, http.StatusNotFound)

	var cnt int64
	f.db.Model(&issue.Worklog{}).Where("id = ?", wl.ID).Count(&cnt)
	if cnt != 1 {
		t.Errorf("worklog deleted across project boundary (rows=%d)", cnt)
	}
}

// TestNested_ProjectWebhookUpdateDelete_OtherProjectChildIs404: B's webhook id
// under A's project → 404, webhook still present. (There is no webhook update
// route; DELETE is the only child-id route.)
func TestNested_ProjectWebhookUpdateDelete_OtherProjectChildIs404(t *testing.T) {
	f := setupNestedFixture(t)
	hook, err := f.webhookSvc.Create(f.projB.ID, "https://example.com/hook", "", []string{"issue_created"})
	if err != nil {
		t.Fatalf("create webhook: %v", err)
	}

	w := f.do(t, f.adminA, http.MethodDelete, "/rest/api/3/project/AAA/webhooks/"+hook.ID, "")
	f.expect(t, "DELETE other-project webhook", w, http.StatusNotFound)

	var cnt int64
	f.db.Model(&webhook.Webhook{}).Where("id = ?", hook.ID).Count(&cnt)
	if cnt != 1 {
		t.Errorf("webhook deleted across project boundary (rows=%d)", cnt)
	}
}

// TestNested_WorkflowStatusUpdateRemove_OtherProjectIs404: a status of B's
// workflow addressed under A's project → 404 on PATCH and DELETE, status
// unchanged and still present.
func TestNested_WorkflowStatusUpdateRemove_OtherProjectIs404(t *testing.T) {
	f := setupNestedFixture(t)
	wfB, err := f.wfSvc.GetWorkflow(f.projB.ID)
	if err != nil {
		t.Fatal(err)
	}
	stB := wfB.Statuses[0]
	base := "/rest/api/3/project/AAA/workflow/statuses/" + stB.ID

	f.expect(t, "PATCH other-project status", f.do(t, f.adminA, http.MethodPatch, base, `{"name":"HIJACKED","category":"done","color":"#000"}`), http.StatusNotFound)
	f.expect(t, "DELETE other-project status", f.do(t, f.adminA, http.MethodDelete, base, ""), http.StatusNotFound)

	var got workflow.WorkflowStatus
	if err := f.db.First(&got, "id = ?", stB.ID).Error; err != nil {
		t.Fatalf("status removed across project boundary: %v", err)
	}
	if got.Name != stB.Name || got.Category != stB.Category {
		t.Errorf("status changed across project boundary: %+v", got)
	}
}

// TestNested_WorkflowTransitionUpdateRemove_OtherProjectIs404: a transition of
// B's workflow addressed under A's project → 404 on PATCH and DELETE,
// transition unchanged and still present.
func TestNested_WorkflowTransitionUpdateRemove_OtherProjectIs404(t *testing.T) {
	f := setupNestedFixture(t)
	wfB, err := f.wfSvc.GetWorkflow(f.projB.ID)
	if err != nil {
		t.Fatal(err)
	}
	trB := wfB.Transitions[0]
	base := "/rest/api/3/project/AAA/workflow/transitions/" + trB.ID

	f.expect(t, "PATCH other-project transition", f.do(t, f.adminA, http.MethodPatch, base, `{"name":"HIJACKED","require_assignee":true}`), http.StatusNotFound)
	f.expect(t, "DELETE other-project transition", f.do(t, f.adminA, http.MethodDelete, base, ""), http.StatusNotFound)

	var got workflow.WorkflowTransition
	if err := f.db.First(&got, "id = ?", trB.ID).Error; err != nil {
		t.Fatalf("transition removed across project boundary: %v", err)
	}
	if got.Name != trB.Name || got.RequireAssignee != trB.RequireAssignee {
		t.Errorf("transition changed across project boundary: %+v", got)
	}
}

// TestNested_ProjectSprintUpdateDelete_OtherProjectChildIs404: B's sprint id
// under A's project → 404 on GET/PATCH/start/complete, sprint unchanged. (The
// legacy project-sprint API has no DELETE route; the agile DELETE resolves the
// project from the sprint itself via BySprintSeq.)
func TestNested_ProjectSprintUpdateDelete_OtherProjectChildIs404(t *testing.T) {
	f := setupNestedFixture(t)
	spB, err := f.sprintSvc.Create(f.projB.ID, "Sprint B", "goal B")
	if err != nil {
		t.Fatalf("create sprint: %v", err)
	}
	base := "/rest/api/3/project/AAA/sprints/" + spB.ID

	f.expect(t, "GET other-project sprint", f.do(t, f.adminA, http.MethodGet, base, ""), http.StatusNotFound)
	f.expect(t, "PATCH other-project sprint", f.do(t, f.adminA, http.MethodPatch, base, `{"name":"HIJACKED","goal":"x"}`), http.StatusNotFound)
	f.expect(t, "START other-project sprint", f.do(t, f.adminA, http.MethodPost, base+"/start", ""), http.StatusNotFound)
	f.expect(t, "COMPLETE other-project sprint", f.do(t, f.adminA, http.MethodPost, base+"/complete", `{"move_open_to_backlog":true}`), http.StatusNotFound)

	var got sprint.Sprint
	if err := f.db.First(&got, "id = ?", spB.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Name != "Sprint B" || got.Goal != "goal B" || got.State != sprint.StateFuture || got.StartDate != nil || got.CompleteDate != nil {
		t.Errorf("sprint changed across project boundary: %+v", got)
	}
}
