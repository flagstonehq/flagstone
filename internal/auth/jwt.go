package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ErrInvalidToken is returned when a token cannot be validated.
var ErrInvalidToken = errors.New("auth: invalid token")

// GenerateAccessToken creates a signed JWT access token for the given user and tenant.
func GenerateAccessToken(userID, tenantID uuid.UUID, role, secret string, ttl time.Duration, sessionID uuid.UUID) (string, error) {
	if len(secret) < 32 {
		return "", fmt.Errorf("auth: jwt secret must be at least 32 characters")
	}
	if !ParseRole(role).Valid() {
		return "", fmt.Errorf("auth: invalid role %q", role)
	}

	claims, err := NewClaims(userID, tenantID, role, time.Now(), ttl, sessionID)
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("auth: sign access token: %w", err)
	}

	return signed, nil
}

// ValidateAccessToken parses and validates a JWT access token, returning the claims.
func ValidateAccessToken(tokenStr, secret string) (*Claims, error) {
	if tokenStr == "" {
		return nil, fmt.Errorf("auth: token is required")
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("auth: jwt secret must be at least 32 characters")
	}

	claims := &Claims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
	)

	token, err := parser.ParseWithClaims(tokenStr, claims, func(_ *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth: validate access token: %w", err)
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}

	if _, err := claims.UserID(); err != nil {
		return nil, err
	}
	if _, err := claims.TenantUUID(); err != nil {
		return nil, err
	}
	if !ParseRole(claims.Role).Valid() {
		return nil, fmt.Errorf("auth: invalid token role %q", claims.Role)
	}

	return claims, nil
}
