package webhook

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

func TestEnqueueAndListRetryable(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)
	h, _ := svc.Create("proj-1", "https://a", "s", []string{"issue_created"})

	if err := svc.EnqueueDelivery(h.ID, "issue_created", h.URL, `{"x":1}`); err != nil {
		t.Fatalf("EnqueueDelivery: %v", err)
	}

	now := time.Now()
	got, err := svc.ListRetryable(now, 50)
	if err != nil {
		t.Fatalf("ListRetryable: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("attesa 1 delivery retryable, %d", len(got))
	}
	if got[0].Status != "pending" || got[0].Payload != `{"x":1}` || got[0].EventType != "issue_created" {
		t.Errorf("delivery accodata errata: %+v", got[0])
	}
}

func TestMarkDeliveryResultBackoffWindow(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)
	h, _ := svc.Create("proj-1", "https://a", "s", []string{"issue_created"})
	svc.EnqueueDelivery(h.ID, "issue_created", h.URL, "{}")

	now := time.Now()
	del, _ := svc.ListRetryable(now, 50)
	id := del[0].ID

	// Simula un fallimento: status='failed', attempts=1, prossimo tentativo tra 30s.
	next := now.Add(30 * time.Second)
	if err := svc.MarkDeliveryResult(id, 500, false, "boom", "failed", 1, &next); err != nil {
		t.Fatalf("MarkDeliveryResult: %v", err)
	}

	// Non ancora dovuto a "now".
	if got, _ := svc.ListRetryable(now, 50); len(got) != 0 {
		t.Errorf("delivery in backoff non deve essere retryable a now, %d", len(got))
	}
	// Dovuto a now+31s.
	if got, _ := svc.ListRetryable(now.Add(31*time.Second), 50); len(got) != 1 {
		t.Errorf("delivery deve essere retryable dopo la finestra, %d", len(got))
	}
}

func TestMarkDeliveryDeadNeverReturned(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)
	h, _ := svc.Create("proj-1", "https://a", "s", []string{"issue_created"})
	svc.EnqueueDelivery(h.ID, "issue_created", h.URL, "{}")

	now := time.Now()
	del, _ := svc.ListRetryable(now, 50)
	id := del[0].ID

	// Raggiunto maxAttempts → 'dead' (nessun prossimo tentativo).
	if err := svc.MarkDeliveryResult(id, 500, false, "boom", "dead", 5, nil); err != nil {
		t.Fatalf("MarkDeliveryResult: %v", err)
	}
	if got, _ := svc.ListRetryable(now.Add(24*time.Hour), 50); len(got) != 0 {
		t.Errorf("una delivery 'dead' non deve mai essere retryable, %d", len(got))
	}
}

func TestGetWebhook(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)
	h, _ := svc.Create("proj-1", "https://a", "sekret", []string{"issue_created"})
	got, err := svc.GetWebhook(h.ID)
	if err != nil {
		t.Fatalf("GetWebhook: %v", err)
	}
	if got.URL != "https://a" || got.Secret != "sekret" {
		t.Errorf("webhook errato: %+v", got)
	}
}
