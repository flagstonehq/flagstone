package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flagstonehq/flagstone/internal/auth"
	"github.com/flagstonehq/flagstone/internal/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateFlag_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)

	env := &storage.Environment{ProjectID: projectID, Slug: "prod", Name: "Production"}
	require.NoError(t, testServer.stores.Environments.Create(context.Background(), env))

	body := `{"key":"my-feature","name":"My Feature","type":"boolean","default_value":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectSlug+"/flags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var resp flagResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "my-feature", resp.Key)
	assert.Equal(t, "My Feature", resp.Name)
	assert.Equal(t, "boolean", resp.Type)
	assert.Equal(t, json.RawMessage("false"), resp.DefaultValue)
	assert.False(t, resp.CreatedAt.IsZero())

	var feCount int
	err = testPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM flag_environments WHERE flag_id = $1", resp.ID).Scan(&feCount)
	require.NoError(t, err)
	assert.Equal(t, 1, feCount)
}

func TestCreateFlag_DuplicateKey(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)

	flag := &storage.Flag{
		ProjectID:    projectID,
		Key:          "dup-key",
		Name:         "Original",
		Type:         "boolean",
		DefaultValue: json.RawMessage("false"),
	}
	require.NoError(t, testServer.stores.Flags.Create(context.Background(), flag))

	body := `{"key":"dup-key","name":"Duplicate","type":"boolean","default_value":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectSlug+"/flags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestCreateFlag_ProjectNotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	_, _, _, _, token := seedProject(t)

	body := `{"key":"my-feature","name":"My Feature","type":"boolean","default_value":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/nonexistent/flags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCreateFlag_ValidationErrors(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	_, _, _, projectSlug, token := seedProject(t)

	tests := []struct {
		name string
		body string
	}{
		{"empty key", `{"key":"","name":"My Feature","type":"boolean","default_value":false}`},
		{"empty name", `{"key":"my-feature","name":"","type":"boolean","default_value":false}`},
		{"invalid type", `{"key":"my-feature","name":"My Feature","type":"invalid","default_value":false}`},
		{"invalid json default", `{"key":"my-feature","name":"My Feature","type":"boolean","default_value":notjson}`},
		{"key too long", `{"key":"` + strings.Repeat("a", 129) + `","name":"My Feature","type":"boolean","default_value":false}`},
		{"key with uppercase", `{"key":"MY-FEATURE","name":"My Feature","type":"boolean","default_value":false}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectSlug+"/flags", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(authBearer(token))
			rec := httptest.NewRecorder()

			testServer.Routes().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestListFlags_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)

	for i := 0; i < 3; i++ {
		flag := &storage.Flag{
			ProjectID:    projectID,
			Key:          "flag-" + uuid.New().String()[:8],
			Name:         "Flag",
			Type:         "boolean",
			DefaultValue: json.RawMessage("false"),
		}
		require.NoError(t, testServer.stores.Flags.Create(context.Background(), flag))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectSlug+"/flags", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp []flagResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Len(t, resp, 3)
}

func TestGetFlag_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)

	flag := &storage.Flag{
		ProjectID:    projectID,
		Key:          "my-feature",
		Name:         "My Feature",
		Type:         "boolean",
		DefaultValue: json.RawMessage("false"),
	}
	require.NoError(t, testServer.stores.Flags.Create(context.Background(), flag))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectSlug+"/flags/my-feature", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp flagResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, flag.ID, resp.ID)
	assert.Equal(t, "my-feature", resp.Key)
	assert.Equal(t, "boolean", resp.Type)
}

func TestGetFlag_NotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	_, _, _, projectSlug, token := seedProject(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectSlug+"/flags/nonexistent", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpdateFlag_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)

	flag := &storage.Flag{
		ProjectID:    projectID,
		Key:          "my-feature",
		Name:         "My Feature",
		Type:         "boolean",
		DefaultValue: json.RawMessage("false"),
	}
	require.NoError(t, testServer.stores.Flags.Create(context.Background(), flag))

	body := `{"name":"Updated Feature","type":"string"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectSlug+"/flags/my-feature", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp flagResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "my-feature", resp.Key, "key should remain unchanged")
	assert.Equal(t, "Updated Feature", resp.Name)
	assert.Equal(t, "string", resp.Type)
	assert.Equal(t, json.RawMessage("false"), resp.DefaultValue)
}

func TestUpdateFlag_NotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	_, _, _, projectSlug, token := seedProject(t)

	body := `{"name":"Updated Feature"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectSlug+"/flags/nonexistent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestArchiveFlag_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)

	flag := &storage.Flag{
		ProjectID:    projectID,
		Key:          "my-feature",
		Name:         "My Feature",
		Type:         "boolean",
		DefaultValue: json.RawMessage("false"),
	}
	require.NoError(t, testServer.stores.Flags.Create(context.Background(), flag))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectSlug+"/flags/my-feature", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	_, err := testServer.stores.Flags.GetByKey(context.Background(), projectID, "my-feature")
	assert.ErrorIs(t, err, storage.ErrFlagNotFound)
}

func TestArchiveFlag_NotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	_, _, _, projectSlug, token := seedProject(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectSlug+"/flags/nonexistent", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestFlags_CrossTenantIsolation(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	tenantA := &storage.Tenant{Slug: "flag-tenant-a-" + uuid.New().String()[:8], Name: "Tenant A", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenantA))
	tenantB := &storage.Tenant{Slug: "flag-tenant-b-" + uuid.New().String()[:8], Name: "Tenant B", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenantB))

	userA := &storage.User{Email: "flag-a@example.com"}
	require.NoError(t, testServer.stores.Users.Create(context.Background(), userA))
	member := &storage.TenantMember{TenantID: tenantA.ID, UserID: userA.ID, Role: "admin"}
	require.NoError(t, testServer.stores.Members.Add(context.Background(), member))

	tokenA, err := auth.GenerateAccessToken(userA.ID, tenantA.ID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, uuid.Nil)
	require.NoError(t, err)

	projA := &storage.Project{TenantID: tenantA.ID, Slug: "proj-a", Name: "Project A"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), projA))
	projB := &storage.Project{TenantID: tenantB.ID, Slug: "proj-b", Name: "Project B"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), projB))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-a/flags", nil)
	req.Header.Set(authBearer(tokenA))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-b/flags", nil)
	req.Header.Set(authBearer(tokenA))
	rec = httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code, "should not see another tenant's project")
}

func TestFlags_RequiresAuth(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	endpoints := []struct {
		method, path, body string
	}{
		{http.MethodPost, "/api/v1/projects/test/flags", `{"key":"flag","name":"Flag","type":"boolean","default_value":false}`},
		{http.MethodGet, "/api/v1/projects/test/flags", ""},
		{http.MethodGet, "/api/v1/projects/test/flags/my-key", ""},
		{http.MethodPut, "/api/v1/projects/test/flags/my-key", `{"name":"Updated"}`},
		{http.MethodDelete, "/api/v1/projects/test/flags/my-key", ""},
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
