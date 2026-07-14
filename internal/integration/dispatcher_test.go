package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
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
	if err := db.AutoMigrate(&webhook.Webhook{}, &webhook.Delivery{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestDispatcher_DeliversToMatchingWebhook(t *testing.T) {
	db := newDB(t)
	whSvc := webhook.NewService(db)
	received := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		received <- string(b)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	whSvc.Create("proj-1", srv.URL, "s", []string{"issue_created"})

	d := NewDispatcher(whSvc, nil, &http.Client{Timeout: 5 * time.Second})
	d.IssueEvent("issue_created", &issue.Issue{ID: "i1", Key: "P-1", Title: "Hello", ProjectID: "proj-1"})

	select {
	case body := <-received:
		if body == "" {
			t.Error("payload vuoto")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("webhook non consegnato entro il timeout")
	}
	// la delivery è registrata
	var cnt int64
	db.Model(&webhook.Delivery{}).Count(&cnt)
	// attende breve per la goroutine di record (se async): riprova
	for i := 0; i < 20 && cnt == 0; i++ {
		time.Sleep(50 * time.Millisecond)
		db.Model(&webhook.Delivery{}).Count(&cnt)
	}
	if cnt == 0 {
		t.Error("delivery non registrata")
	}
}

func TestDispatcher_SkipsNonMatchingEvent(t *testing.T) {
	db := newDB(t)
	whSvc := webhook.NewService(db)
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true; w.WriteHeader(200) }))
	defer srv.Close()
	whSvc.Create("proj-1", srv.URL, "", []string{"issue_updated"})

	d := NewDispatcher(whSvc, nil, &http.Client{Timeout: 2 * time.Second})
	d.IssueEvent("issue_created", &issue.Issue{ID: "i1", Key: "P-1", ProjectID: "proj-1"})
	time.Sleep(300 * time.Millisecond)
	if hit {
		t.Error("un evento non sottoscritto non deve consegnare")
	}
}
