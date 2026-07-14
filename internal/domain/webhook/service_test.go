package webhook

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
	if err := db.AutoMigrate(&Webhook{}, &Delivery{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCreateAndList(t *testing.T) {
	svc := NewService(newDB(t))
	h, err := svc.Create("proj-1", "https://example.com/hook", "s3cr3t", []string{"issue_created", "issue_updated"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if h.ID == "" || h.URL != "https://example.com/hook" {
		t.Errorf("webhook errato: %+v", h)
	}
	list, err := svc.ListByProject("proj-1")
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("attesa 1 webhook, %d", len(list))
	}
	if got := list[0].Events(); len(got) != 2 || got[0] != "issue_created" {
		t.Errorf("events non deserializzati: %v", got)
	}
}

func TestListActiveForEvent(t *testing.T) {
	svc := NewService(newDB(t))
	svc.Create("proj-1", "https://a", "", []string{"issue_created"})
	svc.Create("proj-1", "https://b", "", []string{"issue_updated"})
	svc.Create("proj-2", "https://c", "", []string{"issue_created"})
	hooks, err := svc.ListActiveForEvent("proj-1", "issue_created")
	if err != nil {
		t.Fatalf("ListActiveForEvent: %v", err)
	}
	if len(hooks) != 1 || hooks[0].URL != "https://a" {
		t.Errorf("filtro evento errato: %+v", hooks)
	}
}

func TestDeleteAndRecordDelivery(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)
	h, _ := svc.Create("proj-1", "https://a", "", []string{"issue_created"})
	if err := svc.RecordDelivery(h.ID, "issue_created", h.URL, 200, true, ""); err != nil {
		t.Fatalf("RecordDelivery: %v", err)
	}
	var cnt int64
	db.Model(&Delivery{}).Where("webhook_id = ?", h.ID).Count(&cnt)
	if cnt != 1 {
		t.Errorf("attesa 1 delivery, %d", cnt)
	}
	if err := svc.Delete(h.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if l, _ := svc.ListByProject("proj-1"); len(l) != 0 {
		t.Error("webhook dovrebbe essere eliminato")
	}
}
