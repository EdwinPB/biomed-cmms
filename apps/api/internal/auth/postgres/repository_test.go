package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
	"github.com/edwinpolo/biomed-cmms/api/internal/dbtest"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

func newTestRepo(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()
	pool := dbtest.Pool(t)
	return NewRepository(pool), pool
}

func truncateAuthTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE auth_sessions, request_events, rfps, service_requests, equipment, users, tenants`); err != nil {
		t.Fatalf("truncate auth tables: %v", err)
	}
}

func insertTenant(t *testing.T, pool *pgxpool.Pool, slug string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO tenants (slug, name) VALUES ($1, $2) RETURNING id`, slug, "Test Hospital").Scan(&id); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	return id
}

func insertUser(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (tenant_id, email, password_hash, full_name, role)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		tenantID, email, "$2a$10$AAZLkEqOuWupLfw.D5sFyuhAmW/CXDmJ5SehlpyNfaEZh1Xeoe3iO", "Dev User", auth.RoleAdmin).Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func uniqueSlug(t *testing.T) string {
	t.Helper()
	return "auth-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}

func TestGetTenantBySlug(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	slug := uniqueSlug(t)
	tenantID := insertTenant(t, pool, slug)

	got, err := repo.GetTenantBySlug(ctx, slug)
	if err != nil {
		t.Fatalf("GetTenantBySlug() error = %v", err)
	}
	if got.ID != tenantID {
		t.Errorf("GetTenantBySlug() id = %v, want %v", got.ID, tenantID)
	}
	if got.Slug != slug || got.Name != "Test Hospital" {
		t.Errorf("GetTenantBySlug() = %+v", got)
	}
	if got.Status != tenant.StatusActive {
		t.Errorf("GetTenantBySlug() status = %q, want active", got.Status)
	}
}

func TestGetTenantBySlugNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()

	_, err := repo.GetTenantBySlug(ctx, "does-not-exist")
	if !errors.Is(err, tenant.ErrNotFound) {
		t.Errorf("GetTenantBySlug() error = %v, want tenant.ErrNotFound", err)
	}
}

func TestGetUserByTenantEmail(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))
	userID := insertUser(t, pool, tenantID, "dev@local.test")

	got, err := repo.GetUserByTenantEmail(ctx, tenantID, "dev@local.test")
	if err != nil {
		t.Fatalf("GetUserByTenantEmail() error = %v", err)
	}
	if got.ID != userID {
		t.Errorf("GetUserByTenantEmail() id = %v, want %v", got.ID, userID)
	}
	if got.Email != "dev@local.test" || got.FullName != "Dev User" {
		t.Errorf("GetUserByTenantEmail() = %+v", got)
	}
	if got.Role != auth.RoleAdmin {
		t.Errorf("GetUserByTenantEmail() role = %q, want admin", got.Role)
	}
	if !got.IsActive {
		t.Error("GetUserByTenantEmail() is_active = false, want true")
	}
	if got.PasswordHash == "" {
		t.Error("GetUserByTenantEmail() password_hash is empty")
	}
}

func TestGetUserByTenantEmailNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))
	insertUser(t, pool, tenantID, "dev@local.test")

	_, err := repo.GetUserByTenantEmail(ctx, tenantID, "nobody@local.test")
	if !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("GetUserByTenantEmail() error = %v, want auth.ErrUserNotFound", err)
	}
}

func TestGetUserByTenantEmailScopedToTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantA := insertTenant(t, pool, uniqueSlug(t))
	tenantB := insertTenant(t, pool, uniqueSlug(t))
	insertUser(t, pool, tenantA, "dev@local.test")

	_, err := repo.GetUserByTenantEmail(ctx, tenantB, "dev@local.test")
	if !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("GetUserByTenantEmail() across tenants error = %v, want auth.ErrUserNotFound", err)
	}
}

func TestCreateAndGetSessionRoundTrip(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	slug := uniqueSlug(t)
	tenantID := insertTenant(t, pool, slug)
	userID := insertUser(t, pool, tenantID, "dev@local.test")
	expires := time.Now().UTC().Add(12 * time.Hour)

	if err := repo.CreateSession(ctx, auth.CreateSessionParams{
		TokenHash: "aabbccdd",
		UserID:    userID,
		TenantID:  tenantID,
		ExpiresAt: expires,
	}); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	rec, err := repo.GetSessionByTokenHash(ctx, "aabbccdd")
	if err != nil {
		t.Fatalf("GetSessionByTokenHash() error = %v", err)
	}
	if rec.ID == uuid.Nil {
		t.Error("GetSessionByTokenHash() id is nil")
	}
	if rec.TokenHash != "aabbccdd" {
		t.Errorf("TokenHash = %q", rec.TokenHash)
	}
	if !rec.ExpiresAt.Equal(expires) {
		t.Errorf("ExpiresAt = %v, want %v", rec.ExpiresAt, expires)
	}
	if rec.User.ID != userID || rec.User.Role != auth.RoleAdmin {
		t.Errorf("session user = %+v", rec.User)
	}
	if rec.Tenant.ID != tenantID || rec.Tenant.Slug != slug || rec.Tenant.Name != "Test Hospital" {
		t.Errorf("session tenant = %+v", rec.Tenant)
	}
}

func TestGetSessionByTokenHashNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()

	_, err := repo.GetSessionByTokenHash(ctx, "nope")
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("GetSessionByTokenHash() error = %v, want auth.ErrSessionNotFound", err)
	}
}

func TestDeleteSession(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))
	userID := insertUser(t, pool, tenantID, "dev@local.test")

	if err := repo.CreateSession(ctx, auth.CreateSessionParams{TokenHash: "tok", UserID: userID, TenantID: tenantID, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if err := repo.DeleteSession(ctx, "tok"); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if _, err := repo.GetSessionByTokenHash(ctx, "tok"); !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("session still present after delete, error = %v", err)
	}
	if err := repo.DeleteSession(ctx, "tok"); !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("DeleteSession() twice error = %v, want auth.ErrSessionNotFound", err)
	}
}

func TestTouchSession(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))
	userID := insertUser(t, pool, tenantID, "dev@local.test")

	if err := repo.CreateSession(ctx, auth.CreateSessionParams{TokenHash: "tok", UserID: userID, TenantID: tenantID, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	touched := time.Now().Add(time.Minute)
	if err := repo.TouchSession(ctx, "tok", touched); err != nil {
		t.Fatalf("TouchSession() error = %v", err)
	}

	rec, err := repo.GetSessionByTokenHash(ctx, "tok")
	if err != nil {
		t.Fatalf("GetSessionByTokenHash() error = %v", err)
	}
	if !rec.LastUsedAt.Equal(touched) {
		t.Errorf("LastUsedAt = %v, want %v", rec.LastUsedAt, touched)
	}

	if err := repo.TouchSession(ctx, "missing", touched); !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("TouchSession() missing error = %v, want auth.ErrSessionNotFound", err)
	}
}

func TestSessionRevokedByUserDelete(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))
	userID := insertUser(t, pool, tenantID, "dev@local.test")

	if err := repo.CreateSession(ctx, auth.CreateSessionParams{TokenHash: "tok", UserID: userID, TenantID: tenantID, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	_, err := repo.GetSessionByTokenHash(ctx, "tok")
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("session survived user deletion, error = %v, want auth.ErrSessionNotFound", err)
	}
}

func TestListUsersScopedToTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))
	otherTenantID := insertTenant(t, pool, uniqueSlug(t))
	insertUser(t, pool, tenantID, "admin@local.test")
	insertUser(t, pool, tenantID, "requester@local.test")
	insertUser(t, pool, otherTenantID, "other-tenant@local.test")

	got, err := repo.ListUsers(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListUsers() returned %d users, want 2 (cross-tenant user must not appear)", len(got))
	}
	emails := make(map[string]bool, len(got))
	for _, u := range got {
		emails[u.Email] = true
		if u.TenantID != tenantID {
			t.Errorf("ListUsers() leaked user from tenant %v", u.TenantID)
		}
	}
	if !emails["admin@local.test"] || !emails["requester@local.test"] {
		t.Errorf("ListUsers() = %v, want tenant users", emails)
	}
	if emails["other-tenant@local.test"] {
		t.Error("ListUsers() returned a user from another tenant")
	}
}

func TestListUsersEmptyTenant(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))

	got, err := repo.ListUsers(ctx, tenantID)
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

func TestCreateUserPersistsScopedRow(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))

	created, err := repo.CreateUser(ctx, auth.CreateParams{
		TenantID:     tenantID,
		Email:        "new@local.test",
		FullName:     "New User",
		Role:         auth.RoleBiomedic,
		PasswordHash: "$2a$10$AAZLkEqOuWupLfw.D5sFyuhAmW/CXDmJ5SehlpyNfaEZh1Xeoe3iO",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if created.ID == uuid.Nil || created.TenantID != tenantID {
		t.Errorf("CreateUser() = %+v", created)
	}
	if created.Email != "new@local.test" || created.Role != auth.RoleBiomedic || !created.IsActive {
		t.Errorf("CreateUser() = %+v", created)
	}

	got, err := repo.GetUserByTenantEmail(ctx, tenantID, "new@local.test")
	if err != nil {
		t.Fatalf("GetUserByTenantEmail() error = %v", err)
	}
	if got.PasswordHash == "new-password" {
		t.Error("stored password_hash is plaintext")
	}
}

func TestCreateUserDuplicateEmailSameTenantConflict(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))
	insertUser(t, pool, tenantID, "dup@local.test")

	_, err := repo.CreateUser(ctx, auth.CreateParams{
		TenantID:     tenantID,
		Email:        "dup@local.test",
		Role:         auth.RoleRequester,
		PasswordHash: "$2a$10$AAZLkEqOuWupLfw.D5sFyuhAmW/CXDmJ5SehlpyNfaEZh1Xeoe3iO",
		IsActive:     true,
	})
	if !errors.Is(err, auth.ErrConflict) {
		t.Fatalf("CreateUser() error = %v, want auth.ErrConflict", err)
	}
}

func TestCreateUserSameEmailDifferentTenantAllowed(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))
	otherTenantID := insertTenant(t, pool, uniqueSlug(t))
	insertUser(t, pool, tenantID, "shared@local.test")

	_, err := repo.CreateUser(ctx, auth.CreateParams{
		TenantID:     otherTenantID,
		Email:        "shared@local.test",
		Role:         auth.RoleRequester,
		PasswordHash: "$2a$10$AAZLkEqOuWupLfw.D5sFyuhAmW/CXDmJ5SehlpyNfaEZh1Xeoe3iO",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("CreateUser() same email in another tenant = %v, want allowed", err)
	}

	got, err := repo.ListUsers(ctx, otherTenantID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("ListUsers() = %d, want 1", len(got))
	}
}

func TestUpdateUserRoleAndActivePersist(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))
	userID := insertUser(t, pool, tenantID, "dev@local.test")

	role := auth.RoleBiomedic
	active := false
	got, err := repo.UpdateUser(ctx, auth.UpdateParams{ID: userID, TenantID: tenantID, Role: &role, IsActive: &active})
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if got.Role != auth.RoleBiomedic || got.IsActive {
		t.Errorf("UpdateUser() = %+v", got)
	}

	persisted, err := repo.GetUserByTenantEmail(ctx, tenantID, "dev@local.test")
	if err != nil {
		t.Fatalf("GetUserByTenantEmail() error = %v", err)
	}
	if persisted.Role != auth.RoleBiomedic || persisted.IsActive {
		t.Errorf("persisted user = %+v", persisted)
	}
}

func TestUpdateUserRoleOnlyPersistsRole(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))
	userID := insertUser(t, pool, tenantID, "dev@local.test")

	role := auth.RoleRequester
	if _, err := repo.UpdateUser(ctx, auth.UpdateParams{ID: userID, TenantID: tenantID, Role: &role}); err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}

	persisted, err := repo.GetUserByTenantEmail(ctx, tenantID, "dev@local.test")
	if err != nil {
		t.Fatalf("GetUserByTenantEmail() error = %v", err)
	}
	if persisted.Role != auth.RoleRequester {
		t.Errorf("persisted role = %q, want requester", persisted.Role)
	}
	if !persisted.IsActive {
		t.Error("is_active changed when only role was updated")
	}
}

func TestUpdateUserIsActiveOnlyPersistsActive(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))
	userID := insertUser(t, pool, tenantID, "dev@local.test")

	active := false
	if _, err := repo.UpdateUser(ctx, auth.UpdateParams{ID: userID, TenantID: tenantID, IsActive: &active}); err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}

	persisted, err := repo.GetUserByTenantEmail(ctx, tenantID, "dev@local.test")
	if err != nil {
		t.Fatalf("GetUserByTenantEmail() error = %v", err)
	}
	if persisted.IsActive {
		t.Error("is_active = true, want false")
	}
	if persisted.Role != auth.RoleAdmin {
		t.Errorf("role changed when only is_active was updated: %q", persisted.Role)
	}
}

func TestUpdateUserForeignTenantIDNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))
	otherTenantID := insertTenant(t, pool, uniqueSlug(t))
	userID := insertUser(t, pool, tenantID, "dev@local.test")

	active := true
	_, err := repo.UpdateUser(ctx, auth.UpdateParams{ID: userID, TenantID: otherTenantID, IsActive: &active})
	if !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("UpdateUser() foreign tenant error = %v, want auth.ErrUserNotFound", err)
	}
}

func TestUpdateUserNonexistentIDNotFound(t *testing.T) {
	repo, pool := newTestRepo(t)
	truncateAuthTables(t, pool)
	ctx := context.Background()
	tenantID := insertTenant(t, pool, uniqueSlug(t))

	active := true
	_, err := repo.UpdateUser(ctx, auth.UpdateParams{ID: uuid.New(), TenantID: tenantID, IsActive: &active})
	if !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("UpdateUser() nonexistent error = %v, want auth.ErrUserNotFound", err)
	}
}
