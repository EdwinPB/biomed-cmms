package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/edwinpolo/biomed-cmms/api/internal/migrations"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

// Integration tests against a local PostgreSQL test database. They reuse the
// same docker-compose Postgres instance; if it is unreachable the suite is
// skipped so `go test ./...` still passes on machines without Docker.

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	os.Exit(runTestSuite(m))
}

func runTestSuite(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	url := testDatabaseURL()

	if err := ensureTestDatabase(ctx, url); err != nil {
		fmt.Printf("skipping tenant repository integration tests: %v\n", err)
		return 0
	}
	if err := migrateTestDatabase(ctx, url); err != nil {
		fmt.Printf("skipping tenant repository integration tests: %v\n", err)
		return 0
	}

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		fmt.Printf("skipping tenant repository integration tests: %v\n", err)
		return 0
	}
	defer pool.Close()
	testPool = pool

	return m.Run()
}

func testDatabaseURL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://biomed:biomed@localhost:5432/biomed_cmms_test?sslmode=disable"
}

func ensureTestDatabase(ctx context.Context, url string) error {
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

func migrateTestDatabase(ctx context.Context, url string) error {
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

func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	if testPool == nil {
		t.Skip("postgres not available")
	}
	return NewRepository(testPool)
}

func uniqueSlug(t *testing.T) string {
	t.Helper()
	return "t-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}

func truncateTables(t *testing.T) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), `TRUNCATE users, tenants`); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func TestCreate(t *testing.T) {
	repo := newTestRepo(t)
	truncateTables(t)
	ctx := context.Background()

	got, err := repo.Create(ctx, tenant.CreateParams{Slug: uniqueSlug(t), Name: "Acme Health"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got.ID == uuid.Nil {
		t.Error("Create() generated id is nil")
	}
	if got.Slug == "" {
		t.Error("Create() slug is empty")
	}
	if got.Name != "Acme Health" {
		t.Errorf("Create() name = %q, want %q", got.Name, "Acme Health")
	}
	if got.Status != tenant.StatusActive {
		t.Errorf("Create() status = %q, want %q", got.Status, tenant.StatusActive)
	}
	if got.CreatedAt.IsZero() {
		t.Error("Create() created_at is zero")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("Create() updated_at is zero")
	}
}

func TestGetByID(t *testing.T) {
	repo := newTestRepo(t)
	truncateTables(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, tenant.CreateParams{Slug: uniqueSlug(t), Name: "Acme Health"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got != created {
		t.Errorf("GetByID() = %+v, want %+v", got, created)
	}
}

func TestGetBySlug(t *testing.T) {
	repo := newTestRepo(t)
	truncateTables(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, tenant.CreateParams{Slug: uniqueSlug(t), Name: "Acme Health"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetBySlug(ctx, created.Slug)
	if err != nil {
		t.Fatalf("GetBySlug() error = %v", err)
	}
	if got != created {
		t.Errorf("GetBySlug() = %+v, want %+v", got, created)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	repo := newTestRepo(t)
	truncateTables(t)

	_, err := repo.GetByID(context.Background(), uuid.MustParse("00000000-0000-0000-0000-000000000001"))
	if !errors.Is(err, tenant.ErrNotFound) {
		t.Errorf("GetByID() error = %v, want tenant.ErrNotFound", err)
	}
}

func TestGetBySlugNotFound(t *testing.T) {
	repo := newTestRepo(t)
	truncateTables(t)

	_, err := repo.GetBySlug(context.Background(), "does-not-exist")
	if !errors.Is(err, tenant.ErrNotFound) {
		t.Errorf("GetBySlug() error = %v, want tenant.ErrNotFound", err)
	}
}

func TestCreateDuplicateSlug(t *testing.T) {
	repo := newTestRepo(t)
	truncateTables(t)
	ctx := context.Background()

	params := tenant.CreateParams{Slug: uniqueSlug(t), Name: "Acme Health"}
	if _, err := repo.Create(ctx, params); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err := repo.Create(ctx, params)
	if !errors.Is(err, tenant.ErrConflict) {
		t.Errorf("Create() duplicate error = %v, want tenant.ErrConflict", err)
	}
}
