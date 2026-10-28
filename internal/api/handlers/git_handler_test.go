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
	if _, err := gitSvc.CreateProvider(p.ID, "github", "https://github.com/acme/demo", "ghp_secret", "hmac-secret"); err != nil {
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
	if strings.Contains(w.Body.String(), "ghp_secret") || strings.Contains(w.Body.String(), "hmac-secret") {
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
		"token": "ghp_plain", "webhook_secret": "hmac-secret",
	})
	req := httptest.NewRequest("POST", "/rest/api/3/project/DEMO/git/providers", bytes.NewReader(body))
	req.SetPathValue("key", "DEMO")
	w := httptest.NewRecorder()
	h.ConfigureProvider(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "ghp_plain") || strings.Contains(w.Body.String(), "hmac-secret") || strings.Contains(w.Body.String(), "token_encrypted") {
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
	cfg, err := gitSvc.CreateProvider(p.ID, "github", "https://github.com/acme/demo", "ghp_secret", "hmac-secret")
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
		req.Header.Set("X-Hub-Signature-256", signGitHub("hmac-secret", payload))
		w := httptest.NewRecorder()
		h.Webhook(w, req)
		return w
	}

	if w := post(*cfg.WebhookURLToken); w.Code != http.StatusOK {
		t.Errorf("URL token: status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if w := post("hmac-secret"); w.Code != http.StatusNotFound {
		t.Errorf("HMAC secret as URL token: status = %d, want 404: %s", w.Code, w.Body.String())
	}
}
