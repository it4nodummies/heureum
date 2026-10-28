package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	"github.com/it4nodummies/heureum/internal/config"
	"github.com/it4nodummies/heureum/internal/domain/auth"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/store"
)

func setupAuthHandler(t *testing.T) (*AuthHandler, *store.Store) {
	t.Helper()
	return setupAuthHandlerWithSignup(t, true)
}

func setupAuthHandlerWithSignup(t *testing.T, signupOpen bool) (*AuthHandler, *store.Store) {
	t.Helper()
	os.Setenv("APP_SECRET", "test-secret-min-32-chars-long-key!!")
	os.Setenv("DB_DRIVER", "sqlite")
	os.Setenv("DB_DSN", "file::memory:?cache=shared")
	t.Cleanup(func() {
		os.Clearenv()
	})
	cfg, _ := config.Load()
	s, _ := store.New(cfg.DB, "test")
	s.DB.AutoMigrate(&user.User{}, &auth.APIToken{})
	svc := auth.NewService(s.DB, cfg.Secret)
	return NewAuthHandler(svc, signupOpen), s
}

func TestRegisterUser(t *testing.T) {
	h, s := setupAuthHandler(t)
	defer s.Close()
	body := map[string]string{"email": "new@example.com", "username": "newuser", "password": "password123"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201, body: %s", rec.Code, rec.Body.String())
	}
}

func TestRegisterUser_SignupClosedReturns403AndDoesNotCreateUser(t *testing.T) {
	h, s := setupAuthHandlerWithSignup(t, false)
	defer s.Close()
	// The instance must already have a user: on an empty instance the first
	// registration is always accepted (bootstrap), see TestRegister_FirstUserBecomesAdminEvenWhenClosed.
	if _, err := h.svc.Register("existing@example.com", "existing", "Existing", "password123"); err != nil {
		t.Fatal(err)
	}
	body := map[string]string{"email": "blocked@example.com", "username": "blockeduser", "password": "password123"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ErrorMessages []string `json:"errorMessages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v — body: %s", err, rec.Body.String())
	}
	if len(resp.ErrorMessages) == 0 || resp.ErrorMessages[0] != "signup is disabled on this instance" {
		t.Errorf("errorMessages = %v, want [\"signup is disabled on this instance\"]", resp.ErrorMessages)
	}

	var count int64
	s.DB.Model(&user.User{}).Where("email = ?", "blocked@example.com").Count(&count)
	if count != 0 {
		t.Errorf("user was created despite signup being closed, count = %d", count)
	}
}

func TestLoginUser(t *testing.T) {
	h, s := setupAuthHandler(t)
	defer s.Close()
	h.svc.Register("login@test.com", "loginuser", "Login", "password123")
	body := map[string]string{"email": "login@test.com", "password": "password123"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateAPITokenHandler(t *testing.T) {
	h, s := setupAuthHandler(t)
	defer s.Close()
	u, err := h.svc.Register("token@test.com", "tokenuser", "Token", "password123")
	if err != nil {
		t.Fatal(err)
	}
	authedRequest := func(body []byte) *http.Request {
		req := httptest.NewRequest("POST", "/rest/api/3/auth/api-tokens", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		return req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, u.ID))
	}

	t.Run("authorized request creates token", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.CreateAPIToken(rec, authedRequest([]byte(`{"label":"ci"}`)))
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201, body: %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			ID        string `json:"id"`
			Label     string `json:"label"`
			Token     string `json:"token"`
			CreatedAt string `json:"created_at"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v — body: %s", err, rec.Body.String())
		}
		if !strings.HasPrefix(resp.Token, "ojt_") {
			t.Errorf("token = %q, want prefix ojt_", resp.Token)
		}
		if resp.Label != "ci" {
			t.Errorf("label = %q, want %q", resp.Label, "ci")
		}
		if resp.ID == "" {
			t.Error("id must not be empty")
		}
		if resp.CreatedAt == "" {
			t.Error("created_at must not be empty")
		}
	})

	t.Run("malformed JSON returns 400", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.CreateAPIToken(rec, authedRequest([]byte(`{not json`)))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400, body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("label over 255 chars returns 400 with field error", func(t *testing.T) {
		long := strings.Repeat("x", 256)
		body, _ := json.Marshal(map[string]string{"label": long})
		rec := httptest.NewRecorder()
		h.CreateAPIToken(rec, authedRequest(body))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400, body: %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Errors map[string]string `json:"errors"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v — body: %s", err, rec.Body.String())
		}
		if resp.Errors["label"] != "Label must be at most 255 characters." {
			t.Errorf("errors = %v, want label field error", resp.Errors)
		}
	})
}

// ── Round 27: first-user bootstrap vs. closed signup ──────────────────────────

// newAuthHandlerIsolated builds an AuthHandler over a fresh, private in-memory
// DB (unlike setupAuthHandlerWithSignup, which shares one cache across tests),
// so assertions about "the users table is empty" are reliable.
func newAuthHandlerIsolated(t *testing.T, signupOpen bool) (*AuthHandler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&user.User{}, &auth.APIToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := auth.NewService(db, "test-secret-min-32-chars-long-key!!")
	return NewAuthHandler(svc, signupOpen), db
}

func postRegister(t *testing.T, h *AuthHandler, email, username string) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"email": email, "username": username, "password": "password123"})
	req := httptest.NewRequest("POST", "/rest/api/3/auth/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	return rec
}

func TestRegister_FirstUserBecomesAdminEvenWhenClosed(t *testing.T) {
	h, db := newAuthHandlerIsolated(t, false)

	rec := postRegister(t, h, "first@example.com", "first")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (bootstrap on an empty instance), body: %s", rec.Code, rec.Body.String())
	}

	var u user.User
	if err := db.Where("email = ?", "first@example.com").First(&u).Error; err != nil {
		t.Fatalf("first user not persisted: %v", err)
	}
	if !u.IsAdmin {
		t.Errorf("first user is_admin = false, want true")
	}
	var resp struct {
		IsAdmin bool `json:"is_admin"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — body: %s", err, rec.Body.String())
	}
	if !resp.IsAdmin {
		t.Errorf("response is_admin = false, want true")
	}
}

func TestRegister_SecondUserRefusedWhenClosed(t *testing.T) {
	h, db := newAuthHandlerIsolated(t, false)
	if rec := postRegister(t, h, "first@example.com", "first"); rec.Code != http.StatusCreated {
		t.Fatalf("bootstrap status = %d, want 201, body: %s", rec.Code, rec.Body.String())
	}

	rec := postRegister(t, h, "second@example.com", "second")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("second register status = %d, want 403, body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ErrorMessages []string `json:"errorMessages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — body: %s", err, rec.Body.String())
	}
	if len(resp.ErrorMessages) == 0 || resp.ErrorMessages[0] != "signup is disabled on this instance" {
		t.Errorf("errorMessages = %v, want [\"signup is disabled on this instance\"]", resp.ErrorMessages)
	}

	var count int64
	db.Model(&user.User{}).Count(&count)
	if count != 1 {
		t.Errorf("users count = %d, want 1 (second user must not be created)", count)
	}
}

func TestRegister_SecondUserAllowedWhenOpen(t *testing.T) {
	h, db := newAuthHandlerIsolated(t, true)
	if rec := postRegister(t, h, "first@example.com", "first"); rec.Code != http.StatusCreated {
		t.Fatalf("bootstrap status = %d, want 201, body: %s", rec.Code, rec.Body.String())
	}

	rec := postRegister(t, h, "second@example.com", "second")
	if rec.Code != http.StatusCreated {
		t.Fatalf("second register status = %d, want 201, body: %s", rec.Code, rec.Body.String())
	}

	var second user.User
	if err := db.Where("email = ?", "second@example.com").First(&second).Error; err != nil {
		t.Fatalf("second user not persisted: %v", err)
	}
	if second.IsAdmin {
		t.Errorf("second user is_admin = true, want false (only the bootstrap user is admin)")
	}
	var admins int64
	db.Model(&user.User{}).Where("is_admin = ?", true).Count(&admins)
	if admins != 1 {
		t.Errorf("admin count = %d, want 1", admins)
	}
}

// ── Round 27 / Task 7: API token lifecycle ────────────────────────────────────

func apiTokenRequest(userID, method, target string, body []byte) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	return req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
}

func TestCreateAPIToken_ExpiresInDays(t *testing.T) {
	h, _ := newAuthHandlerIsolated(t, true)
	u, err := h.svc.Register("token@test.com", "tokenuser", "Token", "password123")
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	h.svc.Now = func() time.Time { return fixed }

	t.Run("expiresInDays sets expires_at to now plus N days", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.CreateAPIToken(rec, apiTokenRequest(u.ID, "POST", "/rest/api/3/auth/api-tokens", []byte(`{"label":"ci","expiresInDays":1}`)))
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201, body: %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Token     string     `json:"token"`
			ExpiresAt *time.Time `json:"expires_at"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v — body: %s", err, rec.Body.String())
		}
		if resp.Token == "" {
			t.Error("plaintext token must still be returned once at creation")
		}
		if resp.ExpiresAt == nil || !resp.ExpiresAt.Equal(fixed.Add(24*time.Hour)) {
			t.Errorf("expires_at = %v, want %v", resp.ExpiresAt, fixed.Add(24*time.Hour))
		}
	})

	t.Run("omitted expiresInDays means no expiry", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.CreateAPIToken(rec, apiTokenRequest(u.ID, "POST", "/rest/api/3/auth/api-tokens", []byte(`{"label":"forever"}`)))
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201, body: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if v, ok := resp["expires_at"]; ok && v != nil {
			t.Errorf("expires_at = %v, want absent/null", v)
		}
	})

	for _, bad := range []string{`{"label":"x","expiresInDays":0}`, `{"label":"x","expiresInDays":-3}`} {
		t.Run("non-positive expiresInDays is 400: "+bad, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.CreateAPIToken(rec, apiTokenRequest(u.ID, "POST", "/rest/api/3/auth/api-tokens", []byte(bad)))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body: %s", rec.Code, rec.Body.String())
			}
			var resp struct {
				Errors map[string]string `json:"errors"`
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &resp)
			if resp.Errors["expiresInDays"] == "" {
				t.Errorf("want a field error on expiresInDays, got %s", rec.Body.String())
			}
		})
	}
}

func TestListAndRevokeAPITokenHandlers(t *testing.T) {
	h, db := newAuthHandlerIsolated(t, true)
	alice, err := h.svc.Register("alice@test.com", "alice", "Alice", "password123")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := h.svc.Register("bob@test.com", "bob", "Bob", "password123")
	if err != nil {
		t.Fatal(err)
	}
	exp := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	tokA, plainA, err := h.svc.CreateAPIToken(alice.ID, "alice-ci", &exp)
	if err != nil {
		t.Fatal(err)
	}
	tokB, _, err := h.svc.CreateAPIToken(bob.ID, "bob-ci", nil)
	if err != nil {
		t.Fatal(err)
	}

	// GET: alice sees only her own token, with expires_at and without any secret.
	rec := httptest.NewRecorder()
	h.ListAPITokens(rec, apiTokenRequest(alice.ID, "GET", "/rest/api/3/auth/api-tokens", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v — body: %s", err, rec.Body.String())
	}
	if len(list) != 1 || list[0]["id"] != tokA.ID || list[0]["label"] != "alice-ci" {
		t.Fatalf("list = %v, want exactly alice's token", list)
	}
	if list[0]["expires_at"] != "2030-01-01T00:00:00Z" {
		t.Errorf("expires_at = %v, want 2030-01-01T00:00:00Z", list[0]["expires_at"])
	}
	for _, secret := range []string{"token_hash", tokA.TokenHash, plainA, `"token"`} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("list body leaks %q: %s", secret, rec.Body.String())
		}
	}

	// DELETE bob's id as alice → 404, bob's row untouched.
	rec = httptest.NewRecorder()
	req := apiTokenRequest(alice.ID, "DELETE", "/rest/api/3/auth/api-tokens/"+tokB.ID, nil)
	req.SetPathValue("id", tokB.ID)
	h.RevokeAPIToken(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("revoke other's token status = %d, want 404, body: %s", rec.Code, rec.Body.String())
	}
	var n int64
	db.Model(&auth.APIToken{}).Where("id = ?", tokB.ID).Count(&n)
	if n != 1 {
		t.Errorf("bob's token row count = %d, want 1", n)
	}

	// DELETE own id → 204, then GET is empty and DELETE again is 404.
	rec = httptest.NewRecorder()
	req = apiTokenRequest(alice.ID, "DELETE", "/rest/api/3/auth/api-tokens/"+tokA.ID, nil)
	req.SetPathValue("id", tokA.ID)
	h.RevokeAPIToken(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke own token status = %d, want 204, body: %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ListAPITokens(rec, apiTokenRequest(alice.ID, "GET", "/rest/api/3/auth/api-tokens", nil))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("list after revoke: status=%d body=%q, want 200 []", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = apiTokenRequest(alice.ID, "DELETE", "/rest/api/3/auth/api-tokens/"+tokA.ID, nil)
	req.SetPathValue("id", tokA.ID)
	h.RevokeAPIToken(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("second revoke status = %d, want 404", rec.Code)
	}
}

// TestAuthMiddleware_ExpiredAPITokenIs401 wires the real Auth middleware to the
// real verifier (exactly as router.go does) and proves an expired API token
// is refused with 401 on a protected route, while a live one passes.
func TestAuthMiddleware_ExpiredAPITokenIs401(t *testing.T) {
	h, _ := newAuthHandlerIsolated(t, true)
	u, err := h.svc.Register("mw@test.com", "mwuser", "MW", "password123")
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	h.svc.Now = func() time.Time { return fixed }
	past := fixed.Add(-time.Hour)
	_, expired, err := h.svc.CreateAPIToken(u.ID, "expired", &past)
	if err != nil {
		t.Fatal(err)
	}
	future := fixed.Add(time.Hour)
	_, live, err := h.svc.CreateAPIToken(u.ID, "live", &future)
	if err != nil {
		t.Fatal(err)
	}

	protected := middleware.Auth("test-secret-min-32-chars-long-key!!", h.svc.VerifyAPIToken)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	call := func(plain string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/rest/api/3/myself", nil)
		req.SetBasicAuth("mw@test.com", plain)
		rec := httptest.NewRecorder()
		protected.ServeHTTP(rec, req)
		return rec
	}
	if rec := call(expired); rec.Code != http.StatusUnauthorized {
		t.Errorf("expired token status = %d, want 401, body: %s", rec.Code, rec.Body.String())
	}
	if rec := call(live); rec.Code != http.StatusOK {
		t.Errorf("live token status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
}
