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

func newAuditFixture(t *testing.T) (*AuditStore, *TenantStore, *UserStore, context.Context) {
	t.Helper()
	skipIfShort(t)
	return NewAuditStore(testPool), NewTenantStore(testPool), NewUserStore(testPool), context.Background()
}

func addAuditEntry(t *testing.T, store *AuditStore, ctx context.Context, entry *AuditLogEntry) {
	t.Helper()
	require.NoError(t, store.Insert(ctx, entry))
}

func TestAuditStore_Insert(t *testing.T) {
	store, tenantStore, _, ctx := newAuditFixture(t)
	truncateTables(t, "audit_log", "tenants", "users")

	tenant := &Tenant{Slug: "audit-ins", Name: "Audit Ins", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))

	changes := json.RawMessage(`{"before":null,"after":{"key":"test"}}`)
	entry := &AuditLogEntry{
		TenantID:     tenant.ID,
		ActorType:    "system",
		Action:       "flag.created",
		ResourceType: "flag",
		Changes:      &changes,
	}

	err := store.Insert(ctx, entry)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, entry.ID)
	assert.False(t, entry.CreatedAt.IsZero())
}

func TestAuditStore_Query(t *testing.T) {
	store, tenantStore, _, ctx := newAuditFixture(t)
	truncateTables(t, "audit_log", "tenants", "users")

	tenant := &Tenant{Slug: "audit-q", Name: "Audit Q", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))

	nested := json.RawMessage(`{"before":null,"after":{}}`)
	for i := 0; i < 5; i++ {
		e := &AuditLogEntry{
			TenantID:     tenant.ID,
			ActorType:    "system",
			Action:       fmt.Sprintf("test.action.%d", i),
			ResourceType: "flag",
			ResourceID:   uuidPtr(uuid.New()),
			Changes:      &nested,
		}
		addAuditEntry(t, store, ctx, e)
		time.Sleep(time.Millisecond)
	}

	t.Run("no filters returns all", func(t *testing.T) {
		page, err := store.Query(ctx, AuditLogFilter{
			TenantID: tenant.ID,
			Limit:    100,
		})
		require.NoError(t, err)
		assert.Len(t, page.Entries, 5)
		assert.Equal(t, int64(5), page.Total)
	})

	t.Run("paginates", func(t *testing.T) {
		page, err := store.Query(ctx, AuditLogFilter{
			TenantID: tenant.ID,
			Limit:    2,
			Offset:   0,
		})
		require.NoError(t, err)
		assert.Len(t, page.Entries, 2)
		assert.Equal(t, int64(5), page.Total)
	})

	t.Run("scoped to tenant", func(t *testing.T) {
		other := &Tenant{Slug: "audit-other", Name: "Other", Plan: "free"}
		require.NoError(t, tenantStore.Create(ctx, other))

		page, err := store.Query(ctx, AuditLogFilter{
			TenantID: other.ID,
			Limit:    100,
		})
		require.NoError(t, err)
		assert.Empty(t, page.Entries)
		assert.Equal(t, int64(0), page.Total)
	})
}

func TestAuditStore_QueryWithFilters(t *testing.T) {
	store, tenantStore, userStore, ctx := newAuditFixture(t)
	truncateTables(t, "audit_log", "tenants", "users")

	tenant := &Tenant{Slug: "audit-filt", Name: "Audit Filt", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))

	actor := &User{Email: "audit-actor@example.com"}
	require.NoError(t, userStore.Create(ctx, actor))

	nested := json.RawMessage(`{"before":null,"after":{}}`)

	flagID := uuid.New()
	e1 := &AuditLogEntry{
		TenantID:     tenant.ID,
		ActorType:    "user",
		Action:       "flag.created",
		ResourceType: "flag",
		ResourceID:   &flagID,
		Changes:      &nested,
	}
	addAuditEntry(t, store, ctx, e1)

	e2 := &AuditLogEntry{
		TenantID:     tenant.ID,
		ActorType:    "user",
		Action:       "flag.updated",
		ResourceType: "flag",
		ResourceID:   &flagID,
		Changes:      &nested,
	}
	addAuditEntry(t, store, ctx, e2)

	e3 := &AuditLogEntry{
		TenantID:     tenant.ID,
		ActorType:    "system",
		Action:       "env.created",
		ResourceType: "environment",
		Changes:      &nested,
	}
	addAuditEntry(t, store, ctx, e3)

	actorID := actor.ID
	e4 := &AuditLogEntry{
		TenantID:     tenant.ID,
		ActorID:      &actorID,
		ActorType:    "api_key",
		Action:       "key.revoked",
		ResourceType: "api_key",
		Changes:      &nested,
	}
	addAuditEntry(t, store, ctx, e4)

	matchingAction := "flag.created"
	matchingType := "environment"
	t.Run("filter by action", func(t *testing.T) {
		page, err := store.Query(ctx, AuditLogFilter{
			TenantID: tenant.ID,
			Action:   &matchingAction,
			Limit:    100,
		})
		require.NoError(t, err)
		assert.Len(t, page.Entries, 1)
		assert.Equal(t, int64(1), page.Total)
	})

	t.Run("filter by resource_type", func(t *testing.T) {
		page, err := store.Query(ctx, AuditLogFilter{
			TenantID:     tenant.ID,
			ResourceType: &matchingType,
			Limit:        100,
		})
		require.NoError(t, err)
		assert.Len(t, page.Entries, 1)
		assert.Equal(t, int64(1), page.Total)
	})

	t.Run("filter by actor_id", func(t *testing.T) {
		page, err := store.Query(ctx, AuditLogFilter{
			TenantID: tenant.ID,
			ActorID:  &actorID,
			Limit:    100,
		})
		require.NoError(t, err)
		assert.Len(t, page.Entries, 1)
		assert.Equal(t, int64(1), page.Total)
	})
}
