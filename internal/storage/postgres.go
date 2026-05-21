package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates and verifies a pgx connection pool with production defaults.
// It pings the database with a 2-second timeout before returning.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("storage.NewPool: parse config: %w", err)
	}

	poolCfg.MaxConns = 25
	poolCfg.MinConns = 5
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.HealthCheckPeriod = 30 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("storage.NewPool: create pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("storage.NewPool: ping database: %w", err)
	}
	return pool, nil
}
