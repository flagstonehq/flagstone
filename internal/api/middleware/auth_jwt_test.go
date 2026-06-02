package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/flagstonehq/flagstone/internal/auth"
)

func TestAuthJWT_validToken(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	userID := uuid.New()
	tenantID := uuid.New()

	token, err := auth.GenerateAccessToken(userID, tenantID, "admin", secret, 15*time.Minute, uuid.Nil)
	require.NoError(t, err)

	handler := AuthJWT(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromContext(r.Context())
		require.NotNil(t, claims)
		assert.Equal(t, userID.String(), claims.Subject)
		assert.Equal(t, tenantID.String(), claims.TenantID)
		assert.Equal(t, "admin", claims.Role)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthJWT_missingHeader(t *testing.T) {
	handler := AuthJWT("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var body ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_CREDENTIALS", body.Error.Code)
}

func TestAuthJWT_wrongScheme(t *testing.T) {
	handler := AuthJWT("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthJWT_expiredToken(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	userID := uuid.New()
	tenantID := uuid.New()

	claims, err := auth.NewClaims(userID, tenantID, "viewer", time.Now().Add(-2*time.Minute), 1*time.Minute, uuid.Nil)
	require.NoError(t, err)

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)

	handler := AuthJWT(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthJWT_wrongSecret(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()

	token, err := auth.GenerateAccessToken(userID, tenantID, "owner", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 15*time.Minute, uuid.Nil)
	require.NoError(t, err)

	handler := AuthJWT("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
