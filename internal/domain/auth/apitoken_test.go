package auth

import (
	"testing"

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
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, last_used_at TIMESTAMP)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreateAndVerifyAPIToken(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('u1', 'a@b.c', 'ab')`)
	svc := NewService(db, "test-secret")

	tok, plaintext, err := svc.CreateAPIToken("u1", "ci")
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

	_, plaintext, err := svc.CreateAPIToken("u1", "ci")
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

	_, tokenA, err := svc.CreateAPIToken("uA", "alice-token")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.CreateAPIToken("uB", "bob-token"); err != nil {
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
