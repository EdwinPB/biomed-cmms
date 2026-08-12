// Package dbtest provides shared bootstrap for repository integration tests:
// it creates the test database on demand, applies the embedded migrations, and
// opens a connection pool. Tests skip when PostgreSQL is unreachable.
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

var (
	setupOnce sync.Once
	setupPool *pgxpool.Pool
	setupErr  error
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

	if err := ensureDatabase(ctx, url); err != nil {
		return nil, err
	}
	if err := applyMigrations(ctx, url); err != nil {
		return nil, err
	}
	return pgxpool.New(ctx, url)
}

func ensureDatabase(ctx context.Context, url string) error {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return err
	}

	name := cfg.ConnConfig.Database
	cfg.ConnConfig.Database = "postgres"

	conn, err := pgx.ConnectConfig(ctx, cfg.ConnConfig)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = conn.Exec(ctx, "CREATE DATABASE "+name)
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
