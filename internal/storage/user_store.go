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

// UserStore handles persistence and retrieval of users.
type UserStore struct {
	pool *pgxpool.Pool
}

// NewUserStore creates a new UserStore backed by the given connection pool.
func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{
		pool: pool,
	}
}

// Create inserts a new user. Generates an ID if not set.
// Returns ErrDuplicateKey if the email already exists.
func (s *UserStore) Create(ctx context.Context, user *User) error {
	const query = `
	  INSERT INTO users (id, email, password_hash, email_verified_at, last_login_at)
	  VALUES ($1, $2, $3, $4, $5)
	  RETURNING created_at, updated_at
	`

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	user.Email = strings.TrimSpace(user.Email)

	err := s.pool.QueryRow(
		ctx,
		query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.EmailVerifiedAt,
		user.LastLoginAt,
	).Scan(&user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("storage.UserStore.Create: %w", err)
	}

	return nil
}

// GetByID retrieves a user by the UUID.
// Returns ErrUserNotFound if the user does not exist.
func (s *UserStore) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	const query = `
	  SELECT id, email, password_hash, email_verified_at, last_login_at, created_at,
	updated_at
	    FROM users
		WHERE id = $1
	`

	var (
		user            User
		passwordHash    sql.NullString
		emailVerifiedAt sql.NullTime
		lastLoginAt     sql.NullTime
	)

	if err := s.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&passwordHash,
		&emailVerifiedAt,
		&lastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("storage.UserStore.GetByID: %w", err)
	}

	user.PasswordHash = stringPtrFromNull(passwordHash)
	user.EmailVerifiedAt = timePtrFromNull(emailVerifiedAt)
	user.LastLoginAt = timePtrFromNull(lastLoginAt)

	return &user, nil
}

// GetByEmail retrieves a user by email using the database CITEXT column.
// Returns ErrUserNotFound if the user does not exist.
func (s *UserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	const query = `
		SELECT id, email, password_hash, email_verified_at, last_login_at, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var (
		user            User
		passwordHash    sql.NullString
		emailVerifiedAt sql.NullTime
		lastLoginAt     sql.NullTime
	)
	if err := s.pool.QueryRow(ctx, query, strings.TrimSpace(email)).Scan(
		&user.ID,
		&user.Email,
		&passwordHash,
		&emailVerifiedAt,
		&lastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("storage.UserStore.GetByEmail: %w", err)
	}
	user.PasswordHash = stringPtrFromNull(passwordHash)
	user.EmailVerifiedAt = timePtrFromNull(emailVerifiedAt)
	user.LastLoginAt = timePtrFromNull(lastLoginAt)
	return &user, nil
}

// UpdateLastLogin sets last_login_at to the provided timestamp.
// Returns ErrUserNotFound if the user does not exist.
func (s *UserStore) UpdateLastLogin(ctx context.Context, userID uuid.UUID, at time.Time) error {
	const query = `
		UPDATE users
		SET last_login_at = $2
		WHERE id = $1
	`

	tag, err := s.pool.Exec(ctx, query, userID, at)
	if err != nil {
		return fmt.Errorf("storage.UserStore.UpdateLastLogin: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

// UpdatePasswordHash updates the stored password hash for a user.
// Returns ErrUserNotFound if the user does not exist.
func (s *UserStore) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	const query = `
			UPDATE users
			SET password_hash = $2
			WHERE id = $1
	`

	tag, err := s.pool.Exec(ctx, query, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("storage.UserStore.UpdatePasswordHash: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

// VerifyEmail marks a user's email as verified at the provided timestamp.
// Returns ErrUserNotFound if the user does not exist.
func (s *UserStore) VerifyEmail(ctx context.Context, userID uuid.UUID, at time.Time) error {
	const query = `
		UPDATE users
		SET email_verified_at = $2
		WHERE id = $1
	`

	tag, err := s.pool.Exec(ctx, query, userID, at)
	if err != nil {
		return fmt.Errorf("storage.UserStore.VerifyEmail: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

func stringPtrFromNull(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func timePtrFromNull(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}

	t := v.Time
	return &t
}
