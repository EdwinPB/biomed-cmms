package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubHealthChecker struct {
	pingErr error
}

func (s *stubHealthChecker) Ping(_ context.Context) error {
	return s.pingErr
}

func doHealthRequest(t *testing.T, hc HealthChecker) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(&stubTenantService{}, testAuthService(), &stubRequestService{}, &stubRFPService{}, &stubEquipmentService{}, hc, testSessionCookieName)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthOK(t *testing.T) {
	rec := doHealthRequest(t, &stubHealthChecker{})

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`body["status"] = %q, want "ok"`, body["status"])
	}
}

func TestHealthDBUnavailable(t *testing.T) {
	rec := doHealthRequest(t, &stubHealthChecker{pingErr: errors.New("connection refused")})

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /health status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "database unavailable" {
		t.Errorf(`body["error"] = %q, want "database unavailable"`, body["error"])
	}
}
