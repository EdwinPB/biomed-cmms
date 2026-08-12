package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/equipment"
	"github.com/edwinpolo/biomed-cmms/api/internal/servicerequest"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant/service"
)

type fakeTenantService struct {
	createFn func(ctx context.Context, params tenant.CreateParams) (tenant.Tenant, error)
}

func (f *fakeTenantService) CreateTenant(ctx context.Context, params tenant.CreateParams) (tenant.Tenant, error) {
	if f.createFn != nil {
		return f.createFn(ctx, params)
	}
	return tenant.Tenant{}, errors.New("fakeTenantService: Create not configured")
}

func doRequest(t *testing.T, svc TenantService, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(svc, &stubRequestService{}, &stubRFPService{}, &stubEquipmentService{})
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type stubRequestService struct{}

func (s *stubRequestService) CreateRequest(context.Context, servicerequest.CreateParams) (servicerequest.ServiceRequest, error) {
	return servicerequest.ServiceRequest{}, errors.New("stubRequestService: CreateRequest not configured")
}

func (s *stubRequestService) TransitionRequest(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, servicerequest.Status) (servicerequest.ServiceRequest, error) {
	return servicerequest.ServiceRequest{}, errors.New("stubRequestService: TransitionRequest not configured")
}

func (s *stubRequestService) RequestHistory(context.Context, uuid.UUID, uuid.UUID) ([]servicerequest.RequestEvent, error) {
	return nil, errors.New("stubRequestService: RequestHistory not configured")
}

func (s *stubRequestService) GetRequest(context.Context, uuid.UUID, uuid.UUID) (servicerequest.ServiceRequest, error) {
	return servicerequest.ServiceRequest{}, errors.New("stubRequestService: GetRequest not configured")
}

func (s *stubRequestService) ListRequests(context.Context, uuid.UUID) ([]servicerequest.ServiceRequest, error) {
	return nil, errors.New("stubRequestService: ListRequests not configured")
}

type stubEquipmentService struct{}

func (s *stubEquipmentService) ListEquipment(context.Context, uuid.UUID) ([]equipment.Equipment, error) {
	return nil, errors.New("stubEquipmentService: ListEquipment not configured")
}

func TestCreateTenantSuccess(t *testing.T) {
	created := tenant.Tenant{
		ID:        uuid.MustParse("9c5b7f0e-2b2c-4f1e-8a2f-5e0c1a2b3c4d"),
		Slug:      "acme-health",
		Name:      "Acme Health",
		Status:    tenant.StatusActive,
		CreatedAt: mustTime("2026-08-11T20:00:00Z"),
		UpdatedAt: mustTime("2026-08-11T20:00:00Z"),
	}

	svc := &fakeTenantService{createFn: func(_ context.Context, params tenant.CreateParams) (tenant.Tenant, error) {
		if params.Slug != "acme-health" || params.Name != "Acme Health" {
			t.Errorf("CreateTenant() params = %+v", params)
		}
		return created, nil
	}}

	rec := doRequest(t, svc, http.MethodPost, "/api/v1/tenants", `{"slug":"acme-health","name":"Acme Health"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var body tenantResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != created.ID {
		t.Errorf("id = %s, want %s", body.ID, created.ID)
	}
	if body.Slug != "acme-health" || body.Name != "Acme Health" {
		t.Errorf("body = %+v", body)
	}
	if body.Status != tenant.StatusActive {
		t.Errorf("status = %q, want active", body.Status)
	}
	if body.CreatedAt == "" || body.UpdatedAt == "" {
		t.Error("timestamps missing from response")
	}
}

func TestCreateTenantValidationError(t *testing.T) {
	svc := &fakeTenantService{createFn: func(context.Context, tenant.CreateParams) (tenant.Tenant, error) {
		return tenant.Tenant{}, service.ErrSlugRequired
	}}

	rec := doRequest(t, svc, http.MethodPost, "/api/v1/tenants", `{"slug":"","name":"Acme Health"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "slug is required") {
		t.Errorf("body = %q, want validation message", rec.Body.String())
	}
}

func TestCreateTenantNameRequired(t *testing.T) {
	svc := &fakeTenantService{createFn: func(context.Context, tenant.CreateParams) (tenant.Tenant, error) {
		return tenant.Tenant{}, service.ErrNameRequired
	}}

	rec := doRequest(t, svc, http.MethodPost, "/api/v1/tenants", `{"slug":"acme-health","name":""}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "name is required") {
		t.Errorf("body = %q, want validation message", rec.Body.String())
	}
}

func TestCreateTenantConflict(t *testing.T) {
	svc := &fakeTenantService{createFn: func(context.Context, tenant.CreateParams) (tenant.Tenant, error) {
		return tenant.Tenant{}, tenant.ErrConflict
	}}

	rec := doRequest(t, svc, http.MethodPost, "/api/v1/tenants", `{"slug":"acme-health","name":"Acme Health"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if !strings.Contains(rec.Body.String(), "already exists") {
		t.Errorf("body = %q, want conflict message", rec.Body.String())
	}
}

func TestCreateTenantInternalError(t *testing.T) {
	svc := &fakeTenantService{createFn: func(context.Context, tenant.CreateParams) (tenant.Tenant, error) {
		return tenant.Tenant{}, errors.New("connection reset")
	}}

	rec := doRequest(t, svc, http.MethodPost, "/api/v1/tenants", `{"slug":"acme-health","name":"Acme Health"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("body leaks internal error: %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "internal server error") {
		t.Errorf("body = %q, want generic message", rec.Body.String())
	}
}

func TestCreateTenantMalformedJSON(t *testing.T) {
	svc := &fakeTenantService{}

	rec := doRequest(t, svc, http.MethodPost, "/api/v1/tenants", `{"slug":`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "invalid JSON") {
		t.Errorf("body = %q, want invalid JSON message", rec.Body.String())
	}
}

func TestCreateTenantEmptyBody(t *testing.T) {
	svc := &fakeTenantService{}

	rec := doRequest(t, svc, http.MethodPost, "/api/v1/tenants", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHealth(t *testing.T) {
	svc := &fakeTenantService{}

	rec := doRequest(t, svc, http.MethodGet, "/health", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Errorf("body = %q, want ok status", rec.Body.String())
	}
}

func TestCreateTenantMethodNotAllowed(t *testing.T) {
	svc := &fakeTenantService{}

	rec := doRequest(t, svc, http.MethodGet, "/api/v1/tenants", "")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
