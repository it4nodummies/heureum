package git

import (
	"strings"
	"testing"
)

func TestCrypto_RoundTrip(t *testing.T) {
	key := KeyFromSecret("app-secret-at-least-32-bytes-long-xxxxxxxx")
	if len(key) != 32 {
		t.Fatalf("key len = %d, want 32 (AES-256)", len(key))
	}
	ct, err := Encrypt(key, "ghp_plaintext-token")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	pt, err := Decrypt(key, ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if pt != "ghp_plaintext-token" {
		t.Fatalf("round trip = %q", pt)
	}
	if ct == "ghp_plaintext-token" || strings.Contains(ct, "plaintext") {
		t.Fatalf("ciphertext leaks plaintext: %q", ct)
	}
}

func TestCrypto_PrefixAndDistinctNonces(t *testing.T) {
	key := KeyFromSecret("s")
	a, err := Encrypt(key, "same")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Encrypt(key, "same")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a, "v1:") || !strings.HasPrefix(b, "v1:") {
		t.Fatalf("missing v1: prefix: %q %q", a, b)
	}
	if !IsEncrypted(a) || IsEncrypted("plain") {
		t.Fatalf("IsEncrypted misclassifies")
	}
	if a == b {
		t.Fatalf("two encryptions of the same plaintext are identical (nonce reuse?)")
	}
}

func TestCrypto_WrongKeyFails(t *testing.T) {
	ct, err := Encrypt(KeyFromSecret("one"), "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(KeyFromSecret("two"), ct); err == nil {
		t.Fatalf("Decrypt with the wrong key succeeded")
	}
}

func TestCrypto_RejectsMalformed(t *testing.T) {
	key := KeyFromSecret("k")
	for _, bad := range []string{"", "plain-legacy-token", "v1:", "v1:!!!not-base64!!!", "v1:AAAA", "v2:" + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} {
		if _, err := Decrypt(key, bad); err == nil {
			t.Errorf("Decrypt(%q) succeeded, want error", bad)
		}
	}
	// Tampered ciphertext must fail GCM authentication.
	ct, _ := Encrypt(key, "secret")
	tampered := ct[:len(ct)-2] + "AA"
	if tampered == ct {
		tampered = ct[:len(ct)-2] + "BB"
	}
	if _, err := Decrypt(key, tampered); err == nil {
		t.Errorf("tampered ciphertext decrypted")
	}
	if _, err := Encrypt([]byte("short"), "x"); err == nil {
		t.Errorf("Encrypt with a non-32-byte key succeeded")
	}
}
