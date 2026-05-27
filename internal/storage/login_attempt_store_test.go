package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLoginAttemptFixture(t *testing.T) (*LoginAttemptStore, *TenantStore, *UserStore, context.Context) {
	t.Helper()
	skipIfShort(t)
	return NewLoginAttemptStore(testPool), NewTenantStore(testPool), NewUserStore(testPool), context.Background()
}

func TestLoginAttemptStore_Record(t *testing.T) {
	store, tenantStore, userStore, ctx := newLoginAttemptFixture(t)
	truncateTables(t, "login_attempts", "sessions", "tenant_members", "users", "tenants")

	tenant := &Tenant{Slug: "la-rec", Name: "LA Rec", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "la-rec@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	attempt := &LoginAttempt{
		UserID:    user.ID,
		IPAddress: fakeIP("10.0.0.1"),
	}
	err := store.Record(ctx, attempt)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, attempt.ID)
	assert.False(t, attempt.AttemptedAt.IsZero())
}

func TestLoginAttemptStore_CountSince(t *testing.T) {
	store, tenantStore, userStore, ctx := newLoginAttemptFixture(t)
	truncateTables(t, "login_attempts", "sessions", "tenant_members", "users", "tenants")

	tenant := &Tenant{Slug: "la-cnt", Name: "LA Cnt", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "la-cnt@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	for i := 0; i < 3; i++ {
		require.NoError(t, store.Record(ctx, &LoginAttempt{UserID: user.ID}))
	}

	t.Run("returns count within window", func(t *testing.T) {
		count, err := store.CountSince(ctx, user.ID, time.Now().UTC().Add(-time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("returns zero outside window", func(t *testing.T) {
		count, err := store.CountSince(ctx, user.ID, time.Now().UTC().Add(time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("scoped to user", func(t *testing.T) {
		other := &User{Email: "la-other@example.com"}
		require.NoError(t, userStore.Create(ctx, other))

		count, err := store.CountSince(ctx, other.ID, time.Now().UTC().Add(-time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestLoginAttemptStore_ClearForUser(t *testing.T) {
	store, tenantStore, userStore, ctx := newLoginAttemptFixture(t)
	truncateTables(t, "login_attempts", "sessions", "tenant_members", "users", "tenants")

	tenant := &Tenant{Slug: "la-clr", Name: "LA Clr", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "la-clr@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	for i := 0; i < 3; i++ {
		require.NoError(t, store.Record(ctx, &LoginAttempt{UserID: user.ID}))
	}

	err := store.ClearForUser(ctx, user.ID)
	require.NoError(t, err)

	count, err := store.CountSince(ctx, user.ID, time.Now().UTC().Add(-time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestLoginAttemptStore_DeleteOlderThan(t *testing.T) {
	store, tenantStore, userStore, ctx := newLoginAttemptFixture(t)
	truncateTables(t, "login_attempts", "sessions", "tenant_members", "users", "tenants")

	tenant := &Tenant{Slug: "la-old", Name: "LA Old", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "la-old@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	now := time.Now().UTC()

	require.NoError(t, store.Record(ctx, &LoginAttempt{
		UserID:      user.ID,
		AttemptedAt: now.Add(-2 * time.Hour),
	}))
	require.NoError(t, store.Record(ctx, &LoginAttempt{
		UserID:      user.ID,
		AttemptedAt: now.Add(-time.Hour),
	}))
	require.NoError(t, store.Record(ctx, &LoginAttempt{
		UserID:      user.ID,
		AttemptedAt: now,
	}))

	n, err := store.DeleteOlderThan(ctx, now.Add(-90*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}
