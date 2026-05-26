package api

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thomas-vilte/flagstone/internal/config"
	"github.com/thomas-vilte/flagstone/internal/storage"
	"go.uber.org/zap"
)

var testServer *Server
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	flag.Parse()

	if !testing.Short() {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			dsn = "postgres://flagstone:flagstone_dev@localhost:5432/flagstone?sslmode=disable"
		}

		ctx := context.Background()
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			if err := pool.Ping(ctx); err == nil {
				testPool = pool
				stores := storage.NewStores(pool)
				cfg := &config.Config{
					JWTSecret:       "0123456789abcdef0123456789abcdef",
					BcryptCost:      4,
					AccessTokenTTL:  15 * time.Minute,
					RefreshTokenTTL: 7 * 24 * time.Hour,
				}
				testServer = NewServer(stores, pool, cfg, zap.NewNop())
			}
		}
	}

	code := m.Run()

	if testPool != nil {
		testPool.Close()
	}

	os.Exit(code)
}

func skipIfNoDB(t *testing.T) {
	t.Helper()
	if testPool == nil {
		t.Skip("Skipping test: Postgres not available")
	}
}

func truncateTables(t *testing.T, tables ...string) {
	t.Helper()
	if testPool == nil {
		t.Fatal("testPool is nil; Postgres not available")
	}

	ctx := context.Background()
	for _, table := range tables {
		_, err := testPool.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE")
		if err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
}
