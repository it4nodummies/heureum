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

	plaintext, err := svc.CreateAPIToken("u1", "ci")
	if err != nil {
		t.Fatal(err)
	}
	if len(plaintext) < 24 {
		t.Fatalf("token too short: %q", plaintext)
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
