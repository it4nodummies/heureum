package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"github.com/it4nodummies/heureum/internal/domain/user"
)

type APIToken struct {
	ID         string     `gorm:"primaryKey;type:text" json:"id"`
	UserID     string     `gorm:"type:text;not null;index" json:"user_id"`
	Label      string     `gorm:"type:text;not null;default:''" json:"label"`
	TokenHash  string     `gorm:"type:text;not null;uniqueIndex" json:"-"`
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	// ExpiresAt is nil for a token that never expires. A token is rejected
	// from the instant expires_at is reached (the bound is exclusive).
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func (APIToken) TableName() string { return "api_tokens" }

func hashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// Clock is the single expiry clock seam (Service.Now when set, time.Now
// otherwise), normalized to UTC. Both the create handler (expires_at = Clock
// + N days) and VerifyAPIToken go through it so they cannot drift. UTC matters:
// expires_at/last_used_at are TIMESTAMP (no tz) columns, and pgx stores the
// wall-clock digits of whatever zone the value carries and reads them back as
// UTC — a local-zoned value on a UTC+N host would be accepted for N extra
// hours past the expiry shown to the client. UTC digits round-trip exactly.
func (s *Service) Clock() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// CreateAPIToken generates a token, stores its hash and returns the created
// record along with the plaintext (shown to the user only once, the same way
// Atlassian does). expiresAt nil means the token never expires.
func (s *Service) CreateAPIToken(userID, label string, expiresAt *time.Time) (*APIToken, string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", err
	}
	plaintext := "ojt_" + base64.RawURLEncoding.EncodeToString(raw)
	tok := APIToken{ID: generateID(), UserID: userID, Label: label, TokenHash: hashToken(plaintext), ExpiresAt: expiresAt}
	if err := s.DB.Create(&tok).Error; err != nil {
		return nil, "", err
	}
	return &tok, plaintext, nil
}

// ListAPITokens returns the caller's own tokens (never another user's), oldest
// first. The hash is never serialized (json:"-") and the plaintext is not
// stored at all, so the result carries no secret.
func (s *Service) ListAPITokens(userID string) ([]APIToken, error) {
	tokens := []APIToken{}
	if err := s.DB.Where("user_id = ?", userID).Order("created_at ASC, id ASC").Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

// ErrTokenNotFound is returned by RevokeAPIToken when no token with that id
// belongs to the caller — an unknown id and another user's id are
// deliberately indistinguishable, so ids cannot be probed across accounts.
var ErrTokenNotFound = errors.New("api token not found")

// RevokeAPIToken deletes the token with the given id only if it belongs to
// userID. The ownership predicate is part of the DELETE itself (no read-then-
// delete), so a foreign id touches no row and surfaces as ErrTokenNotFound.
func (s *Service) RevokeAPIToken(userID, id string) error {
	res := s.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&APIToken{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrTokenNotFound
	}
	return nil
}

// ErrInvalidToken is returned by VerifyAPIToken whenever the email/token pair
// cannot be matched to an active user — unknown email, deactivated account,
// a token that does not belong to that user, or an expired token. Callers
// that need to distinguish authentication failures from other errors should
// test with errors.Is(err, ErrInvalidToken).
var ErrInvalidToken = errors.New("invalid email or api token")

// VerifyAPIToken implements Jira's Basic auth: email + api token. It is the
// single lookup path used by the Auth middleware, so the expiry check here is
// what turns an expired token into a 401 on every protected route.
func (s *Service) VerifyAPIToken(email, plaintext string) (string, error) {
	var u user.User
	if err := s.DB.Where("email = ? AND is_active = ?", email, true).First(&u).Error; err != nil {
		return "", ErrInvalidToken
	}
	var tok APIToken
	if err := s.DB.Where("user_id = ? AND token_hash = ?", u.ID, hashToken(plaintext)).
		First(&tok).Error; err != nil {
		return "", ErrInvalidToken
	}
	now := s.Clock()
	if tok.ExpiresAt != nil && !now.Before(*tok.ExpiresAt) {
		return "", ErrInvalidToken
	}
	if err := s.DB.Model(&tok).Update("last_used_at", &now).Error; err != nil {
		log.Printf("auth: update last_used_at: %v", err)
	}
	return u.ID, nil
}
