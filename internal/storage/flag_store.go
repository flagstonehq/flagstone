package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FlagStore handles persistence and retrieval of flags.
type FlagStore struct {
	pool *pgxpool.Pool
}

// NewFlagStore creates a new FlagStore backed by the given connection pool.
func NewFlagStore(pool *pgxpool.Pool) *FlagStore {
	return &FlagStore{
		pool: pool,
	}
}

// Create inserts a new flag. Generates an ID if not set.
// Returns ErrDuplicateKey if the project already has a flag with the same key.
func (s *FlagStore) Create(ctx context.Context, flag *Flag) error {
	const query = `
		INSERT INTO flags (
			id,
			project_id,
			key,
			name,
			description,
			type,
			default_value,
			archived_at,
			created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`
	if flag.ID == uuid.Nil {
		flag.ID = uuid.New()
	}
	flag.Key = strings.TrimSpace(flag.Key)
	flag.Name = strings.TrimSpace(flag.Name)
	err := s.pool.QueryRow(
		ctx,
		query,
		flag.ID,
		flag.ProjectID,
		flag.Key,
		flag.Name,
		flag.Description,
		flag.Type,
		flag.DefaultValue,
		flag.ArchivedAt,
		flag.CreatedBy,
	).Scan(&flag.CreatedAt, &flag.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("storage.FlagStore.Create: %w", err)
	}
	return nil
}

// GetByKey retrieves a non-archived flag by project and key.
// Returns ErrFlagNotFound if the flag does not exist.
func (s *FlagStore) GetByKey(ctx context.Context, projectID uuid.UUID, key string) (*Flag, error) {
	const query = `
	SELECT
	  id,
      project_id,
      key,
      name,
      description,
      type,
      default_value,
      archived_at,
      created_at,
      updated_at,
      created_by
    FROM flags
    WHERE project_id = $1
      AND key = $2
      AND archived_at IS NULL
	`

	var (
		flag        Flag
		description sql.NullString
		archivedAt  sql.NullTime
		createdBy   uuid.NullUUID
	)

	if err := s.pool.QueryRow(ctx, query, projectID, strings.TrimSpace(key)).Scan(
		&flag.ID,
		&flag.ProjectID,
		&flag.Key,
		&flag.Name,
		&description,
		&flag.Type,
		&flag.DefaultValue,
		&archivedAt,
		&flag.CreatedAt,
		&flag.UpdatedAt,
		&createdBy,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFlagNotFound
		}
		return nil, fmt.Errorf("storage.FlagStore.GetByKey: %w", err)
	}

	flag.Description = stringPtrFromNull(description)
	flag.ArchivedAt = timePtrFromNull(archivedAt)
	flag.CreatedBy = uuidPtrFromNull(createdBy)

	return &flag, nil
}

// ListByProject returns all non-archived flags for a project ordered by creation time.
func (s *FlagStore) ListByProject(ctx context.Context, projectID uuid.UUID) ([]Flag, error) {
	const query = `
		SELECT
			id,
			project_id,
			key,
			name,
			description,
			type,
			default_value,
			archived_at,
			created_at,
			updated_at,
			created_by
		FROM flags
		WHERE project_id = $1
		  AND archived_at IS NULL
		ORDER BY created_at ASC
	`
	rows, err := s.pool.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("storage.FlagStore.ListByProject: %w", err)
	}
	defer rows.Close()
	flags := make([]Flag, 0)
	for rows.Next() {
		var (
			flag        Flag
			description sql.NullString
			archivedAt  sql.NullTime
			createdBy   uuid.NullUUID
		)
		if err := rows.Scan(
			&flag.ID,
			&flag.ProjectID,
			&flag.Key,
			&flag.Name,
			&description,
			&flag.Type,
			&flag.DefaultValue,
			&archivedAt,
			&flag.CreatedAt,
			&flag.UpdatedAt,
			&createdBy,
		); err != nil {
			return nil, fmt.Errorf("storage.FlagStore.ListByProject: scan row: %w", err)
		}
		flag.Description = stringPtrFromNull(description)
		flag.ArchivedAt = timePtrFromNull(archivedAt)
		flag.CreatedBy = uuidPtrFromNull(createdBy)
		flags = append(flags, flag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage.FlagStore.ListByProject: iterate rows: %w", err)
	}
	return flags, nil
}

// Update changes a flag's mutable fields.
// Returns ErrFlagNotFound if the flag does not exist.
// Returns ErrDuplicateKey if the new key already exists within the project.
func (s *FlagStore) Update(ctx context.Context, flag *Flag) error {
	const query = `
		UPDATE flags
		SET key = $2, name = $3, description = $4, type = $5, default_value = $6
		WHERE id = $1
		  AND archived_at IS NULL
		RETURNING updated_at
	`
	flag.Key = strings.TrimSpace(flag.Key)
	flag.Name = strings.TrimSpace(flag.Name)
	err := s.pool.QueryRow(
		ctx,
		query,
		flag.ID,
		flag.Key,
		flag.Name,
		flag.Description,
		flag.Type,
		flag.DefaultValue,
	).Scan(&flag.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateKey
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrFlagNotFound
		}
		return fmt.Errorf("storage.FlagStore.Update: %w", err)
	}
	return nil
}

// Archive marks a flag as archived.
// Returns ErrFlagNotFound if the flag does not exist.
func (s *FlagStore) Archive(ctx context.Context, id uuid.UUID, archivedAt time.Time) error {
	const query = `
		UPDATE flags
		SET archived_at = $2
		WHERE id = $1
		  AND archived_at IS NULL
	`
	tag, err := s.pool.Exec(ctx, query, id, archivedAt)
	if err != nil {
		return fmt.Errorf("storage.FlagStore.Archive: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFlagNotFound
	}
	return nil
}
