package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MemberStore handles tenant membership persistence and role management.
type MemberStore struct {
	pool *pgxpool.Pool
}

// NewMemberStore creates a new MemberStore backed by the given connection pool.
func NewMemberStore(pool *pgxpool.Pool) *MemberStore {
	return &MemberStore{
		pool: pool,
	}
}

// Add inserts a user into a tenant with a role.
// Returns ErrDuplicateKey if the membership already exists.
func (s *MemberStore) Add(ctx context.Context, member *TenantMember) error {
	const query = `
		INSERT INTO tenant_members (tenant_id, user_id, role)
		VALUES ($1, $2, $3)
		RETURNING created_at
	`

	err := s.pool.QueryRow(
		ctx,
		query,
		member.TenantID,
		member.UserID,
		member.Role,
	).Scan(&member.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("storage.MemberStore.Add: %w", err)
	}

	return nil
}

// GetRole returns the role of a user inside a tenant.
// Returns ErrNotFound if the membership does not exist.
func (s *MemberStore) GetRole(ctx context.Context, tenantID, userID uuid.UUID) (string, error) {
	const query = `
		SELECT role
		FROM tenant_members
		WHERE tenant_id = $1 AND user_id = $2
	`

	var role string
	if err := s.pool.QueryRow(ctx, query, tenantID, userID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("storage.MemberStore.GetRole: %w", err)
	}

	return role, nil
}

// ListByTenant returns all tenant members ordered by creation time.
func (s *MemberStore) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]TenantMember, error) {
	const query = `
		SELECT tenant_id, user_id, role, created_at
		FROM tenant_members
		WHERE tenant_id = $1
		ORDER BY created_at ASC
	`

	rows, err := s.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("storage.MemberStore.ListByTenant: %w", err)
	}
	defer rows.Close()

	members := make([]TenantMember, 0)
	for rows.Next() {
		var member TenantMember
		if err := rows.Scan(
			&member.TenantID,
			&member.UserID,
			&member.Role,
			&member.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("storage.MemberStore.ListByTenant: scan row: %w", err)
		}
		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage.MemberStore.ListByTenant: iterate rows: %w", err)
	}

	return members, nil
}

// UpdateRole changes the role of a tenant member.
// Returns ErrNotFound if the membership does not exist.
func (s *MemberStore) UpdateRole(ctx context.Context, tenantID, userID uuid.UUID, role string) error {
	const query = `
		UPDATE tenant_members
		SET role = $3
		WHERE tenant_id = $1 AND user_id = $2
	`

	tag, err := s.pool.Exec(ctx, query, tenantID, userID, role)
	if err != nil {
		return fmt.Errorf("storage.MemberStore.UpdateRole: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// Remove deletes a tenant membership.
// Returns ErrNotFound if the membership does not exist.
func (s *MemberStore) Remove(ctx context.Context, tenantID, userID uuid.UUID) error {
	const query = `
		DELETE FROM tenant_members
		WHERE tenant_id = $1 AND user_id = $2
	`

	tag, err := s.pool.Exec(ctx, query, tenantID, userID)
	if err != nil {
		return fmt.Errorf("storage.MemberStore.Remove: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
