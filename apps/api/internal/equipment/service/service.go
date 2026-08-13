// Package service implements equipment application/use-case logic.
//
// It depends only on the equipment domain boundary (the Repository interface),
// never on PostgreSQL or any infrastructure package.
package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
	"github.com/edwinpolo/biomed-cmms/api/internal/equipment"
)

type Service struct {
	repo equipment.Repository
}

func New(repo equipment.Repository) *Service {
	return &Service{repo: repo}
}

// ListEquipment returns the tenant's equipment, newest first. Requesters are
// forbidden from viewing equipment; admin and biomedic callers pass through.
// Tenant scoping is delegated to the repository.
func (s *Service) ListEquipment(ctx context.Context, tenantID uuid.UUID, role auth.Role) ([]equipment.Equipment, error) {
	if role == auth.RoleRequester {
		return nil, equipment.ErrForbidden
	}
	return s.repo.ListByTenant(ctx, tenantID)
}

// ListSelectable returns the tenant's equipment for request-creation
// selection. Unlike ListEquipment it is not role-gated: the HTTP layer
// projects the bounded selection view, so requesters never see the full
// staff equipment response. Tenant scoping is delegated to the repository.
func (s *Service) ListSelectable(ctx context.Context, tenantID uuid.UUID) ([]equipment.Equipment, error) {
	return s.repo.ListByTenant(ctx, tenantID)
}
