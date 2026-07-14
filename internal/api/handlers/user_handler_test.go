package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/it4nodummies/heureum/internal/api/middleware"
)

const testBaseURL = "http://localhost:8080"

func TestGetMe(t *testing.T) {
	h, s := setupAuthHandler(t)
	defer s.Close()
	u, _ := h.svc.Register("me@test.com", "meuser", "Me User", "password123")
	userH := NewUserHandler(s.DB, testBaseURL)
	req := httptest.NewRequest("GET", "/users/me", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, u.ID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	userH.GetMe(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// TestGetMyself_UserNotFound copre il ramo handler: un token valido (userID
// presente in context, come lo metterebbe il middleware) il cui utente non
// esiste più nel DB deve dare 401 con corpo Jira-shaped (errorMessages).
func TestGetMyself_UserNotFound(t *testing.T) {
	_, s := setupAuthHandler(t)
	defer s.Close()
	userH := NewUserHandler(s.DB, testBaseURL)
	req := httptest.NewRequest("GET", "/rest/api/3/myself", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "does-not-exist")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	userH.GetMyself(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var body struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	if len(body.ErrorMessages) == 0 {
		t.Errorf("errorMessages should be non-empty, got %+v", body)
	}
}
