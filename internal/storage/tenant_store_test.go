package storage

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTenantStore(t *testing.T) (*TenantStore, context.Context) {
	t.Helper()
	skipIfShort(t)
	store := NewTenantStore(testPool)
	ctx := context.Background()
	return store, ctx
}

func TestTenantStore_Create(t *testing.T) {
	store, ctx := newTenantStore(t)
	truncateTables(t, "tenants")

	tenant := &Tenant{
		Slug: "test-tenant",
		Name: "Test Tenant",
		Plan: "free",
	}

	err := store.Create(ctx, tenant)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, tenant.ID)
	assert.False(t, tenant.CreatedAt.IsZero())
	assert.False(t, tenant.UpdatedAt.IsZero())

	t.Run("duplicate slug", func(t *testing.T) {
		dup := &Tenant{
			Slug: "test-tenant",
			Name: "Another",
			Plan: "team",
		}
		err := store.Create(ctx, dup)
		assert.ErrorIs(t, err, ErrDuplicateKey)
	})
}

func TestTenantStore_GetBySlug(t *testing.T) {
	store, ctx := newTenantStore(t)
	truncateTables(t, "tenants")

	original := &Tenant{
		Slug: "get-by-slug",
		Name: "Get By Slug",
		Plan: "team",
	}
	require.NoError(t, store.Create(ctx, original))

	t.Run("found", func(t *testing.T) {
		got, err := store.GetBySlug(ctx, "get-by-slug")
		require.NoError(t, err)
		assert.Equal(t, original.ID, got.ID)
		assert.Equal(t, "get-by-slug", got.Slug)
		assert.Equal(t, "team", got.Plan)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.GetBySlug(ctx, "nonexistent")
		assert.ErrorIs(t, err, ErrTenantNotFound)
	})
}

func TestTenantStore_GetByID(t *testing.T) {
	store, ctx := newTenantStore(t)
	truncateTables(t, "tenants")

	original := &Tenant{
		Slug: "get-by-id",
		Name: "Get By ID",
		Plan: "enterprise",
	}
	require.NoError(t, store.Create(ctx, original))

	t.Run("found", func(t *testing.T) {
		got, err := store.GetByID(ctx, original.ID)
		require.NoError(t, err)
		assert.Equal(t, original.Slug, got.Slug)
		assert.Equal(t, "enterprise", got.Plan)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.GetByID(ctx, uuid.New())
		assert.ErrorIs(t, err, ErrTenantNotFound)
	})
}

func TestTenantStore_ExistsAny(t *testing.T) {
	store, ctx := newTenantStore(t)
	truncateTables(t, "tenants")

	t.Run("false on empty", func(t *testing.T) {
		exists, err := store.ExistsAny(ctx)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("true after insert", func(t *testing.T) {
		tenant := &Tenant{Slug: "exists-check", Name: "Exists", Plan: "free"}
		require.NoError(t, store.Create(ctx, tenant))

		exists, err := store.ExistsAny(ctx)
		require.NoError(t, err)
		assert.True(t, exists)
	})
}
