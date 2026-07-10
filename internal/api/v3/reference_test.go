package v3

import "testing"

func TestStandardPriorities(t *testing.T) {
	ps := StandardPriorities("http://h")
	if len(ps) != 5 {
		t.Fatalf("got %d priorities, want 5", len(ps))
	}
	if ps[0].Name != "Highest" || ps[0].ID != "1" {
		t.Errorf("first priority = %+v", ps[0])
	}
	if ps[0].Self != "http://h/rest/api/3/priority/1" {
		t.Errorf("self = %q", ps[0].Self)
	}
}

func TestPriorityForEnum(t *testing.T) {
	p := PriorityForEnum("high", "http://h")
	if p.ID != "2" || p.Name != "High" {
		t.Errorf("high → %+v", p)
	}
	if PriorityForEnum("weird", "http://h").ID != "3" {
		t.Error("unknown priority must default to Medium (3)")
	}
}

func TestJiraStatus_Category(t *testing.T) {
	s := JiraStatus("s1", "In Progress", "inprogress", "http://h")
	if s.StatusCategory.Key != "indeterminate" || s.StatusCategory.Name != "In Progress" {
		t.Errorf("statusCategory = %+v", s.StatusCategory)
	}
	if JiraStatus("s2", "Done", "done", "http://h").StatusCategory.Key != "done" {
		t.Error("done category key must be 'done'")
	}
	if JiraStatus("s3", "To Do", "todo", "http://h").StatusCategory.Key != "new" {
		t.Error("todo category key must be 'new'")
	}
}
