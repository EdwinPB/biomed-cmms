// Package postgres provides a PostgreSQL-backed implementation of
// equipment.Repository. All SQL lives here, never in the domain layer.
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

	"github.com/edwinpolo/biomed-cmms/api/internal/equipment"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const equipmentColumns = "id, tenant_id, asset_tag, name, serial_number, location, status, created_at, updated_at"

func (r *Repository) Create(ctx context.Context, params equipment.CreateParams) (equipment.Equipment, error) {
	const query = `INSERT INTO equipment (tenant_id, asset_tag, name, serial_number, location, status)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING ` + equipmentColumns

	status := params.Status
	if status == "" {
		status = equipment.StatusOperational
	}

	e, err := scanEquipment(r.pool.QueryRow(ctx, query,
		params.TenantID, params.AssetTag, params.Name, params.SerialNumber, params.Location, status))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return equipment.Equipment{}, equipment.ErrConflict
		}
		return equipment.Equipment{}, fmt.Errorf("create equipment: %w", err)
	}
	return e, nil
}

func (r *Repository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (equipment.Equipment, error) {
	const query = `SELECT ` + equipmentColumns + ` FROM equipment WHERE id = $1 AND tenant_id = $2`

	e, err := scanEquipment(r.pool.QueryRow(ctx, query, id, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return equipment.Equipment{}, equipment.ErrNotFound
	}
	if err != nil {
		return equipment.Equipment{}, fmt.Errorf("get equipment by id: %w", err)
	}
	return e, nil
}

func (r *Repository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]equipment.Equipment, error) {
	const query = `SELECT ` + equipmentColumns + ` FROM equipment WHERE tenant_id = $1 ORDER BY created_at DESC, id`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list equipment by tenant: %w", err)
	}
	defer rows.Close()

	list := make([]equipment.Equipment, 0)
	for rows.Next() {
		e, err := scanEquipment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan equipment row: %w", err)
		}
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate equipment rows: %w", err)
	}
	return list, nil
}

func scanEquipment(row pgx.Row) (equipment.Equipment, error) {
	var e equipment.Equipment
	err := row.Scan(&e.ID, &e.TenantID, &e.AssetTag, &e.Name, &e.SerialNumber, &e.Location, &e.Status, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}
