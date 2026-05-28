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

// rbacFixture is everything an RBAC test needs to construct an HTTP request
// for a given role: the JWT, plus the slugs/keys of pre-seeded resources so
// the request hits an existing tenant/project/etc instead of 404-ing for
// reasons unrelated to RBAC.
type rbacFixture struct {
	tenantID    uuid.UUID
	projectSlug string
	envSlug     string
	flagKey     string
	segmentKey  string
	token       string
}

// seedProjectAtRole creates a tenant + project + one user assigned to the
// given role, plus an env, flag, and segment so every RBAC test has a real
// target. Returns a fixture struct rather than 6+ positional values.
func seedProjectAtRole(t *testing.T, role string) rbacFixture {
	t.Helper()

	password := "securepass123"
	hash, err := auth.HashPassword(password, testServer.cfg.BcryptCost)
	require.NoError(t, err)
	ctx := context.Background()

	tenant := &storage.Tenant{Slug: "rbac-tenant-" + uuid.New().String()[:8], Name: "RBAC Tenant", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(ctx, tenant))

	user := &storage.User{Email: "rbac-" + uuid.New().String()[:6] + "@example.com", PasswordHash: &hash}
	require.NoError(t, testServer.stores.Users.Create(ctx, user))

	require.NoError(t, testServer.stores.Members.Add(ctx, &storage.TenantMember{
		TenantID: tenant.ID, UserID: user.ID, Role: role,
	}))

	project := &storage.Project{TenantID: tenant.ID, Slug: "rbac-proj-" + uuid.New().String()[:8], Name: "RBAC Project"}
	require.NoError(t, testServer.stores.Projects.Create(ctx, project))

	env := &storage.Environment{ProjectID: project.ID, Slug: "production", Name: "Production"}
	require.NoError(t, testServer.stores.Environments.Create(ctx, env))

	flag := &storage.Flag{
		ProjectID:    project.ID,
		Key:          "rbac-flag",
		Name:         "RBAC Flag",
		Type:         "boolean",
		DefaultValue: []byte("false"),
	}
	require.NoError(t, testServer.stores.Flags.Create(ctx, flag))

	segment := &storage.Segment{
		ProjectID: project.ID,
		Key:       "rbac-segment",
		Name:      "RBAC Segment",
		Rules:     []byte("[]"),
	}
	require.NoError(t, testServer.stores.Segments.Create(ctx, segment))

	accessToken, err := auth.GenerateAccessToken(user.ID, tenant.ID, role, testServer.cfg.JWTSecret, testServer.cfg.AccessTokenTTL)
	require.NoError(t, err)

	return rbacFixture{
		tenantID:    tenant.ID,
		projectSlug: project.Slug,
		envSlug:     env.Slug,
		flagKey:     flag.Key,
		segmentKey:  segment.Key,
		token:       accessToken,
	}
}

// rbacTestCase is one row of the RBAC matrix from PLAN.md / SECURITY.md —
// a (method, path, body) tuple that should be allowed at minRole and above
// and forbidden below. Tests below run this against every role.
type rbacTestCase struct {
	method  string
	pathFn  func(fx rbacFixture) string
	body    string
	minRole string
}

var allRoles = []string{"viewer", "member", "admin", "owner"}

func roleAtLeast(role, minimum string) bool {
	rank := map[string]int{"viewer": 1, "member": 2, "admin": 3, "owner": 4}
	return rank[role] >= rank[minimum]
}

func runRBACMatrix(t *testing.T, tc rbacTestCase) {
	t.Helper()
	for _, role := range allRoles {
		t.Run(role, func(t *testing.T) {
			truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "api_keys", "flags", "segments", "projects", "tenant_members", "users", "tenants")
			fx := seedProjectAtRole(t, role)

			req := httptest.NewRequest(tc.method, tc.pathFn(fx), strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set(authBearer(fx.token))
			rec := httptest.NewRecorder()
			testServer.Routes().ServeHTTP(rec, req)

			if roleAtLeast(role, tc.minRole) {
				// Role meets the threshold — must NOT be 403. The handler may
				// still return other codes (404, 400, 200, 201, 204, 409) but
				// permission was granted.
				assert.NotEqual(t, http.StatusForbidden, rec.Code,
					"role %s (>= %s) should not be forbidden; got %d", role, tc.minRole, rec.Code)
			} else {
				assert.Equal(t, http.StatusForbidden, rec.Code,
					"role %s (< %s) must be forbidden; got %d", role, tc.minRole, rec.Code)
			}
		})
	}
}

func TestRBAC_CreateProject(t *testing.T) {
	skipIfNoDB(t)
	runRBACMatrix(t, rbacTestCase{
		method:  http.MethodPost,
		pathFn:  func(_ rbacFixture) string { return "/api/v1/projects" },
		body:    `{"slug":"new-proj","name":"New Project"}`,
		minRole: "admin",
	})
}

func TestRBAC_UpdateProject(t *testing.T) {
	skipIfNoDB(t)
	runRBACMatrix(t, rbacTestCase{
		method:  http.MethodPut,
		pathFn:  func(fx rbacFixture) string { return "/api/v1/projects/" + fx.projectSlug },
		body:    `{"name":"Updated Name"}`,
		minRole: "admin",
	})
}

func TestRBAC_CreateFlag(t *testing.T) {
	skipIfNoDB(t)
	runRBACMatrix(t, rbacTestCase{
		method:  http.MethodPost,
		pathFn:  func(fx rbacFixture) string { return "/api/v1/projects/" + fx.projectSlug + "/flags" },
		body:    `{"key":"new-flag","name":"New Flag","type":"boolean","default_value":false}`,
		minRole: "member",
	})
}

func TestRBAC_UpdateFlag(t *testing.T) {
	skipIfNoDB(t)
	runRBACMatrix(t, rbacTestCase{
		method:  http.MethodPut,
		pathFn:  func(fx rbacFixture) string { return "/api/v1/projects/" + fx.projectSlug + "/flags/" + fx.flagKey },
		body:    `{"name":"Updated Flag"}`,
		minRole: "member",
	})
}

func TestRBAC_ArchiveFlag(t *testing.T) {
	skipIfNoDB(t)
	runRBACMatrix(t, rbacTestCase{
		method:  http.MethodDelete,
		pathFn:  func(fx rbacFixture) string { return "/api/v1/projects/" + fx.projectSlug + "/flags/" + fx.flagKey },
		minRole: "admin",
	})
}

func TestRBAC_CreateSegment(t *testing.T) {
	skipIfNoDB(t)
	runRBACMatrix(t, rbacTestCase{
		method:  http.MethodPost,
		pathFn:  func(fx rbacFixture) string { return "/api/v1/projects/" + fx.projectSlug + "/segments" },
		body:    `{"key":"new-seg","name":"New Segment","rules":[]}`,
		minRole: "member",
	})
}

func TestRBAC_ArchiveSegment(t *testing.T) {
	skipIfNoDB(t)
	runRBACMatrix(t, rbacTestCase{
		method: http.MethodDelete,
		pathFn: func(fx rbacFixture) string {
			return "/api/v1/projects/" + fx.projectSlug + "/segments/" + fx.segmentKey
		},
		minRole: "admin",
	})
}

func TestRBAC_CreateEnvironment(t *testing.T) {
	skipIfNoDB(t)
	runRBACMatrix(t, rbacTestCase{
		method:  http.MethodPost,
		pathFn:  func(fx rbacFixture) string { return "/api/v1/projects/" + fx.projectSlug + "/environments" },
		body:    `{"slug":"new-env","name":"New Env"}`,
		minRole: "admin",
	})
}

func TestRBAC_DeleteEnvironment(t *testing.T) {
	skipIfNoDB(t)
	runRBACMatrix(t, rbacTestCase{
		method: http.MethodDelete,
		pathFn: func(fx rbacFixture) string {
			return "/api/v1/projects/" + fx.projectSlug + "/environments/" + fx.envSlug
		},
		minRole: "owner",
	})
}

func TestRBAC_CreateAPIKey(t *testing.T) {
	skipIfNoDB(t)
	runRBACMatrix(t, rbacTestCase{
		method: http.MethodPost,
		pathFn: func(fx rbacFixture) string {
			return "/api/v1/projects/" + fx.projectSlug + "/environments/" + fx.envSlug + "/apikeys"
		},
		body:    `{"name":"new-key"}`,
		minRole: "admin",
	})
}

func TestRBAC_ListAPIKeys(t *testing.T) {
	skipIfNoDB(t)
	runRBACMatrix(t, rbacTestCase{
		method: http.MethodGet,
		pathFn: func(fx rbacFixture) string {
			return "/api/v1/projects/" + fx.projectSlug + "/environments/" + fx.envSlug + "/apikeys"
		},
		minRole: "member",
	})
}
