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
)

func doUsersRequest(t *testing.T, auth AuthService, target string, withSession bool) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(&stubTenantService{}, auth, &stubRequestService{}, &stubRFPService{}, &stubEquipmentService{}, &stubHealthChecker{}, testSessionCookieName)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if withSession {
		addSessionCookie(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func doUsersCreateRequest(t *testing.T, auth AuthService, body string, withSession bool) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(&stubTenantService{}, auth, &stubRequestService{}, &stubRFPService{}, &stubEquipmentService{}, &stubHealthChecker{}, testSessionCookieName)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	if withSession {
		addSessionCookie(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func doUsersUpdateRequest(t *testing.T, auth AuthService, id, body string, withSession bool) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(&stubTenantService{}, auth, &stubRequestService{}, &stubRFPService{}, &stubEquipmentService{}, &stubHealthChecker{}, testSessionCookieName)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+id, strings.NewReader(body))
	if withSession {
		addSessionCookie(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func testUsers() []auth.User {
	return []auth.User{
		{
			ID:        uuid.MustParse("77777777-7777-7777-7777-777777777777"),
			TenantID:  uuid.MustParse(testTenantID),
			Email:     "dev@local.test",
			FullName:  "Dev User",
			Role:      auth.RoleAdmin,
			IsActive:  true,
			CreatedAt: mustTime("2026-08-12T10:00:00Z"),
		},
		{
			ID:        uuid.MustParse("88888888-8888-8888-8888-888888888888"),
			TenantID:  uuid.MustParse(testTenantID),
			Email:     "requester@local.test",
			FullName:  "Requester User",
			Role:      auth.RoleRequester,
			IsActive:  true,
			CreatedAt: mustTime("2026-08-12T11:00:00Z"),
		},
	}
}

func TestListUsersAdminSuccess(t *testing.T) {
	var gotTenant uuid.UUID
	var gotRole auth.Role
	authSvc := &stubAuthService{listUsersFn: func(_ context.Context, tenantID uuid.UUID, role auth.Role) ([]auth.User, error) {
		gotTenant = tenantID
		gotRole = role
		return testUsers(), nil
	}}

	rec := doUsersRequest(t, authSvc, "/api/v1/users", true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body userListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Users) != 2 {
		t.Fatalf("users = %d, want 2", len(body.Users))
	}
	first := body.Users[0]
	if first.ID != uuid.MustParse("77777777-7777-7777-7777-777777777777") {
		t.Errorf("first id = %v", first.ID)
	}
	if first.Email != "dev@local.test" || first.FullName != "Dev User" || first.Role != auth.RoleAdmin || !first.IsActive {
		t.Errorf("first user = %+v", first)
	}
	if first.CreatedAt != "2026-08-12T10:00:00Z" {
		t.Errorf("first created_at = %q", first.CreatedAt)
	}
	if gotTenant != uuid.MustParse(testTenantID) {
		t.Errorf("ListUsers() tenant = %v, want %v", gotTenant, testTenantID)
	}
	if gotRole != auth.RoleAdmin {
		t.Errorf("ListUsers() role = %q, want admin", gotRole)
	}
}

func TestListUsersResponseDoesNotExposePasswordHash(t *testing.T) {
	users := testUsers()
	users[0].PasswordHash = "$2a$10$AAZLkEqOuWupLfw.D5sFyuhAmW/CXDmJ5SehlpyNfaEZh1Xeoe3iO"
	authSvc := &stubAuthService{listUsersFn: func(context.Context, uuid.UUID, auth.Role) ([]auth.User, error) {
		return users, nil
	}}

	rec := doUsersRequest(t, authSvc, "/api/v1/users", true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if strings.Contains(body, "password_hash") {
		t.Errorf("response exposes password_hash: %q", body)
	}
	if strings.Contains(body, "$2a$10$") {
		t.Errorf("response exposes bcrypt hash: %q", body)
	}
	if strings.Contains(body, "token") || strings.Contains(body, "session") {
		t.Errorf("response exposes session/token data: %q", body)
	}
}

func TestListUsersBiomedicForbidden(t *testing.T) {
	authSvc := &stubAuthService{listUsersFn: func(context.Context, uuid.UUID, auth.Role) ([]auth.User, error) {
		return nil, auth.ErrForbidden
	}}

	rec := doUsersRequest(t, authSvc, "/api/v1/users", true)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if got := rec.Body.String(); got != `{"error":"forbidden"}`+"\n" {
		t.Errorf("body = %q, want %q", got, `{"error":"forbidden"}`+"\n")
	}
}

func TestListUsersWithoutSession(t *testing.T) {
	authSvc := &stubAuthService{}

	rec := doUsersRequest(t, authSvc, "/api/v1/users", false)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListUsersInternalError(t *testing.T) {
	authSvc := &stubAuthService{listUsersFn: func(context.Context, uuid.UUID, auth.Role) ([]auth.User, error) {
		return nil, errors.New("connection reset")
	}}

	rec := doUsersRequest(t, authSvc, "/api/v1/users", true)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("body leaks internal error: %q", rec.Body.String())
	}
}

func TestCreateUserAdminSuccess(t *testing.T) {
	var gotParams auth.CreateParams
	var gotRole auth.Role
	authSvc := &stubAuthService{createUserFn: func(_ context.Context, params auth.CreateParams, role auth.Role) (auth.User, error) {
		gotParams = params
		gotRole = role
		return auth.User{
			ID:        uuid.MustParse("99999999-9999-9999-9999-999999999999"),
			TenantID:  params.TenantID,
			Email:     params.Email,
			FullName:  params.FullName,
			Role:      params.Role,
			IsActive:  true,
			CreatedAt: mustTime("2026-08-12T12:00:00Z"),
		}, nil
	}}

	rec := doUsersCreateRequest(t, authSvc, `{"email":"new@local.test","full_name":"New User","role":"requester","password":"secret-pass"}`, true)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var body userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != uuid.MustParse("99999999-9999-9999-9999-999999999999") {
		t.Errorf("id = %v", body.ID)
	}
	if body.Email != "new@local.test" || body.FullName != "New User" || body.Role != auth.RoleRequester || !body.IsActive {
		t.Errorf("body = %+v", body)
	}
	if gotParams.TenantID != uuid.MustParse(testTenantID) {
		t.Errorf("CreateUser() tenant = %v, want session tenant %v", gotParams.TenantID, testTenantID)
	}
	if gotRole != auth.RoleAdmin {
		t.Errorf("CreateUser() role = %q, want admin", gotRole)
	}
	if gotParams.PasswordHash != "secret-pass" {
		t.Errorf("CreateUser() password forwarded to service = %q, want plaintext for hashing", gotParams.PasswordHash)
	}
}

func TestCreateUserIgnoresBodyTenantID(t *testing.T) {
	var gotParams auth.CreateParams
	authSvc := &stubAuthService{createUserFn: func(_ context.Context, params auth.CreateParams, role auth.Role) (auth.User, error) {
		gotParams = params
		return auth.User{TenantID: params.TenantID, Email: params.Email, Role: params.Role, IsActive: true}, nil
	}}

	rec := doUsersCreateRequest(t, authSvc, `{"tenant_id":"12345678-1234-1234-1234-123456789012","email":"new@local.test","full_name":"New User","role":"requester","password":"secret-pass"}`, true)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if gotParams.TenantID != uuid.MustParse(testTenantID) {
		t.Errorf("CreateUser() tenant = %v, want session tenant %v (body tenant_id must be ignored)", gotParams.TenantID, testTenantID)
	}
}

func TestCreateUserResponseNoSecrets(t *testing.T) {
	authSvc := &stubAuthService{createUserFn: func(_ context.Context, params auth.CreateParams, role auth.Role) (auth.User, error) {
		return auth.User{
			ID:           uuid.New(),
			TenantID:     params.TenantID,
			Email:        params.Email,
			Role:         params.Role,
			PasswordHash: "$2a$10$AAZLkEqOuWupLfw.D5sFyuhAmW/CXDmJ5SehlpyNfaEZh1Xeoe3iO",
			IsActive:     true,
		}, nil
	}}

	rec := doUsersCreateRequest(t, authSvc, `{"email":"new@local.test","full_name":"New User","role":"requester","password":"secret-pass"}`, true)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	body := rec.Body.String()
	for _, leak := range []string{"password_hash", "$2a$10$", "secret-pass", "session", "token"} {
		if strings.Contains(body, leak) {
			t.Errorf("response leaks %q: %q", leak, body)
		}
	}
}

func TestCreateUserNonAdminForbidden(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleBiomedic, auth.RoleRequester} {
		t.Run(string(role), func(t *testing.T) {
			authSvc := &stubAuthService{createUserFn: func(context.Context, auth.CreateParams, auth.Role) (auth.User, error) {
				return auth.User{}, auth.ErrForbidden
			}}

			rec := doUsersCreateRequest(t, authSvc, `{"email":"new@local.test","full_name":"New User","role":"requester","password":"secret-pass"}`, true)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
			}
			if got := rec.Body.String(); got != `{"error":"forbidden"}`+"\n" {
				t.Errorf("body = %q, want %q", got, `{"error":"forbidden"}`+"\n")
			}
		})
	}
}

func TestCreateUserWithoutSession(t *testing.T) {
	authSvc := &stubAuthService{}

	rec := doUsersCreateRequest(t, authSvc, `{"email":"new@local.test","full_name":"New User","role":"requester","password":"secret-pass"}`, false)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateUserInvalidBody(t *testing.T) {
	authSvc := &stubAuthService{}

	rec := doUsersCreateRequest(t, authSvc, `{not json`, true)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestCreateUserValidationError(t *testing.T) {
	authSvc := &stubAuthService{createUserFn: func(context.Context, auth.CreateParams, auth.Role) (auth.User, error) {
		return auth.User{}, auth.ErrInvalidRole
	}}

	rec := doUsersCreateRequest(t, authSvc, `{"email":"new@local.test","full_name":"New User","role":"superuser","password":"secret-pass"}`, true)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestCreateUserDuplicateEmailConflict(t *testing.T) {
	authSvc := &stubAuthService{createUserFn: func(context.Context, auth.CreateParams, auth.Role) (auth.User, error) {
		return auth.User{}, auth.ErrConflict
	}}

	rec := doUsersCreateRequest(t, authSvc, `{"email":"dup@local.test","full_name":"Dup User","role":"requester","password":"secret-pass"}`, true)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestCreateUserInternalErrorNoLeak(t *testing.T) {
	authSvc := &stubAuthService{createUserFn: func(context.Context, auth.CreateParams, auth.Role) (auth.User, error) {
		return auth.User{}, errors.New("connection reset")
	}}

	rec := doUsersCreateRequest(t, authSvc, `{"email":"new@local.test","full_name":"New User","role":"requester","password":"secret-pass"}`, true)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("body leaks internal error: %q", rec.Body.String())
	}
}

func TestUpdateUserAdminSuccess(t *testing.T) {
	var gotParams auth.UpdateParams
	var gotActor uuid.UUID
	var gotRole auth.Role
	authSvc := &stubAuthService{updateUserFn: func(_ context.Context, params auth.UpdateParams, actorUserID uuid.UUID, role auth.Role) (auth.User, error) {
		gotParams = params
		gotActor = actorUserID
		gotRole = role
		return auth.User{
			ID:        params.ID,
			TenantID:  params.TenantID,
			Email:     "target@local.test",
			FullName:  "Target User",
			Role:      *params.Role,
			IsActive:  true,
			CreatedAt: mustTime("2026-08-12T12:00:00Z"),
		}, nil
	}}

	targetID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	rec := doUsersUpdateRequest(t, authSvc, targetID.String(), `{"role":"biomedic"}`, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != targetID || body.Role != auth.RoleBiomedic || !body.IsActive {
		t.Errorf("body = %+v", body)
	}
	if gotParams.ID != targetID {
		t.Errorf("UpdateUser() id = %v, want %v", gotParams.ID, targetID)
	}
	if gotParams.TenantID != uuid.MustParse(testTenantID) {
		t.Errorf("UpdateUser() tenant = %v, want session tenant %v", gotParams.TenantID, testTenantID)
	}
	if gotParams.Role == nil || *gotParams.Role != auth.RoleBiomedic {
		t.Errorf("UpdateUser() role = %+v", gotParams.Role)
	}
	if gotActor != uuid.MustParse(testUserID) {
		t.Errorf("UpdateUser() actor = %v, want session user %v", gotActor, testUserID)
	}
	if gotRole != auth.RoleAdmin {
		t.Errorf("UpdateUser() role = %q, want admin", gotRole)
	}
}

func TestUpdateUserResponseNoSecrets(t *testing.T) {
	authSvc := &stubAuthService{updateUserFn: func(_ context.Context, params auth.UpdateParams, _ uuid.UUID, _ auth.Role) (auth.User, error) {
		return auth.User{
			ID:           params.ID,
			TenantID:     params.TenantID,
			Email:        "target@local.test",
			Role:         *params.Role,
			PasswordHash: "$2a$10$AAZLkEqOuWupLfw.D5sFyuhAmW/CXDmJ5SehlpyNfaEZh1Xeoe3iO",
			IsActive:     true,
		}, nil
	}}

	rec := doUsersUpdateRequest(t, authSvc, "88888888-8888-8888-8888-888888888888", `{"role":"requester"}`, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, leak := range []string{"password_hash", "$2a$10$", "session", "token"} {
		if strings.Contains(body, leak) {
			t.Errorf("response leaks %q: %q", leak, body)
		}
	}
}

func TestUpdateUserNonAdminForbidden(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleBiomedic, auth.RoleRequester} {
		t.Run(string(role), func(t *testing.T) {
			authSvc := &stubAuthService{updateUserFn: func(context.Context, auth.UpdateParams, uuid.UUID, auth.Role) (auth.User, error) {
				return auth.User{}, auth.ErrForbidden
			}}

			rec := doUsersUpdateRequest(t, authSvc, "88888888-8888-8888-8888-888888888888", `{"is_active":false}`, true)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
			}
			if got := rec.Body.String(); got != `{"error":"forbidden"}`+"\n" {
				t.Errorf("body = %q, want %q", got, `{"error":"forbidden"}`+"\n")
			}
		})
	}
}

func TestUpdateUserWithoutSession(t *testing.T) {
	authSvc := &stubAuthService{}

	rec := doUsersUpdateRequest(t, authSvc, "88888888-8888-8888-8888-888888888888", `{"is_active":false}`, false)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUpdateUserInvalidJSON(t *testing.T) {
	authSvc := &stubAuthService{}

	rec := doUsersUpdateRequest(t, authSvc, "88888888-8888-8888-8888-888888888888", `{not json`, true)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestUpdateUserEmptyBody(t *testing.T) {
	authSvc := &stubAuthService{updateUserFn: func(context.Context, auth.UpdateParams, uuid.UUID, auth.Role) (auth.User, error) {
		return auth.User{}, auth.ErrEmptyUpdate
	}}

	rec := doUsersUpdateRequest(t, authSvc, "88888888-8888-8888-8888-888888888888", `{}`, true)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestUpdateUserInvalidRole(t *testing.T) {
	authSvc := &stubAuthService{updateUserFn: func(context.Context, auth.UpdateParams, uuid.UUID, auth.Role) (auth.User, error) {
		return auth.User{}, auth.ErrInvalidRole
	}}

	rec := doUsersUpdateRequest(t, authSvc, "88888888-8888-8888-8888-888888888888", `{"role":"superuser"}`, true)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestUpdateUserNotFound(t *testing.T) {
	authSvc := &stubAuthService{updateUserFn: func(context.Context, auth.UpdateParams, uuid.UUID, auth.Role) (auth.User, error) {
		return auth.User{}, auth.ErrUserNotFound
	}}

	rec := doUsersUpdateRequest(t, authSvc, "88888888-8888-8888-8888-888888888888", `{"is_active":false}`, true)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestUpdateUserSelfLockout(t *testing.T) {
	authSvc := &stubAuthService{updateUserFn: func(context.Context, auth.UpdateParams, uuid.UUID, auth.Role) (auth.User, error) {
		return auth.User{}, auth.ErrSelfLockout
	}}

	rec := doUsersUpdateRequest(t, authSvc, "88888888-8888-8888-8888-888888888888", `{"is_active":false}`, true)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestUpdateUserInvalidID(t *testing.T) {
	authSvc := &stubAuthService{}

	rec := doUsersUpdateRequest(t, authSvc, "not-a-uuid", `{"is_active":false}`, true)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestUpdateUserInternalErrorNoLeak(t *testing.T) {
	authSvc := &stubAuthService{updateUserFn: func(context.Context, auth.UpdateParams, uuid.UUID, auth.Role) (auth.User, error) {
		return auth.User{}, errors.New("connection reset")
	}}

	rec := doUsersUpdateRequest(t, authSvc, "88888888-8888-8888-8888-888888888888", `{"is_active":false}`, true)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Errorf("body leaks internal error: %q", rec.Body.String())
	}
}
