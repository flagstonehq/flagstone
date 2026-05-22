package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUserStore(t *testing.T) (*UserStore, context.Context) {
	t.Helper()
	skipIfShort(t)
	return NewUserStore(testPool), context.Background()
}

func TestUserStore_Create(t *testing.T) {
	store, ctx := newUserStore(t)
	truncateTables(t, "users")

	user := &User{
		Email: "alice@example.com",
	}

	err := store.Create(ctx, user)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, user.ID)
	assert.False(t, user.CreatedAt.IsZero())
	assert.False(t, user.UpdatedAt.IsZero())

	t.Run("duplicate email", func(t *testing.T) {
		dup := &User{Email: "alice@example.com"}
		err := store.Create(ctx, dup)
		assert.ErrorIs(t, err, ErrDuplicateKey)
	})

	t.Run("case insensitive duplicate", func(t *testing.T) {
		dup := &User{Email: "ALICE@EXAMPLE.COM"}
		err := store.Create(ctx, dup)
		assert.ErrorIs(t, err, ErrDuplicateKey)
	})
}

func TestUserStore_GetByEmail(t *testing.T) {
	store, ctx := newUserStore(t)
	truncateTables(t, "users")

	original := &User{
		Email:        "bob@example.com",
		PasswordHash: strPtr("hash123"),
	}
	require.NoError(t, store.Create(ctx, original))

	t.Run("found", func(t *testing.T) {
		got, err := store.GetByEmail(ctx, "bob@example.com")
		require.NoError(t, err)
		assert.Equal(t, original.ID, got.ID)
		assert.Equal(t, "hash123", *got.PasswordHash)
	})

	t.Run("case insensitive", func(t *testing.T) {
		got, err := store.GetByEmail(ctx, "BOB@EXAMPLE.COM")
		require.NoError(t, err)
		assert.Equal(t, original.ID, got.ID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.GetByEmail(ctx, "nobody@example.com")
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestUserStore_GetByID(t *testing.T) {
	store, ctx := newUserStore(t)
	truncateTables(t, "users")

	original := &User{
		Email:        "carol@example.com",
		PasswordHash: strPtr("hash456"),
	}
	require.NoError(t, store.Create(ctx, original))

	t.Run("found", func(t *testing.T) {
		got, err := store.GetByID(ctx, original.ID)
		require.NoError(t, err)
		assert.Equal(t, original.Email, got.Email)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.GetByID(ctx, uuid.New())
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestUserStore_UpdateLastLogin(t *testing.T) {
	store, ctx := newUserStore(t)
	truncateTables(t, "users")

	user := &User{Email: "dave@example.com"}
	require.NoError(t, store.Create(ctx, user))

	t.Run("success", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Microsecond)
		err := store.UpdateLastLogin(ctx, user.ID, now)
		require.NoError(t, err)

		got, err := store.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.True(t, got.LastLoginAt.Equal(now), "expected %v, got %v", now, got.LastLoginAt)
	})

	t.Run("not found", func(t *testing.T) {
		err := store.UpdateLastLogin(ctx, uuid.New(), time.Now())
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestUserStore_UpdatePasswordHash(t *testing.T) {
	store, ctx := newUserStore(t)
	truncateTables(t, "users")

	user := &User{Email: "eve@example.com"}
	require.NoError(t, store.Create(ctx, user))

	t.Run("success", func(t *testing.T) {
		err := store.UpdatePasswordHash(ctx, user.ID, "newhash")
		require.NoError(t, err)

		got, err := store.GetByEmail(ctx, user.Email)
		require.NoError(t, err)
		assert.Equal(t, "newhash", *got.PasswordHash)
	})

	t.Run("not found", func(t *testing.T) {
		err := store.UpdatePasswordHash(ctx, uuid.New(), "x")
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func TestUserStore_VerifyEmail(t *testing.T) {
	store, ctx := newUserStore(t)
	truncateTables(t, "users")

	user := &User{Email: "frank@example.com"}
	require.NoError(t, store.Create(ctx, user))

	t.Run("success", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Microsecond)
		err := store.VerifyEmail(ctx, user.ID, now)
		require.NoError(t, err)

		got, err := store.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.True(t, got.EmailVerifiedAt.Equal(now), "expected %v, got %v", now, got.EmailVerifiedAt)
	})

	t.Run("not found", func(t *testing.T) {
		err := store.VerifyEmail(ctx, uuid.New(), time.Now())
		assert.ErrorIs(t, err, ErrUserNotFound)
	})
}

func strPtr(s string) *string {
	return &s
}
