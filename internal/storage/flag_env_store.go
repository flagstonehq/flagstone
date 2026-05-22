package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FlagEnvironmentStore handles per-environment flag configuration.
type FlagEnvironmentStore struct {
	pool *pgxpool.Pool
}

// NewFlagEnvironmentStore creates a new FlagEnvironmentStore backed by the given connection pool.
func NewFlagEnvironmentStore(pool *pgxpool.Pool) *FlagEnvironmentStore {
	return &FlagEnvironmentStore{
		pool: pool,
	}
}

// Upsert creates or updates an environment-specific flag configuration.
func (s *FlagEnvironmentStore) Upsert(ctx context.Context, cfg *FlagEnvironment) error {
	const query = `
		INSERT INTO flag_environments (
			flag_id,
			environment_id,
			enabled,
			rules,
			default_value,
			updated_by
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (flag_id, environment_id)
		DO UPDATE SET
			enabled = EXCLUDED.enabled,
			rules = EXCLUDED.rules,
			default_value = EXCLUDED.default_value,
			updated_by = EXCLUDED.updated_by
		RETURNING version, created_at, updated_at
	`
	err := s.pool.QueryRow(
		ctx,
		query,
		cfg.FlagID,
		cfg.EnvironmentID,
		cfg.Enabled,
		cfg.Rules,
		cfg.DefaultValue,
		cfg.UpdatedBy,
	).Scan(&cfg.Version, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return fmt.Errorf("storage.FlagEnvironmentStore.Upsert: %w", err)
	}
	return nil
}

// Get retrieves a flag configuration for a specific environment.
// Returns ErrNotFound if the configuration does not exist.
func (s *FlagEnvironmentStore) Get(ctx context.Context, flagID, environmentID uuid.UUID) (*FlagEnvironment, error) {
	const query = `
		SELECT
			flag_id,
			environment_id,
			enabled,
			rules,
			default_value,
			version,
			created_at,
			updated_at,
			updated_by
		FROM flag_environments
		WHERE flag_id = $1
		  AND environment_id = $2
	`
	var (
		cfg          FlagEnvironment
		defaultValue []byte
		updatedBy    uuid.NullUUID
	)
	if err := s.pool.QueryRow(ctx, query, flagID, environmentID).Scan(
		&cfg.FlagID,
		&cfg.EnvironmentID,
		&cfg.Enabled,
		&cfg.Rules,
		&defaultValue,
		&cfg.Version,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
		&updatedBy,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage.FlagEnvironmentStore.Get: %w", err)
	}
	cfg.DefaultValue = rawMessagePtrFromBytes(defaultValue)
	cfg.UpdatedBy = uuidPtrFromNull(updatedBy)
	return &cfg, nil
}

// UpdateWithVersion updates a flag configuration using optimistic concurrency control.
// Returns ErrVersionConflict if the row was modified since the caller last read it.
func (s *FlagEnvironmentStore) UpdateWithVersion(ctx context.Context, cfg *FlagEnvironment, expectedVersion int64) error {
	const query = `
		UPDATE flag_environments
		SET enabled = $3, rules = $4, default_value = $5, updated_by = $6
		WHERE flag_id = $1
		  AND environment_id = $2
		  AND version = $7
		RETURNING version, updated_at
	`
	err := s.pool.QueryRow(
		ctx,
		query,
		cfg.FlagID,
		cfg.EnvironmentID,
		cfg.Enabled,
		cfg.Rules,
		cfg.DefaultValue,
		cfg.UpdatedBy,
		expectedVersion,
	).Scan(&cfg.Version, &cfg.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrVersionConflict
		}
		return fmt.Errorf("storage.FlagEnvironmentStore.UpdateWithVersion: %w", err)
	}
	return nil
}

// ListByEnvironment returns the flattened active flag configuration for an environment.
func (s *FlagEnvironmentStore) ListByEnvironment(ctx context.Context, environmentID uuid.UUID) ([]FlagEnvironmentConfig, error) {
	const query = `
		SELECT
			f.id,
			fe.environment_id,
			f.key,
			f.name,
			f.type,
			f.default_value AS flag_default_value,
			fe.enabled,
			fe.rules,
			fe.default_value AS environment_default_value,
			fe.version,
			fe.updated_at,
			fe.updated_by
		FROM flags f
		JOIN flag_environments fe ON fe.flag_id = f.id
		WHERE fe.environment_id = $1
		  AND f.archived_at IS NULL
		ORDER BY f.created_at ASC
	`
	rows, err := s.pool.Query(ctx, query, environmentID)
	if err != nil {
		return nil, fmt.Errorf("storage.FlagEnvironmentStore.ListByEnvironment: %w", err)
	}
	defer rows.Close()
	configs := make([]FlagEnvironmentConfig, 0)
	for rows.Next() {
		var (
			cfg                FlagEnvironmentConfig
			environmentDefault []byte
			updatedBy          uuid.NullUUID
		)
		if err := rows.Scan(
			&cfg.FlagID,
			&cfg.EnvironmentID,
			&cfg.FlagKey,
			&cfg.FlagName,
			&cfg.FlagType,
			&cfg.FlagDefaultValue,
			&cfg.Enabled,
			&cfg.Rules,
			&environmentDefault,
			&cfg.Version,
			&cfg.UpdatedAt,
			&updatedBy,
		); err != nil {
			return nil, fmt.Errorf("storage.FlagEnvironmentStore.ListByEnvironment: scan row: %w", err)
		}
		cfg.EnvironmentDefaultValue = rawMessagePtrFromBytes(environmentDefault)
		cfg.UpdatedBy = uuidPtrFromNull(updatedBy)
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage.FlagEnvironmentStore.ListByEnvironment: iterate rows: %w", err)
	}
	return configs, nil
}
