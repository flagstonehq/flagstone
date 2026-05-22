package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAPIKeyFixture(t *testing.T) (
	*APIKeyStore, *EnvironmentStore, *ProjectStore, *TenantStore, context.Context,
) {
	t.Helper()
	skipIfShort(t)
	return NewAPIKeyStore(testPool), NewEnvironmentStore(testPool),
		NewProjectStore(testPool), NewTenantStore(testPool), context.Background()
}

func apiKeyHash(raw string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
}

func apiKeyPrefix(raw string) string {
	if len(raw) < 8 {
		return raw
	}
	return raw[:8]
}

func TestAPIKeyStore_Create(t *testing.T) {
	store, envStore, projStore, tenantStore, ctx := newAPIKeyFixture(t)
	truncateTables(t, "api_keys", "environments", "projects", "tenants")

	tenant := &Tenant{Slug: "key-tenant", Name: "Key Tenant", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "key-proj", Name: "Key Proj"}
	require.NoError(t, projStore.Create(ctx, project))
	env := &Environment{ProjectID: project.ID, Slug: "key-env", Name: "Key Env"}
	require.NoError(t, envStore.Create(ctx, env))

	key := &APIKey{
		EnvironmentID: env.ID,
		Name:          "Test Key",
		KeyHash:       apiKeyHash("fs_live_a1b2c3d4"),
		KeyPrefix:     apiKeyPrefix("fs_live_a1b2c3d4"),
	}

	err := store.Create(ctx, key)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, key.ID)
	assert.False(t, key.CreatedAt.IsZero())

	t.Run("duplicate hash", func(t *testing.T) {
		dup := &APIKey{
			EnvironmentID: env.ID,
			Name:          "Dup",
			KeyHash:       apiKeyHash("fs_live_a1b2c3d4"),
			KeyPrefix:     "fs_live_",
		}
		err := store.Create(ctx, dup)
		assert.ErrorIs(t, err, ErrDuplicateKey)
	})
}

func TestAPIKeyStore_GetByHash(t *testing.T) {
	store, envStore, projStore, tenantStore, ctx := newAPIKeyFixture(t)
	truncateTables(t, "api_keys", "environments", "projects", "tenants")

	tenant := &Tenant{Slug: "key-get", Name: "Key Get", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "key-get-proj", Name: "Key Get Proj"}
	require.NoError(t, projStore.Create(ctx, project))
	env := &Environment{ProjectID: project.ID, Slug: "key-get-env", Name: "Key Get Env"}
	require.NoError(t, envStore.Create(ctx, env))

	raw := "fs_live_secret123"
	original := &APIKey{
		EnvironmentID: env.ID,
		Name:          "Live Key",
		KeyHash:       apiKeyHash(raw),
		KeyPrefix:     apiKeyPrefix(raw),
	}
	require.NoError(t, store.Create(ctx, original))

	t.Run("found", func(t *testing.T) {
		got, err := store.GetByHash(ctx, apiKeyHash(raw))
		require.NoError(t, err)
		assert.Equal(t, original.ID, got.ID)
		assert.Equal(t, "Live Key", got.Name)
	})

	t.Run("not found - wrong hash", func(t *testing.T) {
		_, err := store.GetByHash(ctx, apiKeyHash("wrong"))
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("not found - revoked key", func(t *testing.T) {
		revokedRaw := "fs_live_revoked"
		revoked := &APIKey{
			EnvironmentID: env.ID,
			Name:          "Revoked",
			KeyHash:       apiKeyHash(revokedRaw),
			KeyPrefix:     apiKeyPrefix(revokedRaw),
		}
		require.NoError(t, store.Create(ctx, revoked))
		require.NoError(t, store.Revoke(ctx, revoked.ID, time.Now()))

		_, err := store.GetByHash(ctx, apiKeyHash(revokedRaw))
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestAPIKeyStore_ListByEnvironment(t *testing.T) {
	store, envStore, projStore, tenantStore, ctx := newAPIKeyFixture(t)
	truncateTables(t, "api_keys", "environments", "projects", "tenants")

	tenant := &Tenant{Slug: "key-list", Name: "Key List", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "key-list-proj", Name: "Key List Proj"}
	require.NoError(t, projStore.Create(ctx, project))
	env := &Environment{ProjectID: project.ID, Slug: "key-list-env", Name: "Key List Env"}
	require.NoError(t, envStore.Create(ctx, env))

	for i := 0; i < 3; i++ {
		raw := fmt.Sprintf("fs_live_key_%d", i)
		k := &APIKey{
			EnvironmentID: env.ID,
			Name:          fmt.Sprintf("Key %d", i),
			KeyHash:       apiKeyHash(raw),
			KeyPrefix:     apiKeyPrefix(raw),
		}
		require.NoError(t, store.Create(ctx, k))
	}

	keys, err := store.ListByEnvironment(ctx, env.ID)
	require.NoError(t, err)
	assert.Len(t, keys, 3)

	t.Run("includes revoked", func(t *testing.T) {
		raw := "fs_live_include_revoked"
		revoked := &APIKey{
			EnvironmentID: env.ID,
			Name:          "Revoked In List",
			KeyHash:       apiKeyHash(raw),
			KeyPrefix:     apiKeyPrefix(raw),
		}
		require.NoError(t, store.Create(ctx, revoked))
		require.NoError(t, store.Revoke(ctx, revoked.ID, time.Now()))

		keys, err := store.ListByEnvironment(ctx, env.ID)
		require.NoError(t, err)
		assert.Len(t, keys, 4)
	})
}

func TestAPIKeyStore_Revoke(t *testing.T) {
	store, envStore, projStore, tenantStore, ctx := newAPIKeyFixture(t)
	truncateTables(t, "api_keys", "environments", "projects", "tenants")

	tenant := &Tenant{Slug: "key-revoke", Name: "Key Revoke", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "key-revoke-proj", Name: "Key Revoke Proj"}
	require.NoError(t, projStore.Create(ctx, project))
	env := &Environment{ProjectID: project.ID, Slug: "key-revoke-env", Name: "Key Revoke Env"}
	require.NoError(t, envStore.Create(ctx, env))

	raw := "fs_live_revoke_me"
	original := &APIKey{
		EnvironmentID: env.ID,
		Name:          "To Revoke",
		KeyHash:       apiKeyHash(raw),
		KeyPrefix:     apiKeyPrefix(raw),
	}
	require.NoError(t, store.Create(ctx, original))

	now := time.Now()
	err := store.Revoke(ctx, original.ID, now)
	require.NoError(t, err)

	got, err := store.ListByEnvironment(ctx, env.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.NotNil(t, got[0].RevokedAt)

	t.Run("not found", func(t *testing.T) {
		err := store.Revoke(ctx, uuid.New(), time.Now())
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestAPIKeyStore_UpdateLastUsed(t *testing.T) {
	store, envStore, projStore, tenantStore, ctx := newAPIKeyFixture(t)
	truncateTables(t, "api_keys", "environments", "projects", "tenants")

	tenant := &Tenant{Slug: "key-used", Name: "Key Used", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "key-used-proj", Name: "Key Used Proj"}
	require.NoError(t, projStore.Create(ctx, project))
	env := &Environment{ProjectID: project.ID, Slug: "key-used-env", Name: "Key Used Env"}
	require.NoError(t, envStore.Create(ctx, env))

	raw := "fs_live_used"
	original := &APIKey{
		EnvironmentID: env.ID,
		Name:          "Used Key",
		KeyHash:       apiKeyHash(raw),
		KeyPrefix:     apiKeyPrefix(raw),
	}
	require.NoError(t, store.Create(ctx, original))

	now := time.Now().UTC().Truncate(time.Microsecond)
	err := store.UpdateLastUsed(ctx, original.ID, now)
	require.NoError(t, err)

	got, err := store.ListByEnvironment(ctx, env.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotNil(t, got[0].LastUsedAt)
	assert.True(t, got[0].LastUsedAt.Equal(now), "expected %v, got %v", now, got[0].LastUsedAt)

	t.Run("not found", func(t *testing.T) {
		err := store.UpdateLastUsed(ctx, uuid.New(), time.Now())
		assert.ErrorIs(t, err, ErrNotFound)
	})
}
