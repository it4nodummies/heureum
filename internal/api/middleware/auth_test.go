package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/open-jira/open-jira/internal/domain/auth"
)

func TestAuthMiddlewareValid(t *testing.T) {
	secret := "test-secret-min-32-chars-long-key!!"
	token, _ := auth.GenerateToken(secret, "user-123", time.Hour)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler := Auth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := r.Context().Value(UserIDKey)
		if uid != "user-123" {
			t.Errorf("UserID = %v, want user-123", uid)
		}
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestAuthMiddlewareNoToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	handler := Auth("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
