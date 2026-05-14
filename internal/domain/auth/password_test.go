package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	password := "my-secure-password-123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == password {
		t.Error("hash should not equal original password")
	}
	if !VerifyPassword(hash, password) {
		t.Error("VerifyPassword() should return true")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Error("VerifyPassword() should return false for wrong password")
	}
}
