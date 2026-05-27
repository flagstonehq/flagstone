package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNoPool is returned when BeginTx is called on a Stores that was
// built from a transaction (no pool available to start a nested tx).
var ErrNoPool = errors.New("storage: stores has no pool (already tx-scoped)")

// Stores is a container for all domain stores, sharing a single Querier.
// The pool field is kept for starting transactions; tx-scoped Stores
// (built via WithTx) have a nil pool.
type Stores struct {
	pool *pgxpool.Pool

	Tenants          *TenantStore
	Users            *UserStore
	Sessions         *SessionStore
	Members          *MemberStore
	Projects         *ProjectStore
	Environments     *EnvironmentStore
	APIKeys          *APIKeyStore
	Flags            *FlagStore
	FlagEnvironments *FlagEnvironmentStore
	Segments         *SegmentStore
	AuditLogs        *AuditStore
	LoginAttempts    *LoginAttemptStore
	RevokedTokens    *RevokedTokenStore
}

// NewStores creates all stores from a single connection pool.
func NewStores(pool *pgxpool.Pool) *Stores {
	return newStoresFrom(pool, pool)
}

// WithTx returns a new Stores whose stores all run inside the given tx.
// The returned Stores cannot itself start a transaction (BeginTx returns
// ErrNoPool) — nested transactions are not supported.
func (s *Stores) WithTx(tx pgx.Tx) *Stores {
	return newStoresFrom(nil, tx)
}

// BeginTx starts a transaction. Returns ErrNoPool when called on a Stores
// built from WithTx.
func (s *Stores) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	if s.pool == nil {
		return nil, ErrNoPool
	}
	return s.pool.BeginTx(ctx, opts)
}

func newStoresFrom(pool *pgxpool.Pool, db Querier) *Stores {
	return &Stores{
		pool:             pool,
		Tenants:          NewTenantStore(db),
		Users:            NewUserStore(db),
		Sessions:         NewSessionStore(db),
		Members:          NewMemberStore(db),
		Projects:         NewProjectStore(db),
		Environments:     NewEnvironmentStore(db),
		APIKeys:          NewAPIKeyStore(db),
		Flags:            NewFlagStore(db),
		FlagEnvironments: NewFlagEnvironmentStore(db),
		Segments:         NewSegmentStore(db),
		AuditLogs:        NewAuditStore(db),
		LoginAttempts:    NewLoginAttemptStore(db),
		RevokedTokens:    NewRevokedTokenStore(db),
	}
}
