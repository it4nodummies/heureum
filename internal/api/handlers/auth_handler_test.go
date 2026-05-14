package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

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
	s.DB.AutoMigrate(&user.User{})
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
