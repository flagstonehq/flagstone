package storage

import "github.com/jackc/pgx/v5/pgxpool"

// Stores is a container for all domain stores, sharing a single connection pool.
type Stores struct {
	Tenants *TenantStore
}

// NewStores creates all stores from a single connection pool.
func NewStores(pool *pgxpool.Pool) *Stores {
	return &Stores{
		Tenants: NewTenantStore(pool),
	}
}
