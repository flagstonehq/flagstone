package storage

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/flagstonehq/flagstone/internal/testutil/pgtest"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	flag.Parse()

	var cleanup func()
	if !testing.Short() {
		pool, c, err := pgtest.Setup(context.Background(), "flagstone_test_storage", "../../migrations")
		if err != nil {
			_, _ = os.Stderr.WriteString("storage_test: " + err.Error() + "\n")
			os.Exit(1)
		}
		testPool = pool
		cleanup = c
	}

	code := m.Run()

	if cleanup != nil {
		cleanup()
	}

	os.Exit(code)
}

func skipIfShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
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

func uuidPtr(id uuid.UUID) *uuid.UUID {
	return &id
}
