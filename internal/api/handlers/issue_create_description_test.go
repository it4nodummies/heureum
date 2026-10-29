package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	adfpkg "github.com/it4nodummies/heureum/internal/adf"
	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/version"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
	"github.com/it4nodummies/heureum/internal/storetest"
)

// TestIssueHandler_CreateRendersADFDescription is the API-level pin for the
// description storage invariant: POST /rest/api/3/issue with an ADF
// description, then GET it — fields.description must be the same ADF doc
// (its paragraph text rendered), never a doc whose text is the raw JSON
// string `{"content":{"type":"doc"...` that a broken stored value produced.
// Runs on the real migrated schema.
func TestIssueHandler_CreateRendersADFDescription(t *testing.T) {
	db := storetest.New(t)
	adminUID := uuid.New().String()
	if err := db.Create(&user.User{ID: adminUID, Email: "admin@example.test", Username: adminUID, DisplayName: "Admin", IsActive: true, IsAdmin: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	p := &project.Project{ID: uuid.New().String(), Key: "ADF", Name: "ADF", SeqID: 10000}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}

	issueSvc := issue.NewService(db)
	projSvc := project.NewService(db, &user.User{ID: adminUID})
	userSvc := user.NewService(db)
	versionSvc := version.NewService(db)
	chk := authz.New(userSvc, projSvc, issueSvc, nil, nil, nil, nil, versionSvc)
	h := NewIssueHandler(issueSvc, projSvc, workflow.NewService(db), chk, versionSvc, "http://localhost:8080")

	const para = "Created with a rich description."
	body, _ := json.Marshal(map[string]any{
		"fields": map[string]any{
			"summary": "adf create",
			"project": map[string]string{"key": p.Key},
			"description": map[string]any{
				"type": "doc", "version": 1,
				"content": []map[string]any{{
					"type":    "paragraph",
					"content": []map[string]any{{"type": "text", "text": para}},
				}},
			},
		},
	})
	creq := httptest.NewRequest("POST", "/rest/api/3/issue", bytes.NewReader(body))
	creq = creq.WithContext(context.WithValue(creq.Context(), middleware.UserIDKey, adminUID))
	cw := httptest.NewRecorder()
	h.Create(cw, creq)
	if cw.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", cw.Code, cw.Body.String())
	}
	var created struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(cw.Body.Bytes(), &created); err != nil || created.Key == "" {
		t.Fatalf("create response: %v (%s)", err, cw.Body.String())
	}

	greq := httptest.NewRequest("GET", "/rest/api/3/issue/"+created.Key, nil)
	greq.SetPathValue("issueKey", created.Key)
	gw := httptest.NewRecorder()
	h.Get(gw, greq)
	if gw.Code != http.StatusOK {
		t.Fatalf("get status = %d: %s", gw.Code, gw.Body.String())
	}
	var got v3.IssueBean
	if err := json.Unmarshal(gw.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal issue: %v", err)
	}
	if got.Fields.Description == nil {
		t.Fatalf("description missing: %s", gw.Body.String())
	}
	text := adfpkg.PlainText(*got.Fields.Description)
	if strings.HasPrefix(text, `{"content":`) {
		t.Fatalf("description rendered as raw JSON text: %q", text)
	}
	if text != para {
		t.Fatalf("description text = %q, want %q", text, para)
	}
	if len(got.Fields.Description.Content) != 1 || got.Fields.Description.Content[0].Type != "paragraph" {
		t.Fatalf("description is not the single-paragraph doc sent: %s", gw.Body.String())
	}
}
