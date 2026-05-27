package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// LoginAttemptStore persists failed-login records used to enforce account
// lockout (T19). Successful logins call ClearForUser to wipe the counter.
type LoginAttemptStore struct {
	db Querier
}

// NewLoginAttemptStore creates a new LoginAttemptStore backed by the given Querier.
func NewLoginAttemptStore(db Querier) *LoginAttemptStore {
	return &LoginAttemptStore{db: db}
}

// Record inserts a single failed-login attempt.
func (s *LoginAttemptStore) Record(ctx context.Context, attempt *LoginAttempt) error {
	const query = `
		INSERT INTO login_attempts (id, user_id, ip_address, user_agent, attempted_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING attempted_at
	`

	if attempt.ID == uuid.Nil {
		attempt.ID = uuid.New()
	}
	if attempt.AttemptedAt.IsZero() {
		attempt.AttemptedAt = time.Now().UTC()
	}

	var ip any
	if attempt.IPAddress != nil {
		ip = *attempt.IPAddress
	}

	if err := s.db.QueryRow(
		ctx,
		query,
		attempt.ID,
		attempt.UserID,
		ip,
		attempt.UserAgent,
		attempt.AttemptedAt,
	).Scan(&attempt.AttemptedAt); err != nil {
		return fmt.Errorf("storage.LoginAttemptStore.Record: %w", err)
	}
	return nil
}

// CountSince returns the number of failed-login attempts for the given user
// since the given timestamp. Used to decide if an account is locked.
func (s *LoginAttemptStore) CountSince(ctx context.Context, userID uuid.UUID, since time.Time) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM login_attempts
		WHERE user_id = $1 AND attempted_at >= $2
	`

	var count int
	if err := s.db.QueryRow(ctx, query, userID, since).Scan(&count); err != nil {
		return 0, fmt.Errorf("storage.LoginAttemptStore.CountSince: %w", err)
	}
	return count, nil
}

// ClearForUser deletes every login attempt for the given user. Called after
// a successful login so the lockout counter resets.
func (s *LoginAttemptStore) ClearForUser(ctx context.Context, userID uuid.UUID) error {
	const query = `DELETE FROM login_attempts WHERE user_id = $1`
	if _, err := s.db.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("storage.LoginAttemptStore.ClearForUser: %w", err)
	}
	return nil
}

// DeleteOlderThan removes rows older than the cutoff. Intended for a periodic
// background job; safe to call on a hot path but not cheap.
func (s *LoginAttemptStore) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	const query = `DELETE FROM login_attempts WHERE attempted_at < $1`
	tag, err := s.db.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("storage.LoginAttemptStore.DeleteOlderThan: %w", err)
	}
	return tag.RowsAffected(), nil
}
