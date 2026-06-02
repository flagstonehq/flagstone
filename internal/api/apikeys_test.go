package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/flagstonehq/flagstone/internal/auth"
	"github.com/flagstonehq/flagstone/internal/storage"
)

func seedEnv(t *testing.T) (projectSlug, envSlug string, projectID uuid.UUID, token string) {
	t.Helper()

	_, _, pid, pSlug, tok := seedProject(t)

	env := &storage.Environment{ProjectID: pid, Slug: "prod", Name: "Production"}
	require.NoError(t, testServer.stores.Environments.Create(context.Background(), env))

	return pSlug, env.Slug, pid, tok
}

func TestCreateAPIKey_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	projectSlug, envSlug, _, token := seedEnv(t)

	body := `{"name":"My SDK Key"}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/projects/"+projectSlug+"/environments/"+envSlug+"/apikeys",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var resp createAPIKeyResponseBody
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "My SDK Key", resp.Name)
	assert.NotEmpty(t, resp.Key, "raw key should be returned on creation")
	assert.True(t, strings.HasPrefix(resp.Key, "fs_"), "raw key should start with fs_")
	assert.NotEmpty(t, resp.KeyPrefix)
	assert.NotEmpty(t, resp.CreatedAt)
	assert.False(t, resp.CreatedAt.IsZero())
}

func TestCreateAPIKey_Validation(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	projectSlug, envSlug, _, token := seedEnv(t)

	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/projects/"+projectSlug+"/environments/"+envSlug+"/apikeys",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateAPIKey_WithHint(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	projectSlug, envSlug, _, token := seedEnv(t)

	body := `{"name":"Test Key","env_hint":"test"}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/projects/"+projectSlug+"/environments/"+envSlug+"/apikeys",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var resp createAPIKeyResponseBody
	json.NewDecoder(rec.Body).Decode(&resp)
	assert.True(t, strings.HasPrefix(resp.Key, "fs_test_"), "key should use test hint")
}

func TestListAPIKeys_NoKeys(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	projectSlug, envSlug, _, token := seedEnv(t)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/projects/"+projectSlug+"/environments/"+envSlug+"/apikeys", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp []apiKeyResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Empty(t, resp)
}

func TestListAPIKeys_ReturnsPrefixOnly(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	projectSlug, envSlug, _, token := seedEnv(t)

	createBody := `{"name":"My Key"}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/projects/"+projectSlug+"/environments/"+envSlug+"/apikeys",
		strings.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	req = httptest.NewRequest(http.MethodGet,
		"/api/v1/projects/"+projectSlug+"/environments/"+envSlug+"/apikeys", nil)
	req.Header.Set(authBearer(token))
	rec = httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp []apiKeyResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	require.Len(t, resp, 1)
	assert.NotEmpty(t, resp[0].KeyPrefix)
	assert.Equal(t, "My Key", resp[0].Name)
}

func TestRevokeAPIKey_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	projectSlug, envSlug, _, token := seedEnv(t)

	createBody := `{"name":"To Revoke"}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/projects/"+projectSlug+"/environments/"+envSlug+"/apikeys",
		strings.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created createAPIKeyResponseBody
	json.NewDecoder(rec.Body).Decode(&created)

	req = httptest.NewRequest(http.MethodDelete,
		"/api/v1/projects/"+projectSlug+"/environments/"+envSlug+"/apikeys/"+created.ID.String(), nil)
	req.Header.Set(authBearer(token))
	rec = httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	keys, err := testServer.stores.APIKeys.ListByEnvironment(context.Background(), created.EnvironmentID)
	require.NoError(t, err)
	for _, k := range keys {
		if k.ID == created.ID {
			assert.NotNil(t, k.RevokedAt, "key should be revoked")
			return
		}
	}
	t.Error("revoked key not found in list")
}

func TestRevokeAPIKey_NotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	projectSlug, envSlug, _, token := seedEnv(t)

	req := httptest.NewRequest(http.MethodDelete,
		"/api/v1/projects/"+projectSlug+"/environments/"+envSlug+"/apikeys/"+uuid.New().String(), nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPIKeys_CrossTenantIsolation(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	tenantA := &storage.Tenant{Slug: "ak-tenant-a-" + uuid.New().String()[:8], Name: "Tenant A", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenantA))
	tenantB := &storage.Tenant{Slug: "ak-tenant-b-" + uuid.New().String()[:8], Name: "Tenant B", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenantB))

	userA := &storage.User{Email: "ak-a@example.com"}
	require.NoError(t, testServer.stores.Users.Create(context.Background(), userA))
	member := &storage.TenantMember{TenantID: tenantA.ID, UserID: userA.ID, Role: "admin"}
	require.NoError(t, testServer.stores.Members.Add(context.Background(), member))

	tokenA, err := auth.GenerateAccessToken(userA.ID, tenantA.ID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, uuid.Nil)
	require.NoError(t, err)

	projA := &storage.Project{TenantID: tenantA.ID, Slug: "proj-a", Name: "Project A"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), projA))
	projB := &storage.Project{TenantID: tenantB.ID, Slug: "proj-b", Name: "Project B"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), projB))

	envA := &storage.Environment{ProjectID: projA.ID, Slug: "prod", Name: "Prod"}
	require.NoError(t, testServer.stores.Environments.Create(context.Background(), envA))

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/projects/proj-a/environments/prod/apikeys", nil)
	req.Header.Set(authBearer(tokenA))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet,
		"/api/v1/projects/proj-b/environments/prod/apikeys", nil)
	req.Header.Set(authBearer(tokenA))
	rec = httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code, "should not see another tenant's project")
}

func TestAPIKeys_RequiresAuth(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	endpoints := []struct {
		method, path, body string
	}{
		{http.MethodPost, "/api/v1/projects/test/environments/prod/apikeys", `{"name":"Key"}`},
		{http.MethodGet, "/api/v1/projects/test/environments/prod/apikeys", ""},
		{http.MethodDelete, "/api/v1/projects/test/environments/prod/apikeys/" + uuid.New().String(), ""},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var req *http.Request
			if ep.body != "" {
				req = httptest.NewRequest(ep.method, ep.path, strings.NewReader(ep.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(ep.method, ep.path, nil)
			}
			rec := httptest.NewRecorder()
			testServer.Routes().ServeHTTP(rec, req)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}
