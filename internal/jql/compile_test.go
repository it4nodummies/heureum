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

func TestCompile_ResolutionByName(t *testing.T) {
	c := compile(t, `resolution = Done`)
	if !strings.Contains(c.Where, "resolution_id IN (SELECT id FROM resolutions WHERE name = ?)") {
		t.Errorf("where: %q", c.Where)
	}
	if len(c.Args) != 1 || c.Args[0] != "Done" {
		t.Errorf("args: %v", c.Args)
	}

	c2 := compile(t, `resolution IS EMPTY`)
	if c2.Where != "resolution_id IS NULL" {
		t.Errorf("where: %q", c2.Where)
	}
}

func TestCompile_LabelsNotIn(t *testing.T) {
	c := compile(t, `labels NOT IN (a, b)`)
	if !strings.HasPrefix(c.Where, "NOT (") {
		t.Errorf("where dovrebbe iniziare con NOT (: %q", c.Where)
	}
	if len(c.Args) != 2 {
		t.Errorf("args: %v", c.Args)
	}
}

// Round 28 — `parent` (and the Jira-classic `"Epic Link"` alias) resolve the
// parent issue by key or numeric seq id through a parameterized subquery on
// issues, so user values never touch the SQL text.

func assertArgs(t *testing.T, got []any, want ...any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("args[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestCompile_ParentEqualsKey(t *testing.T) {
	c := compile(t, `parent = EPC-1`)
	want := "issues.parent_id IN (SELECT p.id FROM issues p WHERE p.key = ?)"
	if c.Where != want {
		t.Errorf("where = %q, want %q", c.Where, want)
	}
	assertArgs(t, c.Args, "EPC-1")

	c2 := compile(t, `parent != "EPC-1"`)
	want2 := "issues.parent_id NOT IN (SELECT p.id FROM issues p WHERE p.key = ?)"
	if c2.Where != want2 {
		t.Errorf("where = %q, want %q", c2.Where, want2)
	}
	assertArgs(t, c2.Args, "EPC-1")
}

func TestCompile_ParentEqualsSeqID(t *testing.T) {
	c := compile(t, `parent = 10001`)
	want := "issues.parent_id IN (SELECT p.id FROM issues p WHERE p.seq_id = ?)"
	if c.Where != want {
		t.Errorf("where = %q, want %q", c.Where, want)
	}
	assertArgs(t, c.Args, int64(10001))
}

func TestCompile_ParentIn(t *testing.T) {
	c := compile(t, `parent in (EPC-1, "EPC-2")`)
	want := "issues.parent_id IN (SELECT p.id FROM issues p WHERE p.key IN (?,?))"
	if c.Where != want {
		t.Errorf("where = %q, want %q", c.Where, want)
	}
	assertArgs(t, c.Args, "EPC-1", "EPC-2")

	// Keys and seq ids may be mixed in one list: one subquery, two predicates.
	c2 := compile(t, `parent in (EPC-1, 10002)`)
	want2 := "issues.parent_id IN (SELECT p.id FROM issues p WHERE p.key IN (?) OR p.seq_id IN (?))"
	if c2.Where != want2 {
		t.Errorf("where = %q, want %q", c2.Where, want2)
	}
	assertArgs(t, c2.Args, "EPC-1", int64(10002))
}

func TestCompile_ParentNotIn(t *testing.T) {
	c := compile(t, `parent not in (EPC-1, EPC-2)`)
	want := "issues.parent_id NOT IN (SELECT p.id FROM issues p WHERE p.key IN (?,?))"
	if c.Where != want {
		t.Errorf("where = %q, want %q", c.Where, want)
	}
	assertArgs(t, c.Args, "EPC-1", "EPC-2")
}

func TestCompile_ParentIsEmpty(t *testing.T) {
	c := compile(t, `parent is empty`)
	if c.Where != "issues.parent_id IS NULL" {
		t.Errorf("where = %q", c.Where)
	}
	assertArgs(t, c.Args)
}

func TestCompile_ParentIsNotEmpty(t *testing.T) {
	c := compile(t, `parent is not empty`)
	if c.Where != "issues.parent_id IS NOT NULL" {
		t.Errorf("where = %q", c.Where)
	}
	assertArgs(t, c.Args)
}

func TestCompile_EpicLinkAlias(t *testing.T) {
	// Quoted field name, case-insensitive: identical SQL to `parent`.
	for _, jql := range []string{`"Epic Link" = EPC-1`, `'epic link' = EPC-1`, `"EPIC LINK" = EPC-1`} {
		c := compile(t, jql)
		want := "issues.parent_id IN (SELECT p.id FROM issues p WHERE p.key = ?)"
		if c.Where != want {
			t.Errorf("%s: where = %q, want %q", jql, c.Where, want)
		}
		assertArgs(t, c.Args, "EPC-1")
	}
	c := compile(t, `"Epic Link" is empty`)
	if c.Where != "issues.parent_id IS NULL" {
		t.Errorf("where = %q", c.Where)
	}
	c2 := compile(t, `"Epic Link" in (EPC-1, EPC-2) AND project = DEMO`)
	if !strings.HasPrefix(c2.Where, "(issues.parent_id IN (SELECT p.id FROM issues p WHERE p.key IN (?,?)) AND ") {
		t.Errorf("where = %q", c2.Where)
	}
	assertArgs(t, c2.Args, "EPC-1", "EPC-2", "proj-1")
}

func TestCompile_ParentRejectsOtherOperators(t *testing.T) {
	for _, jql := range []string{`parent ~ EPC`, `parent > 1`} {
		q, err := Parse(jql)
		if err != nil {
			t.Fatalf("Parse(%q): %v", jql, err)
		}
		if _, err := Compile(q, fakeResolver{}); err == nil {
			t.Errorf("%s: atteso errore per operatore non valido", jql)
		}
	}
}

func TestCompile_ParentEpicStillUnsupported(t *testing.T) {
	q, _ := Parse(`parentEpic = EPC-1`)
	if _, err := Compile(q, fakeResolver{}); err == nil || !strings.Contains(err.Error(), "campo non supportato") {
		t.Errorf("parentEpic deve restare non supportato, got %v", err)
	}
}

func TestCompile_OrderByParentRejected(t *testing.T) {
	q, err := Parse(`project = DEMO ORDER BY parent`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := Compile(q, fakeResolver{}); err == nil || !strings.Contains(err.Error(), "campo di ordinamento non supportato") {
		t.Errorf("ORDER BY parent deve essere rifiutato con messaggio chiaro, got %v", err)
	}
}
