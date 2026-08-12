package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/servicerequest"
)

type fakeRepo struct {
	createFn       func(ctx context.Context, params servicerequest.CreateParams) (servicerequest.ServiceRequest, error)
	getByIDFn      func(ctx context.Context, tenantID, id uuid.UUID) (servicerequest.ServiceRequest, error)
	transitionFn   func(ctx context.Context, event servicerequest.RequestEvent) (servicerequest.ServiceRequest, error)
	listEventsFn   func(ctx context.Context, tenantID, requestID uuid.UUID) ([]servicerequest.RequestEvent, error)
	listByTenantFn func(ctx context.Context, tenantID uuid.UUID) ([]servicerequest.ServiceRequest, error)

	lastGetTenantID  uuid.UUID
	lastTransition   servicerequest.RequestEvent
	transitioned     []uuid.UUID
	lastListTenantID uuid.UUID
	lastListRequest  uuid.UUID
}

func (f *fakeRepo) Create(ctx context.Context, params servicerequest.CreateParams) (servicerequest.ServiceRequest, error) {
	if f.createFn != nil {
		return f.createFn(ctx, params)
	}
	return servicerequest.ServiceRequest{}, errors.New("fakeRepo: Create not configured")
}

func (f *fakeRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (servicerequest.ServiceRequest, error) {
	f.lastGetTenantID = tenantID
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, tenantID, id)
	}
	return servicerequest.ServiceRequest{}, errors.New("fakeRepo: GetByID not configured")
}

func (f *fakeRepo) Transition(ctx context.Context, event servicerequest.RequestEvent) (servicerequest.ServiceRequest, error) {
	f.lastTransition = event
	f.transitioned = append(f.transitioned, event.RequestID)
	if f.transitionFn != nil {
		return f.transitionFn(ctx, event)
	}
	return servicerequest.ServiceRequest{}, errors.New("fakeRepo: Transition not configured")
}

func (f *fakeRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]servicerequest.ServiceRequest, error) {
	f.lastListTenantID = tenantID
	if f.listByTenantFn != nil {
		return f.listByTenantFn(ctx, tenantID)
	}
	return nil, errors.New("fakeRepo: ListByTenant not configured")
}

func (f *fakeRepo) ListEvents(ctx context.Context, tenantID, requestID uuid.UUID) ([]servicerequest.RequestEvent, error) {
	f.lastListTenantID = tenantID
	f.lastListRequest = requestID
	if f.listEventsFn != nil {
		return f.listEventsFn(ctx, tenantID, requestID)
	}
	return nil, errors.New("fakeRepo: ListEvents not configured")
}

func testRequest(tenantID, id uuid.UUID, status servicerequest.Status) servicerequest.ServiceRequest {
	return servicerequest.ServiceRequest{ID: id, TenantID: tenantID, Status: status}
}

func TestCreateRequestSuccess(t *testing.T) {
	var gotParams servicerequest.CreateParams
	created := testRequest(uuid.MustParse("11111111-1111-1111-1111-111111111111"), uuid.Nil, servicerequest.StatusPending)
	fake := &fakeRepo{createFn: func(_ context.Context, params servicerequest.CreateParams) (servicerequest.ServiceRequest, error) {
		gotParams = params
		return created, nil
	}}
	svc := New(fake)

	params := validCreateParams()
	got, err := svc.CreateRequest(context.Background(), params)
	if err != nil {
		t.Fatalf("CreateRequest() error = %v", err)
	}
	if got != created {
		t.Errorf("CreateRequest() = %+v, want %+v", got, created)
	}
	if gotParams != params {
		t.Errorf("CreateRequest() repo params = %+v, want %+v", gotParams, params)
	}
}

func validCreateParams() servicerequest.CreateParams {
	return servicerequest.CreateParams{
		TenantID:    uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		EquipmentID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Title:       "Pump not running",
		Description: "Infusion pump reports error on startup.",
		Priority:    servicerequest.PriorityHigh,
		CreatedBy:   uuid.MustParse("33333333-3333-3333-3333-333333333333"),
	}
}

func TestCreateRequestValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*servicerequest.CreateParams)
		want   error
	}{
		{"missing tenant", func(p *servicerequest.CreateParams) { p.TenantID = uuid.Nil }, ErrTenantRequired},
		{"missing equipment", func(p *servicerequest.CreateParams) { p.EquipmentID = uuid.Nil }, ErrEquipmentRequired},
		{"missing created_by", func(p *servicerequest.CreateParams) { p.CreatedBy = uuid.Nil }, ErrCreatedByRequired},
		{"empty title", func(p *servicerequest.CreateParams) { p.Title = "" }, ErrTitleRequired},
		{"whitespace title", func(p *servicerequest.CreateParams) { p.Title = "   " }, ErrTitleRequired},
		{"empty description", func(p *servicerequest.CreateParams) { p.Description = "" }, ErrDescriptionRequired},
		{"invalid priority", func(p *servicerequest.CreateParams) { p.Priority = "urgent" }, ErrInvalidPriority},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			fake := &fakeRepo{createFn: func(context.Context, servicerequest.CreateParams) (servicerequest.ServiceRequest, error) {
				called = true
				return servicerequest.ServiceRequest{}, nil
			}}
			svc := New(fake)

			params := validCreateParams()
			tc.mutate(&params)

			_, err := svc.CreateRequest(context.Background(), params)
			if !errors.Is(err, tc.want) {
				t.Errorf("CreateRequest() error = %v, want %v", err, tc.want)
			}
			if called {
				t.Error("CreateRequest() called repo despite invalid input")
			}
		})
	}
}

func TestCreateRequestEmptyPriorityAllowed(t *testing.T) {
	called := false
	fake := &fakeRepo{createFn: func(context.Context, servicerequest.CreateParams) (servicerequest.ServiceRequest, error) {
		called = true
		return servicerequest.ServiceRequest{}, nil
	}}
	svc := New(fake)

	params := validCreateParams()
	params.Priority = ""

	if _, err := svc.CreateRequest(context.Background(), params); err != nil {
		t.Fatalf("CreateRequest() error = %v", err)
	}
	if !called {
		t.Error("CreateRequest() did not call repo")
	}
}

func TestTransitionRequestAllowed(t *testing.T) {
	tests := []struct {
		from, to servicerequest.Status
	}{
		{servicerequest.StatusPending, servicerequest.StatusAssigned},
		{servicerequest.StatusPending, servicerequest.StatusCancelled},
		{servicerequest.StatusAssigned, servicerequest.StatusInProgress},
		{servicerequest.StatusAssigned, servicerequest.StatusCancelled},
		{servicerequest.StatusInProgress, servicerequest.StatusResolved},
		{servicerequest.StatusInProgress, servicerequest.StatusCancelled},
	}

	for _, tc := range tests {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
			id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
			actorID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
			fake := &fakeRepo{
				getByIDFn: func(_ context.Context, _, _ uuid.UUID) (servicerequest.ServiceRequest, error) {
					return testRequest(tenantID, id, tc.from), nil
				},
				transitionFn: func(_ context.Context, _ servicerequest.RequestEvent) (servicerequest.ServiceRequest, error) {
					return testRequest(tenantID, id, tc.to), nil
				},
			}
			svc := New(fake)

			got, err := svc.TransitionRequest(context.Background(), tenantID, id, actorID, tc.to)
			if err != nil {
				t.Fatalf("TransitionRequest() error = %v", err)
			}
			if got.Status != tc.to {
				t.Errorf("status = %q, want %q", got.Status, tc.to)
			}
			if fake.lastTransition.ToStatus != tc.to {
				t.Errorf("Transition() event to = %q, want %q", fake.lastTransition.ToStatus, tc.to)
			}
			if fake.lastTransition.FromStatus != tc.from {
				t.Errorf("Transition() event from = %q, want %q", fake.lastTransition.FromStatus, tc.from)
			}
			if fake.lastTransition.RequestID != id {
				t.Errorf("Transition() event request id = %v, want %v", fake.lastTransition.RequestID, id)
			}
			if fake.lastTransition.TenantID != tenantID {
				t.Errorf("Transition() event tenant = %v, want %v", fake.lastTransition.TenantID, tenantID)
			}
			if fake.lastTransition.ActorID != actorID {
				t.Errorf("Transition() event actor = %v, want %v", fake.lastTransition.ActorID, actorID)
			}
		})
	}
}

func TestTransitionRequestInvalidRejected(t *testing.T) {
	tests := []struct{ from, to servicerequest.Status }{
		{servicerequest.StatusPending, servicerequest.StatusInProgress},
		{servicerequest.StatusPending, servicerequest.StatusResolved},
		{servicerequest.StatusAssigned, servicerequest.StatusPending},
		{servicerequest.StatusAssigned, servicerequest.StatusResolved},
		{servicerequest.StatusInProgress, servicerequest.StatusPending},
		{servicerequest.StatusInProgress, servicerequest.StatusAssigned},
		{servicerequest.StatusResolved, servicerequest.StatusPending},
		{servicerequest.StatusCancelled, servicerequest.StatusPending},
		{servicerequest.StatusResolved, servicerequest.StatusCancelled},
		{servicerequest.StatusCancelled, servicerequest.StatusResolved},
	}

	for _, tc := range tests {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
			id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
			fake := &fakeRepo{
				getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (servicerequest.ServiceRequest, error) {
					return testRequest(tenantID, id, tc.from), nil
				},
				transitionFn: func(context.Context, servicerequest.RequestEvent) (servicerequest.ServiceRequest, error) {
					return servicerequest.ServiceRequest{}, nil
				},
			}
			svc := New(fake)

			_, err := svc.TransitionRequest(context.Background(), tenantID, id, uuid.MustParse("33333333-3333-3333-3333-333333333333"), tc.to)
			if !errors.Is(err, servicerequest.ErrInvalidTransition) {
				t.Fatalf("TransitionRequest() error = %v, want ErrInvalidTransition", err)
			}
			if len(fake.transitioned) != 0 {
				t.Errorf("Transition() called %d times, want 0", len(fake.transitioned))
			}
		})
	}
}

func TestTransitionRequestNotFoundPropagated(t *testing.T) {
	fake := &fakeRepo{getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (servicerequest.ServiceRequest, error) {
		return servicerequest.ServiceRequest{}, servicerequest.ErrNotFound
	}}
	svc := New(fake)

	_, err := svc.TransitionRequest(context.Background(),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		servicerequest.StatusAssigned)
	if !errors.Is(err, servicerequest.ErrNotFound) {
		t.Errorf("TransitionRequest() error = %v, want ErrNotFound", err)
	}
}

func TestTransitionRequestGetErrorPropagated(t *testing.T) {
	wantErr := errors.New("connection reset")
	fake := &fakeRepo{getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (servicerequest.ServiceRequest, error) {
		return servicerequest.ServiceRequest{}, wantErr
	}}
	svc := New(fake)

	_, err := svc.TransitionRequest(context.Background(),
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		servicerequest.StatusAssigned)
	if !errors.Is(err, wantErr) {
		t.Errorf("TransitionRequest() error = %v, want %v", err, wantErr)
	}
}

func TestTransitionRequestTransitionErrorPropagated(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	wantErr := errors.New("connection reset")
	fake := &fakeRepo{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (servicerequest.ServiceRequest, error) {
			return testRequest(tenantID, id, servicerequest.StatusPending), nil
		},
		transitionFn: func(context.Context, servicerequest.RequestEvent) (servicerequest.ServiceRequest, error) {
			return servicerequest.ServiceRequest{}, wantErr
		},
	}
	svc := New(fake)

	_, err := svc.TransitionRequest(context.Background(), tenantID, id,
		uuid.MustParse("33333333-3333-3333-3333-333333333333"), servicerequest.StatusAssigned)
	if !errors.Is(err, wantErr) {
		t.Errorf("TransitionRequest() error = %v, want %v", err, wantErr)
	}
}

func TestTransitionRequestTenantAndActorPassedToRepository(t *testing.T) {
	tenantID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	actorID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	fake := &fakeRepo{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (servicerequest.ServiceRequest, error) {
			return testRequest(tenantID, id, servicerequest.StatusPending), nil
		},
		transitionFn: func(_ context.Context, _ servicerequest.RequestEvent) (servicerequest.ServiceRequest, error) {
			return testRequest(tenantID, id, servicerequest.StatusAssigned), nil
		},
	}
	svc := New(fake)

	if _, err := svc.TransitionRequest(context.Background(), tenantID, id, actorID, servicerequest.StatusAssigned); err != nil {
		t.Fatalf("TransitionRequest() error = %v", err)
	}
	if fake.lastGetTenantID != tenantID {
		t.Errorf("GetByID() tenant = %v, want %v", fake.lastGetTenantID, tenantID)
	}
	if fake.lastTransition.TenantID != tenantID {
		t.Errorf("Transition() event tenant = %v, want %v", fake.lastTransition.TenantID, tenantID)
	}
	if fake.lastTransition.ActorID != actorID {
		t.Errorf("Transition() event actor = %v, want %v", fake.lastTransition.ActorID, actorID)
	}
}

func TestRequestHistoryForwardsTenantAndRequestID(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	requestID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	want := []servicerequest.RequestEvent{{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333")}}
	fake := &fakeRepo{listEventsFn: func(context.Context, uuid.UUID, uuid.UUID) ([]servicerequest.RequestEvent, error) {
		return want, nil
	}}
	svc := New(fake)

	got, err := svc.RequestHistory(context.Background(), tenantID, requestID)
	if err != nil {
		t.Fatalf("RequestHistory() error = %v", err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("RequestHistory() = %+v, want %+v", got, want)
	}
	if fake.lastListTenantID != tenantID {
		t.Errorf("ListEvents() tenant = %v, want %v", fake.lastListTenantID, tenantID)
	}
	if fake.lastListRequest != requestID {
		t.Errorf("ListEvents() request id = %v, want %v", fake.lastListRequest, requestID)
	}
}

func TestRequestHistoryForwardsEmptySlice(t *testing.T) {
	fake := &fakeRepo{listEventsFn: func(context.Context, uuid.UUID, uuid.UUID) ([]servicerequest.RequestEvent, error) {
		return []servicerequest.RequestEvent{}, nil
	}}
	svc := New(fake)

	got, err := svc.RequestHistory(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("RequestHistory() error = %v", err)
	}
	if got == nil {
		t.Error("RequestHistory() returned nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("RequestHistory() returned %d events, want 0", len(got))
	}
}

func TestRequestHistoryNotFoundPropagated(t *testing.T) {
	fake := &fakeRepo{listEventsFn: func(context.Context, uuid.UUID, uuid.UUID) ([]servicerequest.RequestEvent, error) {
		return nil, servicerequest.ErrNotFound
	}}
	svc := New(fake)

	_, err := svc.RequestHistory(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, servicerequest.ErrNotFound) {
		t.Errorf("RequestHistory() error = %v, want ErrNotFound", err)
	}
}

func TestRequestHistoryRepoErrorPropagated(t *testing.T) {
	wantErr := errors.New("connection reset")
	fake := &fakeRepo{listEventsFn: func(context.Context, uuid.UUID, uuid.UUID) ([]servicerequest.RequestEvent, error) {
		return nil, wantErr
	}}
	svc := New(fake)

	_, err := svc.RequestHistory(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("RequestHistory() error = %v, want %v", err, wantErr)
	}
}

func TestGetRequestForwardsTenantAndID(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	requestID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	want := testRequest(tenantID, requestID, servicerequest.StatusAssigned)
	fake := &fakeRepo{getByIDFn: func(ctx context.Context, tenantID, id uuid.UUID) (servicerequest.ServiceRequest, error) {
		return want, nil
	}}
	svc := New(fake)

	got, err := svc.GetRequest(context.Background(), tenantID, requestID)
	if err != nil {
		t.Fatalf("GetRequest() error = %v", err)
	}
	if got != want {
		t.Errorf("GetRequest() = %+v, want %+v", got, want)
	}
	if fake.lastGetTenantID != tenantID {
		t.Errorf("GetByID() tenant = %v, want %v", fake.lastGetTenantID, tenantID)
	}
}

func TestGetRequestNotFoundPropagated(t *testing.T) {
	fake := &fakeRepo{getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (servicerequest.ServiceRequest, error) {
		return servicerequest.ServiceRequest{}, servicerequest.ErrNotFound
	}}
	svc := New(fake)

	_, err := svc.GetRequest(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, servicerequest.ErrNotFound) {
		t.Errorf("GetRequest() error = %v, want ErrNotFound", err)
	}
}

func TestListRequestsForwardsTenant(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	want := []servicerequest.ServiceRequest{
		testRequest(tenantID, uuid.MustParse("22222222-2222-2222-2222-222222222222"), servicerequest.StatusPending),
		testRequest(tenantID, uuid.MustParse("33333333-3333-3333-3333-333333333333"), servicerequest.StatusResolved),
	}
	fake := &fakeRepo{listByTenantFn: func(_ context.Context, tenantID uuid.UUID) ([]servicerequest.ServiceRequest, error) {
		return want, nil
	}}
	svc := New(fake)

	got, err := svc.ListRequests(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListRequests() error = %v", err)
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ListRequests() = %+v, want %+v", got, want)
	}
	if fake.lastListTenantID != tenantID {
		t.Errorf("ListByTenant() tenant = %v, want %v", fake.lastListTenantID, tenantID)
	}
}

func TestListRequestsForwardsEmptySlice(t *testing.T) {
	fake := &fakeRepo{listByTenantFn: func(context.Context, uuid.UUID) ([]servicerequest.ServiceRequest, error) {
		return []servicerequest.ServiceRequest{}, nil
	}}
	svc := New(fake)

	got, err := svc.ListRequests(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("ListRequests() error = %v", err)
	}
	if got == nil {
		t.Error("ListRequests() returned nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("ListRequests() returned %d requests, want 0", len(got))
	}
}

func TestListRequestsRepoErrorPropagated(t *testing.T) {
	wantErr := errors.New("connection reset")
	fake := &fakeRepo{listByTenantFn: func(context.Context, uuid.UUID) ([]servicerequest.ServiceRequest, error) {
		return nil, wantErr
	}}
	svc := New(fake)

	_, err := svc.ListRequests(context.Background(), uuid.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("ListRequests() error = %v, want %v", err, wantErr)
	}
}
