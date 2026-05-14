package auth

import (
	"testing"
	"time"
)

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "test-secret-min-32-chars-long-key!!"
	token, err := GenerateToken(secret, "user-123", time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	claims, err := ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID = %s, want user-123", claims.UserID)
	}
}

func TestValidateExpiredToken(t *testing.T) {
	secret := "test-secret-min-32-chars-long-key!!"
	token, _ := GenerateToken(secret, "user-123", -time.Hour)
	_, err := ValidateToken(secret, token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestValidateInvalidToken(t *testing.T) {
	_, err := ValidateToken("test-secret-min-32-chars-long-key!!", "invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}
