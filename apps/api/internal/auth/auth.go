// Package auth defines authentication and session domain types plus the
// repository boundary. This package is database-agnostic: it must not import
// pgx, contain SQL, or depend on any infrastructure package.
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

// Role is the functional role of a user within a tenant.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleBiomedic  Role = "biomedic"
	RoleRequester Role = "requester"
)

var (
	ErrTenantNotFound     = errors.New("auth: tenant not found")
	ErrUserNotFound       = errors.New("auth: user not found")
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrUserInactive       = errors.New("auth: user is inactive")
	ErrSessionNotFound    = errors.New("auth: session not found")
	ErrSessionExpired     = errors.New("auth: session expired")
	ErrForbidden          = errors.New("auth: forbidden")
	ErrConflict           = errors.New("auth: user already exists")
	ErrEmailRequired      = errors.New("auth: email is required")
	ErrPasswordRequired   = errors.New("auth: password is required")
	ErrInvalidRole        = errors.New("auth: invalid role")
)

// User is the subset of the users record needed by the auth flows.
type User struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	Email        string
	FullName     string
	PasswordHash string
	Role         Role
	IsActive     bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Session is a server-side login session. Token holds the raw bearer value
// (only ever produced at login); every persisted copy is the SHA-256 hash.
type Session struct {
	Token     string
	ExpiresAt time.Time
	User      User
	Tenant    tenant.Tenant
}

// SessionRecord is a persisted session joined with its user and tenant.
type SessionRecord struct {
	ID         uuid.UUID
	TokenHash  string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastUsedAt time.Time
	User       User
	Tenant     tenant.Tenant
}

// Principal is the authenticated identity carried through the request.
type Principal struct {
	TenantID   uuid.UUID
	UserID     uuid.UUID
	Role       Role
	Email      string
	FullName   string
	TenantSlug string
	TenantName string
}

// Credentials are the inputs to Login.
type Credentials struct {
	TenantSlug string
	Email      string
	Password   string
}

// CreateSessionParams are the inputs to Repository.CreateSession.
type CreateSessionParams struct {
	TokenHash string
	UserID    uuid.UUID
	TenantID  uuid.UUID
	ExpiresAt time.Time
}

// CreateParams are the inputs to Repository.CreateUser. PasswordHash carries
// the plaintext password from the HTTP layer until the service hashes it;
// before persistence it is replaced by the bcrypt digest.
type CreateParams struct {
	TenantID     uuid.UUID
	Email        string
	FullName     string
	Role         Role
	PasswordHash string
	IsActive     bool
}

// Repository is the persistence boundary for auth. Implementations live in
// infrastructure packages; the service depends only on this interface.
type Repository interface {
	GetTenantBySlug(ctx context.Context, slug string) (tenant.Tenant, error)
	GetUserByTenantEmail(ctx context.Context, tenantID uuid.UUID, email string) (User, error)
	ListUsers(ctx context.Context, tenantID uuid.UUID) ([]User, error)
	CreateUser(ctx context.Context, params CreateParams) (User, error)
	CreateSession(ctx context.Context, params CreateSessionParams) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (SessionRecord, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	TouchSession(ctx context.Context, tokenHash string, now time.Time) error
}
