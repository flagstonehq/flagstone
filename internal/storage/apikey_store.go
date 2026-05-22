package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// APIKeyStore handles persistence and retrieval of API keys.
type APIKeyStore struct {
	pool *pgxpool.Pool
}

// NewAPIKeyStore creates a new APIKeyStore backed by the given connection pool.
func NewAPIKeyStore(pool *pgxpool.Pool) *APIKeyStore {
	return &APIKeyStore{
		pool: pool,
	}
}

// Create inserts a new API key. Generates an ID if not set.
// Returns ErrDuplicateKey if the key hash already exists.
func (s *APIKeyStore) Create(ctx context.Context, key *APIKey) error {
	const query = `
			INSERT INTO api_keys (
				id,
				environment_id,
				name,
				key_hash,
				key_prefix,
				last_used_at,
				expires_at,
				revoked_at,
				created_by
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING created_at
	`

	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}

	err := s.pool.QueryRow(
		ctx,
		query,
		key.ID,
		key.EnvironmentID,
		key.Name,
		key.KeyHash,
		key.KeyPrefix,
		key.LastUsedAt,
		key.ExpiresAt,
		key.RevokedAt,
		key.CreatedBy,
	).Scan(&key.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("storage.APIKeyStore.Create: %w", err)
	}

	return nil
}

// GetByHash retrieves an active API key by its SHA-256 hash.
// Returns ErrNotFound if the key does not exist or is revoked.
func (s *APIKeyStore) GetByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	const query = `
		SELECT
			id,
			environment_id,
			name,
			key_hash,
			key_prefix,
			last_used_at,
			expires_at,
			revoked_at,
			created_at,
			created_by
		FROM api_keys
		WHERE key_hash = $1
		  AND revoked_at IS NULL
	`

	var (
		key        APIKey
		lastUsedAt sql.NullTime
		expiresAt  sql.NullTime
		revokedAt  sql.NullTime
		createdBy  uuid.NullUUID
	)

	if err := s.pool.QueryRow(ctx, query, keyHash).Scan(
		&key.ID,
		&key.EnvironmentID,
		&key.Name,
		&key.KeyHash,
		&key.KeyPrefix,
		&lastUsedAt,
		&expiresAt,
		&revokedAt,
		&key.CreatedAt,
		&createdBy,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage.APIKeyStore.GetByHash: %w", err)
	}

	key.LastUsedAt = timePtrFromNull(lastUsedAt)
	key.ExpiresAt = timePtrFromNull(expiresAt)
	key.RevokedAt = timePtrFromNull(revokedAt)
	key.CreatedBy = uuidPtrFromNull(createdBy)

	return &key, nil
}

// ListByEnvironment returns all API keys for an environment ordered by creation time.
func (s *APIKeyStore) ListByEnvironment(ctx context.Context, environmentID uuid.UUID) ([]APIKey, error) {
	const query = `
			SELECT
				id,
				environment_id,
				name,
				key_hash,
				key_prefix,
				last_used_at,
				expires_at,
				revoked_at,
				created_at,
				created_by
			FROM api_keys
			WHERE environment_id = $1
			ORDER BY created_at ASC
		`

	rows, err := s.pool.Query(ctx, query, environmentID)
	if err != nil {
		return nil, fmt.Errorf("storage.APIKeyStore.ListByEnvironment: %w", err)
	}
	defer rows.Close()

	keys := make([]APIKey, 0)
	for rows.Next() {
		var (
			key        APIKey
			lastUsedAt sql.NullTime
			expiresAt  sql.NullTime
			revokedAt  sql.NullTime
			createdBy  uuid.NullUUID
		)

		if err := rows.Scan(
			&key.ID,
			&key.EnvironmentID,
			&key.Name,
			&key.KeyHash,
			&key.KeyPrefix,
			&lastUsedAt,
			&expiresAt,
			&revokedAt,
			&key.CreatedAt,
			&createdBy,
		); err != nil {
			return nil, fmt.Errorf("storage.APIKeyStore.ListByEnvironment: scan row: %w", err)
		}

		key.LastUsedAt = timePtrFromNull(lastUsedAt)
		key.ExpiresAt = timePtrFromNull(expiresAt)
		key.RevokedAt = timePtrFromNull(revokedAt)
		key.CreatedBy = uuidPtrFromNull(createdBy)
		keys = append(keys, key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage.APIKeyStore.ListByEnvironment: iterate rows: %w", err)
	}

	return keys, nil
}

// Revoke marks an API key as revoked at the provided timestamp.
// Returns ErrNotFound if the key does not exist.
func (s *APIKeyStore) Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error {
	const query = `
		UPDATE api_keys
		SET revoked_at = $2
		WHERE id = $1
	`

	tag, err := s.pool.Exec(ctx, query, id, revokedAt)
	if err != nil {
		return fmt.Errorf("storage.APIKeyStore.Revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateLastUsed updates last_used_at for an API key.
// Returns ErrNotFound if the key does not exist.
func (s *APIKeyStore) UpdateLastUsed(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	const query = `
			UPDATE api_keys
			SET last_used_at = $2
			WHERE id = $1
		`

	tag, err := s.pool.Exec(ctx, query, id, usedAt)
	if err != nil {
		return fmt.Errorf("storage.APIKeyStore.UpdateLastUsed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func uuidPtrFromNull(v uuid.NullUUID) *uuid.UUID {
	if !v.Valid {
		return nil
	}

	id := v.UUID
	return &id
}
