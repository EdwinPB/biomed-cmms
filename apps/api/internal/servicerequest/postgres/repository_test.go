package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edwinpolo/biomed-cmms/api/internal/dbtest"
	"github.com/edwinpolo/biomed-cmms/api/internal/servicerequest"
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

func createUser(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (tenant_id, email, password_hash) VALUES ($1, $2, 'unused-hash') RETURNING id`,
		tenantID, "user-"+uniqueString(t)+"@test.dev").Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func createEquipment(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO equipment (tenant_id, asset_tag, name, serial_number) VALUES ($1, $2, 'Device', 'SN') RETURNING id`,
		tenantID, "EQ-"+uniqueString(t)).Scan(&id); err != nil {
		t.Fatalf("insert equipment: %v", err)
	}
	return id
}

func truncateTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE service_requests, equipment, users, tenants`); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func createRequest(t *testing.T, repo *Repository, pool *pgxpool.Pool, tenantID uuid.UUID, mut ...func(*servicerequest.CreateParams)) servicerequest.ServiceRequest {
	t.Helper()
	userID := createUser(t, pool, tenantID)
	equipmentID := createEquipment(t, pool, tenantID)

	params := servicerequest.CreateParams{
		TenantID:    tenantID,
		EquipmentID: equipmentID,
		Title:       "Pump not running",
		Description: "Infusion pump reports error on startup.",
		CreatedBy:   userID,
	}
	for _, m := range mut {
		m(&params)
	}

	sr, err := repo.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return sr
}

func TestCreate(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	sr := createRequest(t, repo, pool, tenantID)

	if sr.ID == uuid.Nil {
		t.Error("Create() generated id is nil")
	}
	if sr.TenantID != tenantID {
		t.Errorf("Create() tenant_id = %v, want %v", sr.TenantID, tenantID)
	}
	if sr.Title != "Pump not running" || sr.Description == "" {
		t.Errorf("Create() title/description = %+v", sr)
	}
	if sr.Priority != servicerequest.PriorityMedium {
		t.Errorf("Create() priority = %q, want medium", sr.Priority)
	}
	if sr.Status != servicerequest.StatusPending {
		t.Errorf("Create() status = %q, want pending", sr.Status)
	}
	if sr.AssignedTo != nil {
		t.Errorf("Create() assigned_to = %v, want nil", *sr.AssignedTo)
	}
	if sr.ResolutionNotes != nil {
		t.Errorf("Create() resolution_notes = %q, want nil", *sr.ResolutionNotes)
	}
	if sr.CreatedAt.IsZero() || sr.UpdatedAt.IsZero() {
		t.Error("Create() timestamps are zero")
	}
}

func TestCreateExplicitPriorityAndAssignee(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	assignee := createUser(t, pool, tenantID)
	sr := createRequest(t, repo, pool, tenantID, func(p *servicerequest.CreateParams) {
		p.Priority = servicerequest.PriorityCritical
		p.AssignedTo = &assignee
	})

	if sr.Priority != servicerequest.PriorityCritical {
		t.Errorf("Create() priority = %q, want critical", sr.Priority)
	}
	if sr.AssignedTo == nil || *sr.AssignedTo != assignee {
		t.Errorf("Create() assigned_to = %v, want %v", sr.AssignedTo, assignee)
	}
}

func TestGetByID(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	created := createRequest(t, repo, pool, tenantID)

	got, err := repo.GetByID(context.Background(), tenantID, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got != created {
		t.Errorf("GetByID() = %+v, want %+v", got, created)
	}
}

func TestUpdateStatus(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	created := createRequest(t, repo, pool, tenantID)

	got, err := repo.UpdateStatus(ctx, tenantID, created.ID, servicerequest.StatusAssigned)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if got.Status != servicerequest.StatusAssigned {
		t.Errorf("UpdateStatus() status = %q, want %q", got.Status, servicerequest.StatusAssigned)
	}
	if got.ID != created.ID {
		t.Errorf("UpdateStatus() id = %v, want %v", got.ID, created.ID)
	}
}

func TestUpdateStatusInvalidValueRejected(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	created := createRequest(t, repo, pool, tenantID)

	_, err := repo.UpdateStatus(ctx, tenantID, created.ID, "bogus")
	if err == nil {
		t.Fatal("UpdateStatus() invalid status error = nil, want error")
	}
}

func TestUpdateStatusWrongTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	created := createRequest(t, repo, pool, tenantA)

	_, err := repo.UpdateStatus(ctx, tenantB, created.ID, servicerequest.StatusAssigned)
	if !errors.Is(err, servicerequest.ErrNotFound) {
		t.Errorf("UpdateStatus() across tenants error = %v, want ErrNotFound", err)
	}
}

func TestUpdateStatusNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	_, err := repo.UpdateStatus(context.Background(), tenantID, uuid.MustParse("00000000-0000-0000-0000-000000000001"), servicerequest.StatusAssigned)
	if !errors.Is(err, servicerequest.ErrNotFound) {
		t.Errorf("UpdateStatus() error = %v, want ErrNotFound", err)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	_, err := repo.GetByID(context.Background(), tenantID, uuid.MustParse("00000000-0000-0000-0000-000000000001"))
	if !errors.Is(err, servicerequest.ErrNotFound) {
		t.Errorf("GetByID() error = %v, want servicerequest.ErrNotFound", err)
	}
}

func TestGetByIDWrongTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	created := createRequest(t, repo, pool, tenantA)

	_, err := repo.GetByID(context.Background(), tenantB, created.ID)
	if err != servicerequest.ErrNotFound {
		t.Errorf("GetByID() across tenants error = %v, want ErrNotFound", err)
	}
}

func TestListByTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	for i := 0; i < 3; i++ {
		createRequest(t, repo, pool, tenantA)
	}
	createRequest(t, repo, pool, tenantB)

	list, err := repo.ListByTenant(context.Background(), tenantA)
	if err != nil {
		t.Fatalf("ListByTenant() error = %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("ListByTenant() returned %d items, want 3", len(list))
	}
	for _, sr := range list {
		if sr.TenantID != tenantA {
			t.Errorf("ListByTenant() leaked request from another tenant: %+v", sr)
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

func TestCreateCrossTenantEquipmentRejected(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	userA := createUser(t, pool, tenantA)
	equipmentB := createEquipment(t, pool, tenantB)

	_, err := repo.Create(ctx, servicerequest.CreateParams{
		TenantID:    tenantA,
		EquipmentID: equipmentB,
		Title:       "Cross tenant",
		Description: "should fail",
		CreatedBy:   userA,
	})
	if err == nil {
		t.Fatal("Create() cross-tenant equipment error = nil, want error")
	}
}

func TestCreateCrossTenantCreatedByRejected(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	userB := createUser(t, pool, tenantB)
	equipmentA := createEquipment(t, pool, tenantA)

	_, err := repo.Create(ctx, servicerequest.CreateParams{
		TenantID:    tenantA,
		EquipmentID: equipmentA,
		Title:       "Cross tenant",
		Description: "should fail",
		CreatedBy:   userB,
	})
	if err == nil {
		t.Fatal("Create() cross-tenant created_by error = nil, want error")
	}
}

func TestCreateCrossTenantAssignedToRejected(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	userA := createUser(t, pool, tenantA)
	userB := createUser(t, pool, tenantB)
	equipmentA := createEquipment(t, pool, tenantA)

	_, err := repo.Create(ctx, servicerequest.CreateParams{
		TenantID:    tenantA,
		EquipmentID: equipmentA,
		Title:       "Cross tenant",
		Description: "should fail",
		CreatedBy:   userA,
		AssignedTo:  &userB,
	})
	if err == nil {
		t.Fatal("Create() cross-tenant assigned_to error = nil, want error")
	}
}

func TestCreateInvalidPriorityRejected(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	_, err := createRequestErr(t, repo, pool, tenantID, func(p *servicerequest.CreateParams) {
		p.Priority = "urgent"
	})
	if err == nil {
		t.Fatal("Create() invalid priority error = nil, want error")
	}
}

func TestCreateInvalidStatusRejected(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	_, err := createRequestErr(t, repo, pool, tenantID, func(p *servicerequest.CreateParams) {
		p.Status = "open"
	})
	if err == nil {
		t.Fatal("Create() invalid status error = nil, want error")
	}
}

func createRequestErr(t *testing.T, repo *Repository, pool *pgxpool.Pool, tenantID uuid.UUID, mut func(*servicerequest.CreateParams)) (servicerequest.ServiceRequest, error) {
	t.Helper()
	userID := createUser(t, pool, tenantID)
	equipmentID := createEquipment(t, pool, tenantID)

	params := servicerequest.CreateParams{
		TenantID:    tenantID,
		EquipmentID: equipmentID,
		Title:       "Pump not running",
		Description: "Infusion pump reports error on startup.",
		CreatedBy:   userID,
	}
	mut(&params)

	return repo.Create(context.Background(), params)
}
