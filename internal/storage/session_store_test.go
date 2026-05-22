package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSessionFixture(t *testing.T) (*SessionStore, *TenantStore, *UserStore, context.Context) {
	t.Helper()
	skipIfShort(t)
	return NewSessionStore(testPool), NewTenantStore(testPool), NewUserStore(testPool), context.Background()
}

func hashToken(tok string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(tok)))
}

func fakeIP(s string) *net.IP {
	ip := net.ParseIP(s)
	return &ip
}

func TestSessionStore_Create(t *testing.T) {
	store, tenantStore, userStore, ctx := newSessionFixture(t)
	truncateTables(t, "sessions", "users", "tenants")

	tenant := &Tenant{Slug: "sess-tenant", Name: "Sess Tenant", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "sess@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	session := &Session{
		UserID:      user.ID,
		TenantID:    tenant.ID,
		RefreshHash: hashToken("refresh_token_1"),
		UserAgent:   strPtr("GoTest/1.0"),
		IPAddress:   fakeIP("10.0.0.1"),
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	err := store.Create(ctx, session)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, session.ID)
	assert.False(t, session.CreatedAt.IsZero())
}

func TestSessionStore_GetByRefreshHash(t *testing.T) {
	store, tenantStore, userStore, ctx := newSessionFixture(t)
	truncateTables(t, "sessions", "users", "tenants")

	tenant := &Tenant{Slug: "sess-get", Name: "Sess Get", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "sess-get@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	original := &Session{
		UserID:      user.ID,
		TenantID:    tenant.ID,
		RefreshHash: hashToken("find_me"),
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	require.NoError(t, store.Create(ctx, original))

	t.Run("found", func(t *testing.T) {
		got, err := store.GetByRefreshHash(ctx, hashToken("find_me"))
		require.NoError(t, err)
		assert.Equal(t, original.ID, got.ID)
		assert.Equal(t, user.ID, got.UserID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.GetByRefreshHash(ctx, hashToken("unknown"))
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestSessionStore_DeleteByID(t *testing.T) {
	store, tenantStore, userStore, ctx := newSessionFixture(t)
	truncateTables(t, "sessions", "users", "tenants")

	tenant := &Tenant{Slug: "sess-del", Name: "Sess Del", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "sess-del@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	session := &Session{
		UserID:      user.ID,
		TenantID:    tenant.ID,
		RefreshHash: hashToken("delete_me"),
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	require.NoError(t, store.Create(ctx, session))

	t.Run("success", func(t *testing.T) {
		err := store.DeleteByID(ctx, session.ID)
		require.NoError(t, err)

		_, err = store.GetByRefreshHash(ctx, hashToken("delete_me"))
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("not found", func(t *testing.T) {
		err := store.DeleteByID(ctx, uuid.New())
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestSessionStore_DeleteByUserID(t *testing.T) {
	store, tenantStore, userStore, ctx := newSessionFixture(t)
	truncateTables(t, "sessions", "users", "tenants")

	tenant := &Tenant{Slug: "sess-delall", Name: "Sess DelAll", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "sess-delall@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	for i := 0; i < 3; i++ {
		s := &Session{
			UserID:      user.ID,
			TenantID:    tenant.ID,
			RefreshHash: hashToken(fmt.Sprintf("bulk_%d", i)),
			ExpiresAt:   time.Now().Add(time.Hour),
		}
		require.NoError(t, store.Create(ctx, s))
	}

	err := store.DeleteByUserID(ctx, user.ID)
	require.NoError(t, err)

	_, err = store.GetByRefreshHash(ctx, hashToken("bulk_0"))
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestSessionStore_DeleteExpired(t *testing.T) {
	store, tenantStore, userStore, ctx := newSessionFixture(t)
	truncateTables(t, "sessions", "users", "tenants")

	tenant := &Tenant{Slug: "sess-exp", Name: "Sess Exp", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "sess-exp@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	expired := &Session{
		UserID:      user.ID,
		TenantID:    tenant.ID,
		RefreshHash: hashToken("expired"),
		ExpiresAt:   time.Now().Add(-time.Hour),
	}
	require.NoError(t, store.Create(ctx, expired))

	valid := &Session{
		UserID:      user.ID,
		TenantID:    tenant.ID,
		RefreshHash: hashToken("valid"),
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	require.NoError(t, store.Create(ctx, valid))

	n, err := store.DeleteExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	_, err = store.GetByRefreshHash(ctx, hashToken("expired"))
	assert.ErrorIs(t, err, ErrNotFound)

	_, err = store.GetByRefreshHash(ctx, hashToken("valid"))
	assert.NoError(t, err)
}
