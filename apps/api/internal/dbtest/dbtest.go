// Package dbtest provides shared bootstrap for repository integration tests:
// it creates the test database on demand, applies the embedded migrations, and
// opens a connection pool. Tests skip when PostgreSQL is unreachable.
//
// Test packages run as separate binaries that share one test database, so
// dbtest serializes them with a Postgres advisory lock held for the lifetime of
// each test binary. This prevents a package's TRUNCATE statements from
// clobbering another package's fixtures mid-test.
package dbtest

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/edwinpolo/biomed-cmms/api/internal/migrations"
)

// advisoryLockKey serializes DB-using test binaries against each other. The
// lock is held on a dedicated connection for the whole test process.
const advisoryLockKey int64 = 0x62696F6D // "biomed"

var (
	setupOnce sync.Once
	setupPool *pgxpool.Pool
	setupErr  error

	// lockConn is held open for the process lifetime; closing it would release
	// the advisory lock and allow another test binary to interleave.
	lockConn *pgx.Conn
)

// Pool returns a connection pool to the migrated test database, skipping the
// calling test when PostgreSQL is not available.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	setupOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		setupPool, setupErr = setup(ctx)
	})
	if setupErr != nil {
		t.Skipf("postgres not available: %v", setupErr)
	}
	return setupPool
}

// URL returns the test database URL, overridable via TEST_DATABASE_URL.
func URL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://biomed:biomed@localhost:5432/biomed_cmms_test?sslmode=disable"
}

func setup(ctx context.Context) (*pgxpool.Pool, error) {
	url := URL()

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	name := cfg.ConnConfig.Database
	cfg.ConnConfig.Database = "postgres"

	conn, err := pgx.ConnectConfig(ctx, cfg.ConnConfig)
	if err != nil {
		return nil, err
	}

	ok := false
	defer func() {
		if !ok {
			conn.Close(ctx)
		}
	}()

	// Serialize against other test binaries before touching the database.
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, advisoryLockKey); err != nil {
		return nil, err
	}

	if err := ensureDatabase(ctx, conn, name); err != nil {
		return nil, err
	}
	if err := applyMigrations(ctx, url); err != nil {
		return nil, err
	}

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}

	lockConn = conn
	ok = true
	return pool, nil
}

func ensureDatabase(ctx context.Context, conn *pgx.Conn, name string) error {
	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err := conn.Exec(ctx, "CREATE DATABASE "+name)
	return err
}

func applyMigrations(ctx context.Context, url string) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer db.Close()

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}

	driver, err := pgxv5.WithInstance(db, &pgxv5.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
