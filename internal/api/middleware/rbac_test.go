package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomas-vilte/flagstone/internal/auth"
)

func TestRequireRole_allowed(t *testing.T) {
	handler := RequireRole(auth.RoleMember)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	claims := &auth.Claims{
		TenantID: uuid.New().String(),
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: uuid.New().String(),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	ctx := WithClaims(req.Context(), claims)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req.WithContext(ctx))

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireRole_exactMatch(t *testing.T) {
	handler := RequireRole(auth.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	claims := &auth.Claims{
		TenantID: uuid.New().String(),
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: uuid.New().String(),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	ctx := WithClaims(req.Context(), claims)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req.WithContext(ctx))

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireRole_denied(t *testing.T) {
	handler := RequireRole(auth.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	claims := &auth.Claims{
		TenantID: uuid.New().String(),
		Role:     "viewer",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: uuid.New().String(),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	ctx := WithClaims(req.Context(), claims)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req.WithContext(ctx))

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var body ErrorResponse
	err := json.NewDecoder(rec.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "INSUFFICIENT_PERMISSIONS", body.Error.Code)
}

func TestRequireRole_noClaims(t *testing.T) {
	handler := RequireRole(auth.RoleViewer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestRequireRole_emptyContext(t *testing.T) {
	handler := RequireRole(auth.RoleViewer)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req.WithContext(context.Background()))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
