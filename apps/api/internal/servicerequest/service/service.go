// Package service implements service-request application/use-case logic.
//
// It depends only on the servicerequest domain boundary (the Repository
// interface and domain rules), never on PostgreSQL or any infrastructure
// package.
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/servicerequest"
)

var (
	ErrTenantRequired      = errors.New("service request: tenant is required")
	ErrEquipmentRequired   = errors.New("service request: equipment is required")
	ErrCreatedByRequired   = errors.New("service request: created_by is required")
	ErrTitleRequired       = errors.New("service request: title is required")
	ErrDescriptionRequired = errors.New("service request: description is required")
	ErrInvalidPriority     = errors.New("service request: invalid priority")
)

type Service struct {
	repo servicerequest.Repository
}

func New(repo servicerequest.Repository) *Service {
	return &Service{repo: repo}
}

// CreateRequest validates the request parameters and persists a new request.
func (s *Service) CreateRequest(ctx context.Context, params servicerequest.CreateParams) (servicerequest.ServiceRequest, error) {
	if err := validateCreate(params); err != nil {
		return servicerequest.ServiceRequest{}, err
	}
	return s.repo.Create(ctx, params)
}

// TransitionRequest loads the request tenant-scoped, validates the status
// change against the domain transition rules, and persists the new status and
// its audit event atomically.
//
// The atomic status update + event insert happens inside a single database
// transaction owned by the repository (Repository.Transition); this service
// never touches transactions.
func (s *Service) TransitionRequest(ctx context.Context, tenantID, id, actorID uuid.UUID, to servicerequest.Status) (servicerequest.ServiceRequest, error) {
	sr, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return servicerequest.ServiceRequest{}, err
	}

	event, err := sr.TransitionTo(to, actorID)
	if err != nil {
		return servicerequest.ServiceRequest{}, err
	}

	return s.repo.Transition(ctx, event)
}

// RequestHistory returns the audit trail for a request, oldest first. Tenant
// scoping is delegated to the repository boundary; the read path adds no
// business logic.
func (s *Service) RequestHistory(ctx context.Context, tenantID, requestID uuid.UUID) ([]servicerequest.RequestEvent, error) {
	return s.repo.ListEvents(ctx, tenantID, requestID)
}

// GetRequest returns a single request scoped to the tenant. Missing rows (or
// rows owned by another tenant) surface as servicerequest.ErrNotFound.
func (s *Service) GetRequest(ctx context.Context, tenantID, id uuid.UUID) (servicerequest.ServiceRequest, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

// ListRequests returns the tenant's requests, newest first. The read path
// adds no business logic; tenant scoping is delegated to the repository.
func (s *Service) ListRequests(ctx context.Context, tenantID uuid.UUID) ([]servicerequest.ServiceRequest, error) {
	return s.repo.ListByTenant(ctx, tenantID)
}

func validateCreate(params servicerequest.CreateParams) error {
	var errs []error
	if params.TenantID == uuid.Nil {
		errs = append(errs, ErrTenantRequired)
	}
	if params.EquipmentID == uuid.Nil {
		errs = append(errs, ErrEquipmentRequired)
	}
	if params.CreatedBy == uuid.Nil {
		errs = append(errs, ErrCreatedByRequired)
	}
	if strings.TrimSpace(params.Title) == "" {
		errs = append(errs, ErrTitleRequired)
	}
	if strings.TrimSpace(params.Description) == "" {
		errs = append(errs, ErrDescriptionRequired)
	}
	if !validPriority(params.Priority) {
		errs = append(errs, ErrInvalidPriority)
	}
	return errors.Join(errs...)
}

func validPriority(p servicerequest.Priority) bool {
	switch p {
	case "", servicerequest.PriorityLow, servicerequest.PriorityMedium,
		servicerequest.PriorityHigh, servicerequest.PriorityCritical:
		return true
	}
	return false
}
