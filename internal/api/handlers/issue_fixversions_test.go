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

	"github.com/it4nodummies/heureum/internal/api/authz"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/version"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

func setupFixVersionsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	db.AutoMigrate(
		&user.User{},
		&project.Project{}, &project.ProjectMember{}, &project.Invite{},
		&workflow.WorkflowStatus{},
		&issue.Issue{}, &issue.Label{},
		&version.Version{}, &version.IssueVersion{},
	)
	return db
}

func newIssueFixVersionsHandler(t *testing.T, db *gorm.DB, adminUID string) *IssueHandler {
	t.Helper()
	issueSvc := issue.NewService(db)
	projSvc := project.NewService(db, &user.User{ID: adminUID})
	userSvc := user.NewService(db)
	versionSvc := version.NewService(db)
	wfSvc := workflow.NewService(db)
	chk := authz.New(userSvc, projSvc, issueSvc, nil, nil, nil, nil, versionSvc)
	return NewIssueHandler(issueSvc, projSvc, wfSvc, chk, versionSvc, "http://localhost:8080")
}

// TestIssueHandler_FixVersions verifica il read + create/update multi
// fix-versions: PUT fields.fixVersions:[{id}] li assegna, GET li riflette,
// PUT [] li azzera.
func TestIssueHandler_FixVersions(t *testing.T) {
	db := setupFixVersionsTestDB(t)
	adminUID := uuid.New().String()
	db.Create(&user.User{ID: adminUID, Email: "admin@example.com", Username: adminUID, DisplayName: "Admin", IsActive: true, IsAdmin: true})

	h := newIssueFixVersionsHandler(t, db, adminUID)
	p := &project.Project{ID: uuid.New().String(), Key: "DEMO", Name: "Demo", SeqID: 10000}
	db.Create(p)

	issueSvc := issue.NewService(db)
	iss, err := issueSvc.Create(p.Key, p.ID, "needs a release", "", issue.PriorityMedium, nil, nil)
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}

	versionSvc := version.NewService(db)
	v1, err := versionSvc.Create(p.ID, "v1.0", "first", nil, nil)
	if err != nil {
		t.Fatalf("create version: %v", err)
	}

	// PUT fields.fixVersions:[{id:v1}]
	body, _ := json.Marshal(map[string]any{
		"fields": map[string]any{
			"fixVersions": []map[string]string{{"id": v1.ID}},
		},
	})
	ureq := httptest.NewRequest("PUT", "/rest/api/3/issue/"+iss.Key, bytes.NewReader(body))
	ureq.SetPathValue("issueKey", iss.Key)
	uw := httptest.NewRecorder()
	h.Update(uw, ureq)
	if uw.Code != http.StatusNoContent {
		t.Fatalf("update status = %d: %s", uw.Code, uw.Body.String())
	}

	// GET -> fields.fixVersions contains v1 (id+name)
	greq := httptest.NewRequest("GET", "/rest/api/3/issue/"+iss.Key, nil)
	greq.SetPathValue("issueKey", iss.Key)
	gw := httptest.NewRecorder()
	h.Get(gw, greq)
	if gw.Code != http.StatusOK {
		t.Fatalf("get status = %d", gw.Code)
	}
	var got v3.IssueBean
	if err := json.Unmarshal(gw.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal issue: %v", err)
	}
	if len(got.Fields.FixVersions) != 1 {
		t.Fatalf("fixVersions len = %d, want 1: %s", len(got.Fields.FixVersions), gw.Body.String())
	}
	if got.Fields.FixVersions[0].ID != v1.ID || got.Fields.FixVersions[0].Name != "v1.0" {
		t.Fatalf("unexpected fixVersion: %+v", got.Fields.FixVersions[0])
	}
	if got.Fields.FixVersions[0].Self == "" {
		t.Errorf("fixVersion self is empty")
	}

	// PUT [] -> cleared
	clearBody, _ := json.Marshal(map[string]any{
		"fields": map[string]any{"fixVersions": []map[string]string{}},
	})
	creq := httptest.NewRequest("PUT", "/rest/api/3/issue/"+iss.Key, bytes.NewReader(clearBody))
	creq.SetPathValue("issueKey", iss.Key)
	cw := httptest.NewRecorder()
	h.Update(cw, creq)
	if cw.Code != http.StatusNoContent {
		t.Fatalf("clear update status = %d: %s", cw.Code, cw.Body.String())
	}

	greq2 := httptest.NewRequest("GET", "/rest/api/3/issue/"+iss.Key, nil)
	greq2.SetPathValue("issueKey", iss.Key)
	gw2 := httptest.NewRecorder()
	h.Get(gw2, greq2)
	var got2 v3.IssueBean
	json.Unmarshal(gw2.Body.Bytes(), &got2)
	if len(got2.Fields.FixVersions) != 0 {
		t.Fatalf("after clear, fixVersions len = %d, want 0", len(got2.Fields.FixVersions))
	}
}
