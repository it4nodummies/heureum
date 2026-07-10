package v3

import (
	"testing"

	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/user"
)

func TestJiraProject_Basic(t *testing.T) {
	lead := &user.User{ID: "u1", DisplayName: "Ada", Email: "ada@x.io", IsActive: true}
	p := project.Project{ID: "p1", Key: "DEMO", Name: "Demo", Description: "d", Type: project.TypeScrum, AssigneeType: "PROJECT_LEAD", Style: "classic"}
	jp := JiraProject(p, lead, nil, "http://h")
	if jp.ID != "p1" || jp.Key != "DEMO" || jp.Name != "Demo" {
		t.Fatalf("basic fields wrong: %+v", jp)
	}
	if jp.Self != "http://h/rest/api/3/project/p1" {
		t.Errorf("self = %q", jp.Self)
	}
	if jp.ProjectTypeKey != "software" {
		t.Errorf("projectTypeKey = %q", jp.ProjectTypeKey)
	}
	if jp.Style != "classic" {
		t.Errorf("style = %q", jp.Style)
	}
	if jp.AssigneeType != "PROJECT_LEAD" {
		t.Errorf("assigneeType = %q", jp.AssigneeType)
	}
	if jp.Lead == nil || jp.Lead.AccountID != "u1" {
		t.Errorf("lead not mapped: %+v", jp.Lead)
	}
	for _, s := range []string{"16x16", "24x24", "32x32", "48x48"} {
		if jp.AvatarUrls[s] == "" {
			t.Errorf("missing avatar size %s", s)
		}
	}
}

func TestJiraProject_WithCategory(t *testing.T) {
	p := project.Project{ID: "p2", Key: "K", Name: "N", Type: project.TypeBusiness}
	cat := &project.ProjectCategory{ID: "c1", Name: "Ops", Description: "operations"}
	jp := JiraProject(p, nil, cat, "http://h")
	if jp.ProjectTypeKey != "business" {
		t.Errorf("projectTypeKey = %q", jp.ProjectTypeKey)
	}
	if jp.ProjectCategory == nil || jp.ProjectCategory.ID != "c1" || jp.ProjectCategory.Name != "Ops" {
		t.Errorf("category not mapped: %+v", jp.ProjectCategory)
	}
	if jp.Lead != nil {
		t.Error("lead should be nil when not provided")
	}
}

func TestJiraProjectType(t *testing.T) {
	jt := JiraProjectType("software", "http://h")
	if jt.Key != "software" || jt.Self == "" {
		t.Errorf("unexpected project type: %+v", jt)
	}
}
