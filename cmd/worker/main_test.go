package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/it4nodummies/heureum/internal/domain/webhook"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log/slog"
	"io"
)

func TestNextDelivery(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

	// success → success, no next attempt.
	if status, attempts, next := nextDelivery(0, true, now); status != "success" || attempts != 1 || next != nil {
		t.Errorf("success: got (%q,%d,%v)", status, attempts, next)
	}

	// first failure (<5) → failed + 30s backoff.
	status, attempts, next := nextDelivery(0, false, now)
	if status != "failed" || attempts != 1 || next == nil {
		t.Fatalf("fail<5: got (%q,%d,%v)", status, attempts, next)
	}
	if want := now.Add(30 * time.Second); !next.Equal(want) {
		t.Errorf("fail<5 backoff: got %v want %v", *next, want)
	}

	// second failure → 60s backoff.
	if _, _, next := nextDelivery(1, false, now); next == nil || !next.Equal(now.Add(60*time.Second)) {
		t.Errorf("second fail backoff: got %v", next)
	}

	// reaching maxAttempts (attempts 4 → newAttempts 5) → dead, no next.
	if status, attempts, next := nextDelivery(4, false, now); status != "dead" || attempts != 5 || next != nil {
		t.Errorf("fail@5: got (%q,%d,%v)", status, attempts, next)
	}
}

func TestNextDeliveryBackoffCapped(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	// attempts=3 → newAttempts=4 → 30s<<3 = 240s (still under 1h, not yet dead).
	if _, _, next := nextDelivery(3, false, now); next == nil || !next.Equal(now.Add(240*time.Second)) {
		t.Errorf("attempt4 backoff: got %v", next)
	}
}

func TestProcessWebhookDeliveries(t *testing.T) {
	db := newWorkerDB(t)
	svc := webhook.NewService(db)
	hit := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		hit <- struct{}{}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	h, _ := svc.Create("proj-1", srv.URL, "s", []string{"issue_created"})
	svc.EnqueueDelivery(h.ID, "issue_created", h.URL, `{"event":"issue_created"}`)

	processWebhookDeliveries(slog.Default(), db, srv.Client())

	select {
	case <-hit:
	case <-time.After(2 * time.Second):
		t.Fatal("webhook non consegnato dal worker")
	}

	// La delivery è marcata success e non è più retryable.
	got, _ := svc.ListRetryable(time.Now().Add(time.Hour), 50)
	if len(got) != 0 {
		t.Errorf("una delivery riuscita non deve essere retryable, %d", len(got))
	}
	var d webhook.Delivery
	db.First(&d)
	if d.Status != "success" || !d.Success || d.StatusCode != 200 {
		t.Errorf("stato delivery errato: %+v", d)
	}
}

func newWorkerDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&webhook.Webhook{}, &webhook.Delivery{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}
