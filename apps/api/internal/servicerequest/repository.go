package servicerequest

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the persistence boundary for service requests. Implementations
// live in infrastructure packages. All lookups are tenant-scoped: a tenant can
// never read or reference another tenant's data.
type Repository interface {
	Create(ctx context.Context, params CreateParams) (ServiceRequest, error)
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (ServiceRequest, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]ServiceRequest, error)
}
