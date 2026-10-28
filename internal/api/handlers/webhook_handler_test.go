package handlers

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/webhook"
)

// webhookFixture: one project and the create route wired like router.go (the
// authz middleware is not under test here — only the handler's URL policy).
type webhookFixture struct {
	db  *gorm.DB
	mux *http.ServeMux
}

func setupWebhookFixture(t *testing.T) *webhookFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&user.User{}, &project.Project{}, &project.ProjectMember{}, &webhook.Webhook{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	p := &project.Project{ID: uuid.New().String(), Key: "WHK", Name: "Hooks", SeqID: 10000}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	h := NewWebhookHandler(webhook.NewService(db), project.NewService(db, nil), nil)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /rest/api/3/project/{key}/webhooks", h.Create)
	return &webhookFixture{db: db, mux: mux}
}

// stubResolver replaces the webhook package's DNS seam for the test so no
// real lookup happens; every hostname resolves to the given addresses.
func stubResolver(t *testing.T, ips ...string) {
	t.Helper()
	prev := webhook.LookupIP
	webhook.LookupIP = func(_ context.Context, _ string) ([]net.IPAddr, error) {
		out := make([]net.IPAddr, 0, len(ips))
		for _, s := range ips {
			out = append(out, net.IPAddr{IP: net.ParseIP(s)})
		}
		return out, nil
	}
	t.Cleanup(func() { webhook.LookupIP = prev })
}

func (f *webhookFixture) create(url string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/rest/api/3/project/WHK/webhooks", strings.NewReader(`{"url":"`+url+`","events":["issue_created"]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.mux.ServeHTTP(rr, req)
	return rr
}

// TestCreateWebhook_RejectsPrivateURL: a loopback target is refused with a
// generic 400 (no resolution detail echoed) and nothing is persisted.
func TestCreateWebhook_RejectsPrivateURL(t *testing.T) {
	f := setupWebhookFixture(t)
	rr := f.create("http://127.0.0.1:9/hook")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST private webhook url: status = %d, want 400 (body %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "invalid webhook URL") {
		t.Errorf("body = %s, want the generic 'invalid webhook URL' message", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "127.0.0.1") {
		t.Errorf("body echoes the resolved address: %s", rr.Body.String())
	}
	var n int64
	f.db.Model(&webhook.Webhook{}).Count(&n)
	if n != 0 {
		t.Errorf("webhook rows = %d, want 0 (nothing persisted on reject)", n)
	}
}

// TestCreateWebhook_AcceptsPublicURL: a hostname resolving to a public address
// is accepted (resolver stubbed — no DNS in tests).
func TestCreateWebhook_AcceptsPublicURL(t *testing.T) {
	f := setupWebhookFixture(t)
	stubResolver(t, "93.184.216.34")
	rr := f.create("https://hooks.example.com/heureum")
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST public webhook url: status = %d, want 201 (body %s)", rr.Code, rr.Body.String())
	}
	var n int64
	f.db.Model(&webhook.Webhook{}).Count(&n)
	if n != 1 {
		t.Errorf("webhook rows = %d, want 1", n)
	}
}
