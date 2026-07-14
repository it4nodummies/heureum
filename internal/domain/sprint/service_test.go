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
	done, err := svc.Complete(sp.ID, false)
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
