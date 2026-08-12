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

	"github.com/edwinpolo/biomed-cmms/api/internal/rfp"
	rfpservice "github.com/edwinpolo/biomed-cmms/api/internal/rfp/service"
)

type fakeRFPService struct {
	createFn             func(ctx context.Context, params rfp.CreateParams) (rfp.RFP, error)
	transitionFn         func(ctx context.Context, tenantID, id uuid.UUID, to rfp.Status) (rfp.RFP, error)
	getFn                func(ctx context.Context, tenantID, id uuid.UUID) (rfp.RFP, error)
	getBySRFn            func(ctx context.Context, tenantID, serviceRequestID uuid.UUID) (rfp.RFP, error)
	gotCreateTenant      uuid.UUID
	gotCreateUser        uuid.UUID
	gotCreateServiceReq  uuid.UUID
	gotTransitionTenant  uuid.UUID
	gotTransitionID      uuid.UUID
	gotTransitionStatus  rfp.Status
	gotGetTenant         uuid.UUID
	gotGetID             uuid.UUID
	gotGetBySRTenant     uuid.UUID
	gotGetBySRServiceReq uuid.UUID
}

func (f *fakeRFPService) CreateRFP(ctx context.Context, params rfp.CreateParams) (rfp.RFP, error) {
	f.gotCreateTenant = params.TenantID
	f.gotCreateUser = params.CreatedBy
	f.gotCreateServiceReq = params.ServiceRequestID
	if f.createFn != nil {
		return f.createFn(ctx, params)
	}
	return rfp.RFP{}, errors.New("fakeRFPService: Create not configured")
}

func (f *fakeRFPService) TransitionRFP(ctx context.Context, tenantID, id uuid.UUID, to rfp.Status) (rfp.RFP, error) {
	f.gotTransitionTenant = tenantID
	f.gotTransitionID = id
	f.gotTransitionStatus = to
	if f.transitionFn != nil {
		return f.transitionFn(ctx, tenantID, id, to)
	}
	return rfp.RFP{}, errors.New("fakeRFPService: Transition not configured")
}

func (f *fakeRFPService) GetRFP(ctx context.Context, tenantID, id uuid.UUID) (rfp.RFP, error) {
	f.gotGetTenant = tenantID
	f.gotGetID = id
	if f.getFn != nil {
		return f.getFn(ctx, tenantID, id)
	}
	return rfp.RFP{}, errors.New("fakeRFPService: Get not configured")
}

func (f *fakeRFPService) GetRFPByServiceRequest(ctx context.Context, tenantID, serviceRequestID uuid.UUID) (rfp.RFP, error) {
	f.gotGetBySRTenant = tenantID
	f.gotGetBySRServiceReq = serviceRequestID
	if f.getBySRFn != nil {
		return f.getBySRFn(ctx, tenantID, serviceRequestID)
	}
	return rfp.RFP{}, errors.New("fakeRFPService: GetByServiceRequest not configured")
}

type stubRFPService struct{}

func (s *stubRFPService) CreateRFP(context.Context, rfp.CreateParams) (rfp.RFP, error) {
	return rfp.RFP{}, errors.New("stubRFPService: Create not configured")
}

func (s *stubRFPService) TransitionRFP(context.Context, uuid.UUID, uuid.UUID, rfp.Status) (rfp.RFP, error) {
	return rfp.RFP{}, errors.New("stubRFPService: Transition not configured")
}

func (s *stubRFPService) GetRFP(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
	return rfp.RFP{}, errors.New("stubRFPService: Get not configured")
}

func (s *stubRFPService) GetRFPByServiceRequest(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
	return rfp.RFP{}, errors.New("stubRFPService: GetByServiceRequest not configured")
}

func doRFPSvc(t *testing.T, svc RFPService, method, target, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(&stubTenantService{}, &stubRequestService{}, svc, &stubEquipmentService{})
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func createdRFP() rfp.RFP {
	due := mustTime("2026-09-30T12:00:00Z")
	return rfp.RFP{
		ID:               uuid.MustParse("66666666-6666-6666-6666-666666666666"),
		TenantID:         uuid.MustParse(testTenantID),
		ServiceRequestID: uuid.MustParse("77777777-7777-7777-7777-777777777777"),
		Title:            "MRI replacement",
		Description:      "Procure a replacement MRI scanner.",
		Status:           rfp.StatusDraft,
		DueAt:            &due,
		CreatedBy:        uuid.MustParse(testUserID),
		CreatedAt:        mustTime("2026-08-12T20:00:00Z"),
		UpdatedAt:        mustTime("2026-08-12T20:00:00Z"),
	}
}

func TestCreateRFPSuccess(t *testing.T) {
	fake := &fakeRFPService{createFn: func(context.Context, rfp.CreateParams) (rfp.RFP, error) {
		return createdRFP(), nil
	}}

	rec := doRFPSvc(t, fake, http.MethodPost, "/api/v1/rfps",
		`{"service_request_id":"77777777-7777-7777-7777-777777777777","title":"MRI replacement","description":"Procure a replacement MRI scanner.","due_at":"2026-09-30T12:00:00Z"}`,
		identityHeaders()...)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var body rfpResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != uuid.MustParse("66666666-6666-6666-6666-666666666666") {
		t.Errorf("id = %v", body.ID)
	}
	if body.ServiceRequestID != uuid.MustParse("77777777-7777-7777-7777-777777777777") {
		t.Errorf("service_request_id = %v", body.ServiceRequestID)
	}
	if body.Status != rfp.StatusDraft {
		t.Errorf("status = %q, want draft", body.Status)
	}
	if body.DueAt == nil || *body.DueAt != "2026-09-30T12:00:00Z" {
		t.Errorf("due_at = %v, want 2026-09-30T12:00:00Z", body.DueAt)
	}
	if body.CreatedBy != uuid.MustParse(testUserID) {
		t.Errorf("created_by = %v, want %v", body.CreatedBy, testUserID)
	}
	if strings.Contains(rec.Body.String(), "tenant_id") {
		t.Errorf("response leaks tenant_id: %q", rec.Body.String())
	}

	if fake.gotCreateTenant != uuid.MustParse(testTenantID) {
		t.Errorf("CreateRFP() tenant = %v, want %v", fake.gotCreateTenant, testTenantID)
	}
	if fake.gotCreateUser != uuid.MustParse(testUserID) {
		t.Errorf("CreateRFP() created_by = %v, want %v", fake.gotCreateUser, testUserID)
	}
	if fake.gotCreateServiceReq != uuid.MustParse("77777777-7777-7777-7777-777777777777") {
		t.Errorf("CreateRFP() service_request_id = %v", fake.gotCreateServiceReq)
	}
}

func TestCreateRFPWithoutDueAt(t *testing.T) {
	fake := &fakeRFPService{createFn: func(context.Context, rfp.CreateParams) (rfp.RFP, error) {
		return createdRFP(), nil
	}}

	rec := doRFPSvc(t, fake, http.MethodPost, "/api/v1/rfps",
		`{"service_request_id":"77777777-7777-7777-7777-777777777777","title":"MRI replacement","description":"Procure a replacement MRI scanner."}`,
		identityHeaders()...)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestCreateRFPMissingTenant(t *testing.T) {
	fake := &fakeRFPService{}

	rec := doRFPSvc(t, fake, http.MethodPost, "/api/v1/rfps",
		`{"service_request_id":"77777777-7777-7777-7777-777777777777","title":"T","description":"D"}`,
		"X-User-ID", testUserID)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateRFPMissingUser(t *testing.T) {
	fake := &fakeRFPService{}

	rec := doRFPSvc(t, fake, http.MethodPost, "/api/v1/rfps",
		`{"service_request_id":"77777777-7777-7777-7777-777777777777","title":"T","description":"D"}`,
		"X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateRFPInvalidTenant(t *testing.T) {
	fake := &fakeRFPService{}

	rec := doRFPSvc(t, fake, http.MethodPost, "/api/v1/rfps", `{}`,
		"X-Tenant-ID", "not-a-uuid", "X-User-ID", testUserID)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateRFPMalformedJSON(t *testing.T) {
	fake := &fakeRFPService{}

	rec := doRFPSvc(t, fake, http.MethodPost, "/api/v1/rfps", `{"service_request_id":`, identityHeaders()...)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "invalid JSON") {
		t.Errorf("body = %q, want invalid JSON message", rec.Body.String())
	}
}

func TestCreateRFPInvalidServiceRequestUUID(t *testing.T) {
	fake := &fakeRFPService{}

	rec := doRFPSvc(t, fake, http.MethodPost, "/api/v1/rfps",
		`{"service_request_id":"not-a-uuid","title":"T","description":"D"}`, identityHeaders()...)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateRFPInvalidDueAt(t *testing.T) {
	fake := &fakeRFPService{}

	rec := doRFPSvc(t, fake, http.MethodPost, "/api/v1/rfps",
		`{"service_request_id":"77777777-7777-7777-7777-777777777777","title":"T","description":"D","due_at":"not-a-date"}`,
		identityHeaders()...)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateRFPValidationError(t *testing.T) {
	fake := &fakeRFPService{createFn: func(context.Context, rfp.CreateParams) (rfp.RFP, error) {
		return rfp.RFP{}, rfpservice.ErrTitleRequired
	}}

	rec := doRFPSvc(t, fake, http.MethodPost, "/api/v1/rfps", `{}`, identityHeaders()...)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "title is required") {
		t.Errorf("body = %q, want validation message", rec.Body.String())
	}
}

func TestCreateRFPConflict(t *testing.T) {
	fake := &fakeRFPService{createFn: func(context.Context, rfp.CreateParams) (rfp.RFP, error) {
		return rfp.RFP{}, rfp.ErrConflict
	}}

	rec := doRFPSvc(t, fake, http.MethodPost, "/api/v1/rfps",
		`{"service_request_id":"77777777-7777-7777-7777-777777777777","title":"T","description":"D"}`,
		identityHeaders()...)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestCreateRFPInternalError(t *testing.T) {
	fake := &fakeRFPService{createFn: func(context.Context, rfp.CreateParams) (rfp.RFP, error) {
		return rfp.RFP{}, errors.New("connection reset")
	}}

	rec := doRFPSvc(t, fake, http.MethodPost, "/api/v1/rfps",
		`{"service_request_id":"77777777-7777-7777-7777-777777777777","title":"T","description":"D"}`,
		identityHeaders()...)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("body leaks internal error: %q", rec.Body.String())
	}
}

func TestGetRFPSuccess(t *testing.T) {
	fake := &fakeRFPService{getFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
		return createdRFP(), nil
	}}

	rec := doRFPSvc(t, fake, http.MethodGet, "/api/v1/rfps/66666666-6666-6666-6666-666666666666",
		"", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body rfpResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != uuid.MustParse("66666666-6666-6666-6666-666666666666") {
		t.Errorf("id = %v", body.ID)
	}
	if fake.gotGetTenant != uuid.MustParse(testTenantID) {
		t.Errorf("GetRFP() tenant = %v, want %v", fake.gotGetTenant, testTenantID)
	}
	if fake.gotGetID != uuid.MustParse("66666666-6666-6666-6666-666666666666") {
		t.Errorf("GetRFP() id = %v", fake.gotGetID)
	}
}

func TestGetRFPMissingTenant(t *testing.T) {
	fake := &fakeRFPService{}

	rec := doRFPSvc(t, fake, http.MethodGet, "/api/v1/rfps/66666666-6666-6666-6666-666666666666", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetRFPInvalidUUID(t *testing.T) {
	fake := &fakeRFPService{}

	rec := doRFPSvc(t, fake, http.MethodGet, "/api/v1/rfps/not-a-uuid", "", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetRFPNotFound(t *testing.T) {
	fake := &fakeRFPService{getFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
		return rfp.RFP{}, rfp.ErrNotFound
	}}

	rec := doRFPSvc(t, fake, http.MethodGet, "/api/v1/rfps/66666666-6666-6666-6666-666666666666",
		"", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetRFPWrongTenantBehavesAsNotFound(t *testing.T) {
	fake := &fakeRFPService{getFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
		return rfp.RFP{}, rfp.ErrNotFound
	}}

	rec := doRFPSvc(t, fake, http.MethodGet, "/api/v1/rfps/66666666-6666-6666-6666-666666666666",
		"", "X-Tenant-ID", "99999999-9999-9999-9999-999999999999")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetRFPByServiceRequestSuccess(t *testing.T) {
	fake := &fakeRFPService{getBySRFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
		return createdRFP(), nil
	}}

	rec := doRFPSvc(t, fake, http.MethodGet, "/api/v1/service-requests/77777777-7777-7777-7777-777777777777/rfp",
		"", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body rfpResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ServiceRequestID != uuid.MustParse("77777777-7777-7777-7777-777777777777") {
		t.Errorf("service_request_id = %v", body.ServiceRequestID)
	}
	if fake.gotGetBySRTenant != uuid.MustParse(testTenantID) {
		t.Errorf("GetRFPByServiceRequest() tenant = %v, want %v", fake.gotGetBySRTenant, testTenantID)
	}
	if fake.gotGetBySRServiceReq != uuid.MustParse("77777777-7777-7777-7777-777777777777") {
		t.Errorf("GetRFPByServiceRequest() service request id = %v", fake.gotGetBySRServiceReq)
	}
}

func TestGetRFPByServiceRequestNotFound(t *testing.T) {
	fake := &fakeRFPService{getBySRFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
		return rfp.RFP{}, rfp.ErrNotFound
	}}

	rec := doRFPSvc(t, fake, http.MethodGet, "/api/v1/service-requests/77777777-7777-7777-7777-777777777777/rfp",
		"", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetRFPByServiceRequestWrongTenantBehavesAsNotFound(t *testing.T) {
	fake := &fakeRFPService{getBySRFn: func(context.Context, uuid.UUID, uuid.UUID) (rfp.RFP, error) {
		return rfp.RFP{}, rfp.ErrNotFound
	}}

	rec := doRFPSvc(t, fake, http.MethodGet, "/api/v1/service-requests/77777777-7777-7777-7777-777777777777/rfp",
		"", "X-Tenant-ID", "99999999-9999-9999-9999-999999999999")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestTransitionRFPStatusSuccess(t *testing.T) {
	fake := &fakeRFPService{transitionFn: func(context.Context, uuid.UUID, uuid.UUID, rfp.Status) (rfp.RFP, error) {
		created := createdRFP()
		created.Status = rfp.StatusPublished
		return created, nil
	}}

	rec := doRFPSvc(t, fake, http.MethodPatch, "/api/v1/rfps/66666666-6666-6666-6666-666666666666/status",
		`{"status":"published"}`, "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body rfpResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != rfp.StatusPublished {
		t.Errorf("status = %q, want published", body.Status)
	}
	if fake.gotTransitionTenant != uuid.MustParse(testTenantID) {
		t.Errorf("TransitionRFP() tenant = %v, want %v", fake.gotTransitionTenant, testTenantID)
	}
	if fake.gotTransitionID != uuid.MustParse("66666666-6666-6666-6666-666666666666") {
		t.Errorf("TransitionRFP() id = %v", fake.gotTransitionID)
	}
	if fake.gotTransitionStatus != rfp.StatusPublished {
		t.Errorf("TransitionRFP() status = %q, want published", fake.gotTransitionStatus)
	}
}

func TestTransitionRFPStatusInvalidTransition(t *testing.T) {
	fake := &fakeRFPService{transitionFn: func(context.Context, uuid.UUID, uuid.UUID, rfp.Status) (rfp.RFP, error) {
		return rfp.RFP{}, rfp.ErrInvalidTransition
	}}

	rec := doRFPSvc(t, fake, http.MethodPatch, "/api/v1/rfps/66666666-6666-6666-6666-666666666666/status",
		`{"status":"closed"}`, "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if !strings.Contains(rec.Body.String(), "invalid transition") {
		t.Errorf("body = %q, want invalid transition message", rec.Body.String())
	}
}

func TestTransitionRFPStatusPublishPrecondition(t *testing.T) {
	fake := &fakeRFPService{transitionFn: func(context.Context, uuid.UUID, uuid.UUID, rfp.Status) (rfp.RFP, error) {
		return rfp.RFP{}, rfp.ErrPublishDueAtRequired
	}}

	rec := doRFPSvc(t, fake, http.MethodPatch, "/api/v1/rfps/66666666-6666-6666-6666-666666666666/status",
		`{"status":"published"}`, "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestTransitionRFPStatusNotFound(t *testing.T) {
	fake := &fakeRFPService{transitionFn: func(context.Context, uuid.UUID, uuid.UUID, rfp.Status) (rfp.RFP, error) {
		return rfp.RFP{}, rfp.ErrNotFound
	}}

	rec := doRFPSvc(t, fake, http.MethodPatch, "/api/v1/rfps/66666666-6666-6666-6666-666666666666/status",
		`{"status":"published"}`, "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestTransitionRFPStatusInvalidPathID(t *testing.T) {
	fake := &fakeRFPService{}

	rec := doRFPSvc(t, fake, http.MethodPatch, "/api/v1/rfps/not-a-uuid/status",
		`{"status":"published"}`, "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestTransitionRFPStatusMissingTenant(t *testing.T) {
	fake := &fakeRFPService{}

	rec := doRFPSvc(t, fake, http.MethodPatch, "/api/v1/rfps/66666666-6666-6666-6666-666666666666/status",
		`{"status":"published"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestTransitionRFPStatusMalformedBody(t *testing.T) {
	fake := &fakeRFPService{}

	rec := doRFPSvc(t, fake, http.MethodPatch, "/api/v1/rfps/66666666-6666-6666-6666-666666666666/status",
		`{"status":`, "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestTransitionRFPStatusInternalError(t *testing.T) {
	fake := &fakeRFPService{transitionFn: func(context.Context, uuid.UUID, uuid.UUID, rfp.Status) (rfp.RFP, error) {
		return rfp.RFP{}, errors.New("connection reset")
	}}

	rec := doRFPSvc(t, fake, http.MethodPatch, "/api/v1/rfps/66666666-6666-6666-6666-666666666666/status",
		`{"status":"published"}`, "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("body leaks internal error: %q", rec.Body.String())
	}
}
