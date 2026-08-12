// Package postgres provides a PostgreSQL-backed implementation of
// servicerequest.Repository. All SQL lives here, never in the domain layer.
//
// Tenant isolation for equipment/user references is enforced by composite
// foreign keys in the schema (see migration 000004), so a request can never
// reference another tenant's equipment or users.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edwinpolo/biomed-cmms/api/internal/servicerequest"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const serviceRequestColumns = "id, tenant_id, equipment_id, title, description, priority, status, " +
	"created_by, assigned_to, resolution_notes, created_at, updated_at"

func (r *Repository) Create(ctx context.Context, params servicerequest.CreateParams) (servicerequest.ServiceRequest, error) {
	const query = `INSERT INTO service_requests
		(tenant_id, equipment_id, title, description, priority, status, created_by, assigned_to)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING ` + serviceRequestColumns

	priority := params.Priority
	if priority == "" {
		priority = servicerequest.PriorityMedium
	}
	status := params.Status
	if status == "" {
		status = servicerequest.StatusPending
	}

	sr, err := scanServiceRequest(r.pool.QueryRow(ctx, query,
		params.TenantID, params.EquipmentID, params.Title, params.Description,
		priority, status, params.CreatedBy, params.AssignedTo))
	if err != nil {
		return servicerequest.ServiceRequest{}, fmt.Errorf("create service request: %w", err)
	}
	return sr, nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (servicerequest.ServiceRequest, error) {
	const query = `SELECT ` + serviceRequestColumns + ` FROM service_requests WHERE id = $1 AND tenant_id = $2`

	sr, err := scanServiceRequest(r.pool.QueryRow(ctx, query, id, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return servicerequest.ServiceRequest{}, servicerequest.ErrNotFound
	}
	if err != nil {
		return servicerequest.ServiceRequest{}, fmt.Errorf("get service request by id: %w", err)
	}
	return sr, nil
}

func (r *Repository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]servicerequest.ServiceRequest, error) {
	const query = `SELECT ` + serviceRequestColumns + ` FROM service_requests WHERE tenant_id = $1 ORDER BY created_at DESC, id`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list service requests by tenant: %w", err)
	}
	defer rows.Close()

	list := make([]servicerequest.ServiceRequest, 0)
	for rows.Next() {
		sr, err := scanServiceRequest(rows)
		if err != nil {
			return nil, fmt.Errorf("scan service request row: %w", err)
		}
		list = append(list, sr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service request rows: %w", err)
	}
	return list, nil
}

func scanServiceRequest(row pgx.Row) (servicerequest.ServiceRequest, error) {
	var sr servicerequest.ServiceRequest
	err := row.Scan(&sr.ID, &sr.TenantID, &sr.EquipmentID, &sr.Title, &sr.Description, &sr.Priority, &sr.Status,
		&sr.CreatedBy, &sr.AssignedTo, &sr.ResolutionNotes, &sr.CreatedAt, &sr.UpdatedAt)
	return sr, err
}
