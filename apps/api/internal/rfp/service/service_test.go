package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
	"github.com/edwinpolo/biomed-cmms/api/internal/rfp"
)

type fakeRepo struct {
	createFn              func(ctx context.Context, params rfp.CreateParams) (rfp.RFP, error)
	getByIDFn             func(ctx context.Context, tenantID, id uuid.UUID) (rfp.RFP, error)
	transitionFn          func(ctx context.Context, tenantID, id uuid.UUID, from, to rfp.Status) (rfp.RFP, error)
	getByServiceRequestFn func(ctx context.Context, tenantID, serviceRequestID uuid.UUID) (rfp.RFP, error)

	lastGetTenantID      uuid.UUID
	lastTransitionTenant uuid.UUID
	lastTransitionID     uuid.UUID
	lastTransitionFrom   rfp.Status
	lastTransitionTo     rfp.Status
	transitions          int
	getByServiceRequest  int
	creates              int
}

func (f *fakeRepo) Create(ctx context.Context, params rfp.CreateParams) (rfp.RFP, error) {
	f.creates++
	if f.createFn != nil {
		return f.createFn(ctx, params)
	}
	return rfp.RFP{}, errors.New("fakeRepo: Create not configured")
}

func (f *fakeRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (rfp.RFP, error) {
	f.lastGetTenantID = tenantID
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, tenantID, id)
	}
	return rfp.RFP{}, errors.New("fakeRepo: GetByID not configured")
}

func (f *fakeRepo) GetByServiceRequest(ctx context.Context, tenantID, serviceRequestID uuid.UUID) (rfp.RFP, error) {
	f.getByServiceRequest++
	if f.getByServiceRequestFn != nil {
		return f.getByServiceRequestFn(ctx, tenantID, serviceRequestID)
	}
	return rfp.RFP{}, errors.New("fakeRepo: GetByServiceRequest not configured")
}

func (f *fakeRepo) ListByTenant(context.Context, uuid.UUID) ([]rfp.RFP, error) {
	return nil, errors.New("fakeRepo: ListByTenant not configured")
}

func (f *fakeRepo) Transition(ctx context.Context, tenantID, id uuid.UUID, from, to rfp.Status) (rfp.RFP, error) {
	f.lastTransitionTenant = tenantID
	f.lastTransitionID = id
	f.lastTransitionFrom = from
	f.lastTransitionTo = to
	f.transitions++
	if f.transitionFn != nil {
		return f.transitionFn(ctx, tenantID, id, from, to)
	}
	return rfp.RFP{}, errors.New("fakeRepo: Transition not configured")
}

func validCreateParams() rfp.CreateParams {
	return rfp.CreateParams{
		TenantID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ServiceRequestID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		Title:            "MRI replacement",
		Description:      "Procure a replacement MRI scanner.",
		CreatedBy:        uuid.MustParse("44444444-4444-4444-4444-444444444444"),
	}
}

func futureRFP(tenantID, id uuid.UUID, status rfp.Status) rfp.RFP {
	due := time.Now().Add(24 * time.Hour)
	return rfp.RFP{
		ID:               id,
		TenantID:         tenantID,
		ServiceRequestID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		Title:            "MRI replacement",
		Description:      "Procure a replacement MRI scanner.",
		Status:           status,
		DueAt:            &due,
		CreatedBy:        uuid.MustParse("44444444-4444-4444-4444-444444444444"),
	}
}

func TestCreateRFPSuccess(t *testing.T) {
	var gotParams rfp.CreateParams
	created := futureRFP(uuid.MustParse("11111111-1111-1111-1111-111111111111"), uuid.MustParse("55555555-5555-5555-5555-555555555555"), rfp.StatusDraft)
	fake := &fakeRepo{createFn: func(_ context.Context, params rfp.CreateParams) (rfp.RFP, error) {
		gotParams = params
		return created, nil
	}}
	svc := New(fake)

	params := validCreateParams()
	got, err := svc.CreateRFP(context.Background(), params, auth.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateRFP() error = %v", err)
	}
	if got != created {
		t.Errorf("CreateRFP() = %+v, want %+v", got, created)
	}
	if gotParams != params {
		t.Errorf("CreateRFP() repo params = %+v, want %+v", gotParams, params)
	}
}

func TestCreateRFPValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*rfp.CreateParams)
		want   error
	}{
		{"missing tenant", func(p *rfp.CreateParams) { p.TenantID = uuid.Nil }, ErrTenantRequired},
		{"missing service request", func(p *rfp.CreateParams) { p.ServiceRequestID = uuid.Nil }, ErrServiceRequestRequired},
		{"missing created_by", func(p *rfp.CreateParams) { p.CreatedBy = uuid.Nil }, ErrCreatedByRequired},
		{"empty title", func(p *rfp.CreateParams) { p.Title = "" }, ErrTitleRequired},
		{"empty description", func(p *rfp.CreateParams) { p.Description = "" }, ErrDescriptionRequired},
		{"invalid status", func(p *rfp.CreateParams) { p.Status = "pending" }, ErrInvalidStatus},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			fake := &fakeRepo{createFn: func(context.Context, rfp.CreateParams) (rfp.RFP, error) {
				called = true
				return rfp.RFP{}, nil
			}}
			svc := New(fake)

			params := validCreateParams()
			tc.mutate(&params)

			_, err := svc.CreateRFP(context.Background(), params, auth.RoleAdmin)
			if !errors.Is(err, tc.want) {
				t.Errorf("CreateRFP() error = %v, want %v", err, tc.want)
			}
			if called {
				t.Error("CreateRFP() called repo despite invalid input")
			}
		})
	}
}

func TestCreateRFPEmptyStatusAllowed(t *testing.T) {
	called := false
	fake := &fakeRepo{createFn: func(context.Context, rfp.CreateParams) (rfp.RFP, error) {
		called = true
		return rfp.RFP{}, nil
	}}
	svc := New(fake)

	params := validCreateParams()
	params.Status = ""

	if _, err := svc.CreateRFP(context.Background(), params, auth.RoleAdmin); err != nil {
		t.Fatalf("CreateRFP() error = %v", err)
	}
	if !called {
		t.Error("CreateRFP() did not call repo")
	}
}

func TestTransitionRFPAllowed(t *testing.T) {
	tests := []struct{ from, to rfp.Status }{
		{rfp.StatusDraft, rfp.StatusPublished},
		{rfp.StatusDraft, rfp.StatusCancelled},
		{rfp.StatusPublished, rfp.StatusClosed},
		{rfp.StatusPublished, rfp.StatusCancelled},
	}

	for _, tc := range tests {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
			id := uuid.MustParse("55555555-5555-5555-5555-555555555555")
			fake := &fakeRepo{
				getByIDFn: func(_ context.Context, _, _ uuid.UUID) (rfp.RFP, error) {
					return futureRFP(tenantID, id, tc.from), nil
				},
				transitionFn: func(_ context.Context, _, _ uuid.UUID, _, to rfp.Status) (rfp.RFP, error) {
					updated := futureRFP(tenantID, id, tc.from)
					updated.Status = to
					return updated, nil
				},
			}
			svc := New(fake)

			got, err := svc.TransitionRFP(context.Background(), tenantID, id, auth.RoleAdmin, tc.to)
			if err != nil {
				t.Fatalf("TransitionRFP() error = %v", err)
			}
			if got.Status != tc.to {
				t.Errorf("status = %q, want %q", got.Status, tc.to)
			}
			if fake.lastTransitionFrom != tc.from {
				t.Errorf("Transition() from = %q, want %q", fake.lastTransitionFrom, tc.from)
			}
			if fake.lastTransitionTo != tc.to {
				t.Errorf("Transition() to = %q, want %q", fake.lastTransitionTo, tc.to)
			}
		})
	}
}

func TestTransitionRFPInvalidRejected(t *testing.T) {
	tests := []struct{ from, to rfp.Status }{
		{rfp.StatusDraft, rfp.StatusClosed},
		{rfp.StatusPublished, rfp.StatusDraft},
		{rfp.StatusClosed, rfp.StatusDraft},
		{rfp.StatusCancelled, rfp.StatusPublished},
	}

	for _, tc := range tests {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
			id := uuid.MustParse("55555555-5555-5555-5555-555555555555")
			fake := &fakeRepo{
				getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
					return futureRFP(tenantID, id, tc.from), nil
				},
				transitionFn: func(context.Context, uuid.UUID, uuid.UUID, rfp.Status, rfp.Status) (rfp.RFP, error) {
					return rfp.RFP{}, nil
				},
			}
			svc := New(fake)

			_, err := svc.TransitionRFP(context.Background(), tenantID, id, auth.RoleAdmin, tc.to)
			if !errors.Is(err, rfp.ErrInvalidTransition) {
				t.Fatalf("TransitionRFP() error = %v, want ErrInvalidTransition", err)
			}
			if fake.transitions != 0 {
				t.Errorf("Transition() called %d times, want 0", fake.transitions)
			}
		})
	}
}

func TestTransitionRFPPublishPreconditions(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*rfp.RFP)
		want error
	}{
		{"missing due_at", func(r *rfp.RFP) { r.DueAt = nil }, rfp.ErrPublishDueAtRequired},
		{"past due_at", func(r *rfp.RFP) { past := time.Now().Add(-24 * time.Hour); r.DueAt = &past }, rfp.ErrPublishDueAtInPast},
		{"empty title", func(r *rfp.RFP) { r.Title = "" }, rfp.ErrPublishTitleRequired},
		{"empty description", func(r *rfp.RFP) { r.Description = "" }, rfp.ErrPublishDescriptionRequired},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
			id := uuid.MustParse("55555555-5555-5555-5555-555555555555")
			fake := &fakeRepo{
				getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
					r := futureRFP(tenantID, id, rfp.StatusDraft)
					tc.mut(&r)
					return r, nil
				},
				transitionFn: func(context.Context, uuid.UUID, uuid.UUID, rfp.Status, rfp.Status) (rfp.RFP, error) {
					return rfp.RFP{}, nil
				},
			}
			svc := New(fake)

			_, err := svc.TransitionRFP(context.Background(), tenantID, id, auth.RoleAdmin, rfp.StatusPublished)
			if !errors.Is(err, tc.want) {
				t.Errorf("TransitionRFP() error = %v, want %v", err, tc.want)
			}
			if fake.transitions != 0 {
				t.Errorf("Transition() called %d times, want 0", fake.transitions)
			}
		})
	}
}

func TestTransitionRFPValidPublishSucceeds(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	id := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	fake := &fakeRepo{
		getByIDFn: func(_ context.Context, _, _ uuid.UUID) (rfp.RFP, error) {
			return futureRFP(tenantID, id, rfp.StatusDraft), nil
		},
		transitionFn: func(_ context.Context, _, _ uuid.UUID, _, to rfp.Status) (rfp.RFP, error) {
			updated := futureRFP(tenantID, id, rfp.StatusPublished)
			updated.Status = to
			return updated, nil
		},
	}
	svc := New(fake)

	got, err := svc.TransitionRFP(context.Background(), tenantID, id, auth.RoleAdmin, rfp.StatusPublished)
	if err != nil {
		t.Fatalf("TransitionRFP() error = %v", err)
	}
	if got.Status != rfp.StatusPublished {
		t.Errorf("status = %q, want published", got.Status)
	}
	if fake.transitions != 1 {
		t.Errorf("Transition() called %d times, want 1", fake.transitions)
	}
	if fake.lastTransitionTo != rfp.StatusPublished {
		t.Errorf("Transition() to = %q, want published", fake.lastTransitionTo)
	}
}

func TestTransitionRFPTenantIDPassed(t *testing.T) {
	tenantID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	id := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	fake := &fakeRepo{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
			return futureRFP(tenantID, id, rfp.StatusDraft), nil
		},
		transitionFn: func(_ context.Context, _, _ uuid.UUID, _, to rfp.Status) (rfp.RFP, error) {
			updated := futureRFP(tenantID, id, rfp.StatusDraft)
			updated.Status = to
			return updated, nil
		},
	}
	svc := New(fake)

	if _, err := svc.TransitionRFP(context.Background(), tenantID, id, auth.RoleAdmin, rfp.StatusPublished); err != nil {
		t.Fatalf("TransitionRFP() error = %v", err)
	}
	if fake.lastGetTenantID != tenantID {
		t.Errorf("GetByID() tenant = %v, want %v", fake.lastGetTenantID, tenantID)
	}
	if fake.lastTransitionTenant != tenantID {
		t.Errorf("Transition() tenant = %v, want %v", fake.lastTransitionTenant, tenantID)
	}
	if fake.lastTransitionID != id {
		t.Errorf("Transition() id = %v, want %v", fake.lastTransitionID, id)
	}
}

func TestTransitionRFPNotFoundPropagated(t *testing.T) {
	fake := &fakeRepo{getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
		return rfp.RFP{}, rfp.ErrNotFound
	}}
	svc := New(fake)

	_, err := svc.TransitionRFP(context.Background(), uuid.New(), uuid.New(), auth.RoleAdmin, rfp.StatusPublished)
	if !errors.Is(err, rfp.ErrNotFound) {
		t.Errorf("TransitionRFP() error = %v, want ErrNotFound", err)
	}
}

func TestTransitionRFPGetErrorPropagated(t *testing.T) {
	wantErr := errors.New("connection reset")
	fake := &fakeRepo{getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
		return rfp.RFP{}, wantErr
	}}
	svc := New(fake)

	_, err := svc.TransitionRFP(context.Background(), uuid.New(), uuid.New(), auth.RoleAdmin, rfp.StatusPublished)
	if !errors.Is(err, wantErr) {
		t.Errorf("TransitionRFP() error = %v, want %v", err, wantErr)
	}
}

func TestTransitionRFPTransitionErrorPropagated(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	id := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	wantErr := errors.New("connection reset")
	fake := &fakeRepo{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
			return futureRFP(tenantID, id, rfp.StatusDraft), nil
		},
		transitionFn: func(context.Context, uuid.UUID, uuid.UUID, rfp.Status, rfp.Status) (rfp.RFP, error) {
			return rfp.RFP{}, wantErr
		},
	}
	svc := New(fake)

	_, err := svc.TransitionRFP(context.Background(), tenantID, id, auth.RoleAdmin, rfp.StatusPublished)
	if !errors.Is(err, wantErr) {
		t.Errorf("TransitionRFP() error = %v, want %v", err, wantErr)
	}
}

func TestRFPRequesterForbiddenAllMethods(t *testing.T) {
	tests := []struct {
		name string
		call func(t *testing.T, svc *Service, fake *fakeRepo)
	}{
		{
			name: "CreateRFP",
			call: func(t *testing.T, svc *Service, fake *fakeRepo) {
				_, err := svc.CreateRFP(context.Background(), validCreateParams(), auth.RoleRequester)
				if !errors.Is(err, rfp.ErrForbidden) {
					t.Fatalf("CreateRFP() error = %v, want ErrForbidden", err)
				}
				if fake.creates != 0 {
					t.Errorf("Create() called %d times, want 0", fake.creates)
				}
			},
		},
		{
			name: "TransitionRFP",
			call: func(t *testing.T, svc *Service, fake *fakeRepo) {
				_, err := svc.TransitionRFP(context.Background(), uuid.New(), uuid.New(), auth.RoleRequester, rfp.StatusPublished)
				if !errors.Is(err, rfp.ErrForbidden) {
					t.Fatalf("TransitionRFP() error = %v, want ErrForbidden", err)
				}
				if fake.transitions != 0 {
					t.Errorf("Transition() called %d times, want 0", fake.transitions)
				}
				if fake.lastGetTenantID != uuid.Nil {
					t.Errorf("GetByID() called with tenant %v, want no call", fake.lastGetTenantID)
				}
			},
		},
		{
			name: "GetRFP",
			call: func(t *testing.T, svc *Service, fake *fakeRepo) {
				_, err := svc.GetRFP(context.Background(), uuid.New(), uuid.New(), auth.RoleRequester)
				if !errors.Is(err, rfp.ErrForbidden) {
					t.Fatalf("GetRFP() error = %v, want ErrForbidden", err)
				}
				if fake.lastGetTenantID != uuid.Nil {
					t.Errorf("GetByID() called with tenant %v, want no call", fake.lastGetTenantID)
				}
			},
		},
		{
			name: "GetRFPByServiceRequest",
			call: func(t *testing.T, svc *Service, fake *fakeRepo) {
				_, err := svc.GetRFPByServiceRequest(context.Background(), uuid.New(), uuid.New(), auth.RoleRequester)
				if !errors.Is(err, rfp.ErrForbidden) {
					t.Fatalf("GetRFPByServiceRequest() error = %v, want ErrForbidden", err)
				}
				if fake.getByServiceRequest != 0 {
					t.Errorf("GetByServiceRequest() called %d times, want 0", fake.getByServiceRequest)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeRepo{}
			tc.call(t, New(fake), fake)
		})
	}
}

func TestRFPAdminAndBiomedicAllowed(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleAdmin, auth.RoleBiomedic} {
		t.Run(string(role), func(t *testing.T) {
			fake := &fakeRepo{
				createFn: func(context.Context, rfp.CreateParams) (rfp.RFP, error) {
					return validRFP(), nil
				},
				getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
					return futureRFP(uuid.New(), uuid.New(), rfp.StatusDraft), nil
				},
				getByServiceRequestFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
					return validRFP(), nil
				},
				transitionFn: func(context.Context, uuid.UUID, uuid.UUID, rfp.Status, rfp.Status) (rfp.RFP, error) {
					return futureRFP(uuid.New(), uuid.New(), rfp.StatusPublished), nil
				},
			}
			svc := New(fake)

			if _, err := svc.CreateRFP(context.Background(), validCreateParams(), role); err != nil {
				t.Errorf("CreateRFP() error = %v, want nil", err)
			}
			if _, err := svc.GetRFP(context.Background(), uuid.New(), uuid.New(), role); err != nil {
				t.Errorf("GetRFP() error = %v, want nil", err)
			}
			if _, err := svc.GetRFPByServiceRequest(context.Background(), uuid.New(), uuid.New(), role); err != nil {
				t.Errorf("GetRFPByServiceRequest() error = %v, want nil", err)
			}
		})
	}
}

func validRFP() rfp.RFP {
	return futureRFP(uuid.New(), uuid.New(), rfp.StatusDraft)
}
