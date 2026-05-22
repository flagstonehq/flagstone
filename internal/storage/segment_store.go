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

// SegmentStore handles persistence and retrieval of segments.
type SegmentStore struct {
	pool *pgxpool.Pool
}

// NewSegmentStore creates a new SegmentStore backed by the given connection pool.
func NewSegmentStore(pool *pgxpool.Pool) *SegmentStore {
	return &SegmentStore{
		pool: pool,
	}
}

// Create inserts a new segment. Generates an ID if not set.
// Returns ErrDuplicateKey if the project already has a segment with the same key.
func (s *SegmentStore) Create(ctx context.Context, segment *Segment) error {
	const query = `
			INSERT INTO segments (
				id,
				project_id,
				key,
				name,
				description,
				rules,
				archived_at,
				created_by
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING created_at, updated_at
		`

	if segment.ID == uuid.Nil {
		segment.ID = uuid.New()
	}

	segment.Key = strings.TrimSpace(segment.Key)
	segment.Name = strings.TrimSpace(segment.Name)

	err := s.pool.QueryRow(
		ctx,
		query,
		segment.ID,
		segment.ProjectID,
		segment.Key,
		segment.Name,
		segment.Description,
		segment.Rules,
		segment.ArchivedAt,
		segment.CreatedBy,
	).Scan(&segment.CreatedAt, &segment.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("storage.SegmentStore.Create: %w", err)
	}

	return nil
}

// GetByKey retrieves a non-archived segment by project and key.
// Returns ErrSegmentNotFound if the segment does not exist.
func (s *SegmentStore) GetByKey(ctx context.Context, projectID uuid.UUID, key string) (*Segment, error) {
	const query = `
			SELECT
				id,
				project_id,
				key,
				name,
				description,
				rules,
				archived_at,
				created_at,
				updated_at,
				created_by
			FROM segments
			WHERE project_id = $1
			  AND key = $2
			  AND archived_at IS NULL
		`

	var (
		segment     Segment
		description sql.NullString
		archivedAt  sql.NullTime
		createdBy   uuid.NullUUID
	)

	if err := s.pool.QueryRow(ctx, query, projectID, strings.TrimSpace(key)).Scan(
		&segment.ID,
		&segment.ProjectID,
		&segment.Key,
		&segment.Name,
		&description,
		&segment.Rules,
		&archivedAt,
		&segment.CreatedAt,
		&segment.UpdatedAt,
		&createdBy,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSegmentNotFound
		}
		return nil, fmt.Errorf("storage.SegmentStore.GetByKey: %w", err)
	}

	segment.Description = stringPtrFromNull(description)
	segment.ArchivedAt = timePtrFromNull(archivedAt)
	segment.CreatedBy = uuidPtrFromNull(createdBy)

	return &segment, nil
}

// ListByProject returns all non-archived segments for a project ordered by creation time.
func (s *SegmentStore) ListByProject(ctx context.Context, projectID uuid.UUID) ([]Segment, error) {
	const query = `
			SELECT
				id,
				project_id,
				key,
				name,
				description,
				rules,
				archived_at,
				created_at,
				updated_at,
				created_by
			FROM segments
			WHERE project_id = $1
			  AND archived_at IS NULL
			ORDER BY created_at ASC
		`

	rows, err := s.pool.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("storage.SegmentStore.ListByProject: %w", err)
	}
	defer rows.Close()

	segments := make([]Segment, 0)
	for rows.Next() {
		var (
			segment     Segment
			description sql.NullString
			archivedAt  sql.NullTime
			createdBy   uuid.NullUUID
		)

		if err := rows.Scan(
			&segment.ID,
			&segment.ProjectID,
			&segment.Key,
			&segment.Name,
			&description,
			&segment.Rules,
			&archivedAt,
			&segment.CreatedAt,
			&segment.UpdatedAt,
			&createdBy,
		); err != nil {
			return nil, fmt.Errorf("storage.SegmentStore.ListByProject: scan row: %w", err)
		}

		segment.Description = stringPtrFromNull(description)
		segment.ArchivedAt = timePtrFromNull(archivedAt)
		segment.CreatedBy = uuidPtrFromNull(createdBy)

		segments = append(segments, segment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage.SegmentStore.ListByProject: iterate rows: %w", err)
	}

	return segments, nil
}

// Update changes a segment's mutable fields.
// Returns ErrSegmentNotFound if the segment does not exist.
// Returns ErrDuplicateKey if the new key already exists within the project.
func (s *SegmentStore) Update(ctx context.Context, segment *Segment) error {
	const query = `
		UPDATE segments
		SET key = $2, name = $3, description = $4, rules = $5
		WHERE id = $1
		  AND archived_at IS NULL
		RETURNING updated_at
	`

	segment.Key = strings.TrimSpace(segment.Key)
	segment.Name = strings.TrimSpace(segment.Name)
	err := s.pool.QueryRow(
		ctx,
		query,
		segment.ID,
		segment.Key,
		segment.Name,
		segment.Description,
		segment.Rules,
	).Scan(&segment.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateKey
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSegmentNotFound
		}
		return fmt.Errorf("storage.SegmentStore.Update: %w", err)
	}
	return nil
}

// Archive marks a segment as archived.
// Returns ErrSegmentNotFound if the segment does not exist.
func (s *SegmentStore) Archive(ctx context.Context, id uuid.UUID, archivedAt time.Time) error {
	const query = `
		UPDATE segments
		SET archived_at = $2
		WHERE id = $1
		  AND archived_at IS NULL
	`
	tag, err := s.pool.Exec(ctx, query, id, archivedAt)
	if err != nil {
		return fmt.Errorf("storage.SegmentStore.Archive: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSegmentNotFound
	}
	return nil
}
