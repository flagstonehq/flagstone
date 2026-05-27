package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ProjectStore handles persistence and retrieval of projects.
type ProjectStore struct {
	db Querier
}

// NewProjectStore creates a new ProjectStore backed by the given connection pool.
func NewProjectStore(db Querier) *ProjectStore {
	return &ProjectStore{
		db: db,
	}
}

// Create inserts a new project. Generates an ID if not set.
// Returns ErrDuplicateKey if the tenant already has a project with the same slug.
func (s *ProjectStore) Create(ctx context.Context, project *Project) error {
	const query = `
	  INSERT INTO projects (id, tenant_id, slug, name)
	  VALUES ($1, $2, $3, $4)
	  RETURNING created_at, updated_at
	`

	if project.ID == uuid.Nil {
		project.ID = uuid.New()
	}

	project.Slug = strings.TrimSpace(project.Slug)
	project.Name = strings.TrimSpace(project.Name)

	err := s.db.QueryRow(
		ctx,
		query,
		project.ID,
		project.TenantID,
		project.Slug,
		project.Name,
	).Scan(&project.CreatedAt, &project.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("storage.ProjectStore.Create: %w", err)
	}

	return nil
}

// GetBySlug retrieves a project by tenant and slug.
// Returns ErrProjectNotFound if the project does not exist.
func (s *ProjectStore) GetBySlug(ctx context.Context, tenantID uuid.UUID, slug string) (*Project, error) {
	const query = `
	  SELECT id, tenant_id, slug, name, created_at, updated_at
	  FROM projects
	  WHERE tenant_id = $1 AND slug = $2
	`

	var project Project
	if err := s.db.QueryRow(ctx, query, tenantID, strings.TrimSpace(slug)).Scan(
		&project.ID,
		&project.TenantID,
		&project.Slug,
		&project.Name,
		&project.CreatedAt,
		&project.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("storage.ProjectStore.GetBySlug: %w", err)
	}

	return &project, nil
}

// ListByTenant returns all projects for a tenant ordered by creation time.
func (s *ProjectStore) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]Project, error) {
	const query = `
	  SELECT id, tenant_id, slug, name, created_at, updated_at
	  FROM projects
	  WHERE tenant_id = $1
	  ORDER BY created_at ASC
	`

	rows, err := s.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("storage.ProjectStore.ListByTenant: %w", err)
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		var project Project
		if err := rows.Scan(
			&project.ID,
			&project.TenantID,
			&project.Slug,
			&project.Name,
			&project.CreatedAt,
			&project.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("storage.ProjectStore.ListByTenant: scan row: %w", err)
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage.ProjectStore.ListByTenant: iterate rows: %w", err)
	}

	return projects, nil
}

// Update changes a project's slug and name.
// Returns ErrProjectNotFound if the project does not exist.
// Returns ErrDuplicateKey if the new slug already exists within the tenant.
func (s *ProjectStore) Update(ctx context.Context, project *Project) error {
	const query = `
	  UPDATE projects
	  SET slug = $2, name = $3
	  WHERE id = $1
	  RETURNING updated_at
	`

	project.Slug = strings.TrimSpace(project.Slug)
	project.Name = strings.TrimSpace(project.Name)

	err := s.db.QueryRow(
		ctx,
		query,
		project.ID,
		project.Slug,
		project.Name,
	).Scan(&project.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateKey
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("storage.ProjectStore.Update: %w", err)
	}

	return nil
}
