package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SessionStore handles persistence and retrieval of refresh-token sessions.
type SessionStore struct {
	db Querier
}

// NewSessionStore creates a new SessionStore backed by the given connection pool.
func NewSessionStore(db Querier) *SessionStore {
	return &SessionStore{
		db: db,
	}
}

// Create inserts a new session. Generates an ID if not set.
func (s *SessionStore) Create(ctx context.Context, session *Session) error {
	const query = `
		INSERT INTO sessions (id, user_id, tenant_id, refresh_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at
	`

	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}

	var ip any
	if session.IPAddress != nil {
		ip = *session.IPAddress
	}

	err := s.db.QueryRow(
		ctx,
		query,
		session.ID,
		session.UserID,
		session.TenantID,
		session.RefreshHash,
		session.UserAgent,
		ip,
		session.ExpiresAt,
	).Scan(&session.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("storage.SessionStore.Create: %w", err)
	}

	return nil
}

// GetByRefreshHash retrieves a session by its refresh token hash.
// Returns ErrNotFound if the session does not exist.
func (s *SessionStore) GetByRefreshHash(ctx context.Context, refreshHash string) (*Session, error) {
	const query = `
		SELECT id, user_id, tenant_id, refresh_hash, user_agent, host(ip_address), expires_at, created_at
		FROM sessions
		WHERE refresh_hash = $1
	`
	var (
		session   Session
		userAgent sql.NullString
		ipText    sql.NullString
	)
	if err := s.db.QueryRow(ctx, query, refreshHash).Scan(
		&session.ID,
		&session.UserID,
		&session.TenantID,
		&session.RefreshHash,
		&userAgent,
		&ipText,
		&session.ExpiresAt,
		&session.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage.SessionStore.GetByRefreshHash: %w", err)
	}
	session.UserAgent = stringPtrFromNull(userAgent)
	session.IPAddress = ipPtrFromNull(ipText)
	return &session, nil
}

// DeleteByID removes a session by its ID.
// Returns ErrNotFound if the session does not exist.
func (s *SessionStore) DeleteByID(ctx context.Context, id uuid.UUID) error {
	const query = `
	 DELETE FROM sessions
	 WHERE id = $1
	`

	tag, err := s.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("storage.SessionStore.DeleteByID: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteByUserID removes all sessions for a user.
// Returns nil if the user has no active sessions.
func (s *SessionStore) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	const query = `
		DELETE FROM sessions
		WHERE user_id = $1
	`
	if _, err := s.db.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("storage.SessionStore.DeleteByUserID: %w", err)
	}
	return nil
}

// DeleteExpired removes all expired sessions and returns the number of deleted rows.
func (s *SessionStore) DeleteExpired(ctx context.Context) (int64, error) {
	const query = `
		DELETE FROM sessions
		WHERE expires_at < NOW()
	`

	tag, err := s.db.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("storage.SessionStore.DeleteExpired: %w", err)
	}

	return tag.RowsAffected(), nil
}

func ipPtrFromNull(v sql.NullString) *net.IP {
	if !v.Valid {
		return nil
	}
	ip := net.ParseIP(v.String)
	if ip == nil {
		return nil
	}
	return &ip
}
