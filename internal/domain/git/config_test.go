package git

import (
	"errors"
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

	cfg, err := svc.CreateProvider("proj-1", "github", "https://github.com/acme/x", "ghp_plain", "hmac-secret-0123456789")
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
	if row.WebhookSecret != "hmac-secret-0123456789" {
		t.Fatalf("webhook_secret = %q (must stay usable for HMAC verification)", row.WebhookSecret)
	}

	found, err := svc.FindByWebhookURLToken(*row.WebhookURLToken)
	if err != nil || found.ID != cfg.ID {
		t.Fatalf("FindByWebhookURLToken: %v", err)
	}
	if _, err := svc.FindByWebhookURLToken("hmac-secret-0123456789"); err == nil {
		t.Fatalf("HMAC secret resolved as URL token")
	}

	// Empty token: nothing to encrypt, has no token.
	cfg2, err := svc.CreateProvider("proj-2", "github", "https://github.com/acme/y", "", "hmac-secret-0123456789")
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

// TestCreateProvider_RejectsEmptyWebhookSecret: the HMAC secret is the only
// thing that authenticates inbound events, so an empty one (forgeable
// empty-key HMAC) is refused at create, as is anything shorter than
// MinWebhookSecretLen. Exactly MinWebhookSecretLen is accepted.
func TestCreateProvider_RejectsEmptyWebhookSecret(t *testing.T) {
	svc := NewConfigService(newProviderDB(t), KeyFromSecret("test-secret"))

	if _, err := svc.CreateProvider("proj-1", "github", "https://github.com/acme/x", "", ""); !errors.Is(err, ErrWebhookSecretRequired) {
		t.Errorf("empty webhook_secret: err = %v, want ErrWebhookSecretRequired", err)
	}
	short := strings.Repeat("s", MinWebhookSecretLen-1)
	if _, err := svc.CreateProvider("proj-1", "github", "https://github.com/acme/x", "", short); !errors.Is(err, ErrWebhookSecretTooShort) {
		t.Errorf("%d-char webhook_secret: err = %v, want ErrWebhookSecretTooShort", len(short), err)
	}
	var count int64
	svc.DB().Model(&GitProviderConfig{}).Count(&count)
	if count != 0 {
		t.Fatalf("rejected creates still wrote %d rows", count)
	}

	ok := strings.Repeat("s", MinWebhookSecretLen)
	cfg, err := svc.CreateProvider("proj-1", "github", "https://github.com/acme/x", "", ok)
	if err != nil {
		t.Fatalf("%d-char webhook_secret: unexpected err %v", len(ok), err)
	}
	if cfg.WebhookSecret != ok {
		t.Errorf("stored secret = %q", cfg.WebhookSecret)
	}
}

// TestBitbucket_CreateRefusedOrValidated: there is no Bitbucket signature
// validator (nor payload parser) in the codebase, so creating a `bitbucket`
// provider is refused — otherwise the row would pass the DB CHECK and its
// inbound hook would go through the noop provider. Same for any type the
// code doesn't know, even though the DB CHECK would already stop those.
func TestBitbucket_CreateRefusedOrValidated(t *testing.T) {
	svc := NewConfigService(newProviderDB(t), KeyFromSecret("test-secret"))
	secret := strings.Repeat("s", MinWebhookSecretLen)

	for _, pt := range []string{"bitbucket", "svn", ""} {
		if _, err := svc.CreateProvider("proj-1", pt, "https://bitbucket.org/acme/x", "", secret); !errors.Is(err, ErrProviderTypeUnsupported) {
			t.Errorf("provider_type %q: err = %v, want ErrProviderTypeUnsupported", pt, err)
		}
	}
	var count int64
	svc.DB().Model(&GitProviderConfig{}).Count(&count)
	if count != 0 {
		t.Fatalf("refused creates still wrote %d rows", count)
	}
	for _, pt := range []string{"github", "gitlab", "forgejo", "gitea"} {
		if _, err := svc.CreateProvider("proj-"+pt, pt, "https://example.com/acme/x", "", secret); err != nil {
			t.Errorf("provider_type %q: unexpected err %v", pt, err)
		}
	}
}

// TestEnsureBackfill_CountsEmptyWebhookSecrets: legacy rows with an empty
// secret are not fixed (we can't invent a secret the provider knows), but
// their count is reported so startup can warn the operator to re-save them.
func TestEnsureBackfill_CountsEmptyWebhookSecrets(t *testing.T) {
	db := newProviderDB(t)
	svc := NewConfigService(db, KeyFromSecret("test-secret"))
	for _, r := range []*GitProviderConfig{
		{ID: "l1", ProjectID: "p1", ProviderType: ProviderGitHub, BaseURL: "https://github.com/a/x", WebhookSecret: ""},
		{ID: "l2", ProjectID: "p2", ProviderType: ProviderGitLab, BaseURL: "https://gitlab.com/a/y", WebhookSecret: ""},
		{ID: "l3", ProjectID: "p3", ProviderType: ProviderGitHub, BaseURL: "https://github.com/a/z", WebhookSecret: "a-real-secret-value"},
	} {
		if err := db.Create(r).Error; err != nil {
			t.Fatal(err)
		}
	}
	stats, err := svc.EnsureBackfill()
	if err != nil {
		t.Fatalf("EnsureBackfill: %v", err)
	}
	if stats.EmptyWebhookSecrets != 2 {
		t.Errorf("EmptyWebhookSecrets = %d, want 2", stats.EmptyWebhookSecrets)
	}
	var row GitProviderConfig
	db.First(&row, "id = ?", "l1")
	if row.WebhookSecret != "" {
		t.Errorf("backfill invented a secret: %q", row.WebhookSecret)
	}
}
