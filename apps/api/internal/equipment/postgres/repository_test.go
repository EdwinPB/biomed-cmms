package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edwinpolo/biomed-cmms/api/internal/dbtest"
	"github.com/edwinpolo/biomed-cmms/api/internal/equipment"
)

func newTestRepo(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()
	pool := dbtest.Pool(t)
	return NewRepository(pool), pool
}

func uniqueString(t *testing.T) string {
	t.Helper()
	return strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}

func createTenant(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO tenants (slug, name) VALUES ($1, $2) RETURNING id`,
		"t-"+uniqueString(t), "Test Hospital").Scan(&id); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	return id
}

func truncateTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE equipment, users, tenants`); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func TestCreate(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	got, err := repo.Create(ctx, equipment.CreateParams{
		TenantID:     tenantID,
		AssetTag:     "EQ-001",
		Name:         "Infusion Pump",
		SerialNumber: "SN-1234",
		Location:     "Ward 3",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got.ID == uuid.Nil {
		t.Error("Create() generated id is nil")
	}
	if got.TenantID != tenantID {
		t.Errorf("Create() tenant_id = %v, want %v", got.TenantID, tenantID)
	}
	if got.AssetTag != "EQ-001" || got.Name != "Infusion Pump" || got.SerialNumber != "SN-1234" || got.Location != "Ward 3" {
		t.Errorf("Create() fields = %+v", got)
	}
	if got.Status != equipment.StatusOperational {
		t.Errorf("Create() status = %q, want %q", got.Status, equipment.StatusOperational)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Error("Create() timestamps are zero")
	}
}

func TestCreateExplicitStatus(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	got, err := repo.Create(ctx, equipment.CreateParams{
		TenantID:     tenantID,
		AssetTag:     "EQ-002",
		Name:         "MRI",
		SerialNumber: "SN-99",
		Status:       equipment.StatusMaintenance,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.Status != equipment.StatusMaintenance {
		t.Errorf("Create() status = %q, want %q", got.Status, equipment.StatusMaintenance)
	}
}

func TestGetByID(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	created, err := repo.Create(ctx, equipment.CreateParams{TenantID: tenantID, AssetTag: "EQ-001", Name: "Infusion Pump", SerialNumber: "SN-1"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByID(ctx, tenantID, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got != created {
		t.Errorf("GetByID() = %+v, want %+v", got, created)
	}
}

func TestGetByIDWrongTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	created, err := repo.Create(ctx, equipment.CreateParams{TenantID: tenantA, AssetTag: "EQ-001", Name: "Infusion Pump", SerialNumber: "SN-1"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = repo.GetByID(ctx, tenantB, created.ID)
	if !errors.Is(err, equipment.ErrNotFound) {
		t.Errorf("GetByID() across tenants error = %v, want equipment.ErrNotFound", err)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	_, err := repo.GetByID(context.Background(), tenantID, uuid.MustParse("00000000-0000-0000-0000-000000000001"))
	if !errors.Is(err, equipment.ErrNotFound) {
		t.Errorf("GetByID() error = %v, want equipment.ErrNotFound", err)
	}
}

func TestListByTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	for _, assetTag := range []string{"EQ-A1", "EQ-A2", "EQ-A3"} {
		if _, err := repo.Create(ctx, equipment.CreateParams{TenantID: tenantA, AssetTag: assetTag, Name: "Device " + assetTag, SerialNumber: "SN-" + assetTag}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}
	if _, err := repo.Create(ctx, equipment.CreateParams{TenantID: tenantB, AssetTag: "EQ-B1", Name: "Other Device", SerialNumber: "SN-B1"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	list, err := repo.ListByTenant(ctx, tenantA)
	if err != nil {
		t.Fatalf("ListByTenant() error = %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("ListByTenant() returned %d items, want 3", len(list))
	}
	for _, e := range list {
		if e.TenantID != tenantA {
			t.Errorf("ListByTenant() leaked equipment from another tenant: %+v", e)
		}
	}
}

func TestListByTenantEmpty(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	list, err := repo.ListByTenant(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListByTenant() error = %v", err)
	}
	if list == nil {
		t.Error("ListByTenant() returned nil, want empty slice")
	}
	if len(list) != 0 {
		t.Errorf("ListByTenant() returned %d items, want 0", len(list))
	}
}

func TestCreateDuplicateAssetTagSameTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	params := equipment.CreateParams{TenantID: tenantID, AssetTag: "EQ-001", Name: "Infusion Pump", SerialNumber: "SN-1"}
	if _, err := repo.Create(ctx, params); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err := repo.Create(ctx, params)
	if !errors.Is(err, equipment.ErrConflict) {
		t.Errorf("Create() duplicate error = %v, want equipment.ErrConflict", err)
	}
}

func TestCreateDuplicateAssetTagAcrossTenants(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	if _, err := repo.Create(ctx, equipment.CreateParams{TenantID: tenantA, AssetTag: "EQ-001", Name: "Infusion Pump", SerialNumber: "SN-1"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err := repo.Create(ctx, equipment.CreateParams{TenantID: tenantB, AssetTag: "EQ-001", Name: "Infusion Pump", SerialNumber: "SN-2"})
	if err != nil {
		t.Errorf("Create() same asset tag in different tenant = %v, want success", err)
	}
}
