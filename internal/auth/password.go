package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// DefaultBcryptCost is the default bcrypt cost factor.
const DefaultBcryptCost = 12

// HashPassword hashes a plaintext password using bcrypt.
func HashPassword(plain string, cost int) (string, error) {
	if plain == "" {
		return "", fmt.Errorf("auth: password is required")
	}
	if cost == 0 {
		cost = DefaultBcryptCost
	}
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return "", fmt.Errorf("auth: bcrypt cost must be between %d and %d", bcrypt.MinCost, bcrypt.MaxCost)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(plain), cost)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}

	return string(hash), nil
}

// VerifyPassword verifies a plaintext password against a bcrypt hash.
func VerifyPassword(hash, plain string) error {
	if hash == "" {
		return fmt.Errorf("auth: password hash is required")
	}
	if plain == "" {
		return fmt.Errorf("auth: password is required")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		return fmt.Errorf("auth: verify password: %w", err)
	}
	return nil
}
