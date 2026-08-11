package tenant

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the persistence boundary for tenants. Implementations live in
// infrastructure packages and must return ErrNotFound when a tenant does not
// exist.
type Repository interface {
	Create(ctx context.Context, params CreateParams) (Tenant, error)
	GetByID(ctx context.Context, id uuid.UUID) (Tenant, error)
	GetBySlug(ctx context.Context, slug string) (Tenant, error)
}
