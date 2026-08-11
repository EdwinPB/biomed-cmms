// Package postgres provides a PostgreSQL-backed implementation of
// tenant.Repository. All SQL lives here, never in the domain layer.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const tenantColumns = "id, slug, name, status, created_at, updated_at"

func (r *Repository) Create(ctx context.Context, params tenant.CreateParams) (tenant.Tenant, error) {
	const query = `INSERT INTO tenants (slug, name) VALUES ($1, $2) RETURNING ` + tenantColumns

	t, err := scanTenant(r.pool.QueryRow(ctx, query, params.Slug, params.Name))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return tenant.Tenant{}, tenant.ErrConflict
		}
		return tenant.Tenant{}, fmt.Errorf("create tenant: %w", err)
	}
	return t, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (tenant.Tenant, error) {
	const query = `SELECT ` + tenantColumns + ` FROM tenants WHERE id = $1`

	t, err := scanTenant(r.pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return tenant.Tenant{}, tenant.ErrNotFound
	}
	if err != nil {
		return tenant.Tenant{}, fmt.Errorf("get tenant by id: %w", err)
	}
	return t, nil
}

func (r *Repository) GetBySlug(ctx context.Context, slug string) (tenant.Tenant, error) {
	const query = `SELECT ` + tenantColumns + ` FROM tenants WHERE slug = $1`

	t, err := scanTenant(r.pool.QueryRow(ctx, query, slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return tenant.Tenant{}, tenant.ErrNotFound
	}
	if err != nil {
		return tenant.Tenant{}, fmt.Errorf("get tenant by slug: %w", err)
	}
	return t, nil
}

func scanTenant(row pgx.Row) (tenant.Tenant, error) {
	var t tenant.Tenant
	err := row.Scan(&t.ID, &t.Slug, &t.Name, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}
