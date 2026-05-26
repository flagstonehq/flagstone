package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomas-vilte/flagstone/internal/auth"
	"github.com/thomas-vilte/flagstone/internal/storage"
)

func seedAuthUser(t *testing.T) (tenantID, userID uuid.UUID, password string) {
	t.Helper()

	password = "securepass123"
	hash, err := auth.HashPassword(password, testServer.cfg.BcryptCost)
	require.NoError(t, err)

	tenant := &storage.Tenant{Slug: "auth-tenant-" + uuid.New().String()[:8], Name: "Auth Tenant", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenant))

	user := &storage.User{Email: "auth@example.com", PasswordHash: &hash}
	require.NoError(t, testServer.stores.Users.Create(context.Background(), user))

	member := &storage.TenantMember{TenantID: tenant.ID, UserID: user.ID, Role: "admin"}
	require.NoError(t, testServer.stores.Members.Add(context.Background(), member))

	return tenant.ID, user.ID, password
}

func seedSession(t *testing.T, tenantID, userID uuid.UUID) (rawToken string) {
	t.Helper()

	rawToken, refreshHash, err := auth.GenerateRefreshToken(32)
	require.NoError(t, err)

	session := &storage.Session{
		UserID:      userID,
		TenantID:    tenantID,
		RefreshHash: refreshHash,
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour),
	}
	require.NoError(t, testServer.stores.Sessions.Create(context.Background(), session))

	return rawToken
}

func TestLogin_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	_, _, password := seedAuthUser(t)

	body := `{"email":"auth@example.com","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp tokenResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.Equal(t, "Bearer", resp.TokenType)
	assert.Greater(t, resp.ExpiresIn, int64(0))

	cookies := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	require.NotNil(t, refreshCookie)
	assert.True(t, refreshCookie.HttpOnly)
	assert.Equal(t, "/api/v1/auth", refreshCookie.Path)
}

func TestLogin_InvalidCredentials(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	seedAuthUser(t)

	tests := []struct {
		name        string
		body        string
		wantStatus  int
		wantErrCode string
	}{
		{
			name:        "wrong password",
			body:        `{"email":"auth@example.com","password":"wrongpass"}`,
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "INVALID_CREDENTIALS",
		},
		{
			name:        "unknown email",
			body:        `{"email":"unknown@example.com","password":"securepass123"}`,
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "INVALID_CREDENTIALS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			testServer.Routes().ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			var errResp struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			json.NewDecoder(rec.Body).Decode(&errResp)
			assert.Equal(t, tt.wantErrCode, errResp.Error.Code)
		})
	}
}

func TestLogin_ValidationErrors(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	tests := []struct {
		name string
		body string
	}{
		{
			name: "empty email",
			body: `{"email":"","password":"securepass123"}`,
		},
		{
			name: "invalid email",
			body: `{"email":"not-an-email","password":"securepass123"}`,
		},
		{
			name: "empty password",
			body: `{"email":"auth@example.com","password":""}`,
		},
		{
			name: "empty JSON",
			body: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			testServer.Routes().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)

			var errResp struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			json.NewDecoder(rec.Body).Decode(&errResp)
			assert.Equal(t, "VALIDATION_ERROR", errResp.Error.Code)
		})
	}
}

func TestLogin_WrongContentType(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	body := `{"email":"auth@example.com","password":"securepass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
}

func TestLogin_LargeBody(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	largePayload := strings.Repeat("a", 2*1024*1024)
	body := `{"email":"auth@example.com","password":"` + largePayload + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestLogin_GET_returnsMethodNotAllowed(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestRefresh_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	tenantID, userID, _ := seedAuthUser(t)
	rawRefresh := seedSession(t, tenantID, userID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: rawRefresh})
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp tokenResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.Equal(t, "Bearer", resp.TokenType)
	assert.Greater(t, resp.ExpiresIn, int64(0))

	cookies := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	require.NotNil(t, refreshCookie)
	assert.NotEmpty(t, refreshCookie.Value)
}

func TestRefresh_MissingCookie(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRefresh_ExpiredSession(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	tenantID, userID, _ := seedAuthUser(t)

	rawRefresh, refreshHash, err := auth.GenerateRefreshToken(32)
	require.NoError(t, err)

	session := &storage.Session{
		UserID:      userID,
		TenantID:    tenantID,
		RefreshHash: refreshHash,
		ExpiresAt:   time.Now().UTC().Add(-1 * time.Hour),
	}
	require.NoError(t, testServer.stores.Sessions.Create(context.Background(), session))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: rawRefresh})
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestLogout_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	tenantID, userID, _ := seedAuthUser(t)
	rawRefresh := seedSession(t, tenantID, userID)

	accessToken, err := auth.GenerateAccessToken(userID, tenantID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: rawRefresh})
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	cookies := rec.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			assert.Empty(t, c.Value)
			assert.Equal(t, -1, c.MaxAge)
		}
	}
}

func TestLogout_NoAuth(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
