package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRevokedTokenFixture(t *testing.T) (*RevokedTokenStore, *TenantStore, *UserStore, context.Context) {
	t.Helper()
	skipIfShort(t)
	return NewRevokedTokenStore(testPool), NewTenantStore(testPool), NewUserStore(testPool), context.Background()
}

func TestRevokedTokenStore_Insert(t *testing.T) {
	store, tenantStore, userStore, ctx := newRevokedTokenFixture(t)
	truncateTables(t, "revoked_refresh_tokens", "sessions", "tenant_members", "users", "tenants")

	tenant := &Tenant{Slug: "rt-ins", Name: "RT Ins", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "rt-ins@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	token := &RevokedRefreshToken{
		TokenHash: hashToken("rt_insert_1"),
		UserID:    user.ID,
		TenantID:  tenant.ID,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}

	err := store.Insert(ctx, token)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, token.ID)
	assert.False(t, token.RevokedAt.IsZero())
}

func TestRevokedTokenStore_InsertIdempotent(t *testing.T) {
	store, tenantStore, userStore, ctx := newRevokedTokenFixture(t)
	truncateTables(t, "revoked_refresh_tokens", "sessions", "tenant_members", "users", "tenants")

	tenant := &Tenant{Slug: "rt-idem", Name: "RT Idem", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "rt-idem@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	token := &RevokedRefreshToken{
		TokenHash: hashToken("rt_idempotent"),
		UserID:    user.ID,
		TenantID:  tenant.ID,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}

	require.NoError(t, store.Insert(ctx, token))

	token2 := &RevokedRefreshToken{
		TokenHash: hashToken("rt_idempotent"),
		UserID:    user.ID,
		TenantID:  tenant.ID,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}

	err := store.Insert(ctx, token2)
	require.NoError(t, err)
}

func TestRevokedTokenStore_Lookup(t *testing.T) {
	store, tenantStore, userStore, ctx := newRevokedTokenFixture(t)
	truncateTables(t, "revoked_refresh_tokens", "sessions", "tenant_members", "users", "tenants")

	tenant := &Tenant{Slug: "rt-lkp", Name: "RT Lkp", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "rt-lkp@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	hash := hashToken("rt_lookup_1")
	token := &RevokedRefreshToken{
		TokenHash: hash,
		UserID:    user.ID,
		TenantID:  tenant.ID,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	require.NoError(t, store.Insert(ctx, token))

	t.Run("found", func(t *testing.T) {
		got, err := store.Lookup(ctx, hash)
		require.NoError(t, err)
		assert.Equal(t, user.ID, got.UserID)
		assert.Equal(t, hash, got.TokenHash)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.Lookup(ctx, hashToken("rt_unknown"))
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestRevokedTokenStore_DeleteExpired(t *testing.T) {
	store, tenantStore, userStore, ctx := newRevokedTokenFixture(t)
	truncateTables(t, "revoked_refresh_tokens", "sessions", "tenant_members", "users", "tenants")

	tenant := &Tenant{Slug: "rt-exp", Name: "RT Exp", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "rt-exp@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	expired := &RevokedRefreshToken{
		TokenHash: hashToken("rt_expired"),
		UserID:    user.ID,
		TenantID:  tenant.ID,
		ExpiresAt: time.Now().UTC().Add(-time.Hour),
	}
	require.NoError(t, store.Insert(ctx, expired))

	valid := &RevokedRefreshToken{
		TokenHash: hashToken("rt_valid"),
		UserID:    user.ID,
		TenantID:  tenant.ID,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	require.NoError(t, store.Insert(ctx, valid))

	n, err := store.DeleteExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	_, err = store.Lookup(ctx, hashToken("rt_expired"))
	assert.ErrorIs(t, err, ErrNotFound)

	_, err = store.Lookup(ctx, hashToken("rt_valid"))
	assert.NoError(t, err)
}
