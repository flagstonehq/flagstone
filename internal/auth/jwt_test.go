package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testJWTSecret = "0123456789abcdef0123456789abcdef"

func TestGenerateAndValidateAccessToken(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()

	token, err := GenerateAccessToken(userID, tenantID, RoleAdmin.String(), testJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	claims, err := ValidateAccessToken(token, testJWTSecret)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}

	gotUserID, err := claims.UserID()
	if err != nil {
		t.Fatalf("UserID() failed: %v", err)
	}
	gotTenantID, err := claims.TenantUUID()
	if err != nil {
		t.Fatalf("TenantUUID() failed: %v", err)
	}

	if gotUserID != userID {
		t.Errorf("UserID() = %v, want %v", gotUserID, userID)
	}
	if gotTenantID != tenantID {
		t.Errorf("TenantUUID() = %v, want %v", gotTenantID, tenantID)
	}
	if claims.Role != RoleAdmin.String() {
		t.Errorf("claims.Role = %q, want %q", claims.Role, RoleAdmin.String())
	}
	if claims.Issuer != issuer {
		t.Errorf("claims.Issuer = %q, want %q", claims.Issuer, issuer)
	}
	if claims.ExpiresAt == nil {
		t.Fatal("claims.ExpiresAt must not be nil")
	}
	if claims.IssuedAt == nil {
		t.Fatal("claims.IssuedAt must not be nil")
	}
}

func TestValidateAccessTokenWrongSecret(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()

	token, err := GenerateAccessToken(userID, tenantID, RoleViewer.String(), testJWTSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	if _, err := ValidateAccessToken(token, "abcdef0123456789abcdef0123456789"); err == nil {
		t.Fatal("expected validation to fail with wrong secret")
	}
}

func TestValidateAccessTokenEmptySecret(t *testing.T) {
	if _, err := ValidateAccessToken("some-token", "short"); err == nil {
		t.Fatal("expected validation to fail with short secret")
	}
}

func TestValidateAccessTokenExpired(t *testing.T) {
	now := time.Now().UTC()
	claims, err := NewClaims(uuid.New(), uuid.New(), RoleMember.String(), now.Add(-30*time.Minute), 15*time.Minute)
	if err != nil {
		t.Fatalf("NewClaims failed: %v", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	if _, err := ValidateAccessToken(signed, testJWTSecret); err == nil {
		t.Fatal("expected expired token to fail")
	}
}

func TestValidateAccessTokenEmpty(t *testing.T) {
	if _, err := ValidateAccessToken("", testJWTSecret); err == nil {
		t.Fatal("expected validation to fail with empty token")
	}
}

func TestValidateAccessTokenMalformed(t *testing.T) {
	if _, err := ValidateAccessToken("not-a-valid-jwt", testJWTSecret); err == nil {
		t.Fatal("expected validation to fail with malformed token")
	}
}

func TestGenerateAccessTokenShortSecret(t *testing.T) {
	if _, err := GenerateAccessToken(uuid.New(), uuid.New(), RoleViewer.String(), "short", 15*time.Minute); err == nil {
		t.Fatal("expected error for short secret")
	}
}

func TestGenerateAccessTokenInvalidRole(t *testing.T) {
	if _, err := GenerateAccessToken(uuid.New(), uuid.New(), "hacker", testJWTSecret, 15*time.Minute); err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestValidateAccessTokenWrongAlgorithm(t *testing.T) {
	claims, err := NewClaims(uuid.New(), uuid.New(), RoleViewer.String(), time.Now(), 15*time.Minute)
	if err != nil {
		t.Fatalf("NewClaims failed: %v", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	if _, err := ValidateAccessToken(signed, testJWTSecret); err == nil {
		t.Fatal("expected validation to fail with wrong algorithm")
	}
}
