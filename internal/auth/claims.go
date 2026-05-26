package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const issuer = "flagstone"

// Claims represents the JWT claims for an authenticated user.
type Claims struct {
	TenantID string `json:"tid"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// NewClaims creates a new Claims instance with the provided values.
func NewClaims(userID, tenantID uuid.UUID, role string, now time.Time, ttl time.Duration) (*Claims, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("auth: user id is required")
	}
	if tenantID == uuid.Nil {
		return nil, fmt.Errorf("auth: tenant id is required")
	}
	if role == "" {
		return nil, fmt.Errorf("auth: role is required")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("auth: ttl must be positive")
	}

	now = now.UTC()

	return &Claims{
		TenantID: tenantID.String(),
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}, nil
}

// UserID parses and returns the subject claim as a uuid.UUID.
func (c *Claims) UserID() (uuid.UUID, error) {
	if c == nil {
		return uuid.Nil, fmt.Errorf("auth: claims are nil")
	}
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: invalid subject uuid: %w", err)
	}
	return id, nil
}

// TenantUUID parses and returns the tenant id claim as a uuid.UUID.
func (c *Claims) TenantUUID() (uuid.UUID, error) {
	if c == nil {
		return uuid.Nil, fmt.Errorf("auth: claims are nil")
	}
	id, err := uuid.Parse(c.TenantID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: invalid tenant uuid: %w", err)
	}
	return id, nil
}
