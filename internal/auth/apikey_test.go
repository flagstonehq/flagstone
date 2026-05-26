package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	raw, hash, prefix, err := GenerateAPIKey("live", 32)
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}

	if !strings.HasPrefix(raw, "fs_live_") {
		t.Errorf("expected key to start with \"fs_live_\", got %q", raw)
	}

	if hash == "" {
		t.Error("expected non-empty key hash")
	}

	if prefix == "" {
		t.Error("expected non-empty key prefix")
	}
	if len(prefix) > 12 {
		t.Errorf("prefix too long: %q (%d chars)", prefix, len(prefix))
	}

	if HashAPIKey(raw) != hash {
		t.Error("HashAPIKey(raw) must equal returned hash")
	}

	if !strings.HasPrefix(raw, prefix) {
		t.Errorf("prefix %q must be prefix of raw key %q", prefix, raw)
	}
}

func TestGenerateAPIKeyTestEnv(t *testing.T) {
	raw, _, _, err := GenerateAPIKey("test", 32)
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}
	if !strings.HasPrefix(raw, "fs_test_") {
		t.Errorf("expected key to start with \"fs_test_\", got %q", raw)
	}
}

func TestGenerateAPIKeyInvalidEnv(t *testing.T) {
	invalidHints := []string{"", "prod", "production", "staging", "dev"}
	for _, hint := range invalidHints {
		_, _, _, err := GenerateAPIKey(hint, 32)
		if err == nil {
			t.Errorf("expected error for env hint %q", hint)
		}
	}
}

func TestGenerateAPIKeyUnique(t *testing.T) {
	rawA, hashA, _, err := GenerateAPIKey("test", 32)
	if err != nil {
		t.Fatalf("GenerateAPIKey A failed: %v", err)
	}

	rawB, hashB, _, err := GenerateAPIKey("test", 32)
	if err != nil {
		t.Fatalf("GenerateAPIKey B failed: %v", err)
	}

	if rawA == rawB {
		t.Fatal("expected unique raw keys")
	}
	if hashA == hashB {
		t.Fatal("expected unique hashes")
	}
}

func TestHashAPIKeyDeterministic(t *testing.T) {
	h1 := HashAPIKey("fs_live_testkey123")
	h2 := HashAPIKey("fs_live_testkey123")

	if h1 != h2 {
		t.Fatal("HashAPIKey must be deterministic")
	}
}

func TestGenerateAPIKeyDefaultBytes(t *testing.T) {
	raw, hash, prefix, err := GenerateAPIKey("live", 0)
	if err != nil {
		t.Fatalf("GenerateAPIKey(0) failed: %v", err)
	}
	if raw == "" || hash == "" || prefix == "" {
		t.Fatal("expected non-empty values with default byte count")
	}
}

func TestGenerateAPIKeyPrefixLength(t *testing.T) {
	for _, bytes := range []int{16, 32, 64} {
		_, _, prefix, err := GenerateAPIKey("live", bytes)
		if err != nil {
			t.Fatalf("GenerateAPIKey(%d) failed: %v", bytes, err)
		}
		if len(prefix) > 12 {
			t.Errorf("prefix exceeds 12 chars for %d random bytes: %q", bytes, prefix)
		}
	}
}
