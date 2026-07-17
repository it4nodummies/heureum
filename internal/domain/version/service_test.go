package version

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Version{}, &IssueVersion{}); err != nil {
		t.Fatal(err)
	}
	// Minimal issues + workflow_statuses tables for ProgressCounts joins.
	db.Exec(`CREATE TABLE issues (
		id TEXT PRIMARY KEY, project_id TEXT, status_id TEXT, is_archived BOOLEAN DEFAULT 0)`)
	db.Exec(`CREATE TABLE workflow_statuses (id TEXT PRIMARY KEY, name TEXT, category TEXT)`)
	return db
}

func TestCreateGetAndDates(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	release := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	v, err := svc.Create("proj-1", "v1.0", "first release", &start, &release)
	if err != nil {
		t.Fatal(err)
	}
	if v.ID == "" {
		t.Fatal("expected generated id")
	}

	got, err := svc.Get(v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "v1.0" || got.Description != "first release" {
		t.Errorf("got %+v", got)
	}
	if got.StartDate == nil || !got.StartDate.Equal(start) {
		t.Errorf("start date = %v, want %v", got.StartDate, start)
	}
	if got.ReleaseDate == nil || !got.ReleaseDate.Equal(release) {
		t.Errorf("release date = %v, want %v", got.ReleaseDate, release)
	}
	if got.Released || got.Archived {
		t.Errorf("new version should be unreleased/unarchived, got %+v", got)
	}
}

func TestUpdateReleased(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db)

	v, _ := svc.Create("proj-1", "v1.0", "", nil, nil)

	released := true
	updated, err := svc.Update(v.ID, nil, nil, &released, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Released {
		t.Errorf("expected released=true, got %+v", updated)
	}
	// Name must be unchanged (nil = unchanged).
	if updated.Name != "v1.0" {
		t.Errorf("name changed unexpectedly: %q", updated.Name)
	}
}

func TestListByProject(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db)

	svc.Create("proj-1", "v1.0", "", nil, nil)
	svc.Create("proj-1", "v2.0", "", nil, nil)
	svc.Create("proj-2", "other", "", nil, nil)

	list, err := svc.ListByProject("proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 versions for proj-1, got %d", len(list))
	}
}

func TestSetAndGetFixVersions(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db)

	v1, _ := svc.Create("proj-1", "v1.0", "", nil, nil)
	v2, _ := svc.Create("proj-1", "v2.0", "", nil, nil)

	if err := svc.SetFixVersions("iss-1", []string{v1.ID, v2.ID}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetFixVersions("iss-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 fix versions, got %d", len(got))
	}

	// Removal: reconcile down to only v1.
	if err := svc.SetFixVersions("iss-1", []string{v1.ID}); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.GetFixVersions("iss-1")
	if len(got) != 1 || got[0].ID != v1.ID {
		t.Fatalf("expected only v1 after removal, got %+v", got)
	}
}

func TestProgressCounts(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db)

	v1, _ := svc.Create("proj-1", "v1.0", "", nil, nil)

	db.Exec(`INSERT INTO workflow_statuses (id,name,category) VALUES ('s-done','Done','done')`)
	db.Exec(`INSERT INTO workflow_statuses (id,name,category) VALUES ('s-prog','In Progress','inprogress')`)
	db.Exec(`INSERT INTO issues (id,project_id,status_id,is_archived) VALUES ('iss-1','proj-1','s-done',0)`)
	db.Exec(`INSERT INTO issues (id,project_id,status_id,is_archived) VALUES ('iss-2','proj-1','s-prog',0)`)

	if err := svc.SetFixVersions("iss-1", []string{v1.ID}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetFixVersions("iss-2", []string{v1.ID}); err != nil {
		t.Fatal(err)
	}

	done, total, err := svc.ProgressCounts(v1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done != 1 || total != 2 {
		t.Errorf("progress = (%d,%d), want (1,2)", done, total)
	}
}

func TestDeleteRemovesPivot(t *testing.T) {
	db := newTestDB(t)
	svc := NewService(db)

	v1, _ := svc.Create("proj-1", "v1.0", "", nil, nil)
	svc.SetFixVersions("iss-1", []string{v1.ID})

	if err := svc.Delete(v1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(v1.ID); err == nil {
		t.Error("expected version to be gone after delete")
	}
	var count int64
	db.Model(&IssueVersion{}).Where("version_id = ?", v1.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected pivot rows removed, got %d", count)
	}
}
