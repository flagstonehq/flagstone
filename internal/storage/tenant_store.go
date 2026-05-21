package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TenantStore handles persistence and retrieval of tenants.
type TenantStore struct {
	pool *pgxpool.Pool
}

// NewTenantStore creates a new TenantStore backed by the given connection pool.
func NewTenantStore(pool *pgxpool.Pool) *TenantStore {
	return &TenantStore{
		pool: pool,
	}
}

// ExistsAny returns true if at least one tenant exists in the database.
// Used by the bootstrap endpoint to prevent re-initialization.
func (s *TenantStore) ExistsAny(ctx context.Context) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM tenants
		)
	`

	var exists bool
	if err := s.pool.QueryRow(ctx, query).Scan(&exists); err != nil {
		return false, fmt.Errorf("storage.TenantStore.ExistsAny: %w", err)
	}

	return exists, nil
}

// Create inserts a new tenant. Generates an ID if not set.
// Returns ErrDuplicateKey if the slug already exists.
func (s *TenantStore) Create(ctx context.Context, tenant *Tenant) error {
	const query = `
		INSERT INTO tenants (id, slug, name, plan)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at
	`

	if tenant.ID == uuid.Nil {
		tenant.ID = uuid.New()
	}

	tenant.Slug = strings.TrimSpace(tenant.Slug)
	tenant.Name = strings.TrimSpace(tenant.Name)

	err := s.pool.QueryRow(
		ctx,
		query,
		tenant.ID,
		tenant.Slug,
		tenant.Name,
		tenant.Plan,
	).Scan(&tenant.CreatedAt, &tenant.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("storage.TenantStore.Create: %w", err)
	}

	return nil
}

// GetByID retrieves a tenant by its UUID.
// Returns ErrTenantNotFound if the tenant does not exist.
func (s *TenantStore) GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	const query = `
		SELECT id, slug, name, plan, created_at, updated_at
		FROM tenants
		WHERE id = $1
	`

	var tenant Tenant
	if err := s.pool.QueryRow(ctx, query, id).Scan(
		&tenant.ID,
		&tenant.Slug,
		&tenant.Name,
		&tenant.Plan,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTenantNotFound
		}
		return nil, fmt.Errorf("storage.TenantStore.GetByID: %w", err)
	}

	return &tenant, nil
}

// GetBySlug retrieves a tenant by its URL-safe slug.
// Returns ErrTenantNotFound if the tenant does not exist.
func (s *TenantStore) GetBySlug(ctx context.Context, slug string) (*Tenant, error) {
	const query = `
		SELECT id, slug, name, plan, created_at, updated_at
		FROM tenants
		WHERE slug = $1
	`

	var tenant Tenant
	if err := s.pool.QueryRow(ctx, query, strings.TrimSpace(slug)).Scan(
		&tenant.ID,
		&tenant.Slug,
		&tenant.Name,
		&tenant.Plan,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTenantNotFound
		}
		return nil, fmt.Errorf("storage.TenantStore.GetBySlug: %w", err)
	}

	return &tenant, nil
}

// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505"
}
