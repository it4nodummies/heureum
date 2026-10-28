package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	"github.com/it4nodummies/heureum/internal/domain/git"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

func setupGitTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := db.AutoMigrate(
		&user.User{},
		&project.Project{}, &project.ProjectMember{},
		&workflow.WorkflowStatus{},
		&issue.Issue{},
		&git.GitProviderConfig{}, &git.IssueCommit{}, &git.IssueBranch{}, &git.IssuePullRequest{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newGitTestHandler(t *testing.T, db *gorm.DB) (*GitHandler, *git.ConfigService) {
	t.Helper()
	gitSvc := git.NewConfigService(db, git.KeyFromSecret("handler-test-secret"))
	projSvc := project.NewService(db, nil)
	issueSvc := issue.NewService(db)
	return NewGitHandler(gitSvc, issueSvc, projSvc, nil, "http://localhost:8080/"), gitSvc
}

func seedGitProject(t *testing.T, db *gorm.DB) *project.Project {
	t.Helper()
	p := &project.Project{ID: uuid.New().String(), Key: "DEMO", Name: "Demo", SeqID: 10000}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	return p
}

// TestGetProvider_AdminBodyHasNoSecrets: la risposta della GET non deve mai
// contenere il token del provider né l'HMAC secret, solo has_token e la
// webhook_url da incollare nel provider.
func TestGetProvider_AdminBodyHasNoSecrets(t *testing.T) {
	db := setupGitTestDB(t)
	h, gitSvc := newGitTestHandler(t, db)
	p := seedGitProject(t, db)
	if _, err := gitSvc.CreateProvider(p.ID, "github", "https://github.com/acme/demo", "ghp_secret", "hmac-secret-0123456789"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/rest/api/3/project/DEMO/git/providers", nil)
	req.SetPathValue("key", "DEMO")
	w := httptest.NewRecorder()
	h.GetProvider(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"token_encrypted", "webhook_secret", "webhook_url_token"} {
		if _, ok := body[k]; ok {
			t.Errorf("body exposes %q: %s", k, w.Body.String())
		}
	}
	if strings.Contains(w.Body.String(), "ghp_secret") || strings.Contains(w.Body.String(), "hmac-secret-0123456789") {
		t.Errorf("body contains a secret value: %s", w.Body.String())
	}
	if body["has_token"] != true {
		t.Errorf("has_token = %v, want true", body["has_token"])
	}
	u, _ := body["webhook_url"].(string)
	if !strings.HasPrefix(u, "http://localhost:8080/rest/api/3/webhooks/git/") {
		t.Errorf("webhook_url = %q", u)
	}
	if body["provider_type"] != "github" || body["base_url"] != "https://github.com/acme/demo" {
		t.Errorf("non-secret fields missing: %s", w.Body.String())
	}
}

// TestConfigureProvider_StoresEncryptedTokenAndURLToken: via POST, la riga
// in DB ha il token cifrato (v1:, decifrabile) e un url token distinto
// dall'HMAC secret; la risposta 201 non contiene segreti.
func TestConfigureProvider_StoresEncryptedTokenAndURLToken(t *testing.T) {
	db := setupGitTestDB(t)
	h, _ := newGitTestHandler(t, db)
	seedGitProject(t, db)

	body, _ := json.Marshal(map[string]any{
		"provider_type": "github", "base_url": "https://github.com/acme/demo",
		"token": "ghp_plain", "webhook_secret": "hmac-secret-0123456789",
	})
	req := httptest.NewRequest("POST", "/rest/api/3/project/DEMO/git/providers", bytes.NewReader(body))
	req.SetPathValue("key", "DEMO")
	w := httptest.NewRecorder()
	h.ConfigureProvider(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "ghp_plain") || strings.Contains(w.Body.String(), "hmac-secret-0123456789") || strings.Contains(w.Body.String(), "token_encrypted") {
		t.Errorf("201 body leaks a secret: %s", w.Body.String())
	}

	var row git.GitProviderConfig
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(row.TokenEncrypted, "v1:") {
		t.Fatalf("token_encrypted = %q, want v1: ciphertext", row.TokenEncrypted)
	}
	if pt, err := git.Decrypt(git.KeyFromSecret("handler-test-secret"), row.TokenEncrypted); err != nil || pt != "ghp_plain" {
		t.Fatalf("Decrypt = %q, %v", pt, err)
	}
	if row.WebhookURLToken == nil || len(*row.WebhookURLToken) < 32 || *row.WebhookURLToken == row.WebhookSecret {
		t.Fatalf("webhook_url_token = %v (secret %q)", row.WebhookURLToken, row.WebhookSecret)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if u, _ := resp["webhook_url"].(string); !strings.HasSuffix(u, "/rest/api/3/webhooks/git/"+*row.WebhookURLToken) {
		t.Errorf("webhook_url = %q does not end with the URL token", u)
	}
}

func signGitHub(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// TestWebhook_AcceptsURLToken_RejectsHMACSecret: l'inbound webhook è
// risolto SOLO dal webhook_url_token; l'HMAC secret serve alla firma e non
// deve funzionare come token di URL.
func TestWebhook_AcceptsURLToken_RejectsHMACSecret(t *testing.T) {
	db := setupGitTestDB(t)
	h, gitSvc := newGitTestHandler(t, db)
	p := seedGitProject(t, db)
	cfg, err := gitSvc.CreateProvider(p.ID, "github", "https://github.com/acme/demo", "ghp_secret", "hmac-secret-0123456789")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WebhookURLToken == nil || *cfg.WebhookURLToken == "" {
		t.Fatalf("webhook_url_token not generated")
	}
	payload := []byte(`{"ref":"refs/heads/main","repository":{"html_url":"https://github.com/acme/demo"},"commits":[]}`)

	post := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/rest/api/3/webhooks/git/"+token, bytes.NewReader(payload))
		req.SetPathValue("token", token)
		req.Header.Set("X-Hub-Signature-256", signGitHub("hmac-secret-0123456789", payload))
		w := httptest.NewRecorder()
		h.Webhook(w, req)
		return w
	}

	if w := post(*cfg.WebhookURLToken); w.Code != http.StatusOK {
		t.Errorf("URL token: status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if w := post("hmac-secret-0123456789"); w.Code != http.StatusNotFound {
		t.Errorf("HMAC secret as URL token: status = %d, want 404: %s", w.Code, w.Body.String())
	}
}

// TestGitWebhook_RejectsOversizedBody: the inbound git webhook is
// unauthenticated and reads the whole body (io.ReadAll); behind the global
// body-limit middleware (router.go) a body over 32 MB is refused with 413
// before any parsing, and a normal payload still works through the same
// wrapper.
func TestGitWebhook_RejectsOversizedBody(t *testing.T) {
	db := setupGitTestDB(t)
	h, gitSvc := newGitTestHandler(t, db)
	p := seedGitProject(t, db)
	cfg, err := gitSvc.CreateProvider(p.ID, "github", "https://github.com/acme/demo", "ghp_secret", "hmac-secret-0123456789")
	if err != nil {
		t.Fatal(err)
	}
	// Same wrapping as router.go: the limit sits outside the mux so it covers
	// this unauthenticated route too.
	mux := http.NewServeMux()
	mux.HandleFunc("POST /rest/api/3/webhooks/git/{token}", h.Webhook)
	srv := middleware.MaxBodyBytes(middleware.MaxRequestBodyBytes)(mux)

	post := func(body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/rest/api/3/webhooks/git/"+*cfg.WebhookURLToken, bytes.NewReader(body))
		req.Header.Set("X-Hub-Signature-256", signGitHub("hmac-secret-0123456789", body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		return w
	}

	big := bytes.Repeat([]byte("x"), middleware.MaxRequestBodyBytes+1)
	if w := post(big); w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body: status = %d, want 413: %.200s", w.Code, w.Body.String())
	}
	small := []byte(`{"ref":"refs/heads/main","repository":{"html_url":"https://github.com/acme/demo"},"commits":[]}`)
	if w := post(small); w.Code != http.StatusOK {
		t.Errorf("normal payload through the body limit: status = %d, want 200: %s", w.Code, w.Body.String())
	}
}

// TestConfigureProvider_RejectsEmptyWebhookSecret: POST with an empty (or
// too short) webhook_secret is a 400 with a Jira-style field error on
// webhook_secret, and nothing is written. The secret is the only thing that
// authenticates inbound events; an empty HMAC key is forgeable.
func TestConfigureProvider_RejectsEmptyWebhookSecret(t *testing.T) {
	db := setupGitTestDB(t)
	h, _ := newGitTestHandler(t, db)
	seedGitProject(t, db)

	post := func(secret string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{
			"provider_type": "github", "base_url": "https://github.com/acme/demo",
			"token": "ghp_plain", "webhook_secret": secret,
		})
		req := httptest.NewRequest("POST", "/rest/api/3/project/DEMO/git/providers", bytes.NewReader(body))
		req.SetPathValue("key", "DEMO")
		w := httptest.NewRecorder()
		h.ConfigureProvider(w, req)
		return w
	}

	for _, secret := range []string{"", strings.Repeat("s", git.MinWebhookSecretLen-1)} {
		w := post(secret)
		if w.Code != http.StatusBadRequest {
			t.Errorf("webhook_secret %q: status = %d, want 400: %s", secret, w.Code, w.Body.String())
			continue
		}
		var body struct {
			Errors map[string]string `json:"errors"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Errorf("webhook_secret %q: non-JSON 400 body: %s", secret, w.Body.String())
			continue
		}
		if body.Errors["webhook_secret"] == "" {
			t.Errorf("webhook_secret %q: no field error on webhook_secret: %s", secret, w.Body.String())
		}
	}
	var count int64
	db.Model(&git.GitProviderConfig{}).Count(&count)
	if count != 0 {
		t.Fatalf("rejected POSTs still wrote %d rows", count)
	}
	if w := post(strings.Repeat("s", git.MinWebhookSecretLen)); w.Code != http.StatusCreated {
		t.Errorf("minimum-length webhook_secret: status = %d, want 201: %s", w.Code, w.Body.String())
	}
}

// TestConfigureProvider_RefusesBitbucket: no signature validator exists for
// bitbucket, so the POST is a 400 with a field error on provider_type rather
// than a row whose inbound hook would be unauthenticated.
func TestConfigureProvider_RefusesBitbucket(t *testing.T) {
	db := setupGitTestDB(t)
	h, _ := newGitTestHandler(t, db)
	seedGitProject(t, db)

	body, _ := json.Marshal(map[string]any{
		"provider_type": "bitbucket", "base_url": "https://bitbucket.org/acme/demo",
		"token": "", "webhook_secret": strings.Repeat("s", git.MinWebhookSecretLen),
	})
	req := httptest.NewRequest("POST", "/rest/api/3/project/DEMO/git/providers", bytes.NewReader(body))
	req.SetPathValue("key", "DEMO")
	w := httptest.NewRecorder()
	h.ConfigureProvider(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Errors map[string]string `json:"errors"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Errors["provider_type"] == "" {
		t.Errorf("no field error on provider_type: %s", w.Body.String())
	}
	var count int64
	db.Model(&git.GitProviderConfig{}).Count(&count)
	if count != 0 {
		t.Fatalf("refused POST still wrote %d rows", count)
	}
}

// TestWebhook_LegacyEmptySecretRejected: a pre-R27 row with an empty
// webhook_secret (can't be created any more, but may exist) must have its
// inbound hooks rejected with 401, even when the sender signs with the
// empty key.
func TestWebhook_LegacyEmptySecretRejected(t *testing.T) {
	db := setupGitTestDB(t)
	h, _ := newGitTestHandler(t, db)
	p := seedGitProject(t, db)
	tok := "legacy-url-token-0123456789abcdef"
	legacy := &git.GitProviderConfig{ID: uuid.New().String(), ProjectID: p.ID, ProviderType: git.ProviderGitHub, BaseURL: "https://github.com/acme/demo", WebhookSecret: "", WebhookURLToken: &tok}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"ref":"refs/heads/main","repository":{"html_url":"https://github.com/acme/demo"},"commits":[]}`)
	req := httptest.NewRequest("POST", "/rest/api/3/webhooks/git/"+tok, bytes.NewReader(payload))
	req.SetPathValue("token", tok)
	req.Header.Set("X-Hub-Signature-256", signGitHub("", payload))
	w := httptest.NewRecorder()
	h.Webhook(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("empty-key signature on legacy row: status = %d, want 401: %s", w.Code, w.Body.String())
	}
}
