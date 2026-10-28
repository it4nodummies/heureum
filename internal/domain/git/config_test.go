package git

import (
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newConfigDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&IssueCommit{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestLinkCommitDedupesRepeatedSHA(t *testing.T) {
	svc := NewConfigService(newConfigDB(t), KeyFromSecret("t"))

	created, err := svc.LinkCommit("issue-1", "prov-1", "sha1", "PROJ-1: first", "dev")
	if err != nil {
		t.Fatalf("first LinkCommit: %v", err)
	}
	if !created {
		t.Fatalf("first LinkCommit: expected created=true, got false")
	}

	created, err = svc.LinkCommit("issue-1", "prov-1", "sha1", "PROJ-1: first", "dev")
	if err != nil {
		t.Fatalf("second LinkCommit: %v", err)
	}
	if created {
		t.Fatalf("second LinkCommit: expected created=false, got true")
	}

	var count int64
	svc.DB().Model(&IssueCommit{}).Where("issue_id = ? AND commit_sha = ?", "issue-1", "sha1").Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 issue_commits row, got %d", count)
	}
}

func newProviderDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&GitProviderConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestCreateProvider_StoresEncryptedTokenAndURLToken: il token del provider
// è cifrato a riposo (prefisso v1:) e il token di URL è separato dall'HMAC
// secret.
func TestCreateProvider_StoresEncryptedTokenAndURLToken(t *testing.T) {
	db := newProviderDB(t)
	key := KeyFromSecret("test-secret")
	svc := NewConfigService(db, key)

	cfg, err := svc.CreateProvider("proj-1", "github", "https://github.com/acme/x", "ghp_plain", "hmac-secret")
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	var row GitProviderConfig
	if err := db.First(&row, "id = ?", cfg.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(row.TokenEncrypted, "v1:") {
		t.Fatalf("token_encrypted = %q, want v1: ciphertext", row.TokenEncrypted)
	}
	pt, err := Decrypt(key, row.TokenEncrypted)
	if err != nil || pt != "ghp_plain" {
		t.Fatalf("Decrypt = %q, %v", pt, err)
	}
	if got, err := svc.Token(&row); err != nil || got != "ghp_plain" {
		t.Fatalf("Token() = %q, %v", got, err)
	}
	if row.WebhookURLToken == nil || len(*row.WebhookURLToken) < 32 {
		t.Fatalf("webhook_url_token = %v, want >= 32 chars", row.WebhookURLToken)
	}
	if *row.WebhookURLToken == row.WebhookSecret {
		t.Fatalf("webhook_url_token equals the HMAC secret")
	}
	if row.WebhookSecret != "hmac-secret" {
		t.Fatalf("webhook_secret = %q (must stay usable for HMAC verification)", row.WebhookSecret)
	}

	found, err := svc.FindByWebhookURLToken(*row.WebhookURLToken)
	if err != nil || found.ID != cfg.ID {
		t.Fatalf("FindByWebhookURLToken: %v", err)
	}
	if _, err := svc.FindByWebhookURLToken("hmac-secret"); err == nil {
		t.Fatalf("HMAC secret resolved as URL token")
	}

	// Empty token: nothing to encrypt, has no token.
	cfg2, err := svc.CreateProvider("proj-2", "github", "https://github.com/acme/y", "", "s")
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.TokenEncrypted != "" {
		t.Fatalf("empty token stored as %q", cfg2.TokenEncrypted)
	}
}

// TestEnsureBackfill_EncryptsLegacyAndAddsURLToken: righe pre-R27 (token in
// chiaro, nessun url token) vengono sistemate all'avvio; la seconda
// esecuzione non cambia nulla.
func TestEnsureBackfill_EncryptsLegacyAndAddsURLToken(t *testing.T) {
	db := newProviderDB(t)
	key := KeyFromSecret("test-secret")
	svc := NewConfigService(db, key)

	legacy := &GitProviderConfig{ID: "legacy-1", ProjectID: "p", ProviderType: ProviderGitHub, BaseURL: "https://github.com/acme/x", TokenEncrypted: "plain-legacy-token", WebhookSecret: "s"}
	empty := &GitProviderConfig{ID: "legacy-2", ProjectID: "p2", ProviderType: ProviderGitHub, BaseURL: "https://github.com/acme/y", TokenEncrypted: "", WebhookSecret: "s2"}
	for _, r := range []*GitProviderConfig{legacy, empty} {
		if err := db.Create(r).Error; err != nil {
			t.Fatal(err)
		}
	}

	stats, err := svc.EnsureBackfill()
	if err != nil {
		t.Fatalf("EnsureBackfill: %v", err)
	}
	if stats.Encrypted != 1 || stats.URLTokens != 2 {
		t.Fatalf("stats = %+v, want Encrypted=1 URLTokens=2", stats)
	}

	var rows []GitProviderConfig
	if err := db.Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rows[0].TokenEncrypted, "v1:") {
		t.Fatalf("legacy token not encrypted: %q", rows[0].TokenEncrypted)
	}
	if pt, err := Decrypt(key, rows[0].TokenEncrypted); err != nil || pt != "plain-legacy-token" {
		t.Fatalf("decrypt legacy = %q, %v", pt, err)
	}
	if rows[1].TokenEncrypted != "" {
		t.Fatalf("empty token was touched: %q", rows[1].TokenEncrypted)
	}
	for _, r := range rows {
		if r.WebhookURLToken == nil || len(*r.WebhookURLToken) < 32 {
			t.Fatalf("row %s: webhook_url_token = %v", r.ID, r.WebhookURLToken)
		}
	}
	if *rows[0].WebhookURLToken == *rows[1].WebhookURLToken {
		t.Fatalf("url tokens not unique")
	}

	// Idempotent.
	stats2, err := svc.EnsureBackfill()
	if err != nil {
		t.Fatalf("second EnsureBackfill: %v", err)
	}
	if stats2.Encrypted != 0 || stats2.URLTokens != 0 {
		t.Fatalf("second run stats = %+v, want zeros", stats2)
	}
	var again []GitProviderConfig
	db.Order("id").Find(&again)
	for i := range rows {
		if again[i].TokenEncrypted != rows[i].TokenEncrypted || *again[i].WebhookURLToken != *rows[i].WebhookURLToken {
			t.Fatalf("row %s changed on second run", rows[i].ID)
		}
	}
}
