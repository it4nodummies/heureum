package issue

import (
	"testing"
)

func TestExportRowsResolvesNames(t *testing.T) {
	db := newIssueTestDB(t)
	// issues.project_id and issue_types.project_id are real FKs to projects(id).
	createTestProject(t, db, "proj-1")
	// ExportRows joins workflow_statuses (status_id) and users (assignee_id).
	// workflow_statuses.workflow_id is a real, NOT NULL FK to workflows(id), and
	// workflows.project_id is itself a real FK to projects(id) — neither table
	// is the stub CREATE TABLE this test used to fabricate, which had no such
	// columns at all.
	if err := db.Exec(`INSERT INTO workflows (id, project_id, name) VALUES ('wf-1', 'proj-1', 'Default')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO workflow_statuses (id, workflow_id, name) VALUES ('st-1', 'wf-1', 'In Progress')`).Error; err != nil {
		t.Fatal(err)
	}
	// issue_types is created by the real migrations (it belongs to this
	// package), so its table already exists with a NOT NULL project_id.
	if err := db.Exec(`INSERT INTO issue_types (id,project_id,name) VALUES ('ty-1','proj-1','Bug')`).Error; err != nil {
		t.Fatal(err)
	}
	// users.username is real, NOT NULL and UNIQUE: the old stub table only had
	// id/display_name/email, so this insert would violate that constraint
	// unchanged.
	if err := db.Exec(`INSERT INTO users (id,display_name,email,username) VALUES ('u-1','Ada Lovelace','ada@example.com','ada')`).Error; err != nil {
		t.Fatal(err)
	}

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
	// issues.project_id is a real FK to projects(id).
	createTestProject(t, db, "proj-1")
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
