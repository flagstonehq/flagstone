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

func seedProject(t *testing.T) (tenantID, userID, projectID uuid.UUID, projectSlug, token string) {
	t.Helper()

	password := "securepass123"
	hash, err := auth.HashPassword(password, testServer.cfg.BcryptCost)
	require.NoError(t, err)

	tenant := &storage.Tenant{Slug: "env-tenant-" + uuid.New().String()[:8], Name: "Env Tenant", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenant))

	user := &storage.User{Email: "env@example.com", PasswordHash: &hash}
	require.NoError(t, testServer.stores.Users.Create(context.Background(), user))

	member := &storage.TenantMember{TenantID: tenant.ID, UserID: user.ID, Role: "admin"}
	require.NoError(t, testServer.stores.Members.Add(context.Background(), member))

	project := &storage.Project{TenantID: tenant.ID, Slug: "env-proj-" + uuid.New().String()[:8], Name: "Env Project"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))

	accessToken, err := auth.GenerateAccessToken(user.ID, tenant.ID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, uuid.Nil)
	require.NoError(t, err)

	return tenant.ID, user.ID, project.ID, project.Slug, accessToken
}

func TestCreateEnvironment_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	_, _, _, projectSlug, token := seedProject(t)

	body := `{"slug":"staging-2","name":"Staging 2"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectSlug+"/environments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var resp environmentResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "staging-2", resp.Slug)
	assert.Equal(t, "Staging 2", resp.Name)
	assert.False(t, resp.CreatedAt.IsZero())
}

func TestCreateEnvironment_DuplicateSlug(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)

	env := &storage.Environment{ProjectID: projectID, Slug: "staging-2", Name: "Staging 2"}
	require.NoError(t, testServer.stores.Environments.Create(context.Background(), env))

	body := `{"slug":"staging-2","name":"Staging 2 Copy"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectSlug+"/environments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestCreateEnvironment_ProjectNotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	_, _, _, _, token := seedProject(t)

	body := `{"slug":"staging-2","name":"Staging 2"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/nonexistent/environments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCreateEnvironment_ValidationErrors(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	_, _, _, projectSlug, token := seedProject(t)

	tests := []struct {
		name string
		body string
	}{
		{"empty slug", `{"slug":"","name":"Env"}`},
		{"empty name", `{"slug":"my-env","name":""}`},
		{"slug too long", `{"slug":"` + strings.Repeat("a", 65) + `","name":"Env"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectSlug+"/environments", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(authBearer(token))
			rec := httptest.NewRecorder()

			testServer.Routes().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestListEnvironments_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)

	for i := 0; i < 3; i++ {
		env := &storage.Environment{
			ProjectID: projectID,
			Slug:      "env-" + uuid.New().String()[:8],
			Name:      "Environment",
		}
		require.NoError(t, testServer.stores.Environments.Create(context.Background(), env))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectSlug+"/environments", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp []environmentResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Len(t, resp, 3)
}

func TestEnvironments_RequiresAuth(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	endpoints := []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/v1/projects/test/environments", ""},
		{http.MethodPost, "/api/v1/projects/test/environments", `{"slug":"env","name":"Env"}`},
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
