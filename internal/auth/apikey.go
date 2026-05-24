package auth

import (
	"fmt"
	"strings"
)

const apiKeyPrefixLength = 12

// GenerateAPIKey generates a new API key with the given environment hint
// and returns the raw key, its SHA-256 hash, and its prefix.
func GenerateAPIKey(envHint string, randomBytes int) (rawKey, keyHash, keyPrefix string, err error) {
	hint, err := normalizeEnvHint(envHint)
	if err != nil {
		return "", "", "", err
	}

	token, err := generateOpaqueToken(randomBytes)
	if err != nil {
		return "", "", "", err
	}

	rawKey = "fs_" + hint + "_" + token
	keyHash = HashAPIKey(rawKey)
	keyPrefix = rawKey
	if len(keyPrefix) > apiKeyPrefixLength {
		keyPrefix = keyPrefix[:apiKeyPrefixLength]
	}

	return rawKey, keyHash, keyPrefix, nil
}

// HashAPIKey returns the hex-encoded SHA-256 of a raw API key.
func HashAPIKey(rawKey string) string {
	return sha256Hex(rawKey)
}

// normalizeEnvHint validates and normalizes the environment hint.
func normalizeEnvHint(envHint string) (string, error) {
	hint := strings.ToLower(strings.TrimSpace(envHint))
	switch hint {
	case "live", "test":
		return hint, nil
	default:
		return "", fmt.Errorf("auth: environment hint must be \"live\" or \"test\"")
	}
}
