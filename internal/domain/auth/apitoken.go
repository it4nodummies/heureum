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
}

func (APIToken) TableName() string { return "api_tokens" }

func hashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// CreateAPIToken generates a token, stores its hash and returns the created
// record along with the plaintext (shown to the user only once, the same way
// Atlassian does).
func (s *Service) CreateAPIToken(userID, label string) (*APIToken, string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", err
	}
	plaintext := "ojt_" + base64.RawURLEncoding.EncodeToString(raw)
	tok := APIToken{ID: generateID(), UserID: userID, Label: label, TokenHash: hashToken(plaintext)}
	if err := s.DB.Create(&tok).Error; err != nil {
		return nil, "", err
	}
	return &tok, plaintext, nil
}

// ErrInvalidToken is returned by VerifyAPIToken whenever the email/token pair
// cannot be matched to an active user — unknown email, deactivated account,
// or a token that does not belong to that user. Callers that need to
// distinguish authentication failures from other errors should test with
// errors.Is(err, ErrInvalidToken).
var ErrInvalidToken = errors.New("invalid email or api token")

// VerifyAPIToken implements Jira's Basic auth: email + api token.
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
	now := time.Now()
	if err := s.DB.Model(&tok).Update("last_used_at", &now).Error; err != nil {
		log.Printf("auth: update last_used_at: %v", err)
	}
	return u.ID, nil
}
