package auth

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT, username TEXT,
		display_name TEXT DEFAULT '', avatar_url TEXT DEFAULT '', password_hash TEXT DEFAULT '',
		is_admin BOOLEAN DEFAULT FALSE, is_active BOOLEAN DEFAULT TRUE)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE api_tokens (id TEXT PRIMARY KEY, user_id TEXT NOT NULL,
		label TEXT NOT NULL DEFAULT '', token_hash TEXT NOT NULL UNIQUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, last_used_at TIMESTAMP,
		expires_at TIMESTAMP)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

// ── Round 27 / Task 7: expiry, list, revoke ──────────────────────────────────

func TestAPIToken_ExpiredRejected(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('u1', 'a@b.c', 'ab')`)
	svc := NewService(db, "test-secret")
	fixed := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	svc.Now = func() time.Time { return fixed }

	past := fixed.Add(-time.Minute)
	tok, plaintext, err := svc.CreateAPIToken("u1", "old", &past)
	if err != nil {
		t.Fatal(err)
	}
	if tok.ExpiresAt == nil || !tok.ExpiresAt.Equal(past) {
		t.Fatalf("expires_at = %v, want %v", tok.ExpiresAt, past)
	}
	if _, err := svc.VerifyAPIToken("a@b.c", plaintext); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired token must be rejected with ErrInvalidToken, got %v", err)
	}
	// Exactly at the boundary the token is expired too (expires_at is exclusive).
	svc.Now = func() time.Time { return past }
	if _, err := svc.VerifyAPIToken("a@b.c", plaintext); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("token at expires_at instant must be rejected, got %v", err)
	}
}

func TestAPIToken_NotExpiredAccepted(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('u1', 'a@b.c', 'ab')`)
	svc := NewService(db, "test-secret")
	fixed := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	svc.Now = func() time.Time { return fixed }

	future := fixed.Add(time.Hour)
	_, plaintext, err := svc.CreateAPIToken("u1", "fresh", &future)
	if err != nil {
		t.Fatal(err)
	}
	if userID, err := svc.VerifyAPIToken("a@b.c", plaintext); err != nil || userID != "u1" {
		t.Fatalf("not-yet-expired token: userID=%q err=%v", userID, err)
	}
	// Once the clock passes expires_at the same token stops working.
	svc.Now = func() time.Time { return future.Add(time.Second) }
	if _, err := svc.VerifyAPIToken("a@b.c", plaintext); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("token after expires_at must be rejected, got %v", err)
	}
}

func TestAPIToken_NoExpiryAccepted(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('u1', 'a@b.c', 'ab')`)
	svc := NewService(db, "test-secret")
	// Far in the future: a token without expires_at never expires.
	svc.Now = func() time.Time { return time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC) }

	tok, plaintext, err := svc.CreateAPIToken("u1", "forever", nil)
	if err != nil {
		t.Fatal(err)
	}
	if tok.ExpiresAt != nil {
		t.Fatalf("expires_at = %v, want nil", tok.ExpiresAt)
	}
	if userID, err := svc.VerifyAPIToken("a@b.c", plaintext); err != nil || userID != "u1" {
		t.Fatalf("no-expiry token: userID=%q err=%v", userID, err)
	}
}

func TestListAPITokens_OwnOnlyNoHash(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('uA', 'alice@b.c', 'alice')`)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('uB', 'bob@b.c', 'bob')`)
	svc := NewService(db, "test-secret")

	exp := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	tokA1, plainA1, err := svc.CreateAPIToken("uA", "alice-1", &exp)
	if err != nil {
		t.Fatal(err)
	}
	tokA2, plainA2, err := svc.CreateAPIToken("uA", "alice-2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.CreateAPIToken("uB", "bob-1", nil); err != nil {
		t.Fatal(err)
	}

	list, err := svc.ListAPITokens("uA")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("alice sees %d tokens, want 2: %+v", len(list), list)
	}
	ids := map[string]bool{}
	for _, tk := range list {
		ids[tk.ID] = true
		if tk.UserID != "uA" {
			t.Errorf("token %s belongs to %q, want uA", tk.ID, tk.UserID)
		}
	}
	if !ids[tokA1.ID] || !ids[tokA2.ID] {
		t.Errorf("list ids = %v, want both of alice's tokens", ids)
	}

	raw, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, secret := range []string{"token_hash", tokA1.TokenHash, tokA2.TokenHash, plainA1, plainA2} {
		if strings.Contains(body, secret) {
			t.Errorf("serialized list leaks %q: %s", secret, body)
		}
	}
	if !strings.Contains(body, `"expires_at":"2030-01-01T00:00:00Z"`) {
		t.Errorf("serialized list must carry expires_at for the expiring token: %s", body)
	}

	// An unknown user gets an empty (non-nil) list, never an error.
	empty, err := svc.ListAPITokens("nobody")
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("unknown user list = %#v, want empty non-nil slice", empty)
	}
}

func TestRevoke_OwnTokenDeleted_ThenRejected(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('u1', 'a@b.c', 'ab')`)
	svc := NewService(db, "test-secret")

	tok, plaintext, err := svc.CreateAPIToken("u1", "ci", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.VerifyAPIToken("a@b.c", plaintext); err != nil {
		t.Fatalf("verify before revoke: %v", err)
	}
	if err := svc.RevokeAPIToken("u1", tok.ID); err != nil {
		t.Fatalf("revoke own token: %v", err)
	}
	if _, err := svc.VerifyAPIToken("a@b.c", plaintext); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("revoked token must be rejected immediately, got %v", err)
	}
	var n int64
	db.Table("api_tokens").Where("id = ?", tok.ID).Count(&n)
	if n != 0 {
		t.Errorf("row still present after revoke")
	}
	// Revoking twice is a not-found, not a silent success.
	if err := svc.RevokeAPIToken("u1", tok.ID); !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("second revoke = %v, want ErrTokenNotFound", err)
	}
}

func TestRevoke_OtherUsersTokenIsNotFound(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('uA', 'alice@b.c', 'alice')`)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('uB', 'bob@b.c', 'bob')`)
	svc := NewService(db, "test-secret")

	tokB, plainB, err := svc.CreateAPIToken("uB", "bob-token", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Alice tries to revoke Bob's token by id: indistinguishable from an unknown id.
	err = svc.RevokeAPIToken("uA", tokB.ID)
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("revoking another user's token = %v, want ErrTokenNotFound", err)
	}
	if err := svc.RevokeAPIToken("uA", "does-not-exist"); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("revoking an unknown id = %v, want ErrTokenNotFound", err)
	}
	// Bob's row is intact and his token still works.
	var n int64
	db.Table("api_tokens").Where("id = ?", tokB.ID).Count(&n)
	if n != 1 {
		t.Errorf("bob's token row count = %d, want 1 (untouched)", n)
	}
	if userID, err := svc.VerifyAPIToken("bob@b.c", plainB); err != nil || userID != "uB" {
		t.Errorf("bob verify after alice's attempt: userID=%q err=%v", userID, err)
	}
}

func TestCreateAndVerifyAPIToken(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('u1', 'a@b.c', 'ab')`)
	svc := NewService(db, "test-secret")

	tok, plaintext, err := svc.CreateAPIToken("u1", "ci", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plaintext) < 24 {
		t.Fatalf("token too short: %q", plaintext)
	}
	if tok == nil || tok.ID == "" {
		t.Fatalf("created token record must have an ID, got %+v", tok)
	}
	if tok.Label != "ci" {
		t.Errorf("label = %q, want %q", tok.Label, "ci")
	}
	if tok.CreatedAt.IsZero() {
		t.Error("created_at must be set")
	}

	userID, err := svc.VerifyAPIToken("a@b.c", plaintext)
	if err != nil || userID != "u1" {
		t.Fatalf("verify: userID=%q err=%v", userID, err)
	}

	if _, err := svc.VerifyAPIToken("a@b.c", "wrong-token"); err == nil {
		t.Error("wrong token must fail")
	}
	if _, err := svc.VerifyAPIToken("other@b.c", plaintext); err == nil {
		t.Error("wrong email must fail")
	}
}

func TestVerifyAPIToken_InactiveUser(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('u1', 'a@b.c', 'ab')`)
	svc := NewService(db, "test-secret")

	_, plaintext, err := svc.CreateAPIToken("u1", "ci", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Sanity check: token works while the user is active.
	if _, err := svc.VerifyAPIToken("a@b.c", plaintext); err != nil {
		t.Fatalf("verify before deactivation: %v", err)
	}

	if err := db.Exec(`UPDATE users SET is_active = FALSE WHERE id = 'u1'`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.VerifyAPIToken("a@b.c", plaintext); err == nil {
		t.Error("token of a deactivated user must fail verification")
	}
}

func TestVerifyAPIToken_CrossAccount(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('uA', 'alice@b.c', 'alice')`)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('uB', 'bob@b.c', 'bob')`)
	svc := NewService(db, "test-secret")

	_, tokenA, err := svc.CreateAPIToken("uA", "alice-token", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.CreateAPIToken("uB", "bob-token", nil); err != nil {
		t.Fatal(err)
	}

	// Bob's email with Alice's token must fail: the lookup is scoped by user_id.
	if _, err := svc.VerifyAPIToken("bob@b.c", tokenA); err == nil {
		t.Error("user B must not authenticate with user A's token")
	}
	// Alice's own pairing still works.
	if userID, err := svc.VerifyAPIToken("alice@b.c", tokenA); err != nil || userID != "uA" {
		t.Errorf("alice verify: userID=%q err=%v", userID, err)
	}
}
