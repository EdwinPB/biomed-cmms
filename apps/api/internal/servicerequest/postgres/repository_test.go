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
	if _, err := pool.Exec(context.Background(), `TRUNCATE auth_sessions, request_events, rfps, service_requests, equipment, users, tenants`); err != nil {
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

	got, err := repo.GetByID(context.Background(), tenantID, created.ID, nil)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got != created {
		t.Errorf("GetByID() = %+v, want %+v", got, created)
	}
}

func countEvents(t *testing.T, pool *pgxpool.Pool, tenantID, requestID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM request_events WHERE tenant_id = $1 AND request_id = $2`,
		tenantID, requestID).Scan(&n); err != nil {
		t.Fatalf("count request events: %v", err)
	}
	return n
}

func getRequestStatus(t *testing.T, pool *pgxpool.Pool, tenantID, requestID uuid.UUID) servicerequest.Status {
	t.Helper()
	var status servicerequest.Status
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM service_requests WHERE id = $1 AND tenant_id = $2`,
		requestID, tenantID).Scan(&status); err != nil {
		t.Fatalf("get request status: %v", err)
	}
	return status
}

func TestTransitionCreatesExactlyOneEvent(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	sr := createRequest(t, repo, pool, tenantID)
	actorID := createUser(t, pool, tenantID)

	event := servicerequest.RequestEvent{
		TenantID:   tenantID,
		RequestID:  sr.ID,
		ActorID:    actorID,
		FromStatus: servicerequest.StatusPending,
		ToStatus:   servicerequest.StatusAssigned,
	}
	got, err := repo.Transition(ctx, event)
	if err != nil {
		t.Fatalf("Transition() error = %v", err)
	}
	if got.Status != servicerequest.StatusAssigned {
		t.Errorf("Transition() status = %q, want assigned", got.Status)
	}

	if n := countEvents(t, pool, tenantID, sr.ID); n != 1 {
		t.Fatalf("event count = %d, want exactly 1", n)
	}

	var (
		id         uuid.UUID
		evTenantID uuid.UUID
		evReqID    uuid.UUID
		evActorID  uuid.UUID
		from, to   servicerequest.Status
		createdAt  time.Time
	)
	err = pool.QueryRow(ctx, `SELECT id, tenant_id, request_id, actor_id, from_status, to_status, created_at
		FROM request_events WHERE request_id = $1`, sr.ID).Scan(&id, &evTenantID, &evReqID, &evActorID, &from, &to, &createdAt)
	if err != nil {
		t.Fatalf("scan request event: %v", err)
	}
	if id == uuid.Nil {
		t.Error("event id is nil")
	}
	if evTenantID != tenantID {
		t.Errorf("event tenant = %v, want %v", evTenantID, tenantID)
	}
	if evReqID != sr.ID {
		t.Errorf("event request id = %v, want %v", evReqID, sr.ID)
	}
	if evActorID != actorID {
		t.Errorf("event actor = %v, want %v", evActorID, actorID)
	}
	if from != servicerequest.StatusPending {
		t.Errorf("event from = %q, want pending", from)
	}
	if to != servicerequest.StatusAssigned {
		t.Errorf("event to = %q, want assigned", to)
	}
	if createdAt.IsZero() {
		t.Error("event created_at is zero")
	}
}

func TestTransitionWrongTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	sr := createRequest(t, repo, pool, tenantA)
	actorID := createUser(t, pool, tenantB)

	_, err := repo.Transition(ctx, servicerequest.RequestEvent{
		TenantID:   tenantB,
		RequestID:  sr.ID,
		ActorID:    actorID,
		FromStatus: servicerequest.StatusPending,
		ToStatus:   servicerequest.StatusAssigned,
	})
	if !errors.Is(err, servicerequest.ErrNotFound) {
		t.Errorf("Transition() across tenants error = %v, want ErrNotFound", err)
	}
	if status := getRequestStatus(t, pool, tenantA, sr.ID); status != servicerequest.StatusPending {
		t.Errorf("status = %q, want unchanged pending", status)
	}
	if n := countEvents(t, pool, tenantA, sr.ID); n != 0 {
		t.Errorf("event count = %d, want 0", n)
	}
}

func TestTransitionNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	_, err := repo.Transition(context.Background(), servicerequest.RequestEvent{
		TenantID:   tenantID,
		RequestID:  uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		ActorID:    createUser(t, pool, tenantID),
		FromStatus: servicerequest.StatusPending,
		ToStatus:   servicerequest.StatusAssigned,
	})
	if !errors.Is(err, servicerequest.ErrNotFound) {
		t.Errorf("Transition() error = %v, want ErrNotFound", err)
	}
}

func TestTransitionStaleFromStatusRejected(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	sr := createRequest(t, repo, pool, tenantID)

	_, err := repo.Transition(ctx, servicerequest.RequestEvent{
		TenantID:   tenantID,
		RequestID:  sr.ID,
		ActorID:    createUser(t, pool, tenantID),
		FromStatus: servicerequest.StatusAssigned,
		ToStatus:   servicerequest.StatusInProgress,
	})
	if !errors.Is(err, servicerequest.ErrNotFound) {
		t.Errorf("Transition() stale from error = %v, want ErrNotFound", err)
	}
	if status := getRequestStatus(t, pool, tenantID, sr.ID); status != servicerequest.StatusPending {
		t.Errorf("status = %q, want unchanged pending", status)
	}
	if n := countEvents(t, pool, tenantID, sr.ID); n != 0 {
		t.Errorf("event count = %d, want 0", n)
	}
}

func TestTransitionCrossTenantActorRejected(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	sr := createRequest(t, repo, pool, tenantA)
	actorB := createUser(t, pool, tenantB)

	_, err := repo.Transition(ctx, servicerequest.RequestEvent{
		TenantID:   tenantA,
		RequestID:  sr.ID,
		ActorID:    actorB,
		FromStatus: servicerequest.StatusPending,
		ToStatus:   servicerequest.StatusAssigned,
	})
	if err == nil {
		t.Fatal("Transition() cross-tenant actor error = nil, want error")
	}
	if status := getRequestStatus(t, pool, tenantA, sr.ID); status != servicerequest.StatusPending {
		t.Errorf("status = %q, want unchanged pending (event insert must roll back)", status)
	}
	if n := countEvents(t, pool, tenantA, sr.ID); n != 0 {
		t.Errorf("event count = %d, want 0", n)
	}
}

func TestTransitionInvalidStatusRejected(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	sr := createRequest(t, repo, pool, tenantID)

	_, err := repo.Transition(ctx, servicerequest.RequestEvent{
		TenantID:   tenantID,
		RequestID:  sr.ID,
		ActorID:    createUser(t, pool, tenantID),
		FromStatus: "bogus",
		ToStatus:   "bogus",
	})
	if err == nil {
		t.Fatal("Transition() invalid status error = nil, want error")
	}
	if status := getRequestStatus(t, pool, tenantID, sr.ID); status != servicerequest.StatusPending {
		t.Errorf("status = %q, want unchanged pending (event insert must roll back)", status)
	}
	if n := countEvents(t, pool, tenantID, sr.ID); n != 0 {
		t.Errorf("event count = %d, want 0", n)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	_, err := repo.GetByID(context.Background(), tenantID, uuid.MustParse("00000000-0000-0000-0000-000000000001"), nil)
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

	_, err := repo.GetByID(context.Background(), tenantB, created.ID, nil)
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

	list, err := repo.ListByTenant(context.Background(), tenantA, nil)
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

	list, err := repo.ListByTenant(context.Background(), tenantID, nil)
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

func TestListEventsChronologicalOrder(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	sr := createRequest(t, repo, pool, tenantID)
	actorID := createUser(t, pool, tenantID)

	repo.Transition(ctx, servicerequest.RequestEvent{TenantID: tenantID, RequestID: sr.ID, ActorID: actorID, FromStatus: servicerequest.StatusPending, ToStatus: servicerequest.StatusAssigned})
	repo.Transition(ctx, servicerequest.RequestEvent{TenantID: tenantID, RequestID: sr.ID, ActorID: actorID, FromStatus: servicerequest.StatusAssigned, ToStatus: servicerequest.StatusInProgress})

	events, err := repo.ListEvents(ctx, tenantID, sr.ID, nil)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("ListEvents() returned %d events, want 2", len(events))
	}
	if events[0].FromStatus != servicerequest.StatusPending || events[0].ToStatus != servicerequest.StatusAssigned {
		t.Errorf("first event = %s->%s, want pending->assigned", events[0].FromStatus, events[0].ToStatus)
	}
	if events[1].FromStatus != servicerequest.StatusAssigned || events[1].ToStatus != servicerequest.StatusInProgress {
		t.Errorf("second event = %s->%s, want assigned->in_progress", events[1].FromStatus, events[1].ToStatus)
	}
	if events[0].CreatedAt.After(events[1].CreatedAt) {
		t.Errorf("events out of chronological order: %v then %v", events[0].CreatedAt, events[1].CreatedAt)
	}
	if events[0].ActorID != actorID || events[1].ActorID != actorID {
		t.Errorf("event actors = %v, %v, want %v", events[0].ActorID, events[1].ActorID, actorID)
	}
}

func TestListEventsTiebreakByID(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	sr := createRequest(t, repo, pool, tenantID)
	actorID := createUser(t, pool, tenantID)

	sameTime := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{
		"00000000-0000-0000-0000-00000000000a",
		"00000000-0000-0000-0000-00000000000b",
	} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO request_events (id, tenant_id, request_id, actor_id, from_status, to_status, created_at)
			 VALUES ($1, $2, $3, $4, 'pending', 'assigned', $5)`,
			uuid.MustParse(id), tenantID, sr.ID, actorID, sameTime); err != nil {
			t.Fatalf("insert event: %v", err)
		}
	}

	events, err := repo.ListEvents(ctx, tenantID, sr.ID, nil)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("ListEvents() returned %d events, want 2", len(events))
	}
	if events[0].ID.String() > events[1].ID.String() {
		t.Errorf("equal-timestamp events not ordered by id: %v then %v", events[0].ID, events[1].ID)
	}
}

func TestListEventsEmptyReturnsNonNil(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	sr := createRequest(t, repo, pool, tenantID)

	events, err := repo.ListEvents(ctx, tenantID, sr.ID, nil)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if events == nil {
		t.Error("ListEvents() returned nil, want non-nil empty slice")
	}
	if len(events) != 0 {
		t.Errorf("ListEvents() returned %d events, want 0", len(events))
	}
}

func TestListEventsWrongTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	sr := createRequest(t, repo, pool, tenantA)
	actorID := createUser(t, pool, tenantA)
	if _, err := repo.Transition(ctx, servicerequest.RequestEvent{TenantID: tenantA, RequestID: sr.ID, ActorID: actorID, FromStatus: servicerequest.StatusPending, ToStatus: servicerequest.StatusAssigned}); err != nil {
		t.Fatalf("Transition() error = %v", err)
	}

	_, err := repo.ListEvents(ctx, tenantB, sr.ID, nil)
	if !errors.Is(err, servicerequest.ErrNotFound) {
		t.Errorf("ListEvents() across tenants error = %v, want ErrNotFound", err)
	}
}

func TestListEventsUnknownRequest(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	tenantID := createTenant(t, pool)

	_, err := repo.ListEvents(context.Background(), tenantID, uuid.MustParse("00000000-0000-0000-0000-000000000001"), nil)
	if !errors.Is(err, servicerequest.ErrNotFound) {
		t.Errorf("ListEvents() unknown request error = %v, want ErrNotFound", err)
	}
}

func TestGetByIDCreatedByFilter(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	owner := createUser(t, pool, tenantID)
	sr := createRequest(t, repo, pool, tenantID, func(p *servicerequest.CreateParams) {
		p.CreatedBy = owner
	})

	got, err := repo.GetByID(ctx, tenantID, sr.ID, &owner)
	if err != nil {
		t.Fatalf("GetByID() own request error = %v", err)
	}
	if got.ID != sr.ID {
		t.Errorf("GetByID() id = %v, want %v", got.ID, sr.ID)
	}

	other := createUser(t, pool, tenantID)
	if _, err := repo.GetByID(ctx, tenantID, sr.ID, &other); !errors.Is(err, servicerequest.ErrNotFound) {
		t.Errorf("GetByID() other creator error = %v, want ErrNotFound", err)
	}
}

func TestGetByIDNilCreatorUnfiltered(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	sr := createRequest(t, repo, pool, tenantID)

	got, err := repo.GetByID(ctx, tenantID, sr.ID, nil)
	if err != nil {
		t.Fatalf("GetByID() unfiltered error = %v", err)
	}
	if got.ID != sr.ID {
		t.Errorf("GetByID() id = %v, want %v", got.ID, sr.ID)
	}
}

func TestListByTenantCreatedByFilter(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	userA := createUser(t, pool, tenantID)
	userB := createUser(t, pool, tenantID)

	srA := createRequest(t, repo, pool, tenantID, func(p *servicerequest.CreateParams) {
		p.CreatedBy = userA
	})
	createRequest(t, repo, pool, tenantID, func(p *servicerequest.CreateParams) {
		p.CreatedBy = userB
	})

	list, err := repo.ListByTenant(ctx, tenantID, &userA)
	if err != nil {
		t.Fatalf("ListByTenant() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListByTenant() returned %d items, want 1", len(list))
	}
	if list[0].ID != srA.ID {
		t.Errorf("ListByTenant() id = %v, want %v", list[0].ID, srA.ID)
	}
	if list[0].CreatedBy != userA {
		t.Errorf("ListByTenant() created_by = %v, want %v", list[0].CreatedBy, userA)
	}
}

func TestListEventsCreatedByFilter(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateTables(t, pool)
	ctx := context.Background()
	tenantID := createTenant(t, pool)

	owner := createUser(t, pool, tenantID)
	sr := createRequest(t, repo, pool, tenantID, func(p *servicerequest.CreateParams) {
		p.CreatedBy = owner
	})
	actorID := createUser(t, pool, tenantID)
	if _, err := repo.Transition(ctx, servicerequest.RequestEvent{
		TenantID: tenantID, RequestID: sr.ID, ActorID: actorID,
		FromStatus: servicerequest.StatusPending, ToStatus: servicerequest.StatusAssigned,
	}); err != nil {
		t.Fatalf("Transition() error = %v", err)
	}

	events, err := repo.ListEvents(ctx, tenantID, sr.ID, &owner)
	if err != nil {
		t.Fatalf("ListEvents() own request error = %v", err)
	}
	if len(events) != 1 {
		t.Errorf("ListEvents() returned %d events, want 1", len(events))
	}

	other := createUser(t, pool, tenantID)
	if _, err := repo.ListEvents(ctx, tenantID, sr.ID, &other); !errors.Is(err, servicerequest.ErrNotFound) {
		t.Errorf("ListEvents() other creator error = %v, want ErrNotFound", err)
	}

	events, err = repo.ListEvents(ctx, tenantID, sr.ID, nil)
	if err != nil {
		t.Fatalf("ListEvents() unfiltered error = %v", err)
	}
	if len(events) != 1 {
		t.Errorf("ListEvents() unfiltered returned %d events, want 1", len(events))
	}
}
