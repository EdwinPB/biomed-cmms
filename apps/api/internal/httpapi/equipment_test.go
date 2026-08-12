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

	"github.com/edwinpolo/biomed-cmms/api/internal/equipment"
)

type fakeEquipmentService struct {
	listFn      func(ctx context.Context, tenantID uuid.UUID) ([]equipment.Equipment, error)
	gotTenantID uuid.UUID
}

func (f *fakeEquipmentService) ListEquipment(ctx context.Context, tenantID uuid.UUID) ([]equipment.Equipment, error) {
	f.gotTenantID = tenantID
	if f.listFn != nil {
		return f.listFn(ctx, tenantID)
	}
	return nil, errors.New("fakeEquipmentService: ListEquipment not configured")
}

func doEquipmentSvc(t *testing.T, svc EquipmentService, target string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(&stubTenantService{}, &stubRequestService{}, &stubRFPService{}, svc)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func testEquipment() []equipment.Equipment {
	return []equipment.Equipment{
		{
			ID:           uuid.MustParse("55555555-5555-5555-5555-555555555555"),
			TenantID:     uuid.MustParse(testTenantID),
			AssetTag:     "DEV-001",
			Name:         "Infusion Pump",
			SerialNumber: "SN-001",
			Location:     "ICU",
			Status:       equipment.StatusOperational,
			CreatedAt:    mustTime("2026-08-12T10:00:00Z"),
			UpdatedAt:    mustTime("2026-08-12T10:00:00Z"),
		},
		{
			ID:           uuid.MustParse("66666666-6666-6666-6666-666666666666"),
			TenantID:     uuid.MustParse(testTenantID),
			AssetTag:     "DEV-002",
			Name:         "MRI Scanner",
			SerialNumber: "SN-002",
			Location:     "Imaging",
			Status:       equipment.StatusMaintenance,
			CreatedAt:    mustTime("2026-08-12T11:00:00Z"),
			UpdatedAt:    mustTime("2026-08-12T11:00:00Z"),
		},
	}
}

func TestListEquipmentSuccess(t *testing.T) {
	fake := &fakeEquipmentService{listFn: func(context.Context, uuid.UUID) ([]equipment.Equipment, error) {
		return testEquipment(), nil
	}}

	rec := doEquipmentSvc(t, fake, "/api/v1/equipment", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body equipmentListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Equipment) != 2 {
		t.Fatalf("equipment = %d, want 2", len(body.Equipment))
	}
	first := body.Equipment[0]
	if first.ID != uuid.MustParse("55555555-5555-5555-5555-555555555555") {
		t.Errorf("first id = %v", first.ID)
	}
	if first.Name != "Infusion Pump" || first.AssetTag != "DEV-001" {
		t.Errorf("first item = %s / %s", first.Name, first.AssetTag)
	}
	if first.CreatedAt != "2026-08-12T10:00:00Z" {
		t.Errorf("first created_at = %q", first.CreatedAt)
	}
	if strings.Contains(rec.Body.String(), "tenant_id") {
		t.Errorf("response leaks tenant_id: %q", rec.Body.String())
	}
	if fake.gotTenantID != uuid.MustParse(testTenantID) {
		t.Errorf("ListEquipment() tenant = %v, want %v", fake.gotTenantID, testTenantID)
	}
}

func TestListEquipmentEmpty(t *testing.T) {
	fake := &fakeEquipmentService{listFn: func(context.Context, uuid.UUID) ([]equipment.Equipment, error) {
		return []equipment.Equipment{}, nil
	}}

	rec := doEquipmentSvc(t, fake, "/api/v1/equipment", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body equipmentListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Equipment == nil {
		t.Error("equipment = nil, want non-nil empty slice")
	}
	if len(body.Equipment) != 0 {
		t.Errorf("equipment = %d, want 0", len(body.Equipment))
	}
}

func TestListEquipmentMissingTenant(t *testing.T) {
	fake := &fakeEquipmentService{}

	rec := doEquipmentSvc(t, fake, "/api/v1/equipment")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListEquipmentInvalidTenant(t *testing.T) {
	fake := &fakeEquipmentService{}

	rec := doEquipmentSvc(t, fake, "/api/v1/equipment", "X-Tenant-ID", "not-a-uuid")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListEquipmentInternalError(t *testing.T) {
	fake := &fakeEquipmentService{listFn: func(context.Context, uuid.UUID) ([]equipment.Equipment, error) {
		return nil, errors.New("connection reset")
	}}

	rec := doEquipmentSvc(t, fake, "/api/v1/equipment", "X-Tenant-ID", testTenantID)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("body leaks internal error: %q", rec.Body.String())
	}
}
