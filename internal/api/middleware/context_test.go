package middleware

import (
	"context"
	"testing"

	"github.com/flagstonehq/flagstone/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestRequestIDContext(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req_abc123")
	assert.Equal(t, "req_abc123", RequestIDFromContext(ctx))

	assert.Empty(t, RequestIDFromContext(context.Background()))
}

func TestLoggerContext(t *testing.T) {
	logger := zap.NewNop()
	ctx := WithLogger(context.Background(), logger)
	assert.Equal(t, logger, LoggerFromContext(ctx))

	assert.NotNil(t, LoggerFromContext(context.Background()))
}

func TestClaimsContext(t *testing.T) {
	claims := &auth.Claims{Role: "admin"}
	ctx := WithClaims(context.Background(), claims)
	assert.Equal(t, claims, ClaimsFromContext(ctx))

	assert.Nil(t, ClaimsFromContext(context.Background()))
}

func TestEnvironmentIDContext(t *testing.T) {
	id := uuid.New()
	ctx := WithEnvironmentID(context.Background(), id)
	assert.Equal(t, id, EnvironmentIDFromContext(ctx))

	assert.Equal(t, uuid.Nil, EnvironmentIDFromContext(context.Background()))
}
