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
	"github.com/thomas-vilte/flagstone/internal/auth"
	"github.com/thomas-vilte/flagstone/internal/storage"
)

func seedUserWithRole(t *testing.T, role string) (tenantID, userID uuid.UUID, token string) {
	t.Helper()

	tenant := &storage.Tenant{Slug: "rbac-tenant-" + uuid.New().String()[:8], Name: "RBAC Tenant", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenant))

	user := &storage.User{Email: "rbac-" + uuid.New().String()[:8] + "@example.com"}
	require.NoError(t, testServer.stores.Users.Create(context.Background(), user))

	member := &storage.TenantMember{TenantID: tenant.ID, UserID: user.ID, Role: role}
	require.NoError(t, testServer.stores.Members.Add(context.Background(), member))

	accessToken, err := auth.GenerateAccessToken(user.ID, tenant.ID, role, testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL)
	require.NoError(t, err)

	return tenant.ID, user.ID, accessToken
}

func TestRBAC_ViewerCantCreateFlags(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "api_keys", "projects", "tenant_members", "users", "tenants")

	_, _, token := seedUserWithRole(t, auth.RoleViewer.String())

	project := &storage.Project{TenantID: uuid.Nil, Slug: "test-proj", Name: "Test"}
	project.TenantID, _, _ = seedUserWithRole(t, auth.RoleAdmin.String())
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))

	body := `{"key":"my-flag","name":"My Flag","type":"boolean","default_value":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.Slug+"/flags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRBAC_ViewerCantCreateSegments(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "api_keys", "projects", "tenant_members", "users", "tenants")

	_, _, token := seedUserWithRole(t, auth.RoleViewer.String())

	project := &storage.Project{TenantID: uuid.Nil, Slug: "test-proj", Name: "Test"}
	project.TenantID, _, _ = seedUserWithRole(t, auth.RoleAdmin.String())
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))

	body := `{"key":"my-segment","name":"My Segment","rules":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+project.Slug+"/segments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRBAC_ViewerCantCreateAPIKeys(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "api_keys", "projects", "tenant_members", "users", "tenants")

	_, _, token := seedUserWithRole(t, auth.RoleViewer.String())

	adminTenantID, _, adminToken := seedUserWithRole(t, auth.RoleAdmin.String())
	project := &storage.Project{TenantID: adminTenantID, Slug: "test-proj", Name: "Test"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))
	env := &storage.Environment{ProjectID: project.ID, Slug: "prod", Name: "Production"}
	require.NoError(t, testServer.stores.Environments.Create(context.Background(), env))

	body := `{"name":"My Key"}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/projects/"+project.Slug+"/environments/"+env.Slug+"/apikeys",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	_ = adminToken
}

func TestRBAC_MemberCantArchiveFlags(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "api_keys", "projects", "tenant_members", "users", "tenants")

	tenantID, _, memberToken := seedUserWithRole(t, auth.RoleMember.String())

	project := &storage.Project{TenantID: tenantID, Slug: "test-proj", Name: "Test"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))

	flag := &storage.Flag{ProjectID: project.ID, Key: "my-flag", Name: "My Flag", Type: "boolean", DefaultValue: json.RawMessage("false")}
	require.NoError(t, testServer.stores.Flags.Create(context.Background(), flag))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+project.Slug+"/flags/my-flag", nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRBAC_MemberCantArchiveSegments(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "api_keys", "projects", "tenant_members", "users", "tenants")

	tenantID, _, memberToken := seedUserWithRole(t, auth.RoleMember.String())

	project := &storage.Project{TenantID: tenantID, Slug: "test-proj", Name: "Test"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))

	seg := &storage.Segment{ProjectID: project.ID, Key: "my-seg", Name: "My Seg", Rules: json.RawMessage("[]")}
	require.NoError(t, testServer.stores.Segments.Create(context.Background(), seg))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+project.Slug+"/segments/my-seg", nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRBAC_MemberCantRevokeAPIKeys(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "api_keys", "projects", "tenant_members", "users", "tenants")

	tenantID, _, memberToken := seedUserWithRole(t, auth.RoleMember.String())

	project := &storage.Project{TenantID: tenantID, Slug: "test-proj", Name: "Test"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))
	env := &storage.Environment{ProjectID: project.ID, Slug: "prod", Name: "Production"}
	require.NoError(t, testServer.stores.Environments.Create(context.Background(), env))

	req := httptest.NewRequest(http.MethodDelete,
		"/api/v1/projects/"+project.Slug+"/environments/"+env.Slug+"/apikeys/"+uuid.New().String(), nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRBAC_AdminCanCreateEverywhere(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "api_keys", "projects", "tenant_members", "users", "tenants")

	tenantID, _, adminToken := seedUserWithRole(t, auth.RoleAdmin.String())

	project := &storage.Project{TenantID: tenantID, Slug: "test-proj", Name: "Test"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))
	env := &storage.Environment{ProjectID: project.ID, Slug: "prod", Name: "Production"}
	require.NoError(t, testServer.stores.Environments.Create(context.Background(), env))

	tests := []struct {
		name string
		req  *http.Request
	}{
		{"create flag", httptest.NewRequest(http.MethodPost,
			"/api/v1/projects/"+project.Slug+"/flags",
			strings.NewReader(`{"key":"f1","name":"F1","type":"boolean","default_value":false}`))},
		{"create segment", httptest.NewRequest(http.MethodPost,
			"/api/v1/projects/"+project.Slug+"/segments",
			strings.NewReader(`{"key":"s1","name":"S1","rules":[]}`))},
		{"create apikey", httptest.NewRequest(http.MethodPost,
			"/api/v1/projects/"+project.Slug+"/environments/"+env.Slug+"/apikeys",
			strings.NewReader(`{"name":"K1"}`))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.Header.Set("Content-Type", "application/json")
			tt.req.Header.Set("Authorization", "Bearer "+adminToken)
			rec := httptest.NewRecorder()
			testServer.Routes().ServeHTTP(rec, tt.req)
			assert.Equal(t, http.StatusCreated, rec.Code)
		})
	}
}

func TestRBAC_AdminCanArchiveAndRevoke(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "api_keys", "projects", "tenant_members", "users", "tenants")

	tenantID, _, adminToken := seedUserWithRole(t, auth.RoleAdmin.String())

	project := &storage.Project{TenantID: tenantID, Slug: "test-proj", Name: "Test"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))
	env := &storage.Environment{ProjectID: project.ID, Slug: "prod", Name: "Production"}
	require.NoError(t, testServer.stores.Environments.Create(context.Background(), env))

	flag := &storage.Flag{ProjectID: project.ID, Key: "my-flag", Name: "F", Type: "boolean", DefaultValue: json.RawMessage("false")}
	require.NoError(t, testServer.stores.Flags.Create(context.Background(), flag))

	seg := &storage.Segment{ProjectID: project.ID, Key: "my-seg", Name: "S", Rules: json.RawMessage("[]")}
	require.NoError(t, testServer.stores.Segments.Create(context.Background(), seg))

	apikey := &storage.APIKey{EnvironmentID: env.ID, Name: "K", KeyHash: "h", KeyPrefix: "p"}
	require.NoError(t, testServer.stores.APIKeys.Create(context.Background(), apikey))

	tests := []struct {
		name string
		req  *http.Request
	}{
		{"archive flag", httptest.NewRequest(http.MethodDelete,
			"/api/v1/projects/"+project.Slug+"/flags/my-flag", nil)},
		{"archive segment", httptest.NewRequest(http.MethodDelete,
			"/api/v1/projects/"+project.Slug+"/segments/my-seg", nil)},
		{"revoke apikey", httptest.NewRequest(http.MethodDelete,
			"/api/v1/projects/"+project.Slug+"/environments/"+env.Slug+"/apikeys/"+apikey.ID.String(), nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.Header.Set("Authorization", "Bearer "+adminToken)
			rec := httptest.NewRecorder()
			testServer.Routes().ServeHTTP(rec, tt.req)
			assert.Equal(t, http.StatusNoContent, rec.Code)
		})
	}
}
