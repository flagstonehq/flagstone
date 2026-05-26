package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const defaultRandomBytes = 32

// GenerateRefreshToken generates a cryptographically random refresh token
// and returns the raw token along with its SHA-256 hash.
func GenerateRefreshToken(randomBytes int) (rawToken, tokenHash string, err error) {
	rawToken, err = generateOpaqueToken(randomBytes)
	if err != nil {
		return "", "", err
	}
	return rawToken, HashRefreshToken(rawToken), nil
}

// HashRefreshToken returns the hex-encoded SHA-256 of a raw token.
func HashRefreshToken(rawToken string) string {
	return sha256Hex(rawToken)
}

// generateOpaqueToken generates random bytes and encodes them in base62.
func generateOpaqueToken(randomBytes int) (string, error) {
	if randomBytes == 0 {
		randomBytes = defaultRandomBytes
	}
	if randomBytes < 16 {
		return "", fmt.Errorf("auth: random bytes must be at least 16")
	}

	buf := make([]byte, randomBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: generate random token: %w", err)
	}

	return Base62Encode(buf), nil
}

// sha256Hex returns the hex-encoded SHA-256 sum of a string.
func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
