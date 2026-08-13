package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testAllowedOrigin = "http://localhost:3000"

func TestCORSPreflightSucceedsForAllowedOrigin(t *testing.T) {
	nextCalled := false
	h := CORS(testAllowedOrigin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/requests", nil)
	req.Header.Set("Origin", testAllowedOrigin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if nextCalled {
		t.Error("next handler ran for a preflight request")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != testAllowedOrigin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, testAllowedOrigin)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != corsAllowMethods {
		t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, corsAllowMethods)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != corsAllowHeaders {
		t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, corsAllowHeaders)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != corsAllowCredentials {
		t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, corsAllowCredentials)
	}
}

func TestCORSPreflightFromUnknownOriginGetsNoCORSHeaders(t *testing.T) {
	h := CORS(testAllowedOrigin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler ran for a preflight request")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/requests", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want none", got)
	}
}

func TestCORSPreflightWithoutOrigin(t *testing.T) {
	h := CORS(testAllowedOrigin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler ran for a preflight request")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/requests", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestCORSPreflightRoutedThroughRealHandler(t *testing.T) {
	h := CORS(testAllowedOrigin)(NewHandler(&stubTenantService{}, testAuthService(), &stubRequestService{}, &stubRFPService{}, &stubEquipmentService{}, testSessionCookieName))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/requests", nil)
	req.Header.Set("Origin", testAllowedOrigin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d (405 regression)", rec.Code, http.StatusNoContent)
	}
}

func TestCORSAddsHeadersToActualResponse(t *testing.T) {
	h := CORS(testAllowedOrigin)(NewHandler(&stubTenantService{}, testAuthService(), &stubRequestService{}, &stubRFPService{}, &stubEquipmentService{}, testSessionCookieName))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", testAllowedOrigin)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != testAllowedOrigin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, testAllowedOrigin)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary = %q, want it to include Origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != corsAllowCredentials {
		t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, corsAllowCredentials)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body = %q, want status ok", rec.Body.String())
	}
}

func TestCORSActualResponseFromUnknownOriginGetsNoHeader(t *testing.T) {
	h := CORS(testAllowedOrigin)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want none", got)
	}
}
