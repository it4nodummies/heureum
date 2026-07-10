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

	"github.com/open-jira/open-jira/internal/api/middleware"
	"github.com/open-jira/open-jira/internal/config"
	"github.com/open-jira/open-jira/internal/domain/auth"
	"github.com/open-jira/open-jira/internal/domain/user"
	"github.com/open-jira/open-jira/internal/store"
)

func setupAuthHandler(t *testing.T) (*AuthHandler, *store.Store) {
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
	return NewAuthHandler(svc), s
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
