package contract

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestAPITokenRoutes_ListRevokeExpiry esercita le route estensione (non Jira)
// del ciclo di vita dei token API sul router reale con le migrazioni reali
// (quindi anche 000025_api_token_expiry): POST con expiresInDays → expires_at
// nel body; GET lista il token (senza hash/plaintext); DELETE → 204; GET → vuoto;
// DELETE di un id sconosciuto → 404.
func TestAPITokenRoutes_ListRevokeExpiry(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerUserAndLogin(t, authSvc, "tokens@example.com", "tokens")

	do := func(method, path string, body []byte) (*http.Response, string) {
		t.Helper()
		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, _ := http.NewRequest(method, srv.URL+path, rdr)
		req.Header.Set("Authorization", "Bearer "+jwt)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		return res, string(raw)
	}

	// Create with a 1-day expiry.
	res, body := do("POST", "/rest/api/3/auth/api-tokens", []byte(`{"label":"ci","expiresInDays":1}`))
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201, body: %s", res.StatusCode, body)
	}
	var created struct {
		ID        string     `json:"id"`
		Token     string     `json:"token"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("decode POST body: %v — %s", err, body)
	}
	if created.ID == "" || created.Token == "" {
		t.Fatalf("POST must return id and plaintext token once: %s", body)
	}
	if created.ExpiresAt == nil {
		t.Fatalf("POST with expiresInDays must return expires_at: %s", body)
	}
	left := time.Until(*created.ExpiresAt)
	if left < 23*time.Hour || left > 25*time.Hour {
		t.Errorf("expires_at = %v, want ~24h from now (got %v)", created.ExpiresAt, left)
	}

	// List: contains it, with expires_at, without any secret.
	res, body = do("GET", "/rest/api/3/auth/api-tokens", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200, body: %s", res.StatusCode, body)
	}
	var list []map[string]any
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("decode GET body: %v — %s", err, body)
	}
	if len(list) != 1 || list[0]["id"] != created.ID || list[0]["label"] != "ci" {
		t.Fatalf("GET list = %s, want exactly the created token", body)
	}
	if list[0]["expires_at"] == nil {
		t.Errorf("GET list must include expires_at: %s", body)
	}
	if strings.Contains(body, created.Token) || strings.Contains(body, "token_hash") {
		t.Errorf("GET list leaks a secret: %s", body)
	}

	// Revoke → 204; list → []; revoke again / unknown → 404.
	res, body = do("DELETE", "/rest/api/3/auth/api-tokens/"+created.ID, nil)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204, body: %s", res.StatusCode, body)
	}
	res, body = do("GET", "/rest/api/3/auth/api-tokens", nil)
	if res.StatusCode != http.StatusOK || strings.TrimSpace(body) != "[]" {
		t.Errorf("GET after revoke: status=%d body=%q, want 200 []", res.StatusCode, body)
	}
	res, body = do("DELETE", "/rest/api/3/auth/api-tokens/"+created.ID, nil)
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("DELETE revoked id status = %d, want 404, body: %s", res.StatusCode, body)
	}
	res, body = do("DELETE", "/rest/api/3/auth/api-tokens/does-not-exist", nil)
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("DELETE unknown id status = %d, want 404, body: %s", res.StatusCode, body)
	}

	// The routes are authenticated: no credentials → 401.
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/auth/api-tokens", nil)
	anon, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	anon.Body.Close()
	if anon.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous GET status = %d, want 401", anon.StatusCode)
	}
}
