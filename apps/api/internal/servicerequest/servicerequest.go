// Package servicerequest defines the ServiceRequest domain entity and the
// repository boundary.
//
// This package is database-agnostic: it must not import pgx, contain SQL, or
// depend on any infrastructure package.
package servicerequest

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusAssigned   Status = "assigned"
	StatusInProgress Status = "in_progress"
	StatusResolved   Status = "resolved"
	StatusCancelled  Status = "cancelled"
)

var ErrNotFound = errors.New("service request: not found")

// RequestEvent is an audit record of a status transition on a service request.
type RequestEvent struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	RequestID  uuid.UUID
	ActorID    uuid.UUID
	FromStatus Status
	ToStatus   Status
	CreatedAt  time.Time
}

type ServiceRequest struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	EquipmentID     uuid.UUID
	Title           string
	Description     string
	Priority        Priority
	Status          Status
	CreatedBy       uuid.UUID
	AssignedTo      *uuid.UUID
	ResolutionNotes *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateParams struct {
	TenantID    uuid.UUID
	EquipmentID uuid.UUID
	Title       string
	Description string
	Priority    Priority
	Status      Status
	CreatedBy   uuid.UUID
	AssignedTo  *uuid.UUID
}
