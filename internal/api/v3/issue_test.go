package v3

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

func TestJiraIssue_TopLevel(t *testing.T) {
	iss := issue.Issue{ID: "u-1", SeqID: 10001, Key: "DEMO-1", Title: "Do it",
		Priority: issue.PriorityHigh, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	bean := JiraIssue(IssueInput{Issue: iss, BaseURL: "http://h"})
	if bean.ID != "10001" || bean.Key != "DEMO-1" {
		t.Fatalf("top-level wrong: %+v", bean)
	}
	if bean.Self != "http://h/rest/api/3/issue/10001" {
		t.Errorf("self = %q", bean.Self)
	}
	if bean.Fields.Summary != "Do it" {
		t.Errorf("summary = %q", bean.Fields.Summary)
	}
	if bean.Fields.Priority == nil || bean.Fields.Priority.Name != "High" {
		t.Errorf("priority = %+v", bean.Fields.Priority)
	}
	if bean.Fields.IssueType == nil || bean.Fields.IssueType.Name != "Task" {
		t.Errorf("default issuetype = %+v", bean.Fields.IssueType)
	}
	if bean.Fields.Status == nil || bean.Fields.Status.StatusCategory.Key != "new" {
		t.Errorf("default status = %+v", bean.Fields.Status)
	}
}

func TestJiraIssue_DescriptionADF(t *testing.T) {
	iss := issue.Issue{ID: "u2", SeqID: 10002, Key: "DEMO-2", Title: "x", DescriptionJSON: `{"content":"Hello world"}`}
	bean := JiraIssue(IssueInput{Issue: iss, BaseURL: "http://h"})
	raw, _ := json.Marshal(bean.Fields.Description)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	if doc["type"] != "doc" {
		t.Errorf("description not ADF doc: %s", raw)
	}
}

func TestJiraIssue_AssigneeAndLabels(t *testing.T) {
	iss := issue.Issue{ID: "u3", SeqID: 10003, Key: "DEMO-3", Title: "x"}
	assignee := &user.User{ID: "ua", DisplayName: "Ana", IsActive: true}
	bean := JiraIssue(IssueInput{Issue: iss, BaseURL: "http://h", Assignee: assignee, Labels: []string{"backend", "urgent"}})
	if bean.Fields.Assignee == nil || bean.Fields.Assignee.AccountID != "ua" {
		t.Errorf("assignee = %+v", bean.Fields.Assignee)
	}
	if len(bean.Fields.Labels) != 2 {
		t.Errorf("labels = %v", bean.Fields.Labels)
	}
}
