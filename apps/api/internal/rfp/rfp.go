// Package rfp defines the RFP (request for proposal) domain entity and the
// repository boundary.
//
// An RFP is a procurement request associated with a Service Request; it is a
// separate aggregate, not a Service Request status.
//
// This package is database-agnostic: it must not import pgx, contain SQL, or
// depend on any infrastructure package.
package rfp

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusClosed    Status = "closed"
	StatusCancelled Status = "cancelled"
)

var (
	ErrNotFound = errors.New("rfp: not found")
	ErrConflict = errors.New("rfp: conflict")
)

type RFP struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	ServiceRequestID uuid.UUID
	Title            string
	Description      string
	Status           Status
	DueAt            *time.Time
	CreatedBy        uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreateParams struct {
	TenantID         uuid.UUID
	ServiceRequestID uuid.UUID
	Title            string
	Description      string
	Status           Status
	DueAt            *time.Time
	CreatedBy        uuid.UUID
}
