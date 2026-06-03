package api

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/flagstonehq/flagstone/internal/api/middleware"
	"github.com/flagstonehq/flagstone/internal/config"
	"github.com/flagstonehq/flagstone/internal/storage"
	"github.com/flagstonehq/flagstone/internal/testutil/pgtest"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var testServer *Server
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	flag.Parse()

	var cleanup func()
	if !testing.Short() {
		pool, c, err := pgtest.Setup(context.Background(), "flagstone_test_api", "../../migrations")
		pgtest.RequireInCI(err)
		if err == nil {
			testPool = pool
			cleanup = c
			stores := storage.NewStores(pool)
			cfg := &config.Config{
				JWTSecret:       "0123456789abcdef0123456789abcdef",
				BcryptCost:      4,
				AccessTokenTTL:  15 * time.Minute,
				RefreshTokenTTL: 7 * 24 * time.Hour,
			}
			testServer = NewServer(stores, pool, cfg, zap.NewNop())
			testServer.loginLimiter = middleware.NewIPRateLimiter(1_000_000, time.Minute)
			testServer.refreshLimiter = middleware.NewIPRateLimiter(1_000_000, time.Minute)
		}
	}

	code := m.Run()

	if cleanup != nil {
		cleanup()
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
