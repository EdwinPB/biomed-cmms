package httpapi

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogRequestsCapturesStatus(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	h := LogRequests(logger, next)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests", nil)
	h.ServeHTTP(rec, req)

	out := buf.String()
	for _, want := range []string{"POST", "/api/v1/requests", "418"} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q missing %q", out, want)
		}
	}
}

func TestLogRequestsDefaultsOKOnBodilessWrite(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	h := LogRequests(logger, next)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(rec, req)

	if !strings.Contains(buf.String(), "200") {
		t.Errorf("log %q missing status 200", buf.String())
	}
}

func TestLogRequestsOmitsBodiesAndQuery(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("secret-response-body"))
	})

	h := LogRequests(logger, next)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login?debug=1", nil)
	h.ServeHTTP(rec, req)

	out := buf.String()
	if strings.Contains(out, "secret-response-body") || strings.Contains(out, "debug=1") {
		t.Errorf("log leaks body or query: %q", out)
	}
	if !strings.Contains(out, "/api/v1/auth/login") {
		t.Errorf("log %q missing path", out)
	}
}
