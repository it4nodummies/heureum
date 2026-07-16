package issue

import (
	"testing"
)

func TestExportRowsResolvesNames(t *testing.T) {
	db := newIssueTestDB(t)
	// Joined tables live in other domains; create minimal shapes via raw SQL.
	db.Exec(`CREATE TABLE workflow_statuses (id TEXT PRIMARY KEY, name TEXT)`)
	// issue_types is auto-migrated by newIssueTestDB (it belongs to this package),
	// so its table already exists with a NOT NULL project_id — insert accordingly.
	db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, display_name TEXT, email TEXT)`)
	db.Exec(`INSERT INTO workflow_statuses (id,name) VALUES ('st-1','In Progress')`)
	db.Exec(`INSERT INTO issue_types (id,project_id,name) VALUES ('ty-1','proj-1','Bug')`)
	db.Exec(`INSERT INTO users (id,display_name,email) VALUES ('u-1','Ada Lovelace','ada@example.com')`)

	svc := NewService(db)
	iss, err := svc.Create("DEMO", "proj-1", "Broken login", "", PriorityHigh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	st, ty, as := "st-1", "ty-1", "u-1"
	db.Model(&Issue{}).Where("id = ?", iss.ID).
		Updates(map[string]any{"status_id": &st, "type_id": &ty, "assignee_id": &as, "story_points": 5})

	rows, err := svc.ExportRows("proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.Status != "In Progress" || r.Type != "Bug" || r.Assignee != "Ada Lovelace" {
		t.Errorf("names not resolved: status=%q type=%q assignee=%q", r.Status, r.Type, r.Assignee)
	}
	if r.Priority != "high" || r.StoryPoints != 5 || r.Key == "" {
		t.Errorf("scalar fields wrong: %+v", r)
	}
}

func TestExportRowsUnassignedIsBlank(t *testing.T) {
	db := newIssueTestDB(t)
	db.Exec(`CREATE TABLE workflow_statuses (id TEXT PRIMARY KEY, name TEXT)`)
	// issue_types is auto-migrated by newIssueTestDB; no raw CREATE needed here.
	db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, display_name TEXT, email TEXT)`)
	svc := NewService(db)
	if _, err := svc.Create("DEMO", "proj-1", "No metadata", "", PriorityMedium, nil, nil); err != nil {
		t.Fatal(err)
	}
	rows, err := svc.ExportRows("proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Status != "" || rows[0].Type != "" || rows[0].Assignee != "" {
		t.Errorf("nil FKs should resolve to empty strings, got %+v", rows[0])
	}
}
