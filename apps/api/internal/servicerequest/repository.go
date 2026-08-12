package servicerequest

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the persistence boundary for service requests. Implementations
// live in infrastructure packages. All lookups are tenant-scoped: a tenant can
// never read or reference another tenant's data.
//
// Transition is atomic: it updates the request status and inserts the
// accompanying audit event inside a single database transaction, so a failure
// in either step persists neither. The transaction is opened and committed by
// the infrastructure implementation, never by the domain or service layers.
type Repository interface {
	Create(ctx context.Context, params CreateParams) (ServiceRequest, error)
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (ServiceRequest, error)
	Transition(ctx context.Context, event RequestEvent) (ServiceRequest, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]ServiceRequest, error)
}
