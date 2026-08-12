// Package postgres provides a PostgreSQL-backed implementation of
// rfp.Repository. All SQL lives here, never in the domain layer.
//
// Tenant isolation for service request/user references is enforced by
// composite foreign keys in the schema (see migration 000006), so an RFP can
// never reference another tenant's service request or users. At most one
// active RFP (status draft or published) may exist per service request,
// enforced by a partial unique index.
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

	"github.com/edwinpolo/biomed-cmms/api/internal/rfp"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const rfpColumns = "id, tenant_id, service_request_id, title, description, status, due_at, created_by, created_at, updated_at"

const activeRfpConstraint = "idx_rfps_one_active_per_service_request"

func (r *Repository) Create(ctx context.Context, params rfp.CreateParams) (rfp.RFP, error) {
	const query = `INSERT INTO rfps (tenant_id, service_request_id, title, description, status, due_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING ` + rfpColumns

	status := params.Status
	if status == "" {
		status = rfp.StatusDraft
	}

	created, err := scanRFP(r.pool.QueryRow(ctx, query,
		params.TenantID, params.ServiceRequestID, params.Title, params.Description, status, params.DueAt, params.CreatedBy))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == activeRfpConstraint {
			return rfp.RFP{}, rfp.ErrConflict
		}
		return rfp.RFP{}, fmt.Errorf("create rfp: %w", err)
	}
	return created, nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (rfp.RFP, error) {
	const query = `SELECT ` + rfpColumns + ` FROM rfps WHERE id = $1 AND tenant_id = $2`

	created, err := scanRFP(r.pool.QueryRow(ctx, query, id, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return rfp.RFP{}, rfp.ErrNotFound
	}
	if err != nil {
		return rfp.RFP{}, fmt.Errorf("get rfp by id: %w", err)
	}
	return created, nil
}

// GetByServiceRequest returns the most recently created RFP for a service
// request, tenant-scoped. Returns ErrNotFound when the request has no RFP.
// Transition applies a validated status move. The UPDATE is guarded by the
// RFP's current status (optimistic concurrency): if the row is missing, belongs
// to another tenant, or already moved on, ErrNotFound is returned.
func (r *Repository) Transition(ctx context.Context, tenantID, id uuid.UUID, from, to rfp.Status) (rfp.RFP, error) {
	const query = `UPDATE rfps SET status = $1 WHERE id = $2 AND tenant_id = $3 AND status = $4 RETURNING ` + rfpColumns

	updated, err := scanRFP(r.pool.QueryRow(ctx, query, to, id, tenantID, from))
	if errors.Is(err, pgx.ErrNoRows) {
		return rfp.RFP{}, rfp.ErrNotFound
	}
	if err != nil {
		return rfp.RFP{}, fmt.Errorf("transition rfp: %w", err)
	}
	return updated, nil
}

func (r *Repository) GetByServiceRequest(ctx context.Context, tenantID, serviceRequestID uuid.UUID) (rfp.RFP, error) {
	const query = `SELECT ` + rfpColumns + ` FROM rfps
		WHERE tenant_id = $1 AND service_request_id = $2
		ORDER BY created_at DESC, id DESC
		LIMIT 1`

	created, err := scanRFP(r.pool.QueryRow(ctx, query, tenantID, serviceRequestID))
	if errors.Is(err, pgx.ErrNoRows) {
		return rfp.RFP{}, rfp.ErrNotFound
	}
	if err != nil {
		return rfp.RFP{}, fmt.Errorf("get rfp by service request: %w", err)
	}
	return created, nil
}

func (r *Repository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]rfp.RFP, error) {
	const query = `SELECT ` + rfpColumns + ` FROM rfps WHERE tenant_id = $1 ORDER BY created_at DESC, id`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list rfps by tenant: %w", err)
	}
	defer rows.Close()

	list := make([]rfp.RFP, 0)
	for rows.Next() {
		created, err := scanRFP(rows)
		if err != nil {
			return nil, fmt.Errorf("scan rfp row: %w", err)
		}
		list = append(list, created)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rfp rows: %w", err)
	}
	return list, nil
}

func scanRFP(row pgx.Row) (rfp.RFP, error) {
	var created rfp.RFP
	err := row.Scan(&created.ID, &created.TenantID, &created.ServiceRequestID, &created.Title, &created.Description,
		&created.Status, &created.DueAt, &created.CreatedBy, &created.CreatedAt, &created.UpdatedAt)
	return created, err
}
