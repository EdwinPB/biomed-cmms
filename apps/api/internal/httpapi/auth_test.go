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

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

const (
	testSessionCookieName = "session"
	testSessionToken      = "test-session-token"
)

type stubAuthService struct {
	loginFn        func(ctx context.Context, creds auth.Credentials) (auth.Session, error)
	logoutFn       func(ctx context.Context, tokenHash string) error
	authenticateFn func(ctx context.Context, tokenHash string) (auth.Principal, error)
	listUsersFn    func(ctx context.Context, tenantID uuid.UUID, role auth.Role) ([]auth.User, error)
	createUserFn   func(ctx context.Context, params auth.CreateParams, role auth.Role) (auth.User, error)
	updateUserFn   func(ctx context.Context, params auth.UpdateParams, actorUserID uuid.UUID, role auth.Role) (auth.User, error)
}

func (s *stubAuthService) Login(ctx context.Context, creds auth.Credentials) (auth.Session, error) {
	if s.loginFn != nil {
		return s.loginFn(ctx, creds)
	}
	return auth.Session{}, errors.New("stubAuthService: Login not configured")
}

func (s *stubAuthService) Logout(ctx context.Context, tokenHash string) error {
	if s.logoutFn != nil {
		return s.logoutFn(ctx, tokenHash)
	}
	return nil
}

func (s *stubAuthService) Authenticate(ctx context.Context, tokenHash string) (auth.Principal, error) {
	if s.authenticateFn != nil {
		return s.authenticateFn(ctx, tokenHash)
	}
	return testSessionPrincipal(), nil
}

func (s *stubAuthService) ListUsers(ctx context.Context, tenantID uuid.UUID, role auth.Role) ([]auth.User, error) {
	if s.listUsersFn != nil {
		return s.listUsersFn(ctx, tenantID, role)
	}
	return nil, errors.New("stubAuthService: ListUsers not configured")
}

func (s *stubAuthService) CreateUser(ctx context.Context, params auth.CreateParams, role auth.Role) (auth.User, error) {
	if s.createUserFn != nil {
		return s.createUserFn(ctx, params, role)
	}
	return auth.User{}, errors.New("stubAuthService: CreateUser not configured")
}

func (s *stubAuthService) UpdateUser(ctx context.Context, params auth.UpdateParams, actorUserID uuid.UUID, role auth.Role) (auth.User, error) {
	if s.updateUserFn != nil {
		return s.updateUserFn(ctx, params, actorUserID, role)
	}
	return auth.User{}, errors.New("stubAuthService: UpdateUser not configured")
}

// testSessionPrincipal is the identity the stub authenticates by default. It
// mirrors the historical dev identity (testTenantID / testUserID) so business
// tests keep asserting the same tenant/user propagation.
func testSessionPrincipal() auth.Principal {
	return auth.Principal{
		TenantID:   uuid.MustParse(testTenantID),
		UserID:     uuid.MustParse(testUserID),
		Role:       auth.RoleAdmin,
		Email:      "dev@local.test",
		FullName:   "Dev User",
		TenantSlug: "local-dev",
		TenantName: "Local Dev",
	}
}

// testAuthService returns a stub that authenticates the default principal for
// any session token.
func testAuthService() *stubAuthService {
	return &stubAuthService{}
}

// authRejectingService returns a stub whose Authenticate always fails with the
// given error, for session failure-mode tests.
func authRejectingService(err error) *stubAuthService {
	return &stubAuthService{
		authenticateFn: func(context.Context, string) (auth.Principal, error) {
			return auth.Principal{}, err
		},
	}
}

func addSessionCookie(req *http.Request) {
	req.AddCookie(&http.Cookie{Name: testSessionCookieName, Value: testSessionToken})
}

func doAuthRequest(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func testAuthHandler(auth AuthService) http.Handler {
	return NewHandler(&stubTenantService{}, auth, &stubRequestService{}, &stubRFPService{}, &stubEquipmentService{}, &stubHealthChecker{}, testSessionCookieName)
}

func testAuthUser() auth.User {
	return auth.User{
		ID:           uuid.MustParse(testUserID),
		TenantID:     uuid.MustParse(testTenantID),
		Email:        "dev@local.test",
		FullName:     "Dev User",
		Role:         auth.RoleAdmin,
		IsActive:     true,
		PasswordHash: "bcrypt-hash",
	}
}

func testAuthTenant() tenant.Tenant {
	return tenant.Tenant{
		ID:     uuid.MustParse(testTenantID),
		Slug:   "local-dev",
		Name:   "Local Dev",
		Status: tenant.StatusActive,
	}
}

func TestLoginSuccess(t *testing.T) {
	var gotCreds auth.Credentials
	svc := &stubAuthService{loginFn: func(_ context.Context, creds auth.Credentials) (auth.Session, error) {
		gotCreds = creds
		return auth.Session{
			Token:     "raw-session-token",
			ExpiresAt: mustTime("2026-08-13T00:00:00Z"),
			User:      testAuthUser(),
			Tenant:    testAuthTenant(),
		}, nil
	}}

	rec := doAuthRequest(t, testAuthHandler(svc), http.MethodPost, "/api/v1/auth/login",
		`{"tenant_slug":"local-dev","email":"dev@local.test","password":"dev-password"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotCreds.TenantSlug != "local-dev" || gotCreds.Email != "dev@local.test" || gotCreds.Password != "dev-password" {
		t.Errorf("Login() creds = %+v", gotCreds)
	}

	setCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "session=raw-session-token") {
		t.Errorf("Set-Cookie missing token: %q", setCookie)
	}
	for _, attr := range []string{"HttpOnly", "SameSite=None", "Secure", "Path=/"} {
		if !strings.Contains(setCookie, attr) {
			t.Errorf("Set-Cookie missing %s: %q", attr, setCookie)
		}
	}

	var body authResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.User.Role != auth.RoleAdmin || body.User.Email != "dev@local.test" {
		t.Errorf("user = %+v", body.User)
	}
	if body.Tenant.Slug != "local-dev" || body.Tenant.Name != "Local Dev" {
		t.Errorf("tenant = %+v", body.Tenant)
	}
	if strings.Contains(rec.Body.String(), "password") {
		t.Errorf("response leaks password: %q", rec.Body.String())
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	svc := &stubAuthService{loginFn: func(context.Context, auth.Credentials) (auth.Session, error) {
		return auth.Session{}, auth.ErrInvalidCredentials
	}}

	rec := doAuthRequest(t, testAuthHandler(svc), http.MethodPost, "/api/v1/auth/login",
		`{"tenant_slug":"local-dev","email":"dev@local.test","password":"wrong"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(rec.Body.String(), "invalid credentials") {
		t.Errorf("body = %q, want invalid credentials message", rec.Body.String())
	}
}

func TestLoginInactiveUser(t *testing.T) {
	svc := &stubAuthService{loginFn: func(context.Context, auth.Credentials) (auth.Session, error) {
		return auth.Session{}, auth.ErrUserInactive
	}}

	rec := doAuthRequest(t, testAuthHandler(svc), http.MethodPost, "/api/v1/auth/login",
		`{"tenant_slug":"local-dev","email":"dev@local.test","password":"dev-password"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginMissingFields(t *testing.T) {
	svc := &stubAuthService{}

	rec := doAuthRequest(t, testAuthHandler(svc), http.MethodPost, "/api/v1/auth/login",
		`{"tenant_slug":"","email":"dev@local.test","password":"dev-password"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "tenant_slug, email and password are required") {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestLoginMalformedJSON(t *testing.T) {
	svc := &stubAuthService{}

	rec := doAuthRequest(t, testAuthHandler(svc), http.MethodPost, "/api/v1/auth/login", `{"tenant_slug":`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "invalid JSON") {
		t.Errorf("body = %q, want invalid JSON message", rec.Body.String())
	}
}

func TestLoginInternalError(t *testing.T) {
	svc := &stubAuthService{loginFn: func(context.Context, auth.Credentials) (auth.Session, error) {
		return auth.Session{}, errors.New("connection reset")
	}}

	rec := doAuthRequest(t, testAuthHandler(svc), http.MethodPost, "/api/v1/auth/login",
		`{"tenant_slug":"local-dev","email":"dev@local.test","password":"dev-password"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("body leaks internal error: %q", rec.Body.String())
	}
}

func TestLogoutSuccess(t *testing.T) {
	var gotTokenHash string
	svc := &stubAuthService{logoutFn: func(_ context.Context, tokenHash string) error {
		gotTokenHash = tokenHash
		return nil
	}}

	h := testAuthHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	addSessionCookie(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if gotTokenHash != auth.HashToken(testSessionToken) {
		t.Errorf("Logout() token hash = %q, want %q", gotTokenHash, auth.HashToken(testSessionToken))
	}
	setCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "session=") {
		t.Errorf("logout must clear the session cookie: %q", setCookie)
	}
	cookie, err := http.ParseSetCookie(setCookie)
	if err != nil {
		t.Fatalf("parse set-cookie: %v", err)
	}
	if cookie.MaxAge > 0 {
		t.Errorf("cleared cookie Max-Age = %d, want <= 0", cookie.MaxAge)
	}
}

func TestLogoutWithoutSession(t *testing.T) {
	svc := &stubAuthService{}

	rec := doAuthRequest(t, testAuthHandler(svc), http.MethodPost, "/api/v1/auth/logout", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMeSuccess(t *testing.T) {
	h := testAuthHandler(testAuthService())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	addSessionCookie(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body authResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.User.ID != uuid.MustParse(testUserID) {
		t.Errorf("user id = %v", body.User.ID)
	}
	if body.User.Role != auth.RoleAdmin || body.User.Email != "dev@local.test" {
		t.Errorf("user = %+v", body.User)
	}
	if body.Tenant.ID != uuid.MustParse(testTenantID) || body.Tenant.Slug != "local-dev" {
		t.Errorf("tenant = %+v", body.Tenant)
	}
}

func TestMeWithoutSession(t *testing.T) {
	rec := doAuthRequest(t, testAuthHandler(testAuthService()), http.MethodGet, "/api/v1/auth/me", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSessionMissingCookie(t *testing.T) {
	rec := doAuthRequest(t, testAuthHandler(testAuthService()), http.MethodGet, "/api/v1/requests", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(rec.Body.String(), "authentication required") {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestSessionInvalidToken(t *testing.T) {
	h := testAuthHandler(authRejectingService(auth.ErrSessionNotFound))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/requests", nil)
	addSessionCookie(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(rec.Body.String(), "invalid or expired session") {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestSessionExpired(t *testing.T) {
	h := testAuthHandler(authRejectingService(auth.ErrSessionExpired))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/requests", nil)
	addSessionCookie(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSessionInactiveUser(t *testing.T) {
	h := testAuthHandler(authRejectingService(auth.ErrUserInactive))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/requests", nil)
	addSessionCookie(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
