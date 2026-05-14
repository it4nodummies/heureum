package auth

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/domain/user"
)

type Service struct {
	DB     *gorm.DB
	Secret string
}

func NewService(db *gorm.DB, secret string) *Service {
	return &Service{DB: db, Secret: secret}
}

func generateID() string {
	return uuid.New().String()
}

func (s *Service) Register(email, username, displayName, password string) (*user.User, error) {
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	u := &user.User{
		Email: email, Username: username, DisplayName: displayName,
		PasswordHash: hashed, IsActive: true,
	}
	u.ID = generateID()
	if err := s.DB.Create(u).Error; err != nil {
		return nil, errors.New("email or username already taken")
	}
	return u, nil
}

func (s *Service) Login(email, password string) (string, error) {
	var u user.User
	if err := s.DB.Where("email = ?", email).First(&u).Error; err != nil {
		return "", errors.New("invalid credentials")
	}
	if !VerifyPassword(u.PasswordHash, password) {
		return "", errors.New("invalid credentials")
	}
	return GenerateToken(s.Secret, u.ID, 24*time.Hour)
}
