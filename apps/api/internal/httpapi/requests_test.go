package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/servicerequest"
	srservice "github.com/edwinpolo/biomed-cmms/api/internal/servicerequest/service"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
)

type fakeRequestService struct {
	createFn                                       func(ctx context.Context, params servicerequest.CreateParams) (servicerequest.ServiceRequest, error)
	transitionFn                                   func(ctx context.Context, tenantID, id, actorID uuid.UUID, role auth.Role, to servicerequest.Status) (servicerequest.ServiceRequest, error)
	requestHistoryFn                               func(ctx context.Context, tenantID, requestID, userID uuid.UUID, role auth.Role) ([]servicerequest.RequestEvent, error)
	getRequestFn                                   func(ctx context.Context, tenantID, id, userID uuid.UUID, role auth.Role) (servicerequest.ServiceRequest, error)
	listRequestsFn                                 func(ctx context.Context, tenantID, userID uuid.UUID, role auth.Role) ([]servicerequest.ServiceRequest, error)
	gotCreateTenant, gotCreateUser                 uuid.UUID
	gotTransitionTenant, gotTransitionID, gotActor uuid.UUID
	gotTransitionRole                              auth.Role
	gotHistoryTenant, gotHistoryRequest            uuid.UUID
	gotHistoryUser                                 uuid.UUID
	gotHistoryRole                                 auth.Role
	gotGetTenant, gotGetID                         uuid.UUID
	gotGetUser                                     uuid.UUID
	gotGetRole                                     auth.Role
	gotListTenant, gotListUser                     uuid.UUID
	gotListRole                                    auth.Role
}

func (f *fakeRequestService) CreateRequest(ctx context.Context, params servicerequest.CreateParams) (servicerequest.ServiceRequest, error) {
	f.gotCreateTenant = params.TenantID
	f.gotCreateUser = params.CreatedBy
	if f.createFn != nil {
		return f.createFn(ctx, params)
	}
	return servicerequest.ServiceRequest{}, errors.New("fakeRequestService: Create not configured")
}

func (f *fakeRequestService) TransitionRequest(ctx context.Context, tenantID, id, actorID uuid.UUID, role auth.Role, to servicerequest.Status) (servicerequest.ServiceRequest, error) {
	f.gotTransitionTenant = tenantID
	f.gotTransitionID = id
	f.gotActor = actorID
	f.gotTransitionRole = role
	if f.transitionFn != nil {
		return f.transitionFn(ctx, tenantID, id, actorID, role, to)
	}
	return servicerequest.ServiceRequest{}, errors.New("fakeRequestService: Transition not configured")
}

func (f *fakeRequestService) RequestHistory(ctx context.Context, tenantID, requestID, userID uuid.UUID, role auth.Role) ([]servicerequest.RequestEvent, error) {
	f.gotHistoryTenant = tenantID
	f.gotHistoryRequest = requestID
	f.gotHistoryUser = userID
	f.gotHistoryRole = role
	if f.requestHistoryFn != nil {
		return f.requestHistoryFn(ctx, tenantID, requestID, userID, role)
	}
	return nil, errors.New("fakeRequestService: RequestHistory not configured")
}

func (f *fakeRequestService) GetRequest(ctx context.Context, tenantID, id, userID uuid.UUID, role auth.Role) (servicerequest.ServiceRequest, error) {
	f.gotGetTenant = tenantID
	f.gotGetID = id
	f.gotGetUser = userID
	f.gotGetRole = role
	if f.getRequestFn != nil {
		return f.getRequestFn(ctx, tenantID, id, userID, role)
	}
	return servicerequest.ServiceRequest{}, errors.New("fakeRequestService: GetRequest not configured")
}

func (f *fakeRequestService) ListRequests(ctx context.Context, tenantID, userID uuid.UUID, role auth.Role) ([]servicerequest.ServiceRequest, error) {
	f.gotListTenant = tenantID
	f.gotListUser = userID
	f.gotListRole = role
	if f.listRequestsFn != nil {
		return f.listRequestsFn(ctx, tenantID, userID, role)
	}
	return nil, errors.New("fakeRequestService: ListRequests not configured")
}

const (
	testTenantID = "11111111-1111-1111-1111-111111111111"
	testUserID   = "22222222-2222-2222-2222-222222222222"
)

func doRequestSvc(t *testing.T, svc ServiceRequestService, method, target, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(&stubTenantService{}, testAuthService(), svc, &stubRFPService{}, &stubEquipmentService{}, &stubHealthChecker{}, testSessionCookieName)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	addSessionCookie(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func doRequestSvcNoSession(t *testing.T, svc ServiceRequestService, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(&stubTenantService{}, testAuthService(), svc, &stubRFPService{}, &stubEquipmentService{}, &stubHealthChecker{}, testSessionCookieName)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func doRequestSvcWithAuth(t *testing.T, svc ServiceRequestService, method, target, body string, auth *stubAuthService) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(&stubTenantService{}, auth, svc, &stubRFPService{}, &stubEquipmentService{}, &stubHealthChecker{}, testSessionCookieName)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	addSessionCookie(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type stubTenantService struct{}

func (s *stubTenantService) CreateTenant(context.Context, tenant.CreateParams) (tenant.Tenant, error) {
	return tenant.Tenant{}, errors.New("stubTenantService: not configured")
}

// identityHeaders is retained for call-site compatibility; request identity now
// comes from the session cookie attached by the request helpers.
func identityHeaders() []string {
	return nil
}

func createdRequest() servicerequest.ServiceRequest {
	assignee := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	return servicerequest.ServiceRequest{
		ID:          uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		TenantID:    uuid.MustParse(testTenantID),
		EquipmentID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		Title:       "Pump not running",
		Description: "Infusion pump reports error on startup.",
		Priority:    servicerequest.PriorityHigh,
		Status:      servicerequest.StatusPending,
		CreatedBy:   uuid.MustParse(testUserID),
		AssignedTo:  &assignee,
		CreatedAt:   mustTime("2026-08-11T20:00:00Z"),
		UpdatedAt:   mustTime("2026-08-11T20:00:00Z"),
	}
}

func TestCreateRequestSuccess(t *testing.T) {
	fake := &fakeRequestService{createFn: func(context.Context, servicerequest.CreateParams) (servicerequest.ServiceRequest, error) {
		return createdRequest(), nil
	}}

	rec := doRequestSvc(t, fake, http.MethodPost, "/api/v1/requests",
		`{"equipment_id":"55555555-5555-5555-5555-555555555555","title":"Pump not running","description":"Infusion pump reports error on startup.","priority":"high"}`,
		identityHeaders()...)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var body requestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != uuid.MustParse("44444444-4444-4444-4444-444444444444") {
		t.Errorf("id = %v", body.ID)
	}
	if body.Priority != servicerequest.PriorityHigh || body.Status != servicerequest.StatusPending {
		t.Errorf("priority/status = %q/%q", body.Priority, body.Status)
	}

	if fake.gotCreateTenant != uuid.MustParse(testTenantID) {
		t.Errorf("CreateRequest() tenant = %v, want %v", fake.gotCreateTenant, testTenantID)
	}
	if fake.gotCreateUser != uuid.MustParse(testUserID) {
		t.Errorf("CreateRequest() created_by = %v, want %v", fake.gotCreateUser, testUserID)
	}
}

func TestCreateRequestWithoutSession(t *testing.T) {
	fake := &fakeRequestService{createFn: func(context.Context, servicerequest.CreateParams) (servicerequest.ServiceRequest, error) {
		return createdRequest(), nil
	}}

	rec := doRequestSvcNoSession(t, fake, http.MethodPost, "/api/v1/requests",
		`{"equipment_id":"55555555-5555-5555-5555-555555555555","title":"T","description":"D"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateRequestInvalidSession(t *testing.T) {
	fake := &fakeRequestService{createFn: func(context.Context, servicerequest.CreateParams) (servicerequest.ServiceRequest, error) {
		return createdRequest(), nil
	}}

	rec := doRequestSvcWithAuth(t, fake, http.MethodPost, "/api/v1/requests",
		`{"equipment_id":"55555555-5555-5555-5555-555555555555","title":"T","description":"D"}`,
		authRejectingService(auth.ErrSessionNotFound))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateRequestExpiredSession(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvcWithAuth(t, fake, http.MethodPost, "/api/v1/requests", `{}`,
		authRejectingService(auth.ErrSessionExpired))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateRequestInactiveUserSession(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvcWithAuth(t, fake, http.MethodPost, "/api/v1/requests", `{}`,
		authRejectingService(auth.ErrUserInactive))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateRequestMalformedJSON(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvc(t, fake, http.MethodPost, "/api/v1/requests", `{"equipment_id":`, identityHeaders()...)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "invalid JSON") {
		t.Errorf("body = %q, want invalid JSON message", rec.Body.String())
	}
}

func TestCreateRequestInvalidEquipmentUUID(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvc(t, fake, http.MethodPost, "/api/v1/requests",
		`{"equipment_id":"not-a-uuid","title":"T","description":"D"}`, identityHeaders()...)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateRequestValidationError(t *testing.T) {
	fake := &fakeRequestService{createFn: func(context.Context, servicerequest.CreateParams) (servicerequest.ServiceRequest, error) {
		return servicerequest.ServiceRequest{}, srservice.ErrTitleRequired
	}}

	rec := doRequestSvc(t, fake, http.MethodPost, "/api/v1/requests", `{}`, identityHeaders()...)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "title is required") {
		t.Errorf("body = %q, want validation message", rec.Body.String())
	}
}

func TestCreateRequestInternalError(t *testing.T) {
	fake := &fakeRequestService{createFn: func(context.Context, servicerequest.CreateParams) (servicerequest.ServiceRequest, error) {
		return servicerequest.ServiceRequest{}, errors.New("connection reset")
	}}

	rec := doRequestSvc(t, fake, http.MethodPost, "/api/v1/requests",
		`{"equipment_id":"55555555-5555-5555-5555-555555555555","title":"T","description":"D"}`, identityHeaders()...)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("body leaks internal error: %q", rec.Body.String())
	}
}

func TestTransitionStatusSuccess(t *testing.T) {
	fake := &fakeRequestService{transitionFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role, servicerequest.Status) (servicerequest.ServiceRequest, error) {
		sr := createdRequest()
		sr.Status = servicerequest.StatusAssigned
		return sr, nil
	}}

	rec := doRequestSvc(t, fake, http.MethodPatch, "/api/v1/requests/44444444-4444-4444-4444-444444444444/status",
		`{"status":"assigned"}`, identityHeaders()...)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body requestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != servicerequest.StatusAssigned {
		t.Errorf("status = %q, want assigned", body.Status)
	}
	if fake.gotTransitionTenant != uuid.MustParse(testTenantID) {
		t.Errorf("TransitionRequest() tenant = %v, want %v", fake.gotTransitionTenant, testTenantID)
	}
	if fake.gotTransitionID != uuid.MustParse("44444444-4444-4444-4444-444444444444") {
		t.Errorf("TransitionRequest() id = %v", fake.gotTransitionID)
	}
	if fake.gotActor != uuid.MustParse(testUserID) {
		t.Errorf("TransitionRequest() actor = %v, want %v", fake.gotActor, testUserID)
	}
	if fake.gotTransitionRole != auth.RoleAdmin {
		t.Errorf("TransitionRequest() role = %q, want admin", fake.gotTransitionRole)
	}
}

func TestTransitionStatusRequesterForbidden(t *testing.T) {
	fake := &fakeRequestService{transitionFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role, servicerequest.Status) (servicerequest.ServiceRequest, error) {
		return servicerequest.ServiceRequest{}, servicerequest.ErrForbidden
	}}

	rec := doRequestSvcWithAuth(t, fake, http.MethodPatch, "/api/v1/requests/44444444-4444-4444-4444-444444444444/status",
		`{"status":"assigned"}`, authWithRole(auth.RoleRequester))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if got := rec.Body.String(); got != `{"error":"forbidden"}`+"\n" {
		t.Errorf("body = %q, want %q", got, `{"error":"forbidden"}`+"\n")
	}
	if fake.gotTransitionRole != auth.RoleRequester {
		t.Errorf("TransitionRequest() role = %q, want requester", fake.gotTransitionRole)
	}
}

func TestTransitionStatusBiomedicAllowed(t *testing.T) {
	fake := &fakeRequestService{transitionFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role, servicerequest.Status) (servicerequest.ServiceRequest, error) {
		sr := createdRequest()
		sr.Status = servicerequest.StatusAssigned
		return sr, nil
	}}

	rec := doRequestSvcWithAuth(t, fake, http.MethodPatch, "/api/v1/requests/44444444-4444-4444-4444-444444444444/status",
		`{"status":"assigned"}`, authWithRole(auth.RoleBiomedic))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if fake.gotTransitionRole != auth.RoleBiomedic {
		t.Errorf("TransitionRequest() role = %q, want biomedic", fake.gotTransitionRole)
	}
}

func TestTransitionStatusWithoutSession(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvcNoSession(t, fake, http.MethodPatch, "/api/v1/requests/44444444-4444-4444-4444-444444444444/status",
		`{"status":"assigned"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestTransitionStatusInvalidSession(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvcWithAuth(t, fake, http.MethodPatch, "/api/v1/requests/44444444-4444-4444-4444-444444444444/status",
		`{"status":"assigned"}`, authRejectingService(auth.ErrSessionNotFound))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestTransitionStatusInvalidTransition(t *testing.T) {
	fake := &fakeRequestService{transitionFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role, servicerequest.Status) (servicerequest.ServiceRequest, error) {
		return servicerequest.ServiceRequest{}, servicerequest.ErrInvalidTransition
	}}

	rec := doRequestSvc(t, fake, http.MethodPatch, "/api/v1/requests/44444444-4444-4444-4444-444444444444/status",
		`{"status":"resolved"}`, identityHeaders()...)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if !strings.Contains(rec.Body.String(), "invalid transition") {
		t.Errorf("body = %q, want invalid transition message", rec.Body.String())
	}
}

func TestTransitionStatusNotFound(t *testing.T) {
	fake := &fakeRequestService{transitionFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role, servicerequest.Status) (servicerequest.ServiceRequest, error) {
		return servicerequest.ServiceRequest{}, servicerequest.ErrNotFound
	}}

	rec := doRequestSvc(t, fake, http.MethodPatch, "/api/v1/requests/44444444-4444-4444-4444-444444444444/status",
		`{"status":"assigned"}`, identityHeaders()...)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestTransitionStatusInvalidPathID(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvc(t, fake, http.MethodPatch, "/api/v1/requests/not-a-uuid/status",
		`{"status":"assigned"}`, identityHeaders()...)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestTransitionStatusInactiveUserSession(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvcWithAuth(t, fake, http.MethodPatch, "/api/v1/requests/44444444-4444-4444-4444-444444444444/status",
		`{"status":"assigned"}`, authRejectingService(auth.ErrUserInactive))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestTransitionStatusExpiredSession(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvcWithAuth(t, fake, http.MethodPatch, "/api/v1/requests/44444444-4444-4444-4444-444444444444/status",
		`{"status":"assigned"}`, authRejectingService(auth.ErrSessionExpired))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestTransitionStatusMalformedBody(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvc(t, fake, http.MethodPatch, "/api/v1/requests/44444444-4444-4444-4444-444444444444/status",
		`{"status":`, identityHeaders()...)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestTransitionStatusInternalError(t *testing.T) {
	fake := &fakeRequestService{transitionFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role, servicerequest.Status) (servicerequest.ServiceRequest, error) {
		return servicerequest.ServiceRequest{}, errors.New("connection reset")
	}}

	rec := doRequestSvc(t, fake, http.MethodPatch, "/api/v1/requests/44444444-4444-4444-4444-444444444444/status",
		`{"status":"assigned"}`, identityHeaders()...)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("body leaks internal error: %q", rec.Body.String())
	}
}

func testEvents() []servicerequest.RequestEvent {
	return []servicerequest.RequestEvent{
		{
			ID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			TenantID:   uuid.MustParse(testTenantID),
			RequestID:  uuid.MustParse("44444444-4444-4444-4444-444444444444"),
			ActorID:    uuid.MustParse(testUserID),
			FromStatus: servicerequest.StatusPending,
			ToStatus:   servicerequest.StatusAssigned,
			CreatedAt:  mustTime("2026-08-12T10:00:00Z"),
		},
		{
			ID:         uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			TenantID:   uuid.MustParse(testTenantID),
			RequestID:  uuid.MustParse("44444444-4444-4444-4444-444444444444"),
			ActorID:    uuid.MustParse("33333333-3333-3333-3333-333333333333"),
			FromStatus: servicerequest.StatusAssigned,
			ToStatus:   servicerequest.StatusInProgress,
			CreatedAt:  mustTime("2026-08-12T11:00:00Z"),
		},
	}
}

func TestRequestHistorySuccess(t *testing.T) {
	fake := &fakeRequestService{requestHistoryFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role) ([]servicerequest.RequestEvent, error) {
		return testEvents(), nil
	}}

	rec := doRequestSvc(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444/history",
		"", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body requestHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(body.Events))
	}
	first := body.Events[0]
	if first.ID != uuid.MustParse("11111111-1111-1111-1111-111111111111") {
		t.Errorf("first event id = %v", first.ID)
	}
	if first.ActorID != uuid.MustParse(testUserID) {
		t.Errorf("first event actor = %v, want %v", first.ActorID, testUserID)
	}
	if first.FromStatus != servicerequest.StatusPending || first.ToStatus != servicerequest.StatusAssigned {
		t.Errorf("first event = %s->%s, want pending->assigned", first.FromStatus, first.ToStatus)
	}
	if first.CreatedAt != "2026-08-12T10:00:00Z" {
		t.Errorf("first event created_at = %q", first.CreatedAt)
	}
	if strings.Contains(rec.Body.String(), "tenant_id") {
		t.Errorf("response leaks tenant_id: %q", rec.Body.String())
	}
	if fake.gotHistoryTenant != uuid.MustParse(testTenantID) {
		t.Errorf("RequestHistory() tenant = %v, want %v", fake.gotHistoryTenant, testTenantID)
	}
	if fake.gotHistoryRequest != uuid.MustParse("44444444-4444-4444-4444-444444444444") {
		t.Errorf("RequestHistory() request id = %v", fake.gotHistoryRequest)
	}
}

func TestRequestHistoryEmpty(t *testing.T) {
	fake := &fakeRequestService{requestHistoryFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role) ([]servicerequest.RequestEvent, error) {
		return []servicerequest.RequestEvent{}, nil
	}}

	rec := doRequestSvc(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444/history",
		"", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body requestHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Events == nil {
		t.Error("events = nil, want non-nil empty slice")
	}
	if len(body.Events) != 0 {
		t.Errorf("events = %d, want 0", len(body.Events))
	}
}

func TestRequestHistoryWithoutSession(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvcNoSession(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444/history", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequestHistoryInvalidSession(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvcWithAuth(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444/history",
		"", authRejectingService(auth.ErrSessionNotFound))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequestHistoryInvalidUUID(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvc(t, fake, http.MethodGet, "/api/v1/requests/not-a-uuid/history",
		"", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRequestHistoryNotFound(t *testing.T) {
	fake := &fakeRequestService{requestHistoryFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role) ([]servicerequest.RequestEvent, error) {
		return nil, servicerequest.ErrNotFound
	}}

	rec := doRequestSvc(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444/history",
		"", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRequestHistoryInternalError(t *testing.T) {
	fake := &fakeRequestService{requestHistoryFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role) ([]servicerequest.RequestEvent, error) {
		return nil, errors.New("connection reset")
	}}

	rec := doRequestSvc(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444/history",
		"", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("body leaks internal error: %q", rec.Body.String())
	}
}

func testServiceRequest() servicerequest.ServiceRequest {
	return servicerequest.ServiceRequest{
		ID:          uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		TenantID:    uuid.MustParse(testTenantID),
		EquipmentID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		Title:       "MRI not starting",
		Description: "Machine will not power on.",
		Priority:    servicerequest.PriorityHigh,
		Status:      servicerequest.StatusPending,
		CreatedBy:   uuid.MustParse(testUserID),
		CreatedAt:   mustTime("2026-08-12T10:00:00Z"),
		UpdatedAt:   mustTime("2026-08-12T10:00:00Z"),
	}
}

func TestGetRequestSuccess(t *testing.T) {
	fake := &fakeRequestService{getRequestFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role) (servicerequest.ServiceRequest, error) {
		return testServiceRequest(), nil
	}}

	rec := doRequestSvc(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444",
		"", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body requestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != uuid.MustParse("44444444-4444-4444-4444-444444444444") {
		t.Errorf("id = %v", body.ID)
	}
	if body.Title != "MRI not starting" {
		t.Errorf("title = %q", body.Title)
	}
	if body.Status != servicerequest.StatusPending {
		t.Errorf("status = %q, want pending", body.Status)
	}
	if fake.gotGetTenant != uuid.MustParse(testTenantID) {
		t.Errorf("GetRequest() tenant = %v, want %v", fake.gotGetTenant, testTenantID)
	}
	if fake.gotGetID != uuid.MustParse("44444444-4444-4444-4444-444444444444") {
		t.Errorf("GetRequest() id = %v", fake.gotGetID)
	}
}

func TestGetRequestWithoutSession(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvcNoSession(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetRequestInvalidUUID(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvc(t, fake, http.MethodGet, "/api/v1/requests/not-a-uuid", "", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetRequestNotFound(t *testing.T) {
	fake := &fakeRequestService{getRequestFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role) (servicerequest.ServiceRequest, error) {
		return servicerequest.ServiceRequest{}, servicerequest.ErrNotFound
	}}

	rec := doRequestSvc(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444",
		"", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetRequestWrongTenantBehavesAsNotFound(t *testing.T) {
	fake := &fakeRequestService{getRequestFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role) (servicerequest.ServiceRequest, error) {
		return servicerequest.ServiceRequest{}, servicerequest.ErrNotFound
	}}

	rec := doRequestSvc(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444",
		"", "X-Tenant-ID", "99999999-9999-9999-9999-999999999999")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetRequestInternalError(t *testing.T) {
	fake := &fakeRequestService{getRequestFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role) (servicerequest.ServiceRequest, error) {
		return servicerequest.ServiceRequest{}, errors.New("connection reset")
	}}

	rec := doRequestSvc(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444",
		"", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("body leaks internal error: %q", rec.Body.String())
	}
}

func TestListRequestsSuccess(t *testing.T) {
	second := testServiceRequest()
	second.ID = uuid.MustParse("77777777-7777-7777-7777-777777777777")
	second.Title = "Infusion pump alarm"
	second.Status = servicerequest.StatusResolved
	fake := &fakeRequestService{listRequestsFn: func(context.Context, uuid.UUID, uuid.UUID, auth.Role) ([]servicerequest.ServiceRequest, error) {
		return []servicerequest.ServiceRequest{testServiceRequest(), second}, nil
	}}

	rec := doRequestSvc(t, fake, http.MethodGet, "/api/v1/requests", "", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body requestListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(body.Requests))
	}
	if body.Requests[0].Title != "MRI not starting" || body.Requests[1].Title != "Infusion pump alarm" {
		t.Errorf("requests order/titles wrong: %+v", body.Requests)
	}
	if fake.gotListTenant != uuid.MustParse(testTenantID) {
		t.Errorf("ListRequests() tenant = %v, want %v", fake.gotListTenant, testTenantID)
	}
}

func TestListRequestsEmpty(t *testing.T) {
	fake := &fakeRequestService{listRequestsFn: func(context.Context, uuid.UUID, uuid.UUID, auth.Role) ([]servicerequest.ServiceRequest, error) {
		return []servicerequest.ServiceRequest{}, nil
	}}

	rec := doRequestSvc(t, fake, http.MethodGet, "/api/v1/requests", "", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body requestListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Requests == nil {
		t.Error("requests = nil, want non-nil empty slice")
	}
	if len(body.Requests) != 0 {
		t.Errorf("requests = %d, want 0", len(body.Requests))
	}
}

func TestListRequestsWithoutSession(t *testing.T) {
	fake := &fakeRequestService{}

	rec := doRequestSvcNoSession(t, fake, http.MethodGet, "/api/v1/requests", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListRequestsInternalError(t *testing.T) {
	fake := &fakeRequestService{listRequestsFn: func(context.Context, uuid.UUID, uuid.UUID, auth.Role) ([]servicerequest.ServiceRequest, error) {
		return nil, errors.New("connection reset")
	}}

	rec := doRequestSvc(t, fake, http.MethodGet, "/api/v1/requests", "", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("body leaks internal error: %q", rec.Body.String())
	}
}

// authWithRole returns a stub authenticating the default principal with the
// given role, for read-authorization propagation tests.
func authWithRole(role auth.Role) *stubAuthService {
	return &stubAuthService{authenticateFn: func(context.Context, string) (auth.Principal, error) {
		p := testSessionPrincipal()
		p.Role = role
		return p, nil
	}}
}

func TestListRequestsRequesterScopesToUser(t *testing.T) {
	fake := &fakeRequestService{listRequestsFn: func(context.Context, uuid.UUID, uuid.UUID, auth.Role) ([]servicerequest.ServiceRequest, error) {
		return []servicerequest.ServiceRequest{}, nil
	}}

	rec := doRequestSvcWithAuth(t, fake, http.MethodGet, "/api/v1/requests", "",
		authWithRole(auth.RoleRequester))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if fake.gotListRole != auth.RoleRequester {
		t.Errorf("ListRequests() role = %q, want requester", fake.gotListRole)
	}
	if fake.gotListUser != uuid.MustParse(testUserID) {
		t.Errorf("ListRequests() user = %v, want %v", fake.gotListUser, testUserID)
	}
}

func TestListRequestsAdminUnfiltered(t *testing.T) {
	fake := &fakeRequestService{listRequestsFn: func(context.Context, uuid.UUID, uuid.UUID, auth.Role) ([]servicerequest.ServiceRequest, error) {
		return []servicerequest.ServiceRequest{}, nil
	}}

	rec := doRequestSvcWithAuth(t, fake, http.MethodGet, "/api/v1/requests", "",
		authWithRole(auth.RoleAdmin))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if fake.gotListRole != auth.RoleAdmin {
		t.Errorf("ListRequests() role = %q, want admin", fake.gotListRole)
	}
}

func TestListRequestsBiomedicUnfiltered(t *testing.T) {
	fake := &fakeRequestService{listRequestsFn: func(context.Context, uuid.UUID, uuid.UUID, auth.Role) ([]servicerequest.ServiceRequest, error) {
		return []servicerequest.ServiceRequest{}, nil
	}}

	rec := doRequestSvcWithAuth(t, fake, http.MethodGet, "/api/v1/requests", "",
		authWithRole(auth.RoleBiomedic))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if fake.gotListRole != auth.RoleBiomedic {
		t.Errorf("ListRequests() role = %q, want biomedic", fake.gotListRole)
	}
}

func TestGetRequestRequesterScopesToUser(t *testing.T) {
	fake := &fakeRequestService{getRequestFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role) (servicerequest.ServiceRequest, error) {
		return testServiceRequest(), nil
	}}

	rec := doRequestSvcWithAuth(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444", "",
		authWithRole(auth.RoleRequester))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if fake.gotGetRole != auth.RoleRequester {
		t.Errorf("GetRequest() role = %q, want requester", fake.gotGetRole)
	}
	if fake.gotGetUser != uuid.MustParse(testUserID) {
		t.Errorf("GetRequest() user = %v, want %v", fake.gotGetUser, testUserID)
	}
}

func TestGetRequestAdminUnfiltered(t *testing.T) {
	fake := &fakeRequestService{getRequestFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role) (servicerequest.ServiceRequest, error) {
		return testServiceRequest(), nil
	}}

	rec := doRequestSvcWithAuth(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444", "",
		authWithRole(auth.RoleAdmin))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if fake.gotGetRole != auth.RoleAdmin {
		t.Errorf("GetRequest() role = %q, want admin", fake.gotGetRole)
	}
}

func TestRequestHistoryRequesterScopesToUser(t *testing.T) {
	fake := &fakeRequestService{requestHistoryFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role) ([]servicerequest.RequestEvent, error) {
		return []servicerequest.RequestEvent{}, nil
	}}

	rec := doRequestSvcWithAuth(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444/history", "",
		authWithRole(auth.RoleRequester))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if fake.gotHistoryRole != auth.RoleRequester {
		t.Errorf("RequestHistory() role = %q, want requester", fake.gotHistoryRole)
	}
	if fake.gotHistoryUser != uuid.MustParse(testUserID) {
		t.Errorf("RequestHistory() user = %v, want %v", fake.gotHistoryUser, testUserID)
	}
}

func TestRequestHistoryAdminUnfiltered(t *testing.T) {
	fake := &fakeRequestService{requestHistoryFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, auth.Role) ([]servicerequest.RequestEvent, error) {
		return []servicerequest.RequestEvent{}, nil
	}}

	rec := doRequestSvcWithAuth(t, fake, http.MethodGet, "/api/v1/requests/44444444-4444-4444-4444-444444444444/history", "",
		authWithRole(auth.RoleAdmin))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if fake.gotHistoryRole != auth.RoleAdmin {
		t.Errorf("RequestHistory() role = %q, want admin", fake.gotHistoryRole)
	}
}
