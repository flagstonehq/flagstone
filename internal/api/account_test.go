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
	"github.com/flagstonehq/flagstone/internal/auth"
	"github.com/flagstonehq/flagstone/internal/storage"
)

func TestGetMe_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "api_keys", "tenant_members", "users", "tenants")

	tenantID, userID, _ := seedAuthUser(t)
	token, err := auth.GenerateAccessToken(userID, tenantID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, uuid.Nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp meResponse
	err = json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, userID, resp.ID)
	assert.Equal(t, "auth@example.com", resp.Email)
	assert.Equal(t, "admin", resp.Role)
	assert.False(t, resp.CreatedAt.IsZero())
}

func TestGetMe_NoAuth(t *testing.T) {
	skipIfNoDB(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetMe_InvalidToken(t *testing.T) {
	skipIfNoDB(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer invalidtoken")
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestChangePassword_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "api_keys", "tenant_members", "users", "tenants")

	tenantID, userID, password := seedAuthUser(t)

	session := &storage.Session{
		UserID:      userID,
		TenantID:    tenantID,
		RefreshHash: auth.HashRefreshToken("test-refresh-token"),
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour),
	}
	require.NoError(t, testServer.stores.Sessions.Create(context.Background(), session))

	token, err := auth.GenerateAccessToken(userID, tenantID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, session.ID)
	require.NoError(t, err)

	body := `{"current_password":"` + password + `","new_password":"newsecurepass456","confirm_password":"newsecurepass456"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/me/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)

	user, err := testServer.stores.Users.GetByID(context.Background(), userID)
	require.NoError(t, err)
	require.NotNil(t, user.PasswordHash)
	assert.NoError(t, auth.VerifyPassword(*user.PasswordHash, "newsecurepass456"))

	remaining, err := testServer.stores.Sessions.ListByUserID(context.Background(), userID)
	require.NoError(t, err)
	assert.Empty(t, remaining, "all sessions should be revoked after password change")

	var count int64
	err = testServer.dbPool.QueryRow(context.Background(), `SELECT COUNT(*) FROM audit_log WHERE action = 'auth.password_changed' AND tenant_id = $1`, tenantID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	tenantID, userID, _ := seedAuthUser(t)
	token, err := auth.GenerateAccessToken(userID, tenantID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, uuid.Nil)
	require.NoError(t, err)

	body := `{"current_password":"wrongpassword","new_password":"newsecurepass456","confirm_password":"newsecurepass456"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/me/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestChangePassword_MismatchedConfirm(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	tenantID, userID, password := seedAuthUser(t)
	token, err := auth.GenerateAccessToken(userID, tenantID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, uuid.Nil)
	require.NoError(t, err)

	body := `{"current_password":"` + password + `","new_password":"newsecurepass456","confirm_password":"different"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/me/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestChangePassword_WeakNew(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	tenantID, userID, password := seedAuthUser(t)
	token, err := auth.GenerateAccessToken(userID, tenantID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, uuid.Nil)
	require.NoError(t, err)

	body := `{"current_password":"` + password + `","new_password":"short","confirm_password":"short"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/me/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestChangePassword_NoAuth(t *testing.T) {
	skipIfNoDB(t)

	body := `{"current_password":"pw","new_password":"newsecurepass456","confirm_password":"newsecurepass456"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/me/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListSessions_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "api_keys", "tenant_members", "users", "tenants")

	tenantID, userID, _ := seedAuthUser(t)

	session1 := &storage.Session{
		UserID:      userID,
		TenantID:    tenantID,
		RefreshHash: auth.HashRefreshToken("session-one"),
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour),
	}
	require.NoError(t, testServer.stores.Sessions.Create(context.Background(), session1))

	session2 := &storage.Session{
		UserID:      userID,
		TenantID:    tenantID,
		RefreshHash: auth.HashRefreshToken("session-two"),
		ExpiresAt:   time.Now().UTC().Add(48 * time.Hour),
	}
	require.NoError(t, testServer.stores.Sessions.Create(context.Background(), session2))

	token, err := auth.GenerateAccessToken(userID, tenantID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, session1.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp []sessionResponse
	err = json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Len(t, resp, 2)

	var foundCurrent bool
	for _, s := range resp {
		if s.IsCurrent {
			assert.Equal(t, session1.ID, s.ID, "only session1 should be current")
			foundCurrent = true
		}
	}
	assert.True(t, foundCurrent, "one session should be marked as current")
}

func TestListSessions_FiltersExpired(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "api_keys", "tenant_members", "users", "tenants")

	tenantID, userID, _ := seedAuthUser(t)

	expired := &storage.Session{
		UserID:      userID,
		TenantID:    tenantID,
		RefreshHash: auth.HashRefreshToken("expired-session"),
		ExpiresAt:   time.Now().UTC().Add(-1 * time.Hour),
	}
	require.NoError(t, testServer.stores.Sessions.Create(context.Background(), expired))

	active := &storage.Session{
		UserID:      userID,
		TenantID:    tenantID,
		RefreshHash: auth.HashRefreshToken("active-session"),
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour),
	}
	require.NoError(t, testServer.stores.Sessions.Create(context.Background(), active))

	token, err := auth.GenerateAccessToken(userID, tenantID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, active.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp []sessionResponse
	err = json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Equal(t, active.ID, resp[0].ID)
	assert.True(t, resp[0].IsCurrent)
}

func TestListSessions_NoAuth(t *testing.T) {
	skipIfNoDB(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRevokeSession_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "api_keys", "tenant_members", "users", "tenants")

	tenantID, userID, _ := seedAuthUser(t)

	sessionToKeep := &storage.Session{
		UserID:      userID,
		TenantID:    tenantID,
		RefreshHash: auth.HashRefreshToken("keep-session"),
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour),
	}
	require.NoError(t, testServer.stores.Sessions.Create(context.Background(), sessionToKeep))

	sessionToRevoke := &storage.Session{
		UserID:      userID,
		TenantID:    tenantID,
		RefreshHash: auth.HashRefreshToken("revoke-session"),
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour),
	}
	require.NoError(t, testServer.stores.Sessions.Create(context.Background(), sessionToRevoke))

	token, err := auth.GenerateAccessToken(userID, tenantID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, sessionToKeep.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/sessions/"+sessionToRevoke.ID.String(), nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)

	_, err = testServer.stores.Sessions.GetByID(context.Background(), sessionToRevoke.ID)
	require.ErrorIs(t, err, storage.ErrNotFound, "revoked session should be deleted")

	kept, err := testServer.stores.Sessions.GetByID(context.Background(), sessionToKeep.ID)
	require.NoError(t, err)
	assert.Equal(t, sessionToKeep.ID, kept.ID, "other session should remain")

	var count int64
	err = testServer.dbPool.QueryRow(context.Background(), `SELECT COUNT(*) FROM audit_log WHERE action = 'auth.session_revoked' AND tenant_id = $1`, tenantID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestRevokeSession_NotOwned(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "api_keys", "tenant_members", "users", "tenants")

	tenantID, userID, _ := seedAuthUser(t)

	otherTenantID := uuid.New()
	otherUserID := uuid.New()
	otherHash, err := auth.HashPassword("otherpass123", testServer.cfg.BcryptCost)
	require.NoError(t, err)
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), &storage.Tenant{ID: otherTenantID, Slug: "other-tenant-" + uuid.New().String()[:8], Name: "Other", Plan: "free"}))
	require.NoError(t, testServer.stores.Users.Create(context.Background(), &storage.User{ID: otherUserID, Email: "other@example.com", PasswordHash: &otherHash}))

	otherSession := &storage.Session{
		UserID:      otherUserID,
		TenantID:    otherTenantID,
		RefreshHash: auth.HashRefreshToken("other-session"),
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour),
	}
	require.NoError(t, testServer.stores.Sessions.Create(context.Background(), otherSession))

	token, err := auth.GenerateAccessToken(userID, tenantID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, uuid.Nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/sessions/"+otherSession.ID.String(), nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code, "should not find another user's session")
}

func TestRevokeSession_Current(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "api_keys", "tenant_members", "users", "tenants")

	tenantID, userID, _ := seedAuthUser(t)

	session := &storage.Session{
		UserID:      userID,
		TenantID:    tenantID,
		RefreshHash: auth.HashRefreshToken("current-session"),
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour),
	}
	require.NoError(t, testServer.stores.Sessions.Create(context.Background(), session))

	token, err := auth.GenerateAccessToken(userID, tenantID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, session.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/sessions/"+session.ID.String(), nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRevokeSession_NoAuth(t *testing.T) {
	skipIfNoDB(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/sessions/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRevokeAllSessions_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "api_keys", "tenant_members", "users", "tenants")

	tenantID, userID, _ := seedAuthUser(t)

	for i := 0; i < 3; i++ {
		session := &storage.Session{
			UserID:      userID,
			TenantID:    tenantID,
			RefreshHash: auth.HashRefreshToken("all-session-" + uuid.New().String()),
			ExpiresAt:   time.Now().UTC().Add(24 * time.Hour),
		}
		require.NoError(t, testServer.stores.Sessions.Create(context.Background(), session))
	}

	token, err := auth.GenerateAccessToken(userID, tenantID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, uuid.Nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/sessions", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)

	remaining, err := testServer.stores.Sessions.ListByUserID(context.Background(), userID)
	require.NoError(t, err)
	assert.Empty(t, remaining)

	var count int64
	err = testServer.dbPool.QueryRow(context.Background(), `SELECT COUNT(*) FROM audit_log WHERE action = 'auth.sessions_revoked_all' AND tenant_id = $1`, tenantID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestRevokeAllSessions_NoAuth(t *testing.T) {
	skipIfNoDB(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/sessions", nil)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSetupStatus_NotInitialized(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Initialized bool   `json:"initialized"`
		Message     string `json:"message"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.False(t, resp.Initialized)
	assert.NotEmpty(t, resp.Message)
}

func TestSetupStatus_Initialized(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "tenant_members", "users", "tenants")

	require.NoError(t, testServer.stores.Tenants.Create(context.Background(),
		&storage.Tenant{Slug: "status-test-tenant", Name: "Status Test", Plan: "free"}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Initialized bool   `json:"initialized"`
		Message     string `json:"message"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Initialized)
}
