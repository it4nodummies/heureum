package contract

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/it4nodummies/heureum/internal/api"
	"github.com/it4nodummies/heureum/internal/config"
	"github.com/it4nodummies/heureum/internal/domain/auth"
	"github.com/it4nodummies/heureum/internal/store"
)

// newTestServerWithSignup mirrors newTestServer (myself_test.go) but lets the
// caller control cfg.SignupOpen, to exercise the APP_SIGNUP=closed path.
func newTestServerWithSignup(t *testing.T, signupOpen bool) (*httptest.Server, *auth.Service) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db")
	cfg := &config.Config{
		Port: 0, Env: "test", Secret: "contract-test-secret",
		BaseURL:    "http://localhost:8080",
		SignupOpen: signupOpen,
		DB:         config.DBConfig{Driver: "sqlite", DSN: dsn},
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

func TestRegister_SignupClosed_Returns403AndBlocksLogin(t *testing.T) {
	srv, _ := newTestServerWithSignup(t, false)

	body := map[string]string{"email": "closed@example.com", "username": "closeduser", "password": "password123"}
	b, _ := json.Marshal(body)
	res, err := http.Post(srv.URL+"/rest/api/3/auth/register", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("register status = %d, want 403", res.StatusCode)
	}

	var errBody struct {
		ErrorMessages []string `json:"errorMessages"`
	}
	if err := json.NewDecoder(res.Body).Decode(&errBody); err != nil {
		t.Fatal(err)
	}
	if len(errBody.ErrorMessages) == 0 || errBody.ErrorMessages[0] != "signup is disabled on this instance" {
		t.Errorf("errorMessages = %v, want [\"signup is disabled on this instance\"]", errBody.ErrorMessages)
	}

	// The user must not have been created: login for that email should fail.
	loginBody := map[string]string{"email": "closed@example.com", "password": "password123"}
	lb, _ := json.Marshal(loginBody)
	loginRes, err := http.Post(srv.URL+"/rest/api/3/auth/login", "application/json", bytes.NewReader(lb))
	if err != nil {
		t.Fatal(err)
	}
	defer loginRes.Body.Close()
	if loginRes.StatusCode != http.StatusUnauthorized {
		t.Errorf("login status = %d, want 401 (user should not exist)", loginRes.StatusCode)
	}
}

func TestRegister_SignupOpenByDefault_Returns201(t *testing.T) {
	// Default-open behavior is already covered implicitly by every contract
	// test that registers a user via newTestServer (myself_test.go etc.),
	// since those build config.Config without setting SignupOpen=false and
	// hit the register endpoint successfully. This test makes the default
	// explicit and exercises the HTTP register endpoint directly.
	srv, _ := newTestServerWithSignup(t, true)

	body := map[string]string{"email": "open@example.com", "username": "openuser", "password": "password123"}
	b, _ := json.Marshal(body)
	res, err := http.Post(srv.URL+"/rest/api/3/auth/register", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d, want 201", res.StatusCode)
	}
}
