package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edwinpolo/biomed-cmms/api/internal/dbtest"
	"github.com/edwinpolo/biomed-cmms/api/internal/rfp"
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

func createServiceRequest(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO service_requests (tenant_id, equipment_id, title, description, created_by)
		 VALUES ($1, $2, 'Pump not running', 'Infusion pump error', $3) RETURNING id`,
		tenantID, createEquipment(t, pool, tenantID), createUser(t, pool, tenantID)).Scan(&id); err != nil {
		t.Fatalf("insert service request: %v", err)
	}
	return id
}

func truncateTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE auth_sessions, request_events, rfps, service_requests, equipment, users, tenants`); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func validParams(tenantID, serviceRequestID, createdBy uuid.UUID) rfp.CreateParams {
	return rfp.CreateParams{
		TenantID:         tenantID,
		ServiceRequestID: serviceRequestID,
		Title:            "MRI replacement",
		Description:      "Procure a replacement MRI scanner.",
		CreatedBy:        createdBy,
	}
}

func createRFP(t *testing.T, repo *Repository, params rfp.CreateParams) rfp.RFP {
	t.Helper()
	created, err := repo.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return created
}

func TestCreate(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)
	srID := createServiceRequest(t, pool, tenantID)
	userID := createUser(t, pool, tenantID)

	created := createRFP(t, repo, validParams(tenantID, srID, userID))

	if created.ID == uuid.Nil {
		t.Error("Create() generated id is nil")
	}
	if created.TenantID != tenantID {
		t.Errorf("Create() tenant_id = %v, want %v", created.TenantID, tenantID)
	}
	if created.ServiceRequestID != srID {
		t.Errorf("Create() service_request_id = %v, want %v", created.ServiceRequestID, srID)
	}
	if created.Title != "MRI replacement" || created.Description == "" {
		t.Errorf("Create() title/description = %+v", created)
	}
	if created.Status != rfp.StatusDraft {
		t.Errorf("Create() status = %q, want draft", created.Status)
	}
	if created.CreatedBy != userID {
		t.Errorf("Create() created_by = %v, want %v", created.CreatedBy, userID)
	}
	if created.DueAt != nil {
		t.Errorf("Create() due_at = %v, want nil", *created.DueAt)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Error("Create() timestamps are zero")
	}
}

func TestCreateExplicitStatus(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	params := validParams(tenantID, createServiceRequest(t, pool, tenantID), createUser(t, pool, tenantID))
	params.Status = rfp.StatusPublished

	created := createRFP(t, repo, params)
	if created.Status != rfp.StatusPublished {
		t.Errorf("Create() status = %q, want published", created.Status)
	}
}

func TestCreateWithDueDate(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	due := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	params := validParams(tenantID, createServiceRequest(t, pool, tenantID), createUser(t, pool, tenantID))
	params.DueAt = &due

	created := createRFP(t, repo, params)
	if created.DueAt == nil || !created.DueAt.Equal(due) {
		t.Errorf("Create() due_at = %v, want %v", created.DueAt, due)
	}
}

func TestGetByID(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	created := createRFP(t, repo, validParams(tenantID, createServiceRequest(t, pool, tenantID), createUser(t, pool, tenantID)))

	got, err := repo.GetByID(context.Background(), tenantID, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got != created {
		t.Errorf("GetByID() = %+v, want %+v", got, created)
	}
}

func TestGetByServiceRequest(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)
	srID := createServiceRequest(t, pool, tenantID)

	first := createRFP(t, repo, validParams(tenantID, srID, createUser(t, pool, tenantID)))

	got, err := repo.GetByServiceRequest(context.Background(), tenantID, srID)
	if err != nil {
		t.Fatalf("GetByServiceRequest() error = %v", err)
	}
	if got != first {
		t.Errorf("GetByServiceRequest() = %+v, want %+v", got, first)
	}
}

func TestGetByServiceRequestReturnsLatest(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)
	srID := createServiceRequest(t, pool, tenantID)
	userID := createUser(t, pool, tenantID)

	first := createRFP(t, repo, validParams(tenantID, srID, userID))

	if _, err := pool.Exec(ctx, `UPDATE rfps SET status = 'closed' WHERE id = $1`, first.ID); err != nil {
		t.Fatalf("close first rfp: %v", err)
	}
	second := createRFP(t, repo, validParams(tenantID, srID, userID))

	got, err := repo.GetByServiceRequest(ctx, tenantID, srID)
	if err != nil {
		t.Fatalf("GetByServiceRequest() error = %v", err)
	}
	if got != second {
		t.Errorf("GetByServiceRequest() = %+v, want latest %+v", got, second)
	}
}

func TestListByTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	for i := 0; i < 3; i++ {
		createRFP(t, repo, validParams(tenantA, createServiceRequest(t, pool, tenantA), createUser(t, pool, tenantA)))
	}
	createRFP(t, repo, validParams(tenantB, createServiceRequest(t, pool, tenantB), createUser(t, pool, tenantB)))

	list, err := repo.ListByTenant(context.Background(), tenantA)
	if err != nil {
		t.Fatalf("ListByTenant() error = %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("ListByTenant() returned %d items, want 3", len(list))
	}
	for _, created := range list {
		if created.TenantID != tenantA {
			t.Errorf("ListByTenant() leaked rfp from another tenant: %+v", created)
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

func TestGetByIDWrongTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	created := createRFP(t, repo, validParams(tenantA, createServiceRequest(t, pool, tenantA), createUser(t, pool, tenantA)))

	_, err := repo.GetByID(context.Background(), tenantB, created.ID)
	if !errors.Is(err, rfp.ErrNotFound) {
		t.Errorf("GetByID() across tenants error = %v, want ErrNotFound", err)
	}
}

func TestGetByServiceRequestWrongTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)
	srA := createServiceRequest(t, pool, tenantA)

	createRFP(t, repo, validParams(tenantA, srA, createUser(t, pool, tenantA)))

	_, err := repo.GetByServiceRequest(context.Background(), tenantB, srA)
	if !errors.Is(err, rfp.ErrNotFound) {
		t.Errorf("GetByServiceRequest() across tenants error = %v, want ErrNotFound", err)
	}
}

func TestCreateCrossTenantServiceRequestRejected(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	params := validParams(tenantA, createServiceRequest(t, pool, tenantB), createUser(t, pool, tenantA))

	_, err := repo.Create(context.Background(), params)
	if err == nil {
		t.Fatal("Create() cross-tenant service request error = nil, want error")
	}
}

func TestCreateCrossTenantCreatedByRejected(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	params := validParams(tenantA, createServiceRequest(t, pool, tenantA), createUser(t, pool, tenantB))

	_, err := repo.Create(context.Background(), params)
	if err == nil {
		t.Fatal("Create() cross-tenant created_by error = nil, want error")
	}
}

func TestCreateInvalidStatusRejected(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	params := validParams(tenantID, createServiceRequest(t, pool, tenantID), createUser(t, pool, tenantID))
	params.Status = "pending"

	_, err := repo.Create(context.Background(), params)
	if err == nil {
		t.Fatal("Create() invalid status error = nil, want error")
	}
}

func TestGetByIDNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	_, err := repo.GetByID(context.Background(), tenantID, uuid.MustParse("00000000-0000-0000-0000-000000000001"))
	if !errors.Is(err, rfp.ErrNotFound) {
		t.Errorf("GetByID() error = %v, want ErrNotFound", err)
	}
}

func TestGetByServiceRequestNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	_, err := repo.GetByServiceRequest(context.Background(), tenantID, uuid.MustParse("00000000-0000-0000-0000-000000000001"))
	if !errors.Is(err, rfp.ErrNotFound) {
		t.Errorf("GetByServiceRequest() error = %v, want ErrNotFound", err)
	}
}

func TestCreateActiveConflict(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)
	srID := createServiceRequest(t, pool, tenantID)
	userID := createUser(t, pool, tenantID)

	createRFP(t, repo, validParams(tenantID, srID, userID))

	_, err := repo.Create(ctx, validParams(tenantID, srID, userID))
	if !errors.Is(err, rfp.ErrConflict) {
		t.Errorf("Create() second active rfp error = %v, want ErrConflict", err)
	}
}

func TestCreateAfterCloseAllowed(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)
	srID := createServiceRequest(t, pool, tenantID)
	userID := createUser(t, pool, tenantID)

	first := createRFP(t, repo, validParams(tenantID, srID, userID))

	if _, err := pool.Exec(ctx, `UPDATE rfps SET status = 'closed' WHERE id = $1`, first.ID); err != nil {
		t.Fatalf("close first rfp: %v", err)
	}

	second := createRFP(t, repo, validParams(tenantID, srID, userID))
	if second.Status != rfp.StatusDraft {
		t.Errorf("second rfp status = %q, want draft", second.Status)
	}
}

func TestTransition(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	created := createRFP(t, repo, validParams(tenantID, createServiceRequest(t, pool, tenantID), createUser(t, pool, tenantID)))

	got, err := repo.Transition(ctx, tenantID, created.ID, rfp.StatusDraft, rfp.StatusPublished)
	if err != nil {
		t.Fatalf("Transition() error = %v", err)
	}
	if got.Status != rfp.StatusPublished {
		t.Errorf("Transition() status = %q, want published", got.Status)
	}
	if got.ID != created.ID {
		t.Errorf("Transition() id = %v, want %v", got.ID, created.ID)
	}
	if !got.UpdatedAt.Equal(got.CreatedAt) && got.UpdatedAt.Before(got.CreatedAt) {
		t.Errorf("Transition() updated_at = %v, want >= created_at %v", got.UpdatedAt, got.CreatedAt)
	}
}

func TestTransitionWrongTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	created := createRFP(t, repo, validParams(tenantA, createServiceRequest(t, pool, tenantA), createUser(t, pool, tenantA)))

	_, err := repo.Transition(context.Background(), tenantB, created.ID, rfp.StatusDraft, rfp.StatusPublished)
	if !errors.Is(err, rfp.ErrNotFound) {
		t.Errorf("Transition() across tenants error = %v, want ErrNotFound", err)
	}
}

func TestTransitionNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	_, err := repo.Transition(context.Background(), tenantID, uuid.MustParse("00000000-0000-0000-0000-000000000001"), rfp.StatusDraft, rfp.StatusPublished)
	if !errors.Is(err, rfp.ErrNotFound) {
		t.Errorf("Transition() error = %v, want ErrNotFound", err)
	}
}

func TestTransitionStaleFromRejected(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	created := createRFP(t, repo, validParams(tenantID, createServiceRequest(t, pool, tenantID), createUser(t, pool, tenantID)))

	_, err := repo.Transition(ctx, tenantID, created.ID, rfp.StatusPublished, rfp.StatusClosed)
	if !errors.Is(err, rfp.ErrNotFound) {
		t.Errorf("Transition() stale from error = %v, want ErrNotFound", err)
	}
}
