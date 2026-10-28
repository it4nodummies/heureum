package git

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// cipherPrefix marks a token_encrypted value as AES-256-GCM ciphertext
// (format v1). Values without it are legacy plaintext rows written before
// R27 and are re-encrypted in place by ConfigService.EnsureBackfill.
const cipherPrefix = "v1:"

const gcmNonceSize = 12

// KeyFromSecret derives the 32-byte AES-256 key used for git provider tokens
// from APP_SECRET. This is the single derivation point: rotating APP_SECRET
// re-keys stored tokens (documented limitation — re-save the provider).
func KeyFromSecret(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

// IsEncrypted reports whether a stored token_encrypted value is v1 ciphertext
// (as opposed to a legacy plaintext token).
func IsEncrypted(stored string) bool {
	return strings.HasPrefix(stored, cipherPrefix)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("git crypto: key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Encrypt seals plaintext with AES-256-GCM under key, using a fresh random
// 12-byte nonce. Output format: "v1:" + base64(nonce || ciphertext||tag).
func Encrypt(key []byte, plaintext string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcmNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return cipherPrefix + base64.StdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

// Decrypt reverses Encrypt. It rejects values without the v1 prefix (legacy
// plaintext is never silently returned as if it had been decrypted),
// malformed base64, truncated payloads and GCM authentication failures.
func Decrypt(key []byte, ciphertext string) (string, error) {
	if !IsEncrypted(ciphertext) {
		return "", errors.New("git crypto: not a v1 ciphertext")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ciphertext, cipherPrefix))
	if err != nil {
		return "", fmt.Errorf("git crypto: malformed ciphertext: %w", err)
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	if len(raw) < gcmNonceSize+gcm.Overhead() {
		return "", errors.New("git crypto: ciphertext too short")
	}
	pt, err := gcm.Open(nil, raw[:gcmNonceSize], raw[gcmNonceSize:], nil)
	if err != nil {
		return "", errors.New("git crypto: authentication failed")
	}
	return string(pt), nil
}

// newURLToken returns a random 32-byte token, hex-encoded (64 chars), used
// as the unguessable path segment of the inbound webhook URL. It is distinct
// from the HMAC webhook_secret, which is only ever used to verify signatures.
func newURLToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
