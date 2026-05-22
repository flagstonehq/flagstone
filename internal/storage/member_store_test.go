package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMemberFixture(t *testing.T) (*MemberStore, *TenantStore, *UserStore, context.Context) {
	t.Helper()
	skipIfShort(t)
	return NewMemberStore(testPool), NewTenantStore(testPool), NewUserStore(testPool), context.Background()
}

func TestMemberStore_AddAndGetRole(t *testing.T) {
	store, tenantStore, userStore, ctx := newMemberFixture(t)
	truncateTables(t, "tenant_members", "users", "tenants")

	tenant := &Tenant{Slug: "member-tenant", Name: "Member Tenant", Plan: "team"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "member@example.com"}
	require.NoError(t, userStore.Create(ctx, user))

	t.Run("add and get role", func(t *testing.T) {
		err := store.Add(ctx, &TenantMember{TenantID: tenant.ID, UserID: user.ID, Role: "admin"})
		require.NoError(t, err)

		role, err := store.GetRole(ctx, tenant.ID, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "admin", role)
	})

	t.Run("duplicate add", func(t *testing.T) {
		err := store.Add(ctx, &TenantMember{TenantID: tenant.ID, UserID: user.ID, Role: "member"})
		assert.ErrorIs(t, err, ErrDuplicateKey)
	})

	t.Run("get role not found", func(t *testing.T) {
		_, err := store.GetRole(ctx, tenant.ID, uuid.New())
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestMemberStore_ListByTenant(t *testing.T) {
	store, tenantStore, userStore, ctx := newMemberFixture(t)
	truncateTables(t, "tenant_members", "users", "tenants")

	tenant := &Tenant{Slug: "list-tenant", Name: "List Tenant", Plan: "team"}
	require.NoError(t, tenantStore.Create(ctx, tenant))

	users := make([]*User, 3)
	for i := 0; i < 3; i++ {
		u := &User{Email: fmt.Sprintf("list%d@example.com", i)}
		require.NoError(t, userStore.Create(ctx, u))
		require.NoError(t, store.Add(ctx, &TenantMember{TenantID: tenant.ID, UserID: u.ID, Role: "member"}))
		users[i] = u
	}

	members, err := store.ListByTenant(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Len(t, members, 3)

	t.Run("scoped to tenant", func(t *testing.T) {
		other := &Tenant{Slug: "other-tenant", Name: "Other", Plan: "free"}
		require.NoError(t, tenantStore.Create(ctx, other))

		otherMembers, err := store.ListByTenant(ctx, other.ID)
		require.NoError(t, err)
		assert.Empty(t, otherMembers)
	})
}

func TestMemberStore_UpdateRole(t *testing.T) {
	store, tenantStore, userStore, ctx := newMemberFixture(t)
	truncateTables(t, "tenant_members", "users", "tenants")

	tenant := &Tenant{Slug: "upd-tenant", Name: "Upd Tenant", Plan: "team"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "upd@example.com"}
	require.NoError(t, userStore.Create(ctx, user))
	require.NoError(t, store.Add(ctx, &TenantMember{TenantID: tenant.ID, UserID: user.ID, Role: "member"}))

	t.Run("success", func(t *testing.T) {
		err := store.UpdateRole(ctx, tenant.ID, user.ID, "admin")
		require.NoError(t, err)

		role, err := store.GetRole(ctx, tenant.ID, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "admin", role)
	})

	t.Run("not found", func(t *testing.T) {
		err := store.UpdateRole(ctx, tenant.ID, uuid.New(), "admin")
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestMemberStore_Remove(t *testing.T) {
	store, tenantStore, userStore, ctx := newMemberFixture(t)
	truncateTables(t, "tenant_members", "users", "tenants")

	tenant := &Tenant{Slug: "rm-tenant", Name: "Rm Tenant", Plan: "team"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	user := &User{Email: "rm@example.com"}
	require.NoError(t, userStore.Create(ctx, user))
	require.NoError(t, store.Add(ctx, &TenantMember{TenantID: tenant.ID, UserID: user.ID, Role: "member"}))

	t.Run("success", func(t *testing.T) {
		err := store.Remove(ctx, tenant.ID, user.ID)
		require.NoError(t, err)

		_, err = store.GetRole(ctx, tenant.ID, user.ID)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("not found", func(t *testing.T) {
		err := store.Remove(ctx, tenant.ID, uuid.New())
		assert.ErrorIs(t, err, ErrNotFound)
	})
}
