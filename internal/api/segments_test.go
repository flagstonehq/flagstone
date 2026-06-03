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

func TestCreateSegment_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	_, _, _, projectSlug, token := seedProject(t)

	body := `{"key":"beta-users","name":"Beta Users","rules":{"all":[{"attribute":"email","op":"eq","value":"@beta.com"}]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectSlug+"/segments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var resp segmentResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "beta-users", resp.Key)
	assert.Equal(t, "Beta Users", resp.Name)
	assert.NotEmpty(t, resp.Rules)
	assert.False(t, resp.CreatedAt.IsZero())
}

func TestCreateSegment_DuplicateKey(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)

	seg := &storage.Segment{
		ProjectID: projectID,
		Key:       "beta-users",
		Name:      "Original",
		Rules:     json.RawMessage("[]"),
	}
	require.NoError(t, testServer.stores.Segments.Create(context.Background(), seg))

	body := `{"key":"beta-users","name":"Duplicate","rules":{"all":[]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectSlug+"/segments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestCreateSegment_ProjectNotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	_, _, _, _, token := seedProject(t)

	body := `{"key":"beta-users","name":"Beta Users","rules":{"all":[]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/nonexistent/segments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCreateSegment_ValidationErrors(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	_, _, _, projectSlug, token := seedProject(t)

	tests := []struct {
		name string
		body string
	}{
		{"empty key", `{"key":"","name":"Beta Users","rules":[]}`},
		{"empty name", `{"key":"beta-users","name":"","rules":[]}`},
		{"invalid rules", `{"key":"beta-users","name":"Beta Users","rules":notjson}`},
		{"key too long", `{"key":"` + strings.Repeat("a", 129) + `","name":"Beta Users","rules":[]}`},
		{"key uppercase", `{"key":"BETA-USERS","name":"Beta Users","rules":[]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectSlug+"/segments", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(authBearer(token))
			rec := httptest.NewRecorder()

			testServer.Routes().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestListSegments_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)

	for i := 0; i < 3; i++ {
		seg := &storage.Segment{
			ProjectID: projectID,
			Key:       "seg-" + uuid.New().String()[:8],
			Name:      "Segment",
			Rules:     json.RawMessage("[]"),
		}
		require.NoError(t, testServer.stores.Segments.Create(context.Background(), seg))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectSlug+"/segments", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp []segmentResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Len(t, resp, 3)
}

func TestGetSegment_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)

	seg := &storage.Segment{
		ProjectID: projectID,
		Key:       "beta-users",
		Name:      "Beta Users",
		Rules:     json.RawMessage(`[{"type":"equals","attribute":"email","value":"@beta.com"}]`),
	}
	require.NoError(t, testServer.stores.Segments.Create(context.Background(), seg))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectSlug+"/segments/beta-users", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp segmentResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, seg.ID, resp.ID)
	assert.Equal(t, "beta-users", resp.Key)
	assert.Equal(t, "Beta Users", resp.Name)
}

func TestGetSegment_NotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	_, _, _, projectSlug, token := seedProject(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectSlug+"/segments/nonexistent", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpdateSegment_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)

	seg := &storage.Segment{
		ProjectID: projectID,
		Key:       "beta-users",
		Name:      "Beta Users",
		Rules:     json.RawMessage("[]"),
	}
	require.NoError(t, testServer.stores.Segments.Create(context.Background(), seg))

	body := `{"name":"Beta Users V2","rules":{"all":[{"attribute":"country","op":"eq","value":"ar"}]}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectSlug+"/segments/beta-users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp segmentResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "beta-users", resp.Key, "key should remain unchanged")
	assert.Equal(t, "Beta Users V2", resp.Name)
	assert.Contains(t, string(resp.Rules), "country")
}

func TestUpdateSegment_NotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	_, _, _, projectSlug, token := seedProject(t)

	body := `{"name":"Updated"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectSlug+"/segments/nonexistent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestArchiveSegment_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)

	seg := &storage.Segment{
		ProjectID: projectID,
		Key:       "beta-users",
		Name:      "Beta Users",
		Rules:     json.RawMessage("[]"),
	}
	require.NoError(t, testServer.stores.Segments.Create(context.Background(), seg))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectSlug+"/segments/beta-users", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	_, err := testServer.stores.Segments.GetByKey(context.Background(), projectID, "beta-users")
	assert.ErrorIs(t, err, storage.ErrSegmentNotFound)
}

func TestArchiveSegment_NotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	_, _, _, projectSlug, token := seedProject(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectSlug+"/segments/nonexistent", nil)
	req.Header.Set(authBearer(token))
	rec := httptest.NewRecorder()

	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSegments_CrossTenantIsolation(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	tenantA := &storage.Tenant{Slug: "seg-tenant-a-" + uuid.New().String()[:8], Name: "Tenant A", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenantA))
	tenantB := &storage.Tenant{Slug: "seg-tenant-b-" + uuid.New().String()[:8], Name: "Tenant B", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenantB))

	userA := &storage.User{Email: "seg-a@example.com"}
	require.NoError(t, testServer.stores.Users.Create(context.Background(), userA))
	member := &storage.TenantMember{TenantID: tenantA.ID, UserID: userA.ID, Role: "admin"}
	require.NoError(t, testServer.stores.Members.Add(context.Background(), member))

	tokenA, err := auth.GenerateAccessToken(userA.ID, tenantA.ID, "admin", testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL, uuid.Nil)
	require.NoError(t, err)

	projA := &storage.Project{TenantID: tenantA.ID, Slug: "proj-a", Name: "Project A"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), projA))
	projB := &storage.Project{TenantID: tenantB.ID, Slug: "proj-b", Name: "Project B"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), projB))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-a/segments", nil)
	req.Header.Set(authBearer(tokenA))
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-b/segments", nil)
	req.Header.Set(authBearer(tokenA))
	rec = httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code, "should not see another tenant's project")
}

func TestSegments_RequiresAuth(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "environments", "flags", "segments", "projects", "tenant_members", "users", "tenants")

	endpoints := []struct {
		method, path, body string
	}{
		{http.MethodPost, "/api/v1/projects/test/segments", `{"key":"seg","name":"Seg","rules":[]}`},
		{http.MethodGet, "/api/v1/projects/test/segments", ""},
		{http.MethodGet, "/api/v1/projects/test/segments/my-key", ""},
		{http.MethodPut, "/api/v1/projects/test/segments/my-key", `{"name":"Updated"}`},
		{http.MethodDelete, "/api/v1/projects/test/segments/my-key", ""},
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
func TestUpdateSegment_ValidationErrors(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "segments", "projects", "tenant_members", "users", "tenants")

	_, _, projectID, projectSlug, token := seedProject(t)
	seg := &storage.Segment{ProjectID: projectID, Key: "seg-val", Name: "Seg", Rules: json.RawMessage("{}")}
	require.NoError(t, testServer.stores.Segments.Create(context.Background(), seg))

	tests := []struct {
		name string
		body string
	}{
		{"empty key", `{"key":""}`},
		{"key uppercase", `{"key":"UPPER"}`},
		{"key too long", `{"key":"` + strings.Repeat("a", 129) + `"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectSlug+"/segments/seg-val",
				strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(authBearer(token))
			rec := httptest.NewRecorder()
			testServer.Routes().ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}
