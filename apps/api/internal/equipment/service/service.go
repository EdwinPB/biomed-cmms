// Package service implements equipment application/use-case logic.
//
// It depends only on the equipment domain boundary (the Repository interface),
// never on PostgreSQL or any infrastructure package.
package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/equipment"
)

type Service struct {
	repo equipment.Repository
}

func New(repo equipment.Repository) *Service {
	return &Service{repo: repo}
}

// ListEquipment returns the tenant's equipment, newest first. The read path
// adds no business logic; tenant scoping is delegated to the repository.
func (s *Service) ListEquipment(ctx context.Context, tenantID uuid.UUID) ([]equipment.Equipment, error) {
	return s.repo.ListByTenant(ctx, tenantID)
}
