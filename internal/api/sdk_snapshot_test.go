package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flagstonehq/flagstone/internal/auth"
	"github.com/flagstonehq/flagstone/internal/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedSnapshotEnv(t *testing.T) (projectID, envID uuid.UUID, rawKey string) {
	t.Helper()

	_, _, pid, _, _ := seedProject(t)

	env := &storage.Environment{ProjectID: pid, Slug: "prod", Name: "Production"}
	require.NoError(t, testServer.stores.Environments.Create(context.Background(), env))

	raw, hash, prefix, err := auth.GenerateAPIKey("live", 32)
	require.NoError(t, err)
	ak := &storage.APIKey{
		EnvironmentID: env.ID,
		Name:          "snapshot test key",
		KeyHash:       hash,
		KeyPrefix:     prefix,
	}
	require.NoError(t, testServer.stores.APIKeys.Create(context.Background(), ak))
	return pid, env.ID, raw
}

func TestSDKSnapshot_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flag_environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	pid, envID, rawKey := seedSnapshotEnv(t)

	flag := &storage.Flag{
		ProjectID:    pid,
		Key:          "new-checkout",
		Name:         "New Checkout",
		Type:         "boolean",
		DefaultValue: json.RawMessage("false"),
	}
	require.NoError(t, testServer.stores.Flags.Create(context.Background(), flag))
	fe := &storage.FlagEnvironment{
		FlagID:        flag.ID,
		EnvironmentID: envID,
		Enabled:       true,
		Rules:         json.RawMessage(`[{"conditions":{"attribute":"plan","op":"eq","value":"premium"},"value":true}]`),
	}
	require.NoError(t, testServer.stores.FlagEnvironments.Upsert(context.Background(), fe))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sdk/snapshot", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp snapshotResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "prod", resp.Environment)
	assert.Contains(t, resp.Flags, "new-checkout")
	fc := resp.Flags["new-checkout"]
	assert.True(t, fc.Enabled)
	assert.Equal(t, "boolean", fc.FlagType)
	assert.Equal(t, "new-checkout", fc.Key)
	assert.Len(t, fc.Rules, 1)
	assert.NotZero(t, resp.FetchedAt)
}

func TestSDKSnapshot_NoAuth(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flag_environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sdk/snapshot", nil)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSDKSnapshot_InvalidKey(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flag_environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sdk/snapshot", nil)
	req.Header.Set("Authorization", "Bearer fs_live_does_not_exist")
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSDKSnapshot_EmptyEnvironment(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flag_environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	_, _, rawKey := seedSnapshotEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sdk/snapshot", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp snapshotResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "prod", resp.Environment)
	assert.Empty(t, resp.Flags)
	assert.Empty(t, resp.Segments)
}

func TestSDKSnapshot_IncludesSegments(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "api_keys", "environments", "flag_environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	pid, _, rawKey := seedSnapshotEnv(t)

	seg := &storage.Segment{
		ProjectID: pid,
		Key:       "power-users",
		Name:      "Power Users",
		Rules:     json.RawMessage(`{"attribute":"plan","op":"eq","value":"premium"}`),
	}
	require.NoError(t, testServer.stores.Segments.Create(context.Background(), seg))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sdk/snapshot", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp snapshotResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "prod", resp.Environment)
	assert.Contains(t, resp.Segments, "power-users")
}
