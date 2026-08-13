// Package postgres provides a PostgreSQL-backed implementation of
// auth.Repository. All SQL lives here, never in the domain layer.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetTenantBySlug(ctx context.Context, slug string) (tenant.Tenant, error) {
	const query = `SELECT id, slug, name, status, created_at, updated_at FROM tenants WHERE slug = $1`

	var t tenant.Tenant
	err := r.pool.QueryRow(ctx, query, slug).Scan(&t.ID, &t.Slug, &t.Name, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return tenant.Tenant{}, tenant.ErrNotFound
	}
	if err != nil {
		return tenant.Tenant{}, fmt.Errorf("get tenant by slug: %w", err)
	}
	return t, nil
}

func (r *Repository) GetUserByTenantEmail(ctx context.Context, tenantID uuid.UUID, email string) (auth.User, error) {
	const query = `SELECT id, tenant_id, email, password_hash, full_name, role, is_active, last_login_at, created_at, updated_at
		FROM users WHERE tenant_id = $1 AND email = $2`

	u, err := scanUser(r.pool.QueryRow(ctx, query, tenantID, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("get user by tenant and email: %w", err)
	}
	return u, nil
}

func (r *Repository) ListUsers(ctx context.Context, tenantID uuid.UUID) ([]auth.User, error) {
	const query = `SELECT id, tenant_id, email, password_hash, full_name, role, is_active, last_login_at, created_at, updated_at
		FROM users WHERE tenant_id = $1 ORDER BY created_at DESC, id`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list users by tenant: %w", err)
	}
	defer rows.Close()

	list := make([]auth.User, 0)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user row: %w", err)
		}
		list = append(list, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user rows: %w", err)
	}
	return list, nil
}

func (r *Repository) CreateUser(ctx context.Context, params auth.CreateParams) (auth.User, error) {
	const query = `INSERT INTO users (tenant_id, email, password_hash, full_name, role, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, tenant_id, email, password_hash, full_name, role, is_active, last_login_at, created_at, updated_at`

	u, err := scanUser(r.pool.QueryRow(ctx, query,
		params.TenantID, params.Email, params.PasswordHash, params.FullName, params.Role, params.IsActive))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return auth.User{}, auth.ErrConflict
		}
		return auth.User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (r *Repository) CreateSession(ctx context.Context, params auth.CreateSessionParams) error {
	const query = `INSERT INTO auth_sessions (token_hash, user_id, tenant_id, expires_at)
		VALUES ($1, $2, $3, $4)`

	if _, err := r.pool.Exec(ctx, query, params.TokenHash, params.UserID, params.TenantID, params.ExpiresAt); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *Repository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (auth.SessionRecord, error) {
	const query = `SELECT s.id, s.token_hash, s.expires_at, s.created_at, s.last_used_at,
		u.id, u.tenant_id, u.email, u.password_hash, u.full_name, u.role, u.is_active, u.last_login_at, u.created_at, u.updated_at,
		t.id, t.slug, t.name, t.status, t.created_at, t.updated_at
		FROM auth_sessions s
		JOIN users u ON u.id = s.user_id
		JOIN tenants t ON t.id = s.tenant_id
		WHERE s.token_hash = $1`

	var rec auth.SessionRecord
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&rec.ID, &rec.TokenHash, &rec.ExpiresAt, &rec.CreatedAt, &rec.LastUsedAt,
		&rec.User.ID, &rec.User.TenantID, &rec.User.Email, &rec.User.PasswordHash, &rec.User.FullName, &rec.User.Role, &rec.User.IsActive, &rec.User.LastLoginAt, &rec.User.CreatedAt, &rec.User.UpdatedAt,
		&rec.Tenant.ID, &rec.Tenant.Slug, &rec.Tenant.Name, &rec.Tenant.Status, &rec.Tenant.CreatedAt, &rec.Tenant.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.SessionRecord{}, auth.ErrSessionNotFound
	}
	if err != nil {
		return auth.SessionRecord{}, fmt.Errorf("get session by token hash: %w", err)
	}
	return rec, nil
}

func (r *Repository) DeleteSession(ctx context.Context, tokenHash string) error {
	const query = `DELETE FROM auth_sessions WHERE token_hash = $1`

	tag, err := r.pool.Exec(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrSessionNotFound
	}
	return nil
}

func (r *Repository) TouchSession(ctx context.Context, tokenHash string, now time.Time) error {
	const query = `UPDATE auth_sessions SET last_used_at = $2 WHERE token_hash = $1`

	tag, err := r.pool.Exec(ctx, query, tokenHash, now)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrSessionNotFound
	}
	return nil
}

func scanUser(row pgx.Row) (auth.User, error) {
	var u auth.User
	err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.FullName, &u.Role, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}
