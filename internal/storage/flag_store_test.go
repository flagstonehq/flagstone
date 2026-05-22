package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFlagFixture(t *testing.T) (*FlagStore, *ProjectStore, *TenantStore, context.Context) {
	t.Helper()
	skipIfShort(t)
	return NewFlagStore(testPool), NewProjectStore(testPool), NewTenantStore(testPool), context.Background()
}

func boolDefault(v bool) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestFlagStore_Create(t *testing.T) {
	store, projectStore, tenantStore, ctx := newFlagFixture(t)
	truncateTables(t, "flags", "projects", "tenants")

	tenant := &Tenant{Slug: "flag-tenant", Name: "Flag Tenant", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "flag-proj", Name: "Flag Proj"}
	require.NoError(t, projectStore.Create(ctx, project))

	flag := &Flag{
		ProjectID:    project.ID,
		Key:          "new-checkout",
		Name:         "New Checkout",
		Type:         "boolean",
		DefaultValue: boolDefault(false),
	}

	err := store.Create(ctx, flag)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, flag.ID)

	t.Run("duplicate key", func(t *testing.T) {
		dup := &Flag{
			ProjectID:    project.ID,
			Key:          "new-checkout",
			Name:         "Dup",
			Type:         "boolean",
			DefaultValue: boolDefault(true),
		}
		err := store.Create(ctx, dup)
		assert.ErrorIs(t, err, ErrDuplicateKey)
	})

	t.Run("different project same key", func(t *testing.T) {
		otherProject := &Project{TenantID: tenant.ID, Slug: "other-proj", Name: "Other"}
		require.NoError(t, projectStore.Create(ctx, otherProject))

		ok := &Flag{
			ProjectID:    otherProject.ID,
			Key:          "new-checkout",
			Name:         "Ok",
			Type:         "boolean",
			DefaultValue: boolDefault(true),
		}
		err := store.Create(ctx, ok)
		assert.NoError(t, err)
	})
}

func TestFlagStore_GetByKey(t *testing.T) {
	store, projectStore, tenantStore, ctx := newFlagFixture(t)
	truncateTables(t, "flags", "projects", "tenants")

	tenant := &Tenant{Slug: "flag-get", Name: "FlagGet", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "flag-get-proj", Name: "Flag Get Proj"}
	require.NoError(t, projectStore.Create(ctx, project))

	original := &Flag{
		ProjectID:    project.ID,
		Key:          "feature-x",
		Name:         "Feature X",
		Type:         "boolean",
		DefaultValue: boolDefault(false),
	}
	require.NoError(t, store.Create(ctx, original))

	t.Run("found", func(t *testing.T) {
		got, err := store.GetByKey(ctx, project.ID, "feature-x")
		require.NoError(t, err)
		assert.Equal(t, original.ID, got.ID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.GetByKey(ctx, project.ID, "nope")
		assert.ErrorIs(t, err, ErrFlagNotFound)
	})

	t.Run("archived returns not found", func(t *testing.T) {
		require.NoError(t, store.Archive(ctx, original.ID, time.Now()))
		_, err := store.GetByKey(ctx, project.ID, "feature-x")
		assert.ErrorIs(t, err, ErrFlagNotFound)
	})
}

func TestFlagStore_ListByProject(t *testing.T) {
	store, projectStore, tenantStore, ctx := newFlagFixture(t)
	truncateTables(t, "flags", "projects", "tenants")

	tenant := &Tenant{Slug: "flag-list", Name: "FlagList", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "flag-list-proj", Name: "Flag List Proj"}
	require.NoError(t, projectStore.Create(ctx, project))

	for i := 0; i < 3; i++ {
		f := &Flag{
			ProjectID:    project.ID,
			Key:          fmt.Sprintf("flag-%d", i),
			Name:         fmt.Sprintf("Flag %d", i),
			Type:         "boolean",
			DefaultValue: boolDefault(i%2 == 0),
		}
		require.NoError(t, store.Create(ctx, f))
	}

	flags, err := store.ListByProject(ctx, project.ID)
	require.NoError(t, err)
	assert.Len(t, flags, 3)

	t.Run("excludes archived", func(t *testing.T) {
		archived := &Flag{
			ProjectID:    project.ID,
			Key:          "to-archive",
			Name:         "To Archive",
			Type:         "boolean",
			DefaultValue: boolDefault(false),
		}
		require.NoError(t, store.Create(ctx, archived))
		require.NoError(t, store.Archive(ctx, archived.ID, time.Now()))

		flags, err := store.ListByProject(ctx, project.ID)
		require.NoError(t, err)
		assert.Len(t, flags, 3)
	})
}

func TestFlagStore_Update(t *testing.T) {
	store, projectStore, tenantStore, ctx := newFlagFixture(t)
	truncateTables(t, "flags", "projects", "tenants")

	tenant := &Tenant{Slug: "flag-upd", Name: "FlagUpd", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "flag-upd-proj", Name: "Flag Upd Proj"}
	require.NoError(t, projectStore.Create(ctx, project))

	original := &Flag{
		ProjectID:    project.ID,
		Key:          "before",
		Name:         "Before",
		Type:         "boolean",
		DefaultValue: boolDefault(false),
	}
	require.NoError(t, store.Create(ctx, original))

	original.Key = "after"
	original.Name = "After"
	original.DefaultValue = boolDefault(true)

	err := store.Update(ctx, original)
	require.NoError(t, err)

	got, err := store.GetByKey(ctx, project.ID, "after")
	require.NoError(t, err)
	assert.Equal(t, "After", got.Name)
	assert.JSONEq(t, `true`, string(got.DefaultValue))

	t.Run("not found", func(t *testing.T) {
		f := &Flag{ID: uuid.New(), Key: "x", Name: "x", Type: "boolean", DefaultValue: boolDefault(false)}
		err := store.Update(ctx, f)
		assert.ErrorIs(t, err, ErrFlagNotFound)
	})
}

func TestFlagStore_Archive(t *testing.T) {
	store, projectStore, tenantStore, ctx := newFlagFixture(t)
	truncateTables(t, "flags", "projects", "tenants")

	tenant := &Tenant{Slug: "flag-arch", Name: "FlagArch", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "flag-arch-proj", Name: "Flag Arch Proj"}
	require.NoError(t, projectStore.Create(ctx, project))

	original := &Flag{
		ProjectID:    project.ID,
		Key:          "arch-me",
		Name:         "Arch Me",
		Type:         "boolean",
		DefaultValue: boolDefault(false),
	}
	require.NoError(t, store.Create(ctx, original))

	err := store.Archive(ctx, original.ID, time.Now())
	require.NoError(t, err)

	_, err = store.GetByKey(ctx, project.ID, "arch-me")
	assert.ErrorIs(t, err, ErrFlagNotFound)

	t.Run("not found", func(t *testing.T) {
		err := store.Archive(ctx, uuid.New(), time.Now())
		assert.ErrorIs(t, err, ErrFlagNotFound)
	})
}
