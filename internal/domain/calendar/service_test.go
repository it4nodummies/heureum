package calendar

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newCalTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Exec(`CREATE TABLE issues (
		id TEXT PRIMARY KEY, key TEXT, title TEXT, priority TEXT,
		status_id TEXT, project_id TEXT, is_archived BOOLEAN DEFAULT 0,
		due_date DATETIME, start_date DATETIME)`)
	db.Exec(`CREATE TABLE workflow_statuses (id TEXT PRIMARY KEY, name TEXT)`)
	return db
}

func findDay(data *CalendarData, day int) *CalendarDay {
	for i := range data.Days {
		if data.Days[i].Day == day {
			return &data.Days[i]
		}
	}
	return nil
}

func TestCalendarBucketsStartDateOnlyIssues(t *testing.T) {
	db := newCalTestDB(t)
	// Due-date issue on the 10th.
	db.Exec(`INSERT INTO issues (id,key,title,priority,project_id,is_archived,due_date)
		VALUES ('i1','DEMO-1','Due one','high','proj-1',0,'2026-03-10 00:00:00')`)
	// Start-date-only issue on the 5th (no due_date).
	db.Exec(`INSERT INTO issues (id,key,title,priority,project_id,is_archived,start_date)
		VALUES ('i2','DEMO-2','Start one','low','proj-1',0,'2026-03-05 00:00:00')`)

	svc := NewService(db)
	data, err := svc.GetCalendarData("proj-1", 2026, 3)
	if err != nil {
		t.Fatal(err)
	}
	if d := findDay(data, 10); d == nil || len(d.Issues) != 1 || d.Issues[0].Key != "DEMO-1" {
		t.Errorf("day 10 = %+v, want DEMO-1", d)
	}
	if d := findDay(data, 5); d == nil || len(d.Issues) != 1 || d.Issues[0].Key != "DEMO-2" {
		t.Errorf("day 5 = %+v, want DEMO-2 (start-date bucketing broken)", d)
	}
}

// Crux of the month/year guard: an issue whose due_date is in a DIFFERENT month
// than the one queried but whose start_date falls in the queried month must be
// bucketed on its start day — not skipped, and not placed on a bogus day derived
// from the out-of-month due_date. This guards against a future "simplification"
// of the first switch case back to a plain `iss.DueDate != nil` check.
func TestCalendarFallsBackToStartWhenDueDateOutOfMonth(t *testing.T) {
	db := newCalTestDB(t)
	// due_date in April (out of the queried month), start_date on March 20th.
	db.Exec(`INSERT INTO issues (id,key,title,priority,project_id,is_archived,due_date,start_date)
		VALUES ('i3','DEMO-3','Spans months','medium','proj-1',0,'2026-04-15 00:00:00','2026-03-20 00:00:00')`)

	svc := NewService(db)
	data, err := svc.GetCalendarData("proj-1", 2026, 3)
	if err != nil {
		t.Fatal(err)
	}
	if d := findDay(data, 20); d == nil || len(d.Issues) != 1 || d.Issues[0].Key != "DEMO-3" {
		t.Errorf("day 20 = %+v, want DEMO-3 (start-date fallback for out-of-month due_date broken)", d)
	}
	// The out-of-month due_date day (15) must NOT receive the issue.
	if d := findDay(data, 15); d != nil && len(d.Issues) != 0 {
		t.Errorf("day 15 = %+v, want no issues (out-of-month due_date must not bucket)", d)
	}
}
