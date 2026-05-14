package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-jira/open-jira/internal/api/middleware"
)

func TestGetMe(t *testing.T) {
	h, s := setupAuthHandler(t)
	defer s.Close()
	u, _ := h.svc.Register("me@test.com", "meuser", "Me User", "password123")
	userH := NewUserHandler(s.DB)
	req := httptest.NewRequest("GET", "/users/me", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, u.ID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	userH.GetMe(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
