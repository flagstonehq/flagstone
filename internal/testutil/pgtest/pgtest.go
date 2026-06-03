package pgtest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultDSN is the local-dev Postgres URL used when DATABASE_URL is unset.
const DefaultDSN = "postgres://flagstone:flagstone_dev@localhost:5432/flagstone?sslmode=disable"

// Setup creates an isolated Postgres schema named schemaName, runs every
// migration in migrationsDir against it, and returns a pool whose connections
// default to that schema. The returned cleanup function drops the schema;
// callers should defer it after m.Run().
//
// schemaName must be a valid Postgres identifier. It will be quoted via
// pgx.Identifier.Sanitize so callers don't have to.
//
// migrationsDir must contain *.up.sql files; they're applied in lexical order
// (filenames like 000001_*.up.sql, 000002_*.up.sql sort correctly).
//
// If Postgres is unreachable, Setup returns an error — the caller's TestMain
// should treat that as "DB not available" and skip integration tests.
func Setup(ctx context.Context, schemaName, migrationsDir string) (*pgxpool.Pool, func(), error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = DefaultDSN
	}

	schemaIdent := pgx.Identifier{schemaName}.Sanitize()

	// Bootstrap connection — used only to create the schema and apply
	// migrations. Simple-protocol mode is required: migration files contain
	// multi-statement SQL with BEGIN/COMMIT that the extended protocol
	// rejects.
	bootCfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("pgtest: parse bootstrap config: %w", err)
	}
	bootCfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	boot, err := pgx.ConnectConfig(ctx, bootCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("pgtest: bootstrap connect: %w", err)
	}

	if _, err := boot.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schemaIdent)); err != nil {
		_ = boot.Close(ctx)
		return nil, nil, fmt.Errorf("pgtest: drop schema: %w", err)
	}
	if _, err := boot.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s`, schemaIdent)); err != nil {
		_ = boot.Close(ctx)
		return nil, nil, fmt.Errorf("pgtest: create schema: %w", err)
	}
	// Extensions (citext) live in the public schema. Putting our schema first
	// in search_path makes unqualified table refs (CREATE TABLE tenants ...)
	// land in OUR schema; public stays available for extension types.
	if _, err := boot.Exec(ctx, fmt.Sprintf(`SET search_path TO %s, public`, schemaIdent)); err != nil {
		_ = boot.Close(ctx)
		return nil, nil, fmt.Errorf("pgtest: set search_path: %w", err)
	}

	if err := applyMigrations(ctx, boot, migrationsDir); err != nil {
		_ = boot.Close(ctx)
		return nil, nil, err
	}
	if err := boot.Close(ctx); err != nil {
		return nil, nil, fmt.Errorf("pgtest: close bootstrap: %w", err)
	}

	// Production pool. Each new connection runs SET search_path before it
	// joins the pool, so application code (stores, handlers) sees the
	// isolated schema without any awareness of this setup.
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("pgtest: parse pool config: %w", err)
	}
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, fmt.Sprintf(`SET search_path TO %s, public`, schemaIdent))
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("pgtest: new pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("pgtest: ping pool: %w", err)
	}

	cleanup := func() {
		pool.Close()
		// Fresh connection for cleanup — the pool is gone. Best-effort:
		// if Postgres is down we don't fail the test run, the next setup
		// will drop the stale schema via DROP SCHEMA IF EXISTS.
		bg := context.Background()
		c, err := pgx.Connect(bg, dsn)
		if err != nil {
			return
		}
		defer func() { _ = c.Close(bg) }()
		_, _ = c.Exec(bg, fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schemaIdent))
	}

	return pool, cleanup, nil
}

// RequireInCI turns a Setup error into a fatal failure when running under CI
// (GitHub Actions sets CI=true). Locally — where a developer may not have
// Postgres running — it is a no-op, so TestMain can fall back to skipping.
// This prevents CI from passing green with all DB-backed tests silently skipped.
func RequireInCI(err error) {
	if err != nil && os.Getenv("CI") != "" {
		panic("pgtest: Postgres is required under CI but setup failed: " + err.Error())
	}
}

func applyMigrations(ctx context.Context, conn *pgx.Conn, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("pgtest: glob migrations: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("pgtest: no *.up.sql migrations found in %s", dir)
	}
	sort.Strings(files)

	for _, f := range files {
		sqlBytes, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("pgtest: read %s: %w", f, err)
		}
		if _, err := conn.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("pgtest: apply %s: %w", filepath.Base(f), err)
		}
	}
	return nil
}
