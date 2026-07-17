package sprint

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&Sprint{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCreateFull_AssignsSeqIDAndFields(t *testing.T) {
	svc := NewService(newDB(t))
	boardID := int64(7)
	start := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 28, 17, 0, 0, 0, time.UTC)
	sp, err := svc.CreateFull("proj-1", "Sprint 1", "ship it", &boardID, &start, &end)
	if err != nil {
		t.Fatalf("CreateFull: %v", err)
	}
	if sp.SeqID != 1 {
		t.Errorf("seq_id atteso 1, got %d", sp.SeqID)
	}
	if sp.OriginBoardID == nil || *sp.OriginBoardID != 7 {
		t.Errorf("originBoardID errato: %v", sp.OriginBoardID)
	}
	if sp.StartDate == nil || !sp.StartDate.Equal(start) {
		t.Errorf("startDate errata")
	}
	if sp.State != StateFuture {
		t.Errorf("stato iniziale atteso future, got %s", sp.State)
	}
}

func TestGetBySeqID(t *testing.T) {
	svc := NewService(newDB(t))
	sp, _ := svc.CreateFull("proj-1", "S1", "", nil, nil, nil)
	got, err := svc.GetBySeqID(sp.SeqID)
	if err != nil {
		t.Fatalf("GetBySeqID: %v", err)
	}
	if got.ID != sp.ID {
		t.Error("id mismatch")
	}
}

func TestComplete_SetsCompleteDate(t *testing.T) {
	svc := NewService(newDB(t))
	sp, _ := svc.CreateFull("proj-1", "S1", "", nil, nil, nil)
	svc.Start(sp.ID)
	done, err := svc.Complete(sp.ID, false, nil)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if done.State != StateClosed {
		t.Errorf("stato atteso closed, got %s", done.State)
	}
	if done.CompleteDate == nil {
		t.Error("completeDate deve essere valorizzata dopo Complete")
	}
}

// seedIssuesTables crea le tabelle minime toccate dalla query di Complete
// (issues + workflow_statuses) senza importare il dominio issue (evita cicli).
func seedIssuesTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec("CREATE TABLE workflow_statuses (id TEXT PRIMARY KEY, category TEXT)").Error; err != nil {
		t.Fatalf("create workflow_statuses: %v", err)
	}
	if err := db.Exec("CREATE TABLE issues (id TEXT PRIMARY KEY, sprint_id TEXT, status_id TEXT)").Error; err != nil {
		t.Fatalf("create issues: %v", err)
	}
	db.Exec("INSERT INTO workflow_statuses (id, category) VALUES ('st-todo','todo'), ('st-done','done')")
}

func TestComplete_MoveIncompleteToAnotherSprint(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)
	seedIssuesTables(t, db)

	s1, _ := svc.CreateFull("proj-1", "S1", "", nil, nil, nil)
	s2, _ := svc.CreateFull("proj-1", "S2", "", nil, nil, nil)
	svc.Start(s1.ID)

	// una issue incompleta (todo) e una completata (done) su s1
	db.Exec("INSERT INTO issues (id, sprint_id, status_id) VALUES ('iss-open', ?, 'st-todo')", s1.ID)
	db.Exec("INSERT INTO issues (id, sprint_id, status_id) VALUES ('iss-done', ?, 'st-done')", s1.ID)

	done, err := svc.Complete(s1.ID, false, &s2.ID)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if done.State != StateClosed {
		t.Errorf("s1 atteso closed, got %s", done.State)
	}

	var openSprint, doneSprint string
	db.Raw("SELECT sprint_id FROM issues WHERE id = 'iss-open'").Scan(&openSprint)
	db.Raw("SELECT sprint_id FROM issues WHERE id = 'iss-done'").Scan(&doneSprint)
	if openSprint != s2.ID {
		t.Errorf("issue incompleta attesa su s2 (%s), got %q", s2.ID, openSprint)
	}
	if doneSprint != s1.ID {
		t.Errorf("issue completata deve restare su s1 (%s), got %q", s1.ID, doneSprint)
	}
}

func TestComplete_MoveIncompleteToBacklog(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)
	seedIssuesTables(t, db)

	s3, _ := svc.CreateFull("proj-1", "S3", "", nil, nil, nil)
	svc.Start(s3.ID)
	db.Exec("INSERT INTO issues (id, sprint_id, status_id) VALUES ('iss-open2', ?, 'st-todo')", s3.ID)
	db.Exec("INSERT INTO issues (id, sprint_id, status_id) VALUES ('iss-done2', ?, 'st-done')", s3.ID)

	if _, err := svc.Complete(s3.ID, true, nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var openSprint *string
	var doneSprint string
	db.Raw("SELECT sprint_id FROM issues WHERE id = 'iss-open2'").Scan(&openSprint)
	db.Raw("SELECT sprint_id FROM issues WHERE id = 'iss-done2'").Scan(&doneSprint)
	if openSprint != nil {
		t.Errorf("issue incompleta attesa in backlog (NULL), got %q", *openSprint)
	}
	if doneSprint != s3.ID {
		t.Errorf("issue completata deve restare su s3 (%s), got %q", s3.ID, doneSprint)
	}
}
