package api

import (
	"context"
	"encoding/json"
	"fmt"
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

func seedAuditData(t *testing.T) (tenantID uuid.UUID, token string) {
	t.Helper()
	password := "securepass123"
	hash, err := auth.HashPassword(password, testServer.cfg.BcryptCost)
	require.NoError(t, err)

	tenant := &storage.Tenant{Slug: "audit-tenant-" + uuid.New().String()[:8], Name: "Audit Tenant", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenant))

	user := &storage.User{Email: "audit-" + uuid.New().String()[:6] + "@example.com", PasswordHash: &hash}
	require.NoError(t, testServer.stores.Users.Create(context.Background(), user))

	member := &storage.TenantMember{TenantID: tenant.ID, UserID: user.ID, Role: "viewer"}
	require.NoError(t, testServer.stores.Members.Add(context.Background(), member))

	accessToken, err := auth.GenerateAccessToken(user.ID, tenant.ID, "viewer", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL)
	require.NoError(t, err)

	return tenant.ID, accessToken
}

func TestAuditLog_QueryNoFilters(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "apikeys", "tenant_members", "users", "tenants")

	tenantID, token := seedAuditData(t)

	for i := 0; i < 3; i++ {
		entry := &storage.AuditLogEntry{
			TenantID:     tenantID,
			ActorType:    "user",
			Action:       "flag.created",
			ResourceType: "flag",
		}
		require.NoError(t, testServer.stores.AuditLogs.Insert(context.Background(), entry))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp auditLogResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, int64(3), resp.Total)
	assert.Len(t, resp.Entries, 3)
}

func TestAuditLog_QueryWithFilters(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "apikeys", "tenant_members", "users", "tenants")

	tenantID, token := seedAuditData(t)
	now := time.Now().UTC()

	resourceID := uuid.New()
	entries := []*storage.AuditLogEntry{
		{TenantID: tenantID, ActorType: "user", Action: "flag.created", ResourceType: "flag", ResourceID: &resourceID, CreatedAt: now.Add(-2 * time.Hour)},
		{TenantID: tenantID, ActorType: "user", Action: "flag.updated", ResourceType: "flag", CreatedAt: now.Add(-1 * time.Hour)},
		{TenantID: tenantID, ActorType: "system", Action: "auth.login", ResourceType: "session", CreatedAt: now},
	}
	for _, e := range entries {
		require.NoError(t, testServer.stores.AuditLogs.Insert(context.Background(), e))
	}

	tests := []struct {
		name       string
		query      string
		wantTotal  int64
		wantAction string
	}{
		{"filter by action", "?action=flag.created", 1, "flag.created"},
		{"filter by actor_type", "?actor_type=system", 1, "auth.login"},
		{"filter by resource_id", fmt.Sprintf("?resource_id=%s", resourceID.String()), 1, "flag.created"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/audit"+tt.query, nil)
			req.Header.Set(authBearer(token))
			rec := httptest.NewRecorder()
			testServer.Routes().ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)

			var resp auditLogResponse
			err := json.NewDecoder(rec.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTotal, resp.Total)
			if tt.wantAction != "" && len(resp.Entries) > 0 {
				assert.Equal(t, tt.wantAction, resp.Entries[0].Action)
			}
		})
	}
}

func TestAuditLog_Pagination(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "apikeys", "tenant_members", "users", "tenants")

	tenantID, token := seedAuditData(t)

	for i := 0; i < 5; i++ {
		entry := &storage.AuditLogEntry{
			TenantID:     tenantID,
			ActorType:    "user",
			Action:       "flag.created",
			ResourceType: "flag",
		}
		require.NoError(t, testServer.stores.AuditLogs.Insert(context.Background(), entry))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?limit=2&offset=1", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp auditLogResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, int64(5), resp.Total)
	assert.Equal(t, 2, resp.Limit)
	assert.Equal(t, 1, resp.Offset)
	assert.Len(t, resp.Entries, 2)
}

func TestAuditLog_CrossTenant(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "apikeys", "tenant_members", "users", "tenants")

	tenantA, tokenA := seedAuditData(t)
	tenantB, tokenB := seedAuditData(t)

	entryA := &storage.AuditLogEntry{TenantID: tenantA, ActorType: "user", Action: "flag.created", ResourceType: "flag"}
	require.NoError(t, testServer.stores.AuditLogs.Insert(context.Background(), entryA))

	entryB := &storage.AuditLogEntry{TenantID: tenantB, ActorType: "user", Action: "flag.created", ResourceType: "flag"}
	require.NoError(t, testServer.stores.AuditLogs.Insert(context.Background(), entryB))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	req.Header.Set(authBearer(tokenA))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp auditLogResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
	assert.Len(t, resp.Entries, 1)
	_ = tokenB
}

func TestAuditLog_NoAuth(t *testing.T) {
	skipIfNoDB(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuditLog_MutationsWriteEntries(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "api_keys", "tenant_members", "users", "tenants")

	hash, err := auth.HashPassword("securepass123", testServer.cfg.BcryptCost)
	require.NoError(t, err)

	tenant := &storage.Tenant{Slug: "audit-mut-" + uuid.New().String()[:8], Name: "Audit Mut", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenant))

	user := &storage.User{Email: "audit-mut@example.com", PasswordHash: &hash}
	require.NoError(t, testServer.stores.Users.Create(context.Background(), user))

	require.NoError(t, testServer.stores.Members.Add(context.Background(),
		&storage.TenantMember{TenantID: tenant.ID, UserID: user.ID, Role: "admin"}))

	adminToken, err := auth.GenerateAccessToken(user.ID, tenant.ID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL)
	require.NoError(t, err)

	projBody := `{"slug":"audit-proj","name":"Audit Project"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(projBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(adminToken))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var projResp projectResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&projResp))

	flagBody := `{"key":"audit-flag","name":"Audit Flag","type":"boolean","default_value":false}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects/audit-proj/flags", strings.NewReader(flagBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(adminToken))
	rec = httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	req.Header.Set(authBearer(adminToken))
	rec = httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var auditResp auditLogResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&auditResp))

	actions := make([]string, 0, len(auditResp.Entries))
	for _, e := range auditResp.Entries {
		actions = append(actions, e.Action)
	}
	assert.Contains(t, actions, "project.created", "project.created should be in audit log")
	assert.Contains(t, actions, "flag.created", "flag.created should be in audit log")

	req = httptest.NewRequest(http.MethodGet, "/api/v1/audit?resource_type=flag", nil)
	req.Header.Set(authBearer(adminToken))
	rec = httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var filtered auditLogResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&filtered))
	assert.Equal(t, int64(1), filtered.Total)
	assert.Equal(t, "flag.created", filtered.Entries[0].Action)
	assert.NotNil(t, filtered.Entries[0].ResourceID)
}
