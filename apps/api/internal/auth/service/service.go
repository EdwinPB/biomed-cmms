// Package service implements the authentication use cases. It depends only on
// the auth domain boundary (auth.Repository), never on PostgreSQL.
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

type Service struct {
	repo auth.Repository
	now  func() time.Time
	ttl  time.Duration
}

func New(repo auth.Repository, ttl time.Duration) *Service {
	return &Service{repo: repo, now: time.Now, ttl: ttl}
}

// Login validates credentials and issues a session. The returned session
// carries the raw token, which the caller must hand to the client as a cookie;
// only HashToken(token) is persisted.
func (s *Service) Login(ctx context.Context, creds auth.Credentials) (auth.Session, error) {
	if strings.TrimSpace(creds.TenantSlug) == "" || strings.TrimSpace(creds.Email) == "" || creds.Password == "" {
		return auth.Session{}, auth.ErrInvalidCredentials
	}

	t, err := s.repo.GetTenantBySlug(ctx, strings.TrimSpace(creds.TenantSlug))
	if err != nil {
		if errors.Is(err, tenant.ErrNotFound) {
			return auth.Session{}, auth.ErrInvalidCredentials
		}
		return auth.Session{}, err
	}

	user, err := s.repo.GetUserByTenantEmail(ctx, t.ID, strings.ToLower(strings.TrimSpace(creds.Email)))
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return auth.Session{}, auth.ErrInvalidCredentials
		}
		return auth.Session{}, err
	}
	if !user.IsActive {
		return auth.Session{}, auth.ErrUserInactive
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(creds.Password)); err != nil {
		return auth.Session{}, auth.ErrInvalidCredentials
	}

	token, err := auth.NewToken()
	if err != nil {
		return auth.Session{}, err
	}

	now := s.now()
	expiresAt := now.Add(s.ttl)
	if err := s.repo.CreateSession(ctx, auth.CreateSessionParams{
		TokenHash: auth.HashToken(token),
		UserID:    user.ID,
		TenantID:  t.ID,
		ExpiresAt: expiresAt,
	}); err != nil {
		return auth.Session{}, err
	}

	return auth.Session{Token: token, ExpiresAt: expiresAt, User: user, Tenant: t}, nil
}

// Authenticate resolves a token hash to an active session principal. Expired
// sessions and sessions belonging to deactivated users are rejected.
func (s *Service) Authenticate(ctx context.Context, tokenHash string) (auth.Principal, error) {
	if tokenHash == "" {
		return auth.Principal{}, auth.ErrSessionNotFound
	}

	record, err := s.repo.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return auth.Principal{}, err
	}
	if record.ExpiresAt.Before(s.now()) {
		return auth.Principal{}, auth.ErrSessionExpired
	}
	if !record.User.IsActive {
		return auth.Principal{}, auth.ErrUserInactive
	}

	// Sliding TTL: best-effort refresh of last_used_at. A failure here does not
	// invalidate the request.
	_ = s.repo.TouchSession(ctx, tokenHash, s.now())

	return auth.Principal{
		TenantID:   record.Tenant.ID,
		UserID:     record.User.ID,
		Role:       record.User.Role,
		Email:      record.User.Email,
		FullName:   record.User.FullName,
		TenantSlug: record.Tenant.Slug,
		TenantName: record.Tenant.Name,
	}, nil
}

// ListUsers returns the tenant's users. Only admins may list users; biomedic
// and requester callers are rejected before any repository access. Tenant
// scoping is delegated to the repository.
func (s *Service) ListUsers(ctx context.Context, tenantID uuid.UUID, role auth.Role) ([]auth.User, error) {
	if role != auth.RoleAdmin {
		return nil, auth.ErrForbidden
	}
	return s.repo.ListUsers(ctx, tenantID)
}

// CreateUser creates a user in the authenticated tenant. Only admins may
// create users. The tenant ID comes from the caller, never from the request
// body. Email is trimmed and lowercased, the password is hashed with bcrypt,
// and new users are active by default.
func (s *Service) CreateUser(ctx context.Context, params auth.CreateParams, role auth.Role) (auth.User, error) {
	if role != auth.RoleAdmin {
		return auth.User{}, auth.ErrForbidden
	}
	if err := validateCreateUser(params); err != nil {
		return auth.User{}, err
	}

	params.Email = strings.ToLower(strings.TrimSpace(params.Email))
	params.FullName = strings.TrimSpace(params.FullName)

	hash, err := bcrypt.GenerateFromPassword([]byte(params.PasswordHash), bcrypt.DefaultCost)
	if err != nil {
		return auth.User{}, err
	}
	params.PasswordHash = string(hash)
	params.IsActive = true

	return s.repo.CreateUser(ctx, params)
}

func validateCreateUser(params auth.CreateParams) error {
	var errs []error
	if strings.TrimSpace(params.Email) == "" {
		errs = append(errs, auth.ErrEmailRequired)
	}
	if params.PasswordHash == "" {
		errs = append(errs, auth.ErrPasswordRequired)
	}
	switch params.Role {
	case auth.RoleAdmin, auth.RoleBiomedic, auth.RoleRequester:
	default:
		errs = append(errs, auth.ErrInvalidRole)
	}
	return errors.Join(errs...)
}

// UpdateUser updates a user's role and/or is_active within the authenticated
// tenant. Only admins may update users. The update is always scoped by
// tenant_id + user id, and admins are prevented from deactivating themselves
// or changing their own role away from admin.
func (s *Service) UpdateUser(ctx context.Context, params auth.UpdateParams, actorUserID uuid.UUID, role auth.Role) (auth.User, error) {
	if role != auth.RoleAdmin {
		return auth.User{}, auth.ErrForbidden
	}
	if err := validateUpdateUser(params); err != nil {
		return auth.User{}, err
	}
	if params.ID == actorUserID {
		if params.IsActive != nil && !*params.IsActive {
			return auth.User{}, auth.ErrSelfLockout
		}
		if params.Role != nil && *params.Role != auth.RoleAdmin {
			return auth.User{}, auth.ErrSelfLockout
		}
	}
	return s.repo.UpdateUser(ctx, params)
}

func validateUpdateUser(params auth.UpdateParams) error {
	var errs []error
	if params.Role == nil && params.IsActive == nil {
		errs = append(errs, auth.ErrEmptyUpdate)
	}
	if params.Role != nil {
		switch *params.Role {
		case auth.RoleAdmin, auth.RoleBiomedic, auth.RoleRequester:
		default:
			errs = append(errs, auth.ErrInvalidRole)
		}
	}
	return errors.Join(errs...)
}

// Logout revokes a session. Logging out an already-revoked session is a no-op.
func (s *Service) Logout(ctx context.Context, tokenHash string) error {
	err := s.repo.DeleteSession(ctx, tokenHash)
	if errors.Is(err, auth.ErrSessionNotFound) {
		return nil
	}
	return err
}
