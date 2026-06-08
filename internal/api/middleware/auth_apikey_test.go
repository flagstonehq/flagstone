package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flagstonehq/flagstone/internal/auth"
	"github.com/flagstonehq/flagstone/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthAPIKey_missingHeader(t *testing.T) {
	skipIfNoDB(t)

	handler := AuthAPIKey(testStores, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthAPIKey_unknownKey(t *testing.T) {
	skipIfNoDB(t)

	handler := AuthAPIKey(testStores, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer fs_test_unknownKeyThatDoesNotExist")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthAPIKey_validKey(t *testing.T) {
	skipIfNoDB(t)

	ctx := context.Background()
	envID := seedEnvironment(t)

	key := &storage.APIKey{
		KeyHash:       auth.HashAPIKey("fs_test_myValidTestKey12345"),
		KeyPrefix:     "fs_test_myVa",
		Name:          "test-key",
		EnvironmentID: envID,
	}

	err := testStores.APIKeys.Create(ctx, key)
	require.NoError(t, err)

	handler := AuthAPIKey(testStores, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		envFromCtx := EnvironmentIDFromContext(r.Context())
		assert.Equal(t, envID, envFromCtx)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer fs_test_myValidTestKey12345")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthAPIKey_expiredKey(t *testing.T) {
	skipIfNoDB(t)

	ctx := context.Background()
	envID := seedEnvironment(t)
	past := time.Now().Add(-1 * time.Hour)

	key := &storage.APIKey{
		KeyHash:       auth.HashAPIKey("fs_test_expiredKey0987654321"),
		KeyPrefix:     "fs_test_expi",
		Name:          "expired-key",
		EnvironmentID: envID,
		ExpiresAt:     &past,
	}
	require.NoError(t, testStores.APIKeys.Create(ctx, key))

	handler := AuthAPIKey(testStores, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not be reached when key is expired")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer fs_test_expiredKey0987654321")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthAPIKey_revokedKey(t *testing.T) {
	skipIfNoDB(t)

	ctx := context.Background()
	envID := seedEnvironment(t)

	key := &storage.APIKey{
		KeyHash:       auth.HashAPIKey("fs_test_revokedKey1122334455"),
		KeyPrefix:     "fs_test_revo",
		Name:          "revoked-key",
		EnvironmentID: envID,
	}
	require.NoError(t, testStores.APIKeys.Create(ctx, key))
	require.NoError(t, testStores.APIKeys.Revoke(ctx, key.ID, time.Now()))

	handler := AuthAPIKey(testStores, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not be reached when key is revoked")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer fs_test_revokedKey1122334455")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
