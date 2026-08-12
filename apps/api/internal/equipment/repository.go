package equipment

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the persistence boundary for equipment. Implementations live in
// infrastructure packages. All lookups are tenant-scoped: a tenant can never
// read another tenant's equipment.
type Repository interface {
	Create(ctx context.Context, params CreateParams) (Equipment, error)
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (Equipment, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]Equipment, error)
}
