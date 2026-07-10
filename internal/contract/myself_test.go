package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-jira/open-jira/internal/api"
	"github.com/open-jira/open-jira/internal/config"
	"github.com/open-jira/open-jira/internal/domain/auth"
	"github.com/open-jira/open-jira/internal/store"
)

// newTestServer avvia il router reale su SQLite temporaneo con migrazioni.
func newTestServer(t *testing.T) (*httptest.Server, *auth.Service) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db")
	cfg := &config.Config{
		Port: 0, Env: "test", Secret: "contract-test-secret",
		BaseURL: "http://localhost:8080",
		DB:      config.DBConfig{Driver: "sqlite", DSN: dsn},
	}
	s, err := store.New(cfg.DB, cfg.Env)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := store.RunMigrations(cfg.DB); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(api.NewRouter(cfg, s.DB))
	t.Cleanup(srv.Close)
	return srv, auth.NewService(s.DB, cfg.Secret)
}

func TestMyself_ConformsToContract(t *testing.T) {
	if os.Getenv("SKIP_CONTRACT") != "" {
		t.Skip("SKIP_CONTRACT set")
	}
	srv, authSvc := newTestServer(t)

	if _, err := authSvc.Register("alice@example.com", "alice", "Alice", "password-123"); err != nil {
		t.Fatal(err)
	}
	jwt, err := authSvc.Login("alice@example.com", "password-123")
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/myself", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}

	v, err := NewValidator("../../docs/contracts/jira-platform-v3.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := v.ValidateResponse("GET", "/rest/api/3/myself", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /rest/api/3/myself NON conforme al contratto: %v", err)
	}
}

// TestDefaultAvatar_IsServed verifica che il fallback avatar puntato da
// JiraUser (baseURL + v3.DefaultAvatarPath) sia realmente servito con 200 e
// Content-Type SVG, senza autenticazione.
func TestDefaultAvatar_IsServed(t *testing.T) {
	srv, _ := newTestServer(t)

	res, err := http.Get(srv.URL + "/static/default-avatar.svg")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", ct)
	}
}

func TestMyself_UnauthorizedIsJiraShaped(t *testing.T) {
	srv, _ := newTestServer(t)

	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/myself", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.StatusCode)
	}

	var body struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.ErrorMessages) == 0 {
		t.Errorf("errorMessages should be non-empty, got %+v", body)
	}
}
