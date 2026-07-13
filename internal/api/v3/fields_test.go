package v3

import "testing"

func TestProjectFields_All(t *testing.T) {
	bean := IssueBean{ID: "1", Key: "DEMO-1", Self: "http://x/1", Fields: IssueFields{Summary: "Hello"}}
	f := Fields{} // vuoto => default *all
	m, err := ProjectIssue(bean, f)
	if err != nil {
		t.Fatalf("ProjectIssue: %v", err)
	}
	fields, _ := m["fields"].(map[string]any)
	if fields["summary"] != "Hello" {
		t.Errorf("summary mancante in *all: %v", m)
	}
	if m["key"] != "DEMO-1" {
		t.Errorf("key mancante")
	}
}

func TestProjectFields_Subset(t *testing.T) {
	bean := IssueBean{ID: "1", Key: "DEMO-1", Self: "http://x/1", Fields: IssueFields{Summary: "Hello"}}
	f := ParseFieldsFromList([]string{"summary"})
	m, err := ProjectIssue(bean, f)
	if err != nil {
		t.Fatalf("ProjectIssue: %v", err)
	}
	fields := m["fields"].(map[string]any)
	if _, ok := fields["summary"]; !ok {
		t.Error("summary deve esserci")
	}
	if _, ok := fields["status"]; ok {
		t.Error("status NON deve esserci quando si chiede solo summary")
	}
	// id/key/self restano sempre a top-level
	if m["id"] == nil || m["key"] == nil {
		t.Error("id/key devono restare sempre presenti")
	}
}
