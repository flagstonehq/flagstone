package storage

import "github.com/jackc/pgx/v5/pgxpool"

// Stores is a container for all domain stores, sharing a single connection pool.
type Stores struct {
	Tenants      *TenantStore
	Users        *UserStore
	Sessions     *SessionStore
	Members      *MemberStore
	Projects     *ProjectStore
	Environments *EnvironmentStore
}

// NewStores creates all stores from a single connection pool.
func NewStores(pool *pgxpool.Pool) *Stores {
	return &Stores{
		Tenants:      NewTenantStore(pool),
		Users:        NewUserStore(pool),
		Sessions:     NewSessionStore(pool),
		Members:      NewMemberStore(pool),
		Projects:     NewProjectStore(pool),
		Environments: NewEnvironmentStore(pool),
	}
}
