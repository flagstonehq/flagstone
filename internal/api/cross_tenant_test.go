package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomas-vilte/flagstone/internal/auth"
	"github.com/thomas-vilte/flagstone/internal/storage"
)

// crossTenantFixture builds two tenants with their own project, env, flag,
// segment, and api key. Returns user A's admin token (scoped to tenant A)
// plus the slugs/keys/IDs of resources that belong to tenant B. Tests then
// try to access the B resources using token A and expect 404 — the resource
// "does not exist" from A's point of view, which is the contract we want
// (no enumeration via 403 vs 404).
type crossTenantFixture struct {
	tokenA      string
	tenantBSlug string

	// Tenant B resources — these must NOT be reachable with tokenA.
	projectBSlug string
	envBSlug     string
	flagBKey     string
	segmentBKey  string
	apiKeyBID    uuid.UUID
}

func seedCrossTenantFixture(t *testing.T) crossTenantFixture {
	t.Helper()
	ctx := context.Background()

	tenantA := &storage.Tenant{Slug: "xt-a-" + uuid.New().String()[:8], Name: "Tenant A", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(ctx, tenantA))
	tenantB := &storage.Tenant{Slug: "xt-b-" + uuid.New().String()[:8], Name: "Tenant B", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(ctx, tenantB))

	userA := &storage.User{Email: "xt-a-" + uuid.New().String()[:6] + "@example.com"}
	require.NoError(t, testServer.stores.Users.Create(ctx, userA))
	require.NoError(t, testServer.stores.Members.Add(ctx,
		&storage.TenantMember{TenantID: tenantA.ID, UserID: userA.ID, Role: "admin"}))

	tokenA, err := auth.GenerateAccessToken(userA.ID, tenantA.ID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL)
	require.NoError(t, err)

	projectB := &storage.Project{TenantID: tenantB.ID, Slug: "xt-proj-b", Name: "Project B"}
	require.NoError(t, testServer.stores.Projects.Create(ctx, projectB))

	envB := &storage.Environment{ProjectID: projectB.ID, Slug: "production", Name: "Production"}
	require.NoError(t, testServer.stores.Environments.Create(ctx, envB))

	flagB := &storage.Flag{
		ProjectID:    projectB.ID,
		Key:          "xt-flag-b",
		Name:         "Flag B",
		Type:         "boolean",
		DefaultValue: []byte("false"),
	}
	require.NoError(t, testServer.stores.Flags.Create(ctx, flagB))

	segmentB := &storage.Segment{
		ProjectID: projectB.ID,
		Key:       "xt-seg-b",
		Name:      "Segment B",
		Rules:     []byte("[]"),
	}
	require.NoError(t, testServer.stores.Segments.Create(ctx, segmentB))

	apiKeyB := &storage.APIKey{
		EnvironmentID: envB.ID,
		Name:          "xt-key-b",
		KeyHash:       strings.Repeat("a", 64),
		KeyPrefix:     "fs_test_a",
	}
	require.NoError(t, testServer.stores.APIKeys.Create(ctx, apiKeyB))

	return crossTenantFixture{
		tokenA:       tokenA,
		tenantBSlug:  tenantB.Slug,
		projectBSlug: projectB.Slug,
		envBSlug:     envB.Slug,
		flagBKey:     flagB.Key,
		segmentBKey:  segmentB.Key,
		apiKeyBID:    apiKeyB.ID,
	}
}

// expect404 asserts that user A (token scoped to tenant A) cannot see the
// given path. The handlers look up by (tenantID, slug/key) so foreign
// resources should respond with 404 NOT_FOUND, not 200 or 403.
//
// body is optional — pass "" for GET/DELETE, or a JSON string for PUT/POST.
// When set, Content-Type is added so the body-required middleware lets the
// request reach the handler and we observe the real 404 instead of 415.
func expect404(t *testing.T, token, method, path, body string) {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	var req *http.Request
	if reader != nil {
		req = httptest.NewRequest(method, path, reader)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code, "%s %s should be 404 for foreign tenant", method, path)
}

func TestCrossTenant_Environment_NotVisible(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "api_keys", "flags", "segments", "projects", "tenant_members", "users", "tenants")
	fx := seedCrossTenantFixture(t)

	expect404(t, fx.tokenA, http.MethodGet, "/api/v1/projects/"+fx.projectBSlug+"/environments", "")
}

func TestCrossTenant_Flag_NotVisible(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "api_keys", "flags", "segments", "projects", "tenant_members", "users", "tenants")
	fx := seedCrossTenantFixture(t)

	expect404(t, fx.tokenA, http.MethodGet, "/api/v1/projects/"+fx.projectBSlug+"/flags", "")
	expect404(t, fx.tokenA, http.MethodGet, "/api/v1/projects/"+fx.projectBSlug+"/flags/"+fx.flagBKey, "")
}

func TestCrossTenant_Segment_NotVisible(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "api_keys", "flags", "segments", "projects", "tenant_members", "users", "tenants")
	fx := seedCrossTenantFixture(t)

	expect404(t, fx.tokenA, http.MethodGet, "/api/v1/projects/"+fx.projectBSlug+"/segments", "")
	expect404(t, fx.tokenA, http.MethodGet, "/api/v1/projects/"+fx.projectBSlug+"/segments/"+fx.segmentBKey, "")
}

func TestCrossTenant_APIKey_NotVisible(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "api_keys", "flags", "segments", "projects", "tenant_members", "users", "tenants")
	fx := seedCrossTenantFixture(t)

	expect404(t, fx.tokenA, http.MethodGet,
		"/api/v1/projects/"+fx.projectBSlug+"/environments/"+fx.envBSlug+"/apikeys", "")
}

// And the destructive operations — admin in tenant A must not be able to
// revoke / archive / update / delete a resource that belongs to tenant B.
func TestCrossTenant_NoWriteAcrossTenants(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "api_keys", "flags", "segments", "projects", "tenant_members", "users", "tenants")
	fx := seedCrossTenantFixture(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"archive foreign flag", http.MethodDelete, "/api/v1/projects/" + fx.projectBSlug + "/flags/" + fx.flagBKey, ""},
		{"archive foreign segment", http.MethodDelete, "/api/v1/projects/" + fx.projectBSlug + "/segments/" + fx.segmentBKey, ""},
		{"revoke foreign apikey", http.MethodDelete, "/api/v1/projects/" + fx.projectBSlug + "/environments/" + fx.envBSlug + "/apikeys/" + fx.apiKeyBID.String(), ""},
		{"update foreign project", http.MethodPut, "/api/v1/projects/" + fx.projectBSlug, `{"name":"hijacked"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expect404(t, fx.tokenA, c.method, c.path, c.body)
		})
	}
}
