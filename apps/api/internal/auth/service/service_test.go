package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

type fakeRepo struct {
	tenantBySlug  func(ctx context.Context, slug string) (tenant.Tenant, error)
	userByEmail   func(ctx context.Context, tenantID uuid.UUID, email string) (auth.User, error)
	listUsers     func(ctx context.Context, tenantID uuid.UUID) ([]auth.User, error)
	createUser    func(ctx context.Context, params auth.CreateParams) (auth.User, error)
	updateUser    func(ctx context.Context, params auth.UpdateParams) (auth.User, error)
	createSession func(ctx context.Context, params auth.CreateSessionParams) error
	sessionByHash func(ctx context.Context, tokenHash string) (auth.SessionRecord, error)
	deleteSession func(ctx context.Context, tokenHash string) error
	touchSession  func(ctx context.Context, tokenHash string, now time.Time) error

	gotCreateParams auth.CreateSessionParams
	gotUserParams   auth.CreateParams
	gotUpdateParams auth.UpdateParams
	gotTokenHash    string
	gotTouchNow     time.Time
	gotListTenantID uuid.UUID
}

func (f *fakeRepo) GetTenantBySlug(ctx context.Context, slug string) (tenant.Tenant, error) {
	if f.tenantBySlug != nil {
		return f.tenantBySlug(ctx, slug)
	}
	return tenant.Tenant{}, errors.New("fakeRepo: GetTenantBySlug not configured")
}

func (f *fakeRepo) GetUserByTenantEmail(ctx context.Context, tenantID uuid.UUID, email string) (auth.User, error) {
	if f.userByEmail != nil {
		return f.userByEmail(ctx, tenantID, email)
	}
	return auth.User{}, errors.New("fakeRepo: GetUserByTenantEmail not configured")
}

func (f *fakeRepo) ListUsers(ctx context.Context, tenantID uuid.UUID) ([]auth.User, error) {
	f.gotListTenantID = tenantID
	if f.listUsers != nil {
		return f.listUsers(ctx, tenantID)
	}
	return nil, errors.New("fakeRepo: ListUsers not configured")
}

func (f *fakeRepo) CreateUser(ctx context.Context, params auth.CreateParams) (auth.User, error) {
	f.gotUserParams = params
	if f.createUser != nil {
		return f.createUser(ctx, params)
	}
	return auth.User{}, errors.New("fakeRepo: CreateUser not configured")
}

func (f *fakeRepo) UpdateUser(ctx context.Context, params auth.UpdateParams) (auth.User, error) {
	f.gotUpdateParams = params
	if f.updateUser != nil {
		return f.updateUser(ctx, params)
	}
	return auth.User{}, errors.New("fakeRepo: UpdateUser not configured")
}

func (f *fakeRepo) CreateSession(ctx context.Context, params auth.CreateSessionParams) error {
	f.gotCreateParams = params
	if f.createSession != nil {
		return f.createSession(ctx, params)
	}
	return nil
}

func (f *fakeRepo) GetSessionByTokenHash(ctx context.Context, tokenHash string) (auth.SessionRecord, error) {
	f.gotTokenHash = tokenHash
	if f.sessionByHash != nil {
		return f.sessionByHash(ctx, tokenHash)
	}
	return auth.SessionRecord{}, errors.New("fakeRepo: GetSessionByTokenHash not configured")
}

func (f *fakeRepo) DeleteSession(ctx context.Context, tokenHash string) error {
	f.gotTokenHash = tokenHash
	if f.deleteSession != nil {
		return f.deleteSession(ctx, tokenHash)
	}
	return nil
}

func (f *fakeRepo) TouchSession(ctx context.Context, tokenHash string, now time.Time) error {
	f.gotTokenHash = tokenHash
	f.gotTouchNow = now
	if f.touchSession != nil {
		return f.touchSession(ctx, tokenHash, now)
	}
	return nil
}

const testBcryptHash = "$2a$10$AAZLkEqOuWupLfw.D5sFyuhAmW/CXDmJ5SehlpyNfaEZh1Xeoe3iO"

var (
	testTenantID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testUserID   = uuid.MustParse("22222222-2222-2222-2222-222222222222")
)

func testTenant() tenant.Tenant {
	return tenant.Tenant{ID: testTenantID, Slug: "local-dev", Name: "Local Dev", Status: tenant.StatusActive}
}

func testUser() auth.User {
	return auth.User{
		ID:           testUserID,
		TenantID:     testTenantID,
		Email:        "dev@local.test",
		FullName:     "Dev User",
		PasswordHash: testBcryptHash,
		Role:         auth.RoleAdmin,
		IsActive:     true,
	}
}

func fixedNow() time.Time {
	return mustParse("2026-08-12T12:00:00Z")
}

func mustParse(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func newTestService(repo *fakeRepo, ttl time.Duration) *Service {
	s := New(repo, ttl)
	s.now = fixedNow
	return s
}

func TestLoginSuccess(t *testing.T) {
	repo := &fakeRepo{
		tenantBySlug: func(context.Context, string) (tenant.Tenant, error) { return testTenant(), nil },
		userByEmail: func(context.Context, uuid.UUID, string) (auth.User, error) {
			return testUser(), nil
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	session, err := svc.Login(context.Background(), auth.Credentials{TenantSlug: "local-dev", Email: "dev@local.test", Password: "dev-password"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if session.Token == "" {
		t.Error("Login() token is empty")
	}
	if len(session.Token) != 64 {
		t.Errorf("Login() token length = %d, want 64", len(session.Token))
	}
	if session.Token == auth.HashToken(session.Token) {
		t.Error("raw token must never equal its hash")
	}
	if repo.gotCreateParams.TokenHash != auth.HashToken(session.Token) {
		t.Errorf("stored token hash = %q, want %q", repo.gotCreateParams.TokenHash, auth.HashToken(session.Token))
	}
	if repo.gotCreateParams.UserID != testUserID || repo.gotCreateParams.TenantID != testTenantID {
		t.Errorf("session params = %+v", repo.gotCreateParams)
	}
	wantExpiry := fixedNow().Add(12 * time.Hour)
	if !repo.gotCreateParams.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("expires_at = %v, want %v", repo.gotCreateParams.ExpiresAt, wantExpiry)
	}
	if session.ExpiresAt != repo.gotCreateParams.ExpiresAt {
		t.Errorf("session expiry mismatch: %v vs %v", session.ExpiresAt, repo.gotCreateParams.ExpiresAt)
	}
	if session.User.ID != testUserID || session.Tenant.Slug != "local-dev" {
		t.Errorf("session identity = %+v", session)
	}
}

func TestLoginNormalizesEmail(t *testing.T) {
	var gotEmail string
	repo := &fakeRepo{
		tenantBySlug: func(context.Context, string) (tenant.Tenant, error) { return testTenant(), nil },
		userByEmail: func(_ context.Context, _ uuid.UUID, email string) (auth.User, error) {
			gotEmail = email
			return testUser(), nil
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	if _, err := svc.Login(context.Background(), auth.Credentials{TenantSlug: "local-dev", Email: "  Dev@Local.Test ", Password: "dev-password"}); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if gotEmail != "dev@local.test" {
		t.Errorf("Login() looked up email %q, want normalized %q", gotEmail, "dev@local.test")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	repo := &fakeRepo{
		tenantBySlug: func(context.Context, string) (tenant.Tenant, error) { return testTenant(), nil },
		userByEmail: func(context.Context, uuid.UUID, string) (auth.User, error) {
			return testUser(), nil
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.Login(context.Background(), auth.Credentials{TenantSlug: "local-dev", Email: "dev@local.test", Password: "wrong"})
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want auth.ErrInvalidCredentials", err)
	}
	if repo.gotCreateParams.TokenHash != "" {
		t.Error("Login() created a session on a bad password")
	}
}

func TestLoginTenantNotFound(t *testing.T) {
	repo := &fakeRepo{
		tenantBySlug: func(context.Context, string) (tenant.Tenant, error) {
			return tenant.Tenant{}, tenant.ErrNotFound
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.Login(context.Background(), auth.Credentials{TenantSlug: "nope", Email: "dev@local.test", Password: "dev-password"})
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want auth.ErrInvalidCredentials", err)
	}
}

func TestLoginUserNotFound(t *testing.T) {
	repo := &fakeRepo{
		tenantBySlug: func(context.Context, string) (tenant.Tenant, error) { return testTenant(), nil },
		userByEmail: func(context.Context, uuid.UUID, string) (auth.User, error) {
			return auth.User{}, auth.ErrUserNotFound
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.Login(context.Background(), auth.Credentials{TenantSlug: "local-dev", Email: "nobody@local.test", Password: "dev-password"})
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want auth.ErrInvalidCredentials", err)
	}
}

func TestLoginInactiveUser(t *testing.T) {
	user := testUser()
	user.IsActive = false
	repo := &fakeRepo{
		tenantBySlug: func(context.Context, string) (tenant.Tenant, error) { return testTenant(), nil },
		userByEmail: func(context.Context, uuid.UUID, string) (auth.User, error) {
			return user, nil
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.Login(context.Background(), auth.Credentials{TenantSlug: "local-dev", Email: "dev@local.test", Password: "dev-password"})
	if !errors.Is(err, auth.ErrUserInactive) {
		t.Errorf("Login() error = %v, want auth.ErrUserInactive", err)
	}
	if repo.gotCreateParams.TokenHash != "" {
		t.Error("Login() created a session for an inactive user")
	}
}

func TestLoginEmptyCredentials(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(repo, 12*time.Hour)

	for _, creds := range []auth.Credentials{
		{TenantSlug: "", Email: "dev@local.test", Password: "dev-password"},
		{TenantSlug: "local-dev", Email: "", Password: "dev-password"},
		{TenantSlug: "local-dev", Email: "dev@local.test", Password: ""},
	} {
		if _, err := svc.Login(context.Background(), creds); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Errorf("Login(%+v) error = %v, want auth.ErrInvalidCredentials", creds, err)
		}
	}
}

func TestAuthenticateSuccess(t *testing.T) {
	repo := &fakeRepo{
		sessionByHash: func(context.Context, string) (auth.SessionRecord, error) {
			return auth.SessionRecord{
				ID:        uuid.MustParse("33333333-3333-3333-3333-333333333333"),
				TokenHash: "deadbeef",
				ExpiresAt: fixedNow().Add(time.Hour),
				User:      testUser(),
				Tenant:    testTenant(),
			}, nil
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	principal, err := svc.Authenticate(context.Background(), "deadbeef")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if principal.TenantID != testTenantID || principal.UserID != testUserID {
		t.Errorf("principal tenant/user = %v/%v", principal.TenantID, principal.UserID)
	}
	if principal.Role != auth.RoleAdmin {
		t.Errorf("principal role = %q, want admin", principal.Role)
	}
	if principal.Email != "dev@local.test" || principal.FullName != "Dev User" {
		t.Errorf("principal = %+v", principal)
	}
	if principal.TenantSlug != "local-dev" || principal.TenantName != "Local Dev" {
		t.Errorf("principal tenant = %+v", principal)
	}
	if repo.gotTokenHash != "deadbeef" {
		t.Errorf("repo token hash = %q", repo.gotTokenHash)
	}
	if !repo.gotTouchNow.Equal(fixedNow()) {
		t.Errorf("TouchSession now = %v, want %v", repo.gotTouchNow, fixedNow())
	}
}

func TestAuthenticateEmptyTokenHash(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.Authenticate(context.Background(), "")
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("Authenticate() error = %v, want auth.ErrSessionNotFound", err)
	}
}

func TestAuthenticateSessionNotFound(t *testing.T) {
	repo := &fakeRepo{
		sessionByHash: func(context.Context, string) (auth.SessionRecord, error) {
			return auth.SessionRecord{}, auth.ErrSessionNotFound
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.Authenticate(context.Background(), "nope")
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("Authenticate() error = %v, want auth.ErrSessionNotFound", err)
	}
}

func TestAuthenticateExpiredSession(t *testing.T) {
	repo := &fakeRepo{
		sessionByHash: func(context.Context, string) (auth.SessionRecord, error) {
			return auth.SessionRecord{
				ExpiresAt: fixedNow().Add(-time.Minute),
				User:      testUser(),
				Tenant:    testTenant(),
			}, nil
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.Authenticate(context.Background(), "expired")
	if !errors.Is(err, auth.ErrSessionExpired) {
		t.Errorf("Authenticate() error = %v, want auth.ErrSessionExpired", err)
	}
}

func TestAuthenticateInactiveUser(t *testing.T) {
	user := testUser()
	user.IsActive = false
	repo := &fakeRepo{
		sessionByHash: func(context.Context, string) (auth.SessionRecord, error) {
			return auth.SessionRecord{
				ExpiresAt: fixedNow().Add(time.Hour),
				User:      user,
				Tenant:    testTenant(),
			}, nil
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.Authenticate(context.Background(), "inactive")
	if !errors.Is(err, auth.ErrUserInactive) {
		t.Errorf("Authenticate() error = %v, want auth.ErrUserInactive", err)
	}
}

func TestAuthenticateSlidingTouchFailureTolerated(t *testing.T) {
	repo := &fakeRepo{
		sessionByHash: func(context.Context, string) (auth.SessionRecord, error) {
			return auth.SessionRecord{
				ExpiresAt: fixedNow().Add(time.Hour),
				User:      testUser(),
				Tenant:    testTenant(),
			}, nil
		},
		touchSession: func(context.Context, string, time.Time) error {
			return errors.New("touch failed")
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	if _, err := svc.Authenticate(context.Background(), "token"); err != nil {
		t.Errorf("Authenticate() error = %v, want success despite touch failure", err)
	}
}

func TestLogoutDeletesSession(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(repo, 12*time.Hour)

	if err := svc.Logout(context.Background(), "deadbeef"); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if repo.gotTokenHash != "deadbeef" {
		t.Errorf("Logout() deleted hash = %q", repo.gotTokenHash)
	}
}

func TestLogoutIdempotentWhenSessionGone(t *testing.T) {
	repo := &fakeRepo{
		deleteSession: func(context.Context, string) error { return auth.ErrSessionNotFound },
	}
	svc := newTestService(repo, 12*time.Hour)

	if err := svc.Logout(context.Background(), "gone"); err != nil {
		t.Errorf("Logout() error = %v, want nil for already-revoked session", err)
	}
}

func TestHashTokenDeterministic(t *testing.T) {
	if auth.HashToken("abc") != auth.HashToken("abc") {
		t.Error("HashToken is not deterministic")
	}
	if auth.HashToken("abc") == auth.HashToken("abd") {
		t.Error("HashToken collision on distinct inputs")
	}
	if len(auth.HashToken("abc")) != 64 {
		t.Errorf("HashToken length = %d, want 64", len(auth.HashToken("abc")))
	}
}

func TestNewTokenUnique(t *testing.T) {
	a, err := auth.NewToken()
	if err != nil {
		t.Fatalf("NewToken() error = %v", err)
	}
	b, err := auth.NewToken()
	if err != nil {
		t.Fatalf("NewToken() error = %v", err)
	}
	if a == b {
		t.Error("NewToken() returned duplicate tokens")
	}
	if len(a) != 64 || len(b) != 64 {
		t.Errorf("token lengths = %d/%d, want 64/64", len(a), len(b))
	}
}

func TestListUsersAdminReturnsTenantUsers(t *testing.T) {
	want := []auth.User{
		{ID: testUserID, TenantID: testTenantID, Email: "dev@local.test", Role: auth.RoleAdmin, IsActive: true},
		{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), TenantID: testTenantID, Email: "requester@local.test", Role: auth.RoleRequester, IsActive: true},
	}
	repo := &fakeRepo{listUsers: func(context.Context, uuid.UUID) ([]auth.User, error) {
		return want, nil
	}}
	svc := newTestService(repo, 12*time.Hour)

	got, err := svc.ListUsers(context.Background(), testTenantID, auth.RoleAdmin)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ListUsers() = %+v, want %+v", got, want)
	}
	if repo.gotListTenantID != testTenantID {
		t.Errorf("ListUsers() tenant = %v, want %v", repo.gotListTenantID, testTenantID)
	}
}

func TestListUsersAdminForwardsEmptySlice(t *testing.T) {
	repo := &fakeRepo{listUsers: func(context.Context, uuid.UUID) ([]auth.User, error) {
		return []auth.User{}, nil
	}}
	svc := newTestService(repo, 12*time.Hour)

	got, err := svc.ListUsers(context.Background(), testTenantID, auth.RoleAdmin)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if got == nil {
		t.Error("ListUsers() returned nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("ListUsers() returned %d users, want 0", len(got))
	}
}

func TestListUsersNonAdminForbiddenWithoutRepoAccess(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleBiomedic, auth.RoleRequester} {
		t.Run(string(role), func(t *testing.T) {
			repo := &fakeRepo{
				listUsers: func(context.Context, uuid.UUID) ([]auth.User, error) {
					t.Fatal("ListUsers() must not be called for non-admin")
					return nil, nil
				},
			}
			svc := newTestService(repo, 12*time.Hour)

			_, err := svc.ListUsers(context.Background(), testTenantID, role)
			if !errors.Is(err, auth.ErrForbidden) {
				t.Fatalf("ListUsers() error = %v, want ErrForbidden", err)
			}
			if repo.gotListTenantID != uuid.Nil {
				t.Errorf("ListUsers() called with tenant %v, want no call", repo.gotListTenantID)
			}
		})
	}
}

func TestListUsersRepoErrorPropagated(t *testing.T) {
	wantErr := errors.New("connection reset")
	repo := &fakeRepo{listUsers: func(context.Context, uuid.UUID) ([]auth.User, error) {
		return nil, wantErr
	}}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.ListUsers(context.Background(), testTenantID, auth.RoleAdmin)
	if !errors.Is(err, wantErr) {
		t.Errorf("ListUsers() error = %v, want %v", err, wantErr)
	}
}

func TestCreateUserAdminSuccess(t *testing.T) {
	repo := &fakeRepo{createUser: func(_ context.Context, params auth.CreateParams) (auth.User, error) {
		return auth.User{
			ID:           uuid.MustParse("44444444-4444-4444-4444-444444444444"),
			TenantID:     params.TenantID,
			Email:        params.Email,
			FullName:     params.FullName,
			Role:         params.Role,
			PasswordHash: params.PasswordHash,
			IsActive:     params.IsActive,
		}, nil
	}}
	svc := newTestService(repo, 12*time.Hour)

	got, err := svc.CreateUser(context.Background(), auth.CreateParams{
		TenantID:     testTenantID,
		Email:        "  New.Requester@Local.Test ",
		FullName:     "  New Requester ",
		Role:         auth.RoleRequester,
		PasswordHash: "plain-secret",
	}, auth.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if got.Email != "new.requester@local.test" {
		t.Errorf("CreateUser() email = %q, want normalized", got.Email)
	}
	if got.FullName != "New Requester" {
		t.Errorf("CreateUser() full_name = %q, want trimmed", got.FullName)
	}
	if !got.IsActive {
		t.Error("CreateUser() is_active = false, want default true")
	}

	stored := repo.gotUserParams
	if stored.TenantID != testTenantID {
		t.Errorf("CreateUser() tenant = %v, want %v", stored.TenantID, testTenantID)
	}
	if stored.Email != "new.requester@local.test" {
		t.Errorf("CreateUser() persisted email = %q, want normalized", stored.Email)
	}
	if !stored.IsActive {
		t.Error("CreateUser() persisted is_active = false, want true")
	}
	if stored.PasswordHash == "plain-secret" {
		t.Error("CreateUser() stored the plaintext password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("plain-secret")); err != nil {
		t.Errorf("stored password does not verify against bcrypt: %v", err)
	}
}

func TestCreateUserNonAdminForbiddenWithoutRepoAccess(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleBiomedic, auth.RoleRequester} {
		t.Run(string(role), func(t *testing.T) {
			repo := &fakeRepo{
				createUser: func(context.Context, auth.CreateParams) (auth.User, error) {
					t.Fatal("CreateUser() must not be called for non-admin")
					return auth.User{}, nil
				},
			}
			svc := newTestService(repo, 12*time.Hour)

			_, err := svc.CreateUser(context.Background(), auth.CreateParams{TenantID: testTenantID, Email: "a@b.test", Role: auth.RoleRequester, PasswordHash: "pw"}, role)
			if !errors.Is(err, auth.ErrForbidden) {
				t.Fatalf("CreateUser() error = %v, want ErrForbidden", err)
			}
			if repo.gotUserParams != (auth.CreateParams{}) {
				t.Errorf("CreateUser() called with params %+v, want no call", repo.gotUserParams)
			}
		})
	}
}

func TestCreateUserInvalidRole(t *testing.T) {
	repo := &fakeRepo{
		createUser: func(context.Context, auth.CreateParams) (auth.User, error) {
			t.Fatal("CreateUser() must not be called for invalid input")
			return auth.User{}, nil
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.CreateUser(context.Background(), auth.CreateParams{TenantID: testTenantID, Email: "a@b.test", Role: auth.Role("superuser"), PasswordHash: "pw"}, auth.RoleAdmin)
	if !errors.Is(err, auth.ErrInvalidRole) {
		t.Fatalf("CreateUser() error = %v, want ErrInvalidRole", err)
	}
}

func TestCreateUserMissingEmailAndPassword(t *testing.T) {
	svc := newTestService(&fakeRepo{}, 12*time.Hour)

	_, err := svc.CreateUser(context.Background(), auth.CreateParams{TenantID: testTenantID, Role: auth.RoleRequester}, auth.RoleAdmin)
	if !errors.Is(err, auth.ErrEmailRequired) {
		t.Errorf("CreateUser() error = %v, want ErrEmailRequired", err)
	}
	if !errors.Is(err, auth.ErrPasswordRequired) {
		t.Errorf("CreateUser() error = %v, want ErrPasswordRequired", err)
	}
}

func TestCreateUserDuplicateEmailPropagatesConflict(t *testing.T) {
	repo := &fakeRepo{createUser: func(context.Context, auth.CreateParams) (auth.User, error) {
		return auth.User{}, auth.ErrConflict
	}}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.CreateUser(context.Background(), auth.CreateParams{TenantID: testTenantID, Email: "dup@local.test", Role: auth.RoleRequester, PasswordHash: "pw"}, auth.RoleAdmin)
	if !errors.Is(err, auth.ErrConflict) {
		t.Fatalf("CreateUser() error = %v, want auth.ErrConflict", err)
	}
}

func TestUpdateUserRoleChange(t *testing.T) {
	role := auth.RoleBiomedic
	repo := &fakeRepo{updateUser: func(_ context.Context, params auth.UpdateParams) (auth.User, error) {
		return auth.User{ID: testUserID, TenantID: params.TenantID, Role: *params.Role, IsActive: true}, nil
	}}
	svc := newTestService(repo, 12*time.Hour)

	got, err := svc.UpdateUser(context.Background(), auth.UpdateParams{ID: testUserID, TenantID: testTenantID, Role: &role}, uuid.New(), auth.RoleAdmin)
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if got.Role != auth.RoleBiomedic {
		t.Errorf("UpdateUser() role = %q, want biomedic", got.Role)
	}
	if repo.gotUpdateParams.ID != testUserID || repo.gotUpdateParams.TenantID != testTenantID {
		t.Errorf("UpdateUser() scope = %+v", repo.gotUpdateParams)
	}
	if repo.gotUpdateParams.Role == nil || *repo.gotUpdateParams.Role != auth.RoleBiomedic {
		t.Errorf("UpdateUser() params role = %+v", repo.gotUpdateParams.Role)
	}
	if repo.gotUpdateParams.IsActive != nil {
		t.Errorf("UpdateUser() params is_active set unexpectedly: %+v", *repo.gotUpdateParams.IsActive)
	}
}

func TestUpdateUserActivateAndDeactivate(t *testing.T) {
	for _, active := range []bool{true, false} {
		t.Run(map[bool]string{true: "activate", false: "deactivate"}[active], func(t *testing.T) {
			repo := &fakeRepo{updateUser: func(_ context.Context, params auth.UpdateParams) (auth.User, error) {
				return auth.User{ID: testUserID, TenantID: params.TenantID, Role: auth.RoleRequester, IsActive: *params.IsActive}, nil
			}}
			svc := newTestService(repo, 12*time.Hour)

			got, err := svc.UpdateUser(context.Background(), auth.UpdateParams{ID: testUserID, TenantID: testTenantID, IsActive: &active}, uuid.New(), auth.RoleAdmin)
			if err != nil {
				t.Fatalf("UpdateUser() error = %v", err)
			}
			if got.IsActive != active {
				t.Errorf("UpdateUser() is_active = %v, want %v", got.IsActive, active)
			}
			if repo.gotUpdateParams.IsActive == nil || *repo.gotUpdateParams.IsActive != active {
				t.Errorf("UpdateUser() params is_active = %+v", repo.gotUpdateParams.IsActive)
			}
		})
	}
}

func TestUpdateUserBothFieldsTogether(t *testing.T) {
	role := auth.RoleRequester
	active := false
	repo := &fakeRepo{updateUser: func(_ context.Context, params auth.UpdateParams) (auth.User, error) {
		return auth.User{ID: testUserID, TenantID: params.TenantID, Role: *params.Role, IsActive: *params.IsActive}, nil
	}}
	svc := newTestService(repo, 12*time.Hour)

	got, err := svc.UpdateUser(context.Background(), auth.UpdateParams{ID: testUserID, TenantID: testTenantID, Role: &role, IsActive: &active}, uuid.New(), auth.RoleAdmin)
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if got.Role != auth.RoleRequester || got.IsActive {
		t.Errorf("UpdateUser() = %+v", got)
	}
	if repo.gotUpdateParams.Role == nil || repo.gotUpdateParams.IsActive == nil {
		t.Error("UpdateUser() did not forward both fields")
	}
}

func TestUpdateUserEmptyUpdate(t *testing.T) {
	repo := &fakeRepo{
		updateUser: func(context.Context, auth.UpdateParams) (auth.User, error) {
			t.Fatal("UpdateUser() must not be called for empty update")
			return auth.User{}, nil
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.UpdateUser(context.Background(), auth.UpdateParams{ID: testUserID, TenantID: testTenantID}, uuid.New(), auth.RoleAdmin)
	if !errors.Is(err, auth.ErrEmptyUpdate) {
		t.Fatalf("UpdateUser() error = %v, want ErrEmptyUpdate", err)
	}
}

func TestUpdateUserInvalidRole(t *testing.T) {
	role := auth.Role("superuser")
	repo := &fakeRepo{
		updateUser: func(context.Context, auth.UpdateParams) (auth.User, error) {
			t.Fatal("UpdateUser() must not be called for invalid role")
			return auth.User{}, nil
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.UpdateUser(context.Background(), auth.UpdateParams{ID: testUserID, TenantID: testTenantID, Role: &role}, uuid.New(), auth.RoleAdmin)
	if !errors.Is(err, auth.ErrInvalidRole) {
		t.Fatalf("UpdateUser() error = %v, want ErrInvalidRole", err)
	}
}

func TestUpdateUserNonAdminForbiddenWithoutRepoAccess(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleBiomedic, auth.RoleRequester} {
		t.Run(string(role), func(t *testing.T) {
			active := true
			repo := &fakeRepo{
				updateUser: func(context.Context, auth.UpdateParams) (auth.User, error) {
					t.Fatal("UpdateUser() must not be called for non-admin")
					return auth.User{}, nil
				},
			}
			svc := newTestService(repo, 12*time.Hour)

			_, err := svc.UpdateUser(context.Background(), auth.UpdateParams{ID: testUserID, TenantID: testTenantID, IsActive: &active}, uuid.New(), role)
			if !errors.Is(err, auth.ErrForbidden) {
				t.Fatalf("UpdateUser() error = %v, want ErrForbidden", err)
			}
			if repo.gotUpdateParams != (auth.UpdateParams{}) {
				t.Errorf("UpdateUser() called with params %+v, want no call", repo.gotUpdateParams)
			}
		})
	}
}

func TestUpdateUserNotFoundPropagated(t *testing.T) {
	repo := &fakeRepo{updateUser: func(context.Context, auth.UpdateParams) (auth.User, error) {
		return auth.User{}, auth.ErrUserNotFound
	}}
	svc := newTestService(repo, 12*time.Hour)
	role := auth.RoleAdmin

	_, err := svc.UpdateUser(context.Background(), auth.UpdateParams{ID: uuid.New(), TenantID: testTenantID, Role: &role}, uuid.New(), auth.RoleAdmin)
	if !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("UpdateUser() error = %v, want auth.ErrUserNotFound", err)
	}
}

func TestUpdateUserAdminCannotDeactivateSelf(t *testing.T) {
	active := false
	repo := &fakeRepo{
		updateUser: func(context.Context, auth.UpdateParams) (auth.User, error) {
			t.Fatal("UpdateUser() must not be called for self-lockout")
			return auth.User{}, nil
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.UpdateUser(context.Background(), auth.UpdateParams{ID: testUserID, TenantID: testTenantID, IsActive: &active}, testUserID, auth.RoleAdmin)
	if !errors.Is(err, auth.ErrSelfLockout) {
		t.Fatalf("UpdateUser() error = %v, want ErrSelfLockout", err)
	}
}

func TestUpdateUserAdminCannotDemoteSelf(t *testing.T) {
	role := auth.RoleRequester
	repo := &fakeRepo{
		updateUser: func(context.Context, auth.UpdateParams) (auth.User, error) {
			t.Fatal("UpdateUser() must not be called for self-lockout")
			return auth.User{}, nil
		},
	}
	svc := newTestService(repo, 12*time.Hour)

	_, err := svc.UpdateUser(context.Background(), auth.UpdateParams{ID: testUserID, TenantID: testTenantID, Role: &role}, testUserID, auth.RoleAdmin)
	if !errors.Is(err, auth.ErrSelfLockout) {
		t.Fatalf("UpdateUser() error = %v, want ErrSelfLockout", err)
	}
}

func TestUpdateUserAdminCanUpdateAnotherAdmin(t *testing.T) {
	role := auth.RoleAdmin
	active := false
	otherAdminID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	repo := &fakeRepo{updateUser: func(_ context.Context, params auth.UpdateParams) (auth.User, error) {
		return auth.User{ID: params.ID, TenantID: params.TenantID, Role: *params.Role, IsActive: *params.IsActive}, nil
	}}
	svc := newTestService(repo, 12*time.Hour)

	got, err := svc.UpdateUser(context.Background(), auth.UpdateParams{ID: otherAdminID, TenantID: testTenantID, Role: &role, IsActive: &active}, testUserID, auth.RoleAdmin)
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if got.Role != auth.RoleAdmin || got.IsActive {
		t.Errorf("UpdateUser() = %+v", got)
	}
}

func TestUpdateUserAdminCanSetOwnRoleToAdmin(t *testing.T) {
	role := auth.RoleAdmin
	repo := &fakeRepo{updateUser: func(_ context.Context, params auth.UpdateParams) (auth.User, error) {
		return auth.User{ID: params.ID, TenantID: params.TenantID, Role: *params.Role, IsActive: true}, nil
	}}
	svc := newTestService(repo, 12*time.Hour)

	if _, err := svc.UpdateUser(context.Background(), auth.UpdateParams{ID: testUserID, TenantID: testTenantID, Role: &role}, testUserID, auth.RoleAdmin); err != nil {
		t.Fatalf("UpdateUser() error = %v, want nil for self role=admin", err)
	}
}

func TestUpdateUserRepoErrorPropagated(t *testing.T) {
	wantErr := errors.New("connection reset")
	repo := &fakeRepo{updateUser: func(context.Context, auth.UpdateParams) (auth.User, error) {
		return auth.User{}, wantErr
	}}
	svc := newTestService(repo, 12*time.Hour)
	active := true

	_, err := svc.UpdateUser(context.Background(), auth.UpdateParams{ID: testUserID, TenantID: testTenantID, IsActive: &active}, uuid.New(), auth.RoleAdmin)
	if !errors.Is(err, wantErr) {
		t.Errorf("UpdateUser() error = %v, want %v", err, wantErr)
	}
}
