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

func seedProjectUser(t *testing.T) (tenantID, userID uuid.UUID, token string) {
	t.Helper()

	password := "securepass123"
	hash, err := auth.HashPassword(password, testServer.cfg.BcryptCost)
	require.NoError(t, err)

	tenant := &storage.Tenant{Slug: "proj-tenant-" + uuid.New().String()[:8], Name: "Project Tenant", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenant))

	user := &storage.User{Email: "project@example.com", PasswordHash: &hash}
	require.NoError(t, testServer.stores.Users.Create(context.Background(), user))

	member := &storage.TenantMember{TenantID: tenant.ID, UserID: user.ID, Role: "admin"}
	require.NoError(t, testServer.stores.Members.Add(context.Background(), member))

	accessToken, err := auth.GenerateAccessToken(user.ID, tenant.ID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, uuid.Nil)
	require.NoError(t, err)

	return tenant.ID, user.ID, accessToken
}

func authBearer(token string) (key, value string) {
	return "Authorization", "Bearer " + token
}

func TestCreateProject_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	_, _, token := seedProjectUser(t)

	body := `{"slug":"my-app","name":"My App"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var resp projectResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "my-app", resp.Slug)
	assert.Equal(t, "My App", resp.Name)
	assert.False(t, resp.CreatedAt.IsZero())

	var envCount int
	err = testPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM environments WHERE project_id = $1", resp.ID).Scan(&envCount)
	require.NoError(t, err)
	assert.Equal(t, 3, envCount)
}

func TestCreateProject_DuplicateSlug(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	tenantID, _, token := seedProjectUser(t)

	project := &storage.Project{TenantID: tenantID, Slug: "my-app", Name: "Original"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))

	body := `{"slug":"my-app","name":"Duplicate"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestCreateProject_ValidationErrors(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	_, _, token := seedProjectUser(t)

	tests := []struct {
		name string
		body string
	}{
		{"empty slug", `{"slug":"","name":"My App"}`},
		{"empty name", `{"slug":"my-app","name":""}`},
		{"slug too long", `{"slug":"` + strings.Repeat("a", 65) + `","name":"My App"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(authBearer(token))
			rec := httptest.NewRecorder()

			testServer.Routes().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestListProjects_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	tenantID, _, token := seedProjectUser(t)

	for i := 0; i < 3; i++ {
		p := &storage.Project{
			TenantID: tenantID,
			Slug:     "proj-" + uuid.New().String()[:8],
			Name:     "Project",
		}
		require.NoError(t, testServer.stores.Projects.Create(context.Background(), p))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp []projectResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Len(t, resp, 3)
}

func TestGetProject_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	tenantID, _, token := seedProjectUser(t)

	project := &storage.Project{TenantID: tenantID, Slug: "my-app", Name: "My App"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/my-app", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp projectResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, project.ID, resp.ID)
	assert.Equal(t, "my-app", resp.Slug)
	assert.Equal(t, "My App", resp.Name)
}

func TestGetProject_NotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	_, _, token := seedProjectUser(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/nonexistent", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpdateProject_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	tenantID, _, token := seedProjectUser(t)

	project := &storage.Project{TenantID: tenantID, Slug: "my-app", Name: "My App"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))

	body := `{"name":"My Updated App","slug":"my-app-v2"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/my-app", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp projectResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "my-app-v2", resp.Slug)
	assert.Equal(t, "My Updated App", resp.Name)
}

func TestUpdateProject_Partial(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	tenantID, _, token := seedProjectUser(t)

	project := &storage.Project{TenantID: tenantID, Slug: "my-app", Name: "My App"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))

	body := `{"name":"Only Name Changed"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/my-app", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp projectResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "my-app", resp.Slug, "slug should remain unchanged")
	assert.Equal(t, "Only Name Changed", resp.Name)
}

func TestProjects_CrossTenantIsolation(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	tenantA := &storage.Tenant{Slug: "tenant-a-" + uuid.New().String()[:8], Name: "Tenant A", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenantA))

	tenantB := &storage.Tenant{Slug: "tenant-b-" + uuid.New().String()[:8], Name: "Tenant B", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenantB))

	userA := &storage.User{Email: "user-a@example.com"}
	require.NoError(t, testServer.stores.Users.Create(context.Background(), userA))

	member := &storage.TenantMember{TenantID: tenantA.ID, UserID: userA.ID, Role: "admin"}
	require.NoError(t, testServer.stores.Members.Add(context.Background(), member))

	tokenA, err := auth.GenerateAccessToken(userA.ID, tenantA.ID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, uuid.Nil)
	require.NoError(t, err)

	require.NoError(t, testServer.stores.Projects.Create(context.Background(),
		&storage.Project{TenantID: tenantA.ID, Slug: "my-app", Name: "My App"}))
	require.NoError(t, testServer.stores.Projects.Create(context.Background(),
		&storage.Project{TenantID: tenantB.ID, Slug: "other-app", Name: "Other App"}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set(authBearer(tokenA))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp []projectResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	require.Len(t, resp, 1)
	assert.Equal(t, "my-app", resp[0].Slug)
}

func TestProjects_RequiresAuth(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "projects", "tenant_members", "users", "tenants")

	endpoints := []struct {
		method, path, body string
	}{
		{http.MethodPost, "/api/v1/projects", `{"slug":"test","name":"Test"}`},
		{http.MethodGet, "/api/v1/projects", ""},
		{http.MethodGet, "/api/v1/projects/test", ""},
		{http.MethodPut, "/api/v1/projects/test", `{"name":"Test"}`},
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
