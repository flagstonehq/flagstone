package auth

import "testing"

func TestGenerateRefreshToken(t *testing.T) {
	raw, hash, err := GenerateRefreshToken(32)
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}

	if raw == "" {
		t.Fatal("expected non-empty raw token")
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if HashRefreshToken(raw) != hash {
		t.Fatal("HashRefreshToken(raw) must equal returned hash")
	}
}

func TestGenerateRefreshTokenDefaultBytes(t *testing.T) {
	raw, hash, err := GenerateRefreshToken(0)
	if err != nil {
		t.Fatalf("GenerateRefreshToken(0) failed: %v", err)
	}
	if raw == "" {
		t.Fatal("expected non-empty raw token with default bytes")
	}
	if hash == "" {
		t.Fatal("expected non-empty hash with default bytes")
	}
}

func TestGenerateRefreshTokenTooSmall(t *testing.T) {
	if _, _, err := GenerateRefreshToken(8); err == nil {
		t.Fatal("expected error for fewer than 16 random bytes")
	}
}

func TestGenerateRefreshTokenUnique(t *testing.T) {
	rawA, hashA, err := GenerateRefreshToken(32)
	if err != nil {
		t.Fatalf("GenerateRefreshToken A failed: %v", err)
	}

	rawB, hashB, err := GenerateRefreshToken(32)
	if err != nil {
		t.Fatalf("GenerateRefreshToken B failed: %v", err)
	}

	if rawA == rawB {
		t.Fatal("expected unique raw tokens")
	}
	if hashA == hashB {
		t.Fatal("expected unique hashes")
	}
}

func TestHashRefreshTokenDeterministic(t *testing.T) {
	raw := "test-token-value"
	h1 := HashRefreshToken(raw)
	h2 := HashRefreshToken(raw)

	if h1 != h2 {
		t.Fatal("HashRefreshToken must be deterministic")
	}
}

func TestHashRefreshTokenDifferent(t *testing.T) {
	h1 := HashRefreshToken("token-a")
	h2 := HashRefreshToken("token-b")
	if h1 == h2 {
		t.Fatal("different tokens must produce different hashes")
	}
}
