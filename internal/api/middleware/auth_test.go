package middleware

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/it4nodummies/heureum/internal/domain/auth"
)

func noopVerify(string, string) (string, error) {
	return "", errors.New("no basic verifier configured")
}

func TestAuthMiddlewareValid(t *testing.T) {
	secret := "test-secret-min-32-chars-long-key!!"
	token, _ := auth.GenerateToken(secret, "user-123", time.Hour)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler := Auth(secret, noopVerify)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	handler := Auth("secret", noopVerify)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAuth_BasicToken(t *testing.T) {
	verify := func(email, token string) (string, error) {
		if email == "alice@example.com" && token == "good-token" {
			return "user-1", nil
		}
		return "", errors.New("nope")
	}
	mw := Auth("secret", verify)
	var gotUserID string
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
	}))

	req := httptest.NewRequest("GET", "/rest/api/3/myself", nil)
	req.SetBasicAuth("alice@example.com", "good-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || gotUserID != "user-1" {
		t.Fatalf("code=%d userID=%q", rec.Code, gotUserID)
	}
}

func TestAuth_UnauthorizedJiraFormat(t *testing.T) {
	mw := Auth("secret", func(string, string) (string, error) { return "", errors.New("nope") })
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest("GET", "/rest/api/3/myself", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
	var body struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("401 body is not Jira-shaped JSON: %v — body: %s", err, rec.Body.String())
	}
	if len(body.ErrorMessages) == 0 {
		t.Error("errorMessages must not be empty")
	}
}

func TestAuth_MalformedBasicHeader(t *testing.T) {
	verifyCalled := false
	mw := Auth("secret", func(string, string) (string, error) {
		verifyCalled = true
		return "", errors.New("nope")
	})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler must not be reached")
	}))

	cases := []struct {
		name   string
		header string
	}{
		{"not base64", "Basic !!!notbase64"},
		{"base64 without colon", "Basic " + base64.StdEncoding.EncodeToString([]byte("no-colon-here"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/rest/api/3/myself", nil)
			req.Header.Set("Authorization", tc.header)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("code = %d, want 401", rec.Code)
			}
			var body struct {
				ErrorMessages []string `json:"errorMessages"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("401 body is not Jira-shaped JSON: %v — body: %s", err, rec.Body.String())
			}
			if len(body.ErrorMessages) != 1 ||
				body.ErrorMessages[0] != "Basic authentication with an invalid email or API token." {
				t.Errorf("errorMessages = %v, want the invalid-credentials message, not the Bearer one", body.ErrorMessages)
			}
		})
	}
	if verifyCalled {
		t.Error("verifier must not be called for malformed Basic headers")
	}
}
