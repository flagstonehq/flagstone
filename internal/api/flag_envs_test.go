package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flagstonehq/flagstone/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedFlagWithEnv(t *testing.T) (projectSlug, flagKey, envSlug, token string) {
	t.Helper()

	_, _, projectID, pSlug, tok := seedProject(t)

	env := &storage.Environment{ProjectID: projectID, Slug: "prod", Name: "Production"}
	require.NoError(t, testServer.stores.Environments.Create(context.Background(), env))

	flag := &storage.Flag{
		ProjectID:    projectID,
		Key:          "test-flag",
		Name:         "Test Flag",
		Type:         "boolean",
		DefaultValue: json.RawMessage("false"),
	}
	require.NoError(t, testServer.stores.Flags.Create(context.Background(), flag))

	fe := &storage.FlagEnvironment{
		FlagID:        flag.ID,
		EnvironmentID: env.ID,
		Enabled:       false,
		Rules:         json.RawMessage("[]"),
	}
	require.NoError(t, testServer.stores.FlagEnvironments.Upsert(context.Background(), fe))

	return pSlug, flag.Key, env.Slug, tok
}

func TestGetFlagEnvironment_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	projectSlug, flagKey, envSlug, token := seedFlagWithEnv(t)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/projects/"+projectSlug+"/flags/"+flagKey+"/environments/"+envSlug, nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp flagEnvironmentResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, flagKey, "test-flag")
	assert.False(t, resp.Enabled)
	assert.Equal(t, json.RawMessage("[]"), resp.Rules)
	assert.Equal(t, int64(1), resp.Version)
	assert.NotEmpty(t, resp.CreatedAt)
	assert.NotEmpty(t, resp.UpdatedAt)
}

func TestGetFlagEnvironment_FlagNotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	projectSlug, _, envSlug, token := seedFlagWithEnv(t)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/projects/"+projectSlug+"/flags/nonexistent/environments/"+envSlug, nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetFlagEnvironment_EnvNotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	projectSlug, flagKey, _, token := seedFlagWithEnv(t)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/projects/"+projectSlug+"/flags/"+flagKey+"/environments/nonexistent", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpdateFlagEnvironment_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	projectSlug, flagKey, envSlug, token := seedFlagWithEnv(t)

	body := `{"enabled":true,"rules":[{"conditions":{"all":[{"attribute":"country","op":"eq","value":"ar"}]}}],"version":1}`
	req := httptest.NewRequest(http.MethodPut,
		"/api/v1/projects/"+projectSlug+"/flags/"+flagKey+"/environments/"+envSlug,
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp flagEnvironmentResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Enabled)
	assert.Equal(t, int64(2), resp.Version)
	assert.NotEmpty(t, resp.UpdatedAt)
}

func TestUpdateFlagEnvironment_VersionConflict(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	projectSlug, flagKey, envSlug, token := seedFlagWithEnv(t)

	body := `{"enabled":true,"rules":[],"version":999}`
	req := httptest.NewRequest(http.MethodPut,
		"/api/v1/projects/"+projectSlug+"/flags/"+flagKey+"/environments/"+envSlug,
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestUpdateFlagEnvironment_FlagNotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "projects", "tenant_members", "users", "tenants")

	projectSlug, _, envSlug, token := seedFlagWithEnv(t)

	body := `{"enabled":true,"rules":[],"version":1}`
	req := httptest.NewRequest(http.MethodPut,
		"/api/v1/projects/"+projectSlug+"/flags/nonexistent/environments/"+envSlug,
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
