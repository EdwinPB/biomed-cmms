package rfp

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the persistence boundary for RFPs. Implementations live in
// infrastructure packages. All reads are tenant-scoped, and Create rejects
// cross-tenant service requests or users via composite foreign keys in the
// schema (see migration 000006).
type Repository interface {
	Create(ctx context.Context, params CreateParams) (RFP, error)
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (RFP, error)
	GetByServiceRequest(ctx context.Context, tenantID, serviceRequestID uuid.UUID) (RFP, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]RFP, error)
	// Transition applies a validated status move. The current status (from) is
	// used as an optimistic guard: if the row is missing, belongs to another
	// tenant, or already moved on, ErrNotFound is returned.
	Transition(ctx context.Context, tenantID, id uuid.UUID, from, to Status) (RFP, error)
}
