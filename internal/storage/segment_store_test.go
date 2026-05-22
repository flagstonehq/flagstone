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

func newSegmentFixture(t *testing.T) (*SegmentStore, *ProjectStore, *TenantStore, context.Context) {
	t.Helper()
	skipIfShort(t)
	return NewSegmentStore(testPool), NewProjectStore(testPool), NewTenantStore(testPool), context.Background()
}

func TestSegmentStore_Create(t *testing.T) {
	store, projectStore, tenantStore, ctx := newSegmentFixture(t)
	truncateTables(t, "segments", "projects", "tenants")

	tenant := &Tenant{Slug: "seg-tenant", Name: "Seg Tenant", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "seg-proj", Name: "Seg Proj"}
	require.NoError(t, projectStore.Create(ctx, project))

	segment := &Segment{
		ProjectID:   project.ID,
		Key:         "premium-users",
		Name:        "Premium Users",
		Description: strPtr("Users with premium plan"),
		Rules:       json.RawMessage(`{"all":[{"attr":"plan","op":"eq","value":"premium"}]}`),
	}

	err := store.Create(ctx, segment)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, segment.ID)

	t.Run("duplicate key", func(t *testing.T) {
		dup := &Segment{
			ProjectID: project.ID,
			Key:       "premium-users",
			Name:      "Dup",
			Rules:     json.RawMessage(`[]`),
		}
		err := store.Create(ctx, dup)
		assert.ErrorIs(t, err, ErrDuplicateKey)
	})
}

func TestSegmentStore_GetByKey(t *testing.T) {
	store, projectStore, tenantStore, ctx := newSegmentFixture(t)
	truncateTables(t, "segments", "projects", "tenants")

	tenant := &Tenant{Slug: "seg-get", Name: "Seg Get", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "seg-get-proj", Name: "Seg Get Proj"}
	require.NoError(t, projectStore.Create(ctx, project))

	original := &Segment{
		ProjectID: project.ID,
		Key:       "beta-testers",
		Name:      "Beta Testers",
		Rules:     json.RawMessage(`[]`),
	}
	require.NoError(t, store.Create(ctx, original))

	t.Run("found", func(t *testing.T) {
		got, err := store.GetByKey(ctx, project.ID, "beta-testers")
		require.NoError(t, err)
		assert.Equal(t, original.ID, got.ID)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := store.GetByKey(ctx, project.ID, "nope")
		assert.ErrorIs(t, err, ErrSegmentNotFound)
	})

	t.Run("archived returns not found", func(t *testing.T) {
		require.NoError(t, store.Archive(ctx, original.ID, time.Now()))
		_, err := store.GetByKey(ctx, project.ID, "beta-testers")
		assert.ErrorIs(t, err, ErrSegmentNotFound)
	})
}

func TestSegmentStore_ListByProject(t *testing.T) {
	store, projectStore, tenantStore, ctx := newSegmentFixture(t)
	truncateTables(t, "segments", "projects", "tenants")

	tenant := &Tenant{Slug: "seg-list", Name: "Seg List", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "seg-list-proj", Name: "Seg List Proj"}
	require.NoError(t, projectStore.Create(ctx, project))

	for i := 0; i < 3; i++ {
		s := &Segment{
			ProjectID: project.ID,
			Key:       fmt.Sprintf("seg-%d", i),
			Name:      fmt.Sprintf("Seg %d", i),
			Rules:     json.RawMessage(`[]`),
		}
		require.NoError(t, store.Create(ctx, s))
	}

	segments, err := store.ListByProject(ctx, project.ID)
	require.NoError(t, err)
	assert.Len(t, segments, 3)

	t.Run("excludes archived", func(t *testing.T) {
		archived := &Segment{
			ProjectID: project.ID,
			Key:       "to-archive",
			Name:      "To Archive",
			Rules:     json.RawMessage(`[]`),
		}
		require.NoError(t, store.Create(ctx, archived))
		require.NoError(t, store.Archive(ctx, archived.ID, time.Now()))

		segments, err := store.ListByProject(ctx, project.ID)
		require.NoError(t, err)
		assert.Len(t, segments, 3)
	})
}

func TestSegmentStore_Update(t *testing.T) {
	store, projectStore, tenantStore, ctx := newSegmentFixture(t)
	truncateTables(t, "segments", "projects", "tenants")

	tenant := &Tenant{Slug: "seg-upd", Name: "Seg Upd", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "seg-upd-proj", Name: "Seg Upd Proj"}
	require.NoError(t, projectStore.Create(ctx, project))

	original := &Segment{
		ProjectID: project.ID,
		Key:       "before",
		Name:      "Before",
		Rules:     json.RawMessage(`[{"attr":"a","op":"eq","value":"1"}]`),
	}
	require.NoError(t, store.Create(ctx, original))

	original.Key = "after"
	original.Name = "After"
	original.Rules = json.RawMessage(`[{"attr":"b","op":"eq","value":"2"}]`)

	err := store.Update(ctx, original)
	require.NoError(t, err)

	got, err := store.GetByKey(ctx, project.ID, "after")
	require.NoError(t, err)
	assert.Equal(t, "After", got.Name)

	t.Run("not found", func(t *testing.T) {
		s := &Segment{ID: uuid.New(), Key: "x", Name: "x", Rules: json.RawMessage(`[]`)}
		err := store.Update(ctx, s)
		assert.ErrorIs(t, err, ErrSegmentNotFound)
	})
}

func TestSegmentStore_Archive(t *testing.T) {
	store, projectStore, tenantStore, ctx := newSegmentFixture(t)
	truncateTables(t, "segments", "projects", "tenants")

	tenant := &Tenant{Slug: "seg-arch", Name: "Seg Arch", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "seg-arch-proj", Name: "Seg Arch Proj"}
	require.NoError(t, projectStore.Create(ctx, project))

	original := &Segment{
		ProjectID: project.ID,
		Key:       "arch-me",
		Name:      "Arch Me",
		Rules:     json.RawMessage(`[]`),
	}
	require.NoError(t, store.Create(ctx, original))

	err := store.Archive(ctx, original.ID, time.Now())
	require.NoError(t, err)

	_, err = store.GetByKey(ctx, project.ID, "arch-me")
	assert.ErrorIs(t, err, ErrSegmentNotFound)

	t.Run("not found", func(t *testing.T) {
		err := store.Archive(ctx, uuid.New(), time.Now())
		assert.ErrorIs(t, err, ErrSegmentNotFound)
	})
}
