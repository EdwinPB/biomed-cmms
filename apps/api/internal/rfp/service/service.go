// Package service implements RFP application/use-case logic.
//
// It depends only on the rfp domain boundary (the Repository interface and
// domain rules), never on PostgreSQL or any infrastructure package.
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
	"github.com/edwinpolo/biomed-cmms/api/internal/rfp"
)

var (
	ErrTenantRequired         = errors.New("rfp: tenant is required")
	ErrServiceRequestRequired = errors.New("rfp: service request is required")
	ErrCreatedByRequired      = errors.New("rfp: created_by is required")
	ErrTitleRequired          = errors.New("rfp: title is required")
	ErrDescriptionRequired    = errors.New("rfp: description is required")
	ErrInvalidStatus          = errors.New("rfp: invalid status")
)

type Service struct {
	repo rfp.Repository
}

func New(repo rfp.Repository) *Service {
	return &Service{repo: repo}
}

// requireStaff rejects requesters before any validation or repository access.
// Admin and biomedic callers pass through unchanged.
func requireStaff(role auth.Role) error {
	if role == auth.RoleRequester {
		return rfp.ErrForbidden
	}
	return nil
}

// CreateRFP validates the request parameters and persists a new RFP.
func (s *Service) CreateRFP(ctx context.Context, params rfp.CreateParams, role auth.Role) (rfp.RFP, error) {
	if err := requireStaff(role); err != nil {
		return rfp.RFP{}, err
	}
	if err := validateCreate(params); err != nil {
		return rfp.RFP{}, err
	}
	return s.repo.Create(ctx, params)
}

// TransitionRFP loads the RFP tenant-scoped, validates the status change
// (including publishing preconditions when the target is published), and
// persists the new status.
func (s *Service) TransitionRFP(ctx context.Context, tenantID, id uuid.UUID, role auth.Role, to rfp.Status) (rfp.RFP, error) {
	if err := requireStaff(role); err != nil {
		return rfp.RFP{}, err
	}

	current, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return rfp.RFP{}, err
	}

	from := current.Status
	if err := current.TransitionTo(to); err != nil {
		return rfp.RFP{}, err
	}

	return s.repo.Transition(ctx, tenantID, id, from, to)
}

// GetRFP returns a single RFP, tenant-scoped.
func (s *Service) GetRFP(ctx context.Context, tenantID, id uuid.UUID, role auth.Role) (rfp.RFP, error) {
	if err := requireStaff(role); err != nil {
		return rfp.RFP{}, err
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// GetRFPByServiceRequest returns the current/latest RFP for a service request,
// tenant-scoped.
func (s *Service) GetRFPByServiceRequest(ctx context.Context, tenantID, serviceRequestID uuid.UUID, role auth.Role) (rfp.RFP, error) {
	if err := requireStaff(role); err != nil {
		return rfp.RFP{}, err
	}
	return s.repo.GetByServiceRequest(ctx, tenantID, serviceRequestID)
}

func validateCreate(params rfp.CreateParams) error {
	var errs []error
	if params.TenantID == uuid.Nil {
		errs = append(errs, ErrTenantRequired)
	}
	if params.ServiceRequestID == uuid.Nil {
		errs = append(errs, ErrServiceRequestRequired)
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
	if !validStatus(params.Status) {
		errs = append(errs, ErrInvalidStatus)
	}
	return errors.Join(errs...)
}

func validStatus(s rfp.Status) bool {
	switch s {
	case "", rfp.StatusDraft, rfp.StatusPublished, rfp.StatusClosed, rfp.StatusCancelled:
		return true
	}
	return false
}
