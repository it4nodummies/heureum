package timeline

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTimelineTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// Minimal tables the timeline service queries.
	db.Exec(`CREATE TABLE sprints (id TEXT PRIMARY KEY, project_id TEXT, name TEXT, start_date TIMESTAMP, end_date TIMESTAMP)`)
	db.Exec(`CREATE TABLE issues (id TEXT PRIMARY KEY, project_id TEXT, title TEXT, type_id TEXT, status_id TEXT, sprint_id TEXT, start_date TIMESTAMP, due_date TIMESTAMP, is_archived BOOLEAN DEFAULT 0)`)
	db.Exec(`CREATE TABLE issue_types (id TEXT PRIMARY KEY, name TEXT)`)
	db.Exec(`CREATE TABLE workflow_statuses (id TEXT PRIMARY KEY, name TEXT, category TEXT)`)
	db.Exec(`CREATE TABLE versions (id TEXT PRIMARY KEY, project_id TEXT, name TEXT, description TEXT, start_date TIMESTAMP, release_date TIMESTAMP, released BOOLEAN DEFAULT 0, archived BOOLEAN DEFAULT 0, created_at TIMESTAMP)`)
	db.Exec(`CREATE TABLE issue_versions (issue_id TEXT, version_id TEXT, PRIMARY KEY(issue_id, version_id))`)
	return db
}

func TestGetTimelineDataIncludesVersionBars(t *testing.T) {
	db := newTimelineTestDB(t)
	svc := NewService(db)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	release := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)

	db.Exec(`INSERT INTO versions (id, project_id, name, description, start_date, release_date, released, archived) VALUES (?,?,?,?,?,?,?,?)`,
		"ver-1", "proj-1", "v1.0", "first release", start, release, false, false)

	// One done issue + one not-done issue, both linked to the version.
	db.Exec(`INSERT INTO workflow_statuses (id, name, category) VALUES ('st-done','Done','done'), ('st-todo','To Do','new')`)
	db.Exec(`INSERT INTO issues (id, project_id, title, status_id) VALUES ('iss-1','proj-1','done issue','st-done'), ('iss-2','proj-1','todo issue','st-todo')`)
	db.Exec(`INSERT INTO issue_versions (issue_id, version_id) VALUES ('iss-1','ver-1'), ('iss-2','ver-1')`)

	data, err := svc.GetTimelineData("proj-1", "weeks")
	if err != nil {
		t.Fatal(err)
	}

	var found *TimelineBar
	for i := range data.Bars {
		if data.Bars[i].Type == "version" {
			found = &data.Bars[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected a version bar, got bars: %+v", data.Bars)
	}
	if found.Name != "v1.0" {
		t.Errorf("version bar name = %q, want %q", found.Name, "v1.0")
	}
	if found.ID != "ver-1" {
		t.Errorf("version bar id = %q, want %q", found.ID, "ver-1")
	}
	if found.Progress <= 0 {
		t.Errorf("version bar progress = %v, want > 0", found.Progress)
	}
	if found.StartDate == nil || !found.StartDate.Equal(start) {
		t.Errorf("version bar start = %v, want %v", found.StartDate, start)
	}
	if found.EndDate == nil || !found.EndDate.Equal(release) {
		t.Errorf("version bar end = %v, want %v", found.EndDate, release)
	}
	// 1 of 2 done → 50%.
	if found.Progress != 50 {
		t.Errorf("version bar progress = %v, want 50", found.Progress)
	}
}
