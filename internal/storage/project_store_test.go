package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newProjectFixture(t *testing.T) (*ProjectStore, *TenantStore, context.Context) {
	t.Helper()
	skipIfShort(t)
	return NewProjectStore(testPool), NewTenantStore(testPool), context.Background()
}

func TestProjectStore_Create(t *testing.T) {
	store, tenantStore, ctx := newProjectFixture(t)
	truncateTables(t, "projects", "tenants")

	tenant := &Tenant{Slug: "proj-tenant", Name: "Proj Tenant", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))

	project := &Project{
		TenantID: tenant.ID,
		Slug:     "my-app",
		Name:     "My App",
	}

	err := store.Create(ctx, project)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, project.ID)
	assert.False(t, project.CreatedAt.IsZero())

	t.Run("duplicate slug in same tenant", func(t *testing.T) {
		dup := &Project{TenantID: tenant.ID, Slug: "my-app", Name: "Dup"}
		err := store.Create(ctx, dup)
		assert.ErrorIs(t, err, ErrDuplicateKey)
	})

	t.Run("same slug different tenant", func(t *testing.T) {
		other := &Tenant{Slug: "other-tenant", Name: "Other", Plan: "free"}
		require.NoError(t, tenantStore.Create(ctx, other))

		ok := &Project{TenantID: other.ID, Slug: "my-app", Name: "Ok"}
		err := store.Create(ctx, ok)
		assert.NoError(t, err)
	})
}

func TestProjectStore_GetBySlug(t *testing.T) {
	store, tenantStore, ctx := newProjectFixture(t)
	truncateTables(t, "projects", "tenants")

	tenant := &Tenant{Slug: "proj-get", Name: "Proj Get", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))

	original := &Project{TenantID: tenant.ID, Slug: "my-service", Name: "My Service"}
	require.NoError(t, store.Create(ctx, original))

	t.Run("found", func(t *testing.T) {
		got, err := store.GetBySlug(ctx, tenant.ID, "my-service")
		require.NoError(t, err)
		assert.Equal(t, original.ID, got.ID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.GetBySlug(ctx, tenant.ID, "nope")
		assert.ErrorIs(t, err, ErrProjectNotFound)
	})
}

func TestProjectStore_ListByTenant(t *testing.T) {
	store, tenantStore, ctx := newProjectFixture(t)
	truncateTables(t, "projects", "tenants")

	tenant := &Tenant{Slug: "proj-list", Name: "Proj List", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))

	for i := 0; i < 3; i++ {
		p := &Project{
			TenantID: tenant.ID,
			Slug:     fmt.Sprintf("proj-%d", i),
			Name:     fmt.Sprintf("Project %d", i),
		}
		require.NoError(t, store.Create(ctx, p))
	}

	projects, err := store.ListByTenant(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Len(t, projects, 3)

	t.Run("scoped", func(t *testing.T) {
		other := &Tenant{Slug: "empty-tenant", Name: "Empty", Plan: "free"}
		require.NoError(t, tenantStore.Create(ctx, other))

		empty, err := store.ListByTenant(ctx, other.ID)
		require.NoError(t, err)
		assert.Empty(t, empty)
	})
}

func TestProjectStore_Update(t *testing.T) {
	store, tenantStore, ctx := newProjectFixture(t)
	truncateTables(t, "projects", "tenants")

	tenant := &Tenant{Slug: "proj-upd", Name: "Proj Upd", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))

	original := &Project{TenantID: tenant.ID, Slug: "before", Name: "Before"}
	require.NoError(t, store.Create(ctx, original))

	t.Run("success", func(t *testing.T) {
		original.Slug = "after"
		original.Name = "After"
		err := store.Update(ctx, original)
		require.NoError(t, err)
		assert.False(t, original.UpdatedAt.IsZero())

		got, err := store.GetBySlug(ctx, tenant.ID, "after")
		require.NoError(t, err)
		assert.Equal(t, "After", got.Name)
	})

	t.Run("not found", func(t *testing.T) {
		p := &Project{ID: uuid.New(), Slug: "x", Name: "x"}
		err := store.Update(ctx, p)
		assert.ErrorIs(t, err, ErrProjectNotFound)
	})
}
