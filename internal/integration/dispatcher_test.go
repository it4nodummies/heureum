package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/webhook"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// sqlite ":memory:" gives each pooled connection its OWN empty database. Pin the
	// pool to a single connection so migrate + enqueue + Count share one schema.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&webhook.Webhook{}, &webhook.Delivery{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestDispatcher_EnqueuesPendingDelivery(t *testing.T) {
	db := newDB(t)
	whSvc := webhook.NewService(db)
	whSvc.Create("proj-1", "https://example.com/hook", "s", []string{"issue_created"})

	d := NewDispatcher(whSvc, nil, &http.Client{Timeout: 5 * time.Second})
	d.IssueEvent("issue_created", &issue.Issue{ID: "i1", Key: "P-1", Title: "Hello", ProjectID: "proj-1"})

	// La dispatcher NON esegue HTTP: accoda una riga pending nel DB.
	var deliveries []webhook.Delivery
	if err := db.Find(&deliveries).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("attesa 1 delivery accodata, %d", len(deliveries))
	}
	del := deliveries[0]
	if del.Status != "pending" {
		t.Errorf("status = %q, atteso pending", del.Status)
	}
	if del.Payload == "" {
		t.Errorf("payload deve essere valorizzato")
	}
	if del.EventType != "issue_created" || del.URL != "https://example.com/hook" {
		t.Errorf("delivery errata: %+v", del)
	}
	if del.NextAttemptAt == nil {
		t.Errorf("next_attempt_at deve essere valorizzato (dovuto subito)")
	}
}

func TestDispatcher_SkipsNonMatchingEvent(t *testing.T) {
	db := newDB(t)
	whSvc := webhook.NewService(db)
	whSvc.Create("proj-1", "https://b", "", []string{"issue_updated"})

	d := NewDispatcher(whSvc, nil, &http.Client{Timeout: 2 * time.Second})
	d.IssueEvent("issue_created", &issue.Issue{ID: "i1", Key: "P-1", ProjectID: "proj-1"})

	var cnt int64
	db.Model(&webhook.Delivery{}).Count(&cnt)
	if cnt != 0 {
		t.Errorf("un evento non sottoscritto non deve accodare consegne, %d", cnt)
	}
}
