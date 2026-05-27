package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// EnvironmentStore handles persistence and retrieval of environments.
type EnvironmentStore struct {
	db Querier
}

// NewEnvironmentStore creates a new EnvironmentStore backed by the given connection pool.
func NewEnvironmentStore(db Querier) *EnvironmentStore {
	return &EnvironmentStore{
		db: db,
	}
}

// Create inserts a new environment. Generates an ID if not set.
// Returns ErrDuplicateKey if the project already has an environment with the same slug.
func (s *EnvironmentStore) Create(ctx context.Context, environment *Environment) error {
	const query = `
	  INSERT INTO environments (id, project_id, slug, name)
	  VALUES ($1, $2, $3, $4)
	  RETURNING created_at, updated_at
	`

	if environment.ID == uuid.Nil {
		environment.ID = uuid.New()
	}

	environment.Slug = strings.TrimSpace(environment.Slug)
	environment.Name = strings.TrimSpace(environment.Name)

	err := s.db.QueryRow(
		ctx,
		query,
		environment.ID,
		environment.ProjectID,
		environment.Slug,
		environment.Name,
	).Scan(&environment.CreatedAt, &environment.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("storage.EnvironmentStore.Create: %w", err)
	}

	return nil
}

// GetByID retrieves an environment by its ID.
// Returns ErrEnvironmentNotFound if the environment does not exist.
func (s *EnvironmentStore) GetByID(ctx context.Context, id uuid.UUID) (*Environment, error) {
	const query = `
		SELECT id, project_id, slug, name, created_at, updated_at
		FROM environments
		WHERE id = $1
	`
	var environment Environment
	if err := s.db.QueryRow(ctx, query, id).Scan(
		&environment.ID,
		&environment.ProjectID,
		&environment.Slug,
		&environment.Name,
		&environment.CreatedAt,
		&environment.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEnvironmentNotFound
		}
		return nil, fmt.Errorf("storage.EnvironmentStore.GetByID: %w", err)
	}
	return &environment, nil
}

// GetBySlug retrieves an environment by project and slug.
// Returns ErrEnvironmentNotFound if the environment does not exist.
func (s *EnvironmentStore) GetBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*Environment, error) {
	const query = `
		SELECT id, project_id, slug, name, created_at, updated_at
		FROM environments
		WHERE project_id = $1 AND slug = $2
	`
	var environment Environment
	if err := s.db.QueryRow(ctx, query, projectID, strings.TrimSpace(slug)).Scan(
		&environment.ID,
		&environment.ProjectID,
		&environment.Slug,
		&environment.Name,
		&environment.CreatedAt,
		&environment.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEnvironmentNotFound
		}
		return nil, fmt.Errorf("storage.EnvironmentStore.GetBySlug: %w", err)
	}
	return &environment, nil
}

// ListByProject returns all environments for a project ordered by creation time.
func (s *EnvironmentStore) ListByProject(ctx context.Context, projectID uuid.UUID) ([]Environment, error) {
	const query = `
		SELECT id, project_id, slug, name, created_at, updated_at
		FROM environments
		WHERE project_id = $1
		ORDER BY created_at ASC
	`
	rows, err := s.db.Query(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("storage.EnvironmentStore.ListByProject: %w", err)
	}
	defer rows.Close()
	environments := make([]Environment, 0)
	for rows.Next() {
		var environment Environment
		if err := rows.Scan(
			&environment.ID,
			&environment.ProjectID,
			&environment.Slug,
			&environment.Name,
			&environment.CreatedAt,
			&environment.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("storage.EnvironmentStore.ListByProject: scan row: %w", err)
		}
		environments = append(environments, environment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage.EnvironmentStore.ListByProject: iterate rows: %w", err)
	}
	return environments, nil
}
