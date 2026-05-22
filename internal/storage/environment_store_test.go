package storage

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newEnvironmentFixture(t *testing.T) (*EnvironmentStore, *ProjectStore, *TenantStore, context.Context) {
	t.Helper()
	skipIfShort(t)
	return NewEnvironmentStore(testPool), NewProjectStore(testPool), NewTenantStore(testPool), context.Background()
}

func TestEnvironmentStore_Create(t *testing.T) {
	store, projectStore, tenantStore, ctx := newEnvironmentFixture(t)
	truncateTables(t, "environments", "projects", "tenants")

	tenant := &Tenant{Slug: "env-tenant", Name: "Env Tenant", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "env-proj", Name: "Env Proj"}
	require.NoError(t, projectStore.Create(ctx, project))

	env := &Environment{
		ProjectID: project.ID,
		Slug:      "production",
		Name:      "Production",
	}
	err := store.Create(ctx, env)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, env.ID)

	t.Run("duplicate slug", func(t *testing.T) {
		dup := &Environment{
			ProjectID: project.ID,
			Slug:      "production",
			Name:      "Prod dup",
		}
		err := store.Create(ctx, dup)
		assert.ErrorIs(t, err, ErrDuplicateKey)
	})
}

func TestEnvironmentStore_GetBySlug(t *testing.T) {
	store, projectStore, tenantStore, ctx := newEnvironmentFixture(t)
	truncateTables(t, "environments", "projects", "tenants")

	tenant := &Tenant{Slug: "env-get", Name: "Env Get", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "env-get-proj", Name: "Env Get Proj"}
	require.NoError(t, projectStore.Create(ctx, project))
	original := &Environment{ProjectID: project.ID, Slug: "staging", Name: "Staging"}
	require.NoError(t, store.Create(ctx, original))

	t.Run("found", func(t *testing.T) {
		got, err := store.GetBySlug(ctx, project.ID, "staging")
		require.NoError(t, err)
		assert.Equal(t, original.ID, got.ID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.GetBySlug(ctx, project.ID, "nope")
		assert.ErrorIs(t, err, ErrEnvironmentNotFound)
	})
}

func TestEnvironmentStore_GetByID(t *testing.T) {
	store, projectStore, tenantStore, ctx := newEnvironmentFixture(t)
	truncateTables(t, "environments", "projects", "tenants")

	tenant := &Tenant{Slug: "env-byid", Name: "Env ByID", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "env-byid-proj", Name: "Env ByID Proj"}
	require.NoError(t, projectStore.Create(ctx, project))
	original := &Environment{ProjectID: project.ID, Slug: "dev", Name: "Dev"}
	require.NoError(t, store.Create(ctx, original))

	t.Run("found", func(t *testing.T) {
		got, err := store.GetByID(ctx, original.ID)
		require.NoError(t, err)
		assert.Equal(t, original.Slug, got.Slug)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.GetByID(ctx, uuid.New())
		assert.ErrorIs(t, err, ErrEnvironmentNotFound)
	})
}

func TestEnvironmentStore_ListByProject(t *testing.T) {
	store, projectStore, tenantStore, ctx := newEnvironmentFixture(t)
	truncateTables(t, "environments", "projects", "tenants")

	tenant := &Tenant{Slug: "env-list", Name: "Env List", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "env-list-proj", Name: "Env List Proj"}
	require.NoError(t, projectStore.Create(ctx, project))

	for _, slug := range []string{"dev", "staging", "prod"} {
		e := &Environment{ProjectID: project.ID, Slug: slug, Name: slug}
		require.NoError(t, store.Create(ctx, e))
	}

	envs, err := store.ListByProject(ctx, project.ID)
	require.NoError(t, err)
	assert.Len(t, envs, 3)

	t.Run("scoped", func(t *testing.T) {
		otherProject := &Project{TenantID: tenant.ID, Slug: "other-proj", Name: "Other"}
		require.NoError(t, projectStore.Create(ctx, otherProject))

		empty, err := store.ListByProject(ctx, otherProject.ID)
		require.NoError(t, err)
		assert.Empty(t, empty)
	})
}
