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

//nolint:gocritic // fixture factory returning >5 results for test setup
func newFlagEnvFixture(t *testing.T) (
	*FlagEnvironmentStore, *FlagStore,
	*EnvironmentStore, *ProjectStore, *TenantStore,
	context.Context,
) {
	t.Helper()
	skipIfShort(t)
	return NewFlagEnvironmentStore(testPool), NewFlagStore(testPool),
		NewEnvironmentStore(testPool), NewProjectStore(testPool),
		NewTenantStore(testPool), context.Background()
}

func TestFlagEnvironmentStore_Upsert(t *testing.T) {
	feStore, fStore, envStore, projStore, tenantStore, ctx := newFlagEnvFixture(t)
	truncateTables(t, "flag_environments", "flags", "environments", "projects", "tenants")

	tenant := &Tenant{Slug: "fe-upsert", Name: "FE Upsert", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "fe-upsert-proj", Name: "FE Upsert Proj"}
	require.NoError(t, projStore.Create(ctx, project))
	env := &Environment{ProjectID: project.ID, Slug: "fe-upsert-env", Name: "FE Upsert Env"}
	require.NoError(t, envStore.Create(ctx, env))
	flag := &Flag{
		ProjectID:    project.ID,
		Key:          "fe-upsert-flag",
		Name:         "FE Upsert Flag",
		Type:         "boolean",
		DefaultValue: json.RawMessage(`false`),
	}
	require.NoError(t, fStore.Create(ctx, flag))

	t.Run("create", func(t *testing.T) {
		cfg := &FlagEnvironment{
			FlagID:        flag.ID,
			EnvironmentID: env.ID,
			Enabled:       true,
			Rules:         json.RawMessage(`[]`),
		}
		err := feStore.Upsert(ctx, cfg)
		require.NoError(t, err)
		assert.Equal(t, int64(1), cfg.Version)
	})

	t.Run("update", func(t *testing.T) {
		cfg := &FlagEnvironment{
			FlagID:        flag.ID,
			EnvironmentID: env.ID,
			Enabled:       false,
			Rules:         json.RawMessage(`[{"op":"eq","attr":"country","value":"AR"}]`),
		}
		err := feStore.Upsert(ctx, cfg)
		require.NoError(t, err)
		assert.Equal(t, int64(2), cfg.Version)

		got, err := feStore.Get(ctx, flag.ID, env.ID)
		require.NoError(t, err)
		assert.False(t, got.Enabled)
	})
}

func TestFlagEnvironmentStore_Get(t *testing.T) {
	feStore, fStore, envStore, projStore, tenantStore, ctx := newFlagEnvFixture(t)
	truncateTables(t, "flag_environments", "flags", "environments", "projects", "tenants")

	tenant := &Tenant{Slug: "fe-get", Name: "FE Get", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "fe-get-proj", Name: "FE Get Proj"}
	require.NoError(t, projStore.Create(ctx, project))
	env := &Environment{ProjectID: project.ID, Slug: "fe-get-env", Name: "FE Get Env"}
	require.NoError(t, envStore.Create(ctx, env))
	flag := &Flag{
		ProjectID:    project.ID,
		Key:          "fe-get-flag",
		Name:         "FE Get Flag",
		Type:         "boolean",
		DefaultValue: json.RawMessage(`false`),
	}
	require.NoError(t, fStore.Create(ctx, flag))

	cfg := &FlagEnvironment{
		FlagID:        flag.ID,
		EnvironmentID: env.ID,
		Enabled:       true,
		Rules:         json.RawMessage(`[{"op":"in","attr":"country","values":["US","CA"]}]`),
	}
	require.NoError(t, feStore.Upsert(ctx, cfg))

	t.Run("found", func(t *testing.T) {
		got, err := feStore.Get(ctx, flag.ID, env.ID)
		require.NoError(t, err)
		assert.True(t, got.Enabled)
		assert.Equal(t, int64(1), got.Version)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := feStore.Get(ctx, flag.ID, uuid.New())
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestFlagEnvironmentStore_UpdateWithVersion(t *testing.T) {
	feStore, fStore, envStore, projStore, tenantStore, ctx := newFlagEnvFixture(t)
	truncateTables(t, "flag_environments", "flags", "environments", "projects", "tenants")

	tenant := &Tenant{Slug: "fe-occ", Name: "FE OCC", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "fe-occ-proj", Name: "FE OCC Proj"}
	require.NoError(t, projStore.Create(ctx, project))
	env := &Environment{ProjectID: project.ID, Slug: "fe-occ-env", Name: "FE OCC Env"}
	require.NoError(t, envStore.Create(ctx, env))
	flag := &Flag{
		ProjectID:    project.ID,
		Key:          "fe-occ-flag",
		Name:         "FE OCC Flag",
		Type:         "boolean",
		DefaultValue: json.RawMessage(`false`),
	}
	require.NoError(t, fStore.Create(ctx, flag))

	cfg := &FlagEnvironment{
		FlagID:        flag.ID,
		EnvironmentID: env.ID,
		Enabled:       true,
		Rules:         json.RawMessage(`[]`),
	}
	require.NoError(t, feStore.Upsert(ctx, cfg))

	t.Run("success", func(t *testing.T) {
		update := &FlagEnvironment{
			FlagID:        flag.ID,
			EnvironmentID: env.ID,
			Enabled:       false,
			Rules:         json.RawMessage(`[]`),
		}
		err := feStore.UpdateWithVersion(ctx, update, 1)
		require.NoError(t, err)
		assert.Equal(t, int64(2), update.Version)

		got, err := feStore.Get(ctx, flag.ID, env.ID)
		require.NoError(t, err)
		assert.False(t, got.Enabled)
	})

	t.Run("version conflict", func(t *testing.T) {
		conflict := &FlagEnvironment{
			FlagID:        flag.ID,
			EnvironmentID: env.ID,
			Enabled:       true,
			Rules:         json.RawMessage(`[]`),
		}
		err := feStore.UpdateWithVersion(ctx, conflict, 1)
		assert.ErrorIs(t, err, ErrVersionConflict)
	})
}

func TestFlagEnvironmentStore_ListByEnvironment(t *testing.T) {
	feStore, fStore, envStore, projStore, tenantStore, ctx := newFlagEnvFixture(t)
	truncateTables(t, "flag_environments", "flags", "environments", "projects", "tenants")

	tenant := &Tenant{Slug: "fe-bulk", Name: "FE Bulk", Plan: "free"}
	require.NoError(t, tenantStore.Create(ctx, tenant))
	project := &Project{TenantID: tenant.ID, Slug: "fe-bulk-proj", Name: "FE Bulk Proj"}
	require.NoError(t, projStore.Create(ctx, project))
	env := &Environment{ProjectID: project.ID, Slug: "fe-bulk-env", Name: "FE Bulk Env"}
	require.NoError(t, envStore.Create(ctx, env))

	flags := make([]*Flag, 3)
	for i := 0; i < 3; i++ {
		f := &Flag{
			ProjectID:    project.ID,
			Key:          fmt.Sprintf("bulk-%d", i),
			Name:         fmt.Sprintf("Bulk %d", i),
			Type:         "boolean",
			DefaultValue: json.RawMessage(`false`),
		}
		require.NoError(t, fStore.Create(ctx, f))

		cfg := &FlagEnvironment{
			FlagID:        f.ID,
			EnvironmentID: env.ID,
			Enabled:       i%2 == 0,
			Rules:         json.RawMessage(`[]`),
		}
		require.NoError(t, feStore.Upsert(ctx, cfg))
		flags[i] = f
	}

	configs, err := feStore.ListByEnvironment(ctx, env.ID)
	require.NoError(t, err)
	assert.Len(t, configs, 3)

	t.Run("excludes archived flags", func(t *testing.T) {
		aFlag := &Flag{
			ProjectID:    project.ID,
			Key:          "bulk-archived",
			Name:         "Bulk Archived",
			Type:         "boolean",
			DefaultValue: json.RawMessage(`false`),
		}
		require.NoError(t, fStore.Create(ctx, aFlag))
		cfg := &FlagEnvironment{
			FlagID:        aFlag.ID,
			EnvironmentID: env.ID,
			Enabled:       true,
			Rules:         json.RawMessage(`[]`),
		}
		require.NoError(t, feStore.Upsert(ctx, cfg))
		require.NoError(t, fStore.Archive(ctx, aFlag.ID, time.Now()))

		configs, err := feStore.ListByEnvironment(ctx, env.ID)
		require.NoError(t, err)
		assert.Len(t, configs, 3)
	})

	t.Run("scoped to environment", func(t *testing.T) {
		otherEnv := &Environment{ProjectID: project.ID, Slug: "fe-bulk-other-env", Name: "Other Env"}
		require.NoError(t, envStore.Create(ctx, otherEnv))

		empty, err := feStore.ListByEnvironment(ctx, otherEnv.ID)
		require.NoError(t, err)
		assert.Empty(t, empty)
	})
}
