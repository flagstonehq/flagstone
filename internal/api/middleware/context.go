package middleware

import (
	"context"

	"github.com/google/uuid"
	"github.com/flagstonehq/flagstone/internal/auth"
	"go.uber.org/zap"
)

type contextKey int

const (
	keyRequestID contextKey = iota
	keyLogger
	keyClaims
	keyEnvironmentID
)

// WithRequestID stores a request ID in the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, keyRequestID, requestID)
}

// RequestIDFromContext extracts the request ID from the context.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(keyRequestID).(string); ok {
		return id
	}
	return ""
}

// WithLogger stores a request-scoped logger in the context.
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, keyLogger, logger)
}

// LoggerFromContext extracts the request-scoped logger from the context.
// Returns a no-op logger if none is set.
func LoggerFromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(keyLogger).(*zap.Logger); ok {
		return logger
	}
	return zap.NewNop()
}

// WithClaims stores JWT claims in the context.
func WithClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, keyClaims, claims)
}

// ClaimsFromContext extracts the JWT claims from the context.
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	if claims, ok := ctx.Value(keyClaims).(*auth.Claims); ok {
		return claims
	}
	return nil
}

// WithEnvironmentID stores the authenticated environment ID in the context.
func WithEnvironmentID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, keyEnvironmentID, id)
}

// EnvironmentIDFromContext extracts the environment ID from the context.
func EnvironmentIDFromContext(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(keyEnvironmentID).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}
