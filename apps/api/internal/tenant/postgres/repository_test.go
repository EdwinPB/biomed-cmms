package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edwinpolo/biomed-cmms/api/internal/dbtest"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

func newTestRepo(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()
	pool := dbtest.Pool(t)
	return NewRepository(pool), pool
}

func uniqueSlug(t *testing.T) string {
	t.Helper()
	return "t-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}

func truncateTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE service_requests, equipment, users, tenants`); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func TestCreate(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
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
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
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
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
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
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)

	_, err := repo.GetByID(context.Background(), uuid.MustParse("00000000-0000-0000-0000-000000000001"))
	if !errors.Is(err, tenant.ErrNotFound) {
		t.Errorf("GetByID() error = %v, want tenant.ErrNotFound", err)
	}
}

func TestGetBySlugNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)

	_, err := repo.GetBySlug(context.Background(), "does-not-exist")
	if !errors.Is(err, tenant.ErrNotFound) {
		t.Errorf("GetBySlug() error = %v, want tenant.ErrNotFound", err)
	}
}

func TestCreateDuplicateSlug(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
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
