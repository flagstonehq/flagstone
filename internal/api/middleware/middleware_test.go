package middleware

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/thomas-vilte/flagstone/internal/storage"
)

var testStores *storage.Stores
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	flag.Parse()

	if !testing.Short() {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			dsn = "postgres://flagstone:flagstone_dev@localhost:5432/flagstone?sslmode=disable"
		}

		ctx := context.Background()
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			if err := pool.Ping(ctx); err == nil {
				testPool = pool
				testStores = storage.NewStores(pool)
			}
		}
	}

	code := m.Run()

	if testPool != nil {
		testPool.Close()
	}

	os.Exit(code)
}

func skipIfNoDB(t *testing.T) {
	t.Helper()
	if testStores == nil {
		t.Skip("Skipping test: Postgres not available")
	}
}

// seedEnvironment creates a tenant, project, and environment, returning the environment ID.
// Used by tests that need a valid environment_id for FK constraints.
func seedEnvironment(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	tenant := &storage.Tenant{
		Slug: "test-tenant-" + uuid.New().String()[:8],
		Name: "Test Tenant",
		Plan: "free",
	}
	require.NoError(t, testStores.Tenants.Create(ctx, tenant))

	project := &storage.Project{
		TenantID: tenant.ID,
		Slug:     "test-project",
		Name:     "Test Project",
	}
	require.NoError(t, testStores.Projects.Create(ctx, project))

	env := &storage.Environment{
		ProjectID: project.ID,
		Slug:      "production",
		Name:      "Production",
	}
	require.NoError(t, testStores.Environments.Create(ctx, env))

	return env.ID
}
