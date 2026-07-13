package jql

import (
	"strings"
	"testing"
)

// fakeResolver mappa nomi → id in modo deterministico per i test.
type fakeResolver struct{}

func (fakeResolver) ProjectID(k string) (string, bool) {
	if k == "DEMO" {
		return "proj-1", true
	}
	return "", false
}
func (fakeResolver) UserID(login string) (string, bool) {
	if login == "dev" {
		return "user-dev", true
	}
	return "", false
}
func (fakeResolver) CurrentUserID() string { return "user-me" }

func compile(t *testing.T, jql string) *Compiled {
	t.Helper()
	q, err := Parse(jql)
	if err != nil {
		t.Fatalf("Parse(%q): %v", jql, err)
	}
	c, err := Compile(q, fakeResolver{})
	if err != nil {
		t.Fatalf("Compile(%q): %v", jql, err)
	}
	return c
}

func TestCompile_ProjectEq(t *testing.T) {
	c := compile(t, `project = DEMO`)
	if c.Where != "project_id = ?" {
		t.Errorf("where: %q", c.Where)
	}
	if len(c.Args) != 1 || c.Args[0] != "proj-1" {
		t.Errorf("args: %v", c.Args)
	}
}

func TestCompile_StatusInList(t *testing.T) {
	c := compile(t, `status IN ("To Do", Done)`)
	if !strings.Contains(c.Where, "status_id IN (SELECT id FROM workflow_statuses WHERE name IN (?,?))") {
		t.Errorf("where: %q", c.Where)
	}
	if len(c.Args) != 2 {
		t.Errorf("args: %v", c.Args)
	}
}

func TestCompile_CurrentUser(t *testing.T) {
	c := compile(t, `assignee = currentUser()`)
	if c.Where != "assignee_id = ?" || c.Args[0] != "user-me" {
		t.Errorf("currentUser errato: %q %v", c.Where, c.Args)
	}
}

func TestCompile_AssigneeEmpty(t *testing.T) {
	c := compile(t, `assignee IS EMPTY`)
	if c.Where != "assignee_id IS NULL" {
		t.Errorf("where: %q", c.Where)
	}
}

func TestCompile_SummaryContains(t *testing.T) {
	c := compile(t, `summary ~ login`)
	if !strings.Contains(c.Where, "title LIKE ?") {
		t.Errorf("where: %q", c.Where)
	}
	if c.Args[0] != "%login%" {
		t.Errorf("arg like: %v", c.Args)
	}
}

func TestCompile_PriorityName(t *testing.T) {
	// priority in JQL usa i nomi capitalizzati; li normalizziamo all'enum interno.
	c := compile(t, `priority = High`)
	if c.Where != "priority = ?" || c.Args[0] != "high" {
		t.Errorf("priority errata: %q %v", c.Where, c.Args)
	}
}

func TestCompile_Labels(t *testing.T) {
	c := compile(t, `labels = backend`)
	if !strings.Contains(c.Where, "EXISTS") || !strings.Contains(c.Where, "issue_labels") {
		t.Errorf("labels deve usare EXISTS: %q", c.Where)
	}
	if c.Args[0] != "backend" {
		t.Errorf("arg label: %v", c.Args)
	}
}

func TestCompile_AndOr(t *testing.T) {
	c := compile(t, `project = DEMO AND (status = Done OR assignee = currentUser())`)
	if !strings.Contains(c.Where, " AND ") || !strings.Contains(c.Where, " OR ") {
		t.Errorf("where composta errata: %q", c.Where)
	}
	if len(c.Args) != 3 {
		t.Errorf("args: %v", c.Args)
	}
}

func TestCompile_Not(t *testing.T) {
	c := compile(t, `NOT status = Done`)
	if !strings.HasPrefix(c.Where, "NOT (") {
		t.Errorf("NOT errato: %q", c.Where)
	}
}

func TestCompile_Order(t *testing.T) {
	c := compile(t, `project = DEMO ORDER BY priority DESC, created ASC`)
	if c.Order != "priority DESC, created_at ASC" {
		t.Errorf("order: %q", c.Order)
	}
}

func TestCompile_OrderOnly(t *testing.T) {
	c := compile(t, `ORDER BY created DESC`)
	if c.Where != "" || c.Order != "created_at DESC" {
		t.Errorf("order-only errato: where=%q order=%q", c.Where, c.Order)
	}
}

func TestCompile_UnknownField(t *testing.T) {
	q, _ := Parse(`bogus = 1`)
	if _, err := Compile(q, fakeResolver{}); err == nil {
		t.Error("atteso errore per campo sconosciuto")
	}
}

func TestCompile_UnknownProject(t *testing.T) {
	q, _ := Parse(`project = NOPE`)
	if _, err := Compile(q, fakeResolver{}); err == nil {
		t.Error("atteso errore per progetto inesistente")
	}
}
