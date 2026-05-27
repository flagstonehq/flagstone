package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RevokedTokenStore persists hashes of refresh tokens that must never be
// honored again. handleRefresh consults this store to detect replayed
// refresh tokens (T20) and respond by killing every session for the user.
type RevokedTokenStore struct {
	db Querier
}

// NewRevokedTokenStore creates a new RevokedTokenStore backed by the given Querier.
func NewRevokedTokenStore(db Querier) *RevokedTokenStore {
	return &RevokedTokenStore{db: db}
}

// Insert records a revoked refresh-token hash. If the hash already exists
// (e.g. duplicate revocation race), the call is a no-op — the row is
// already there and that's the state we wanted.
func (s *RevokedTokenStore) Insert(ctx context.Context, token *RevokedRefreshToken) error {
	const query = `
		INSERT INTO revoked_refresh_tokens (id, token_hash, user_id, revoked_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (token_hash) DO NOTHING
	`

	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	if token.RevokedAt.IsZero() {
		token.RevokedAt = time.Now().UTC()
	}

	if _, err := s.db.Exec(
		ctx,
		query,
		token.ID,
		token.TokenHash,
		token.UserID,
		token.RevokedAt,
		token.ExpiresAt,
	); err != nil {
		return fmt.Errorf("storage.RevokedTokenStore.Insert: %w", err)
	}
	return nil
}

// Lookup returns the row for the given hash, or ErrNotFound if no such
// hash has been revoked. Callers use the user_id from the result to know
// whose sessions to kill on replay detection.
func (s *RevokedTokenStore) Lookup(ctx context.Context, tokenHash string) (*RevokedRefreshToken, error) {
	const query = `
		SELECT id, token_hash, user_id, revoked_at, expires_at
		FROM revoked_refresh_tokens
		WHERE token_hash = $1
	`

	var token RevokedRefreshToken
	if err := s.db.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.TokenHash,
		&token.UserID,
		&token.RevokedAt,
		&token.ExpiresAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage.RevokedTokenStore.Lookup: %w", err)
	}
	return &token, nil
}

// DeleteExpired removes rows whose expires_at has passed. Intended for a
// periodic background job.
func (s *RevokedTokenStore) DeleteExpired(ctx context.Context) (int64, error) {
	const query = `DELETE FROM revoked_refresh_tokens WHERE expires_at < NOW()`
	tag, err := s.db.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("storage.RevokedTokenStore.DeleteExpired: %w", err)
	}
	return tag.RowsAffected(), nil
}
