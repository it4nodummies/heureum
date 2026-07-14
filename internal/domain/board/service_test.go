package board

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&Board{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCreate_AssignsSeqID(t *testing.T) {
	svc := NewService(newDB(t))
	b1, err := svc.Create("proj-1", "Board A", "scrum", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if b1.SeqID != 1 {
		t.Errorf("primo seq_id atteso 1, got %d", b1.SeqID)
	}
	b2, _ := svc.Create("proj-1", "Board B", "kanban", nil)
	if b2.SeqID != 2 {
		t.Errorf("secondo seq_id atteso 2, got %d", b2.SeqID)
	}
}

func TestGetBySeqID(t *testing.T) {
	svc := NewService(newDB(t))
	b, _ := svc.Create("proj-1", "Board A", "scrum", nil)
	got, err := svc.GetBySeqID(b.SeqID)
	if err != nil {
		t.Fatalf("GetBySeqID: %v", err)
	}
	if got.ID != b.ID {
		t.Errorf("id mismatch")
	}
	if _, err := svc.GetBySeqID(999); err == nil {
		t.Error("atteso errore per seq_id inesistente")
	}
}

func TestListByProject(t *testing.T) {
	svc := NewService(newDB(t))
	svc.Create("proj-1", "A", "scrum", nil)
	svc.Create("proj-2", "B", "scrum", nil)
	svc.Create("proj-1", "C", "kanban", nil)
	list, err := svc.ListByProject("proj-1")
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("attese 2 board per proj-1, got %d", len(list))
	}
}

func TestDelete(t *testing.T) {
	svc := NewService(newDB(t))
	b, _ := svc.Create("proj-1", "A", "scrum", nil)
	if err := svc.Delete(b.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.GetBySeqID(b.SeqID); err == nil {
		t.Error("board dovrebbe essere eliminata")
	}
}
