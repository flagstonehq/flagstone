package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	password := "correct-horse-battery-staple"
	hash, err := HashPassword(password, DefaultBcryptCost)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == password {
		t.Fatal("hash must not equal plaintext")
	}
	if strings.Contains(hash, password) {
		t.Fatal("hash must not contain plaintext")
	}

	if err := VerifyPassword(hash, password); err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
}

func TestVerifyPasswordWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple", DefaultBcryptCost)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if err := VerifyPassword(hash, "wrong-password"); err == nil {
		t.Fatal("expected VerifyPassword to return error for wrong password")
	}
}

func TestVerifyPasswordWrongHash(t *testing.T) {
	password := "test-password"
	hash, err := HashPassword(password, DefaultBcryptCost)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	otherHash, err := HashPassword("other-password", DefaultBcryptCost)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == otherHash {
		t.Fatal("different passwords must produce different hashes")
	}
}

func TestHashPasswordEmpty(t *testing.T) {
	if _, err := HashPassword("", DefaultBcryptCost); err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestVerifyPasswordEmptyHash(t *testing.T) {
	if err := VerifyPassword("", "password"); err == nil {
		t.Fatal("expected error for empty hash")
	}
}

func TestVerifyPasswordEmptyPassword(t *testing.T) {
	if err := VerifyPassword("somehash", ""); err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestHashPasswordInvalidCost(t *testing.T) {
	if _, err := HashPassword("password", 0); err != nil {
		t.Fatal("cost 0 should use default")
	}

	if _, err := HashPassword("password", 3); err == nil {
		t.Fatal("expected error for cost below min")
	}

	if _, err := HashPassword("password", 50); err == nil {
		t.Fatal("expected error for cost above max")
	}
}

func TestDefaultBcryptCost(t *testing.T) {
	if DefaultBcryptCost < 10 {
		t.Fatal("default bcrypt cost should be at least 10")
	}
}
