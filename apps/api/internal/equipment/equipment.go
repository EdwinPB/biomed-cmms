// Package equipment defines the Equipment domain entity and the repository
// boundary.
//
// This package is database-agnostic: it must not import pgx, contain SQL, or
// depend on any infrastructure package.
package equipment

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusOperational Status = "operational"
	StatusMaintenance Status = "maintenance"
	StatusRetired     Status = "retired"
)

var (
	ErrNotFound  = errors.New("equipment: not found")
	ErrConflict  = errors.New("equipment: already exists")
	ErrForbidden = errors.New("equipment: forbidden")
)

type Equipment struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	AssetTag     string
	Name         string
	SerialNumber string
	Location     string
	Status       Status
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateParams struct {
	TenantID     uuid.UUID
	AssetTag     string
	Name         string
	SerialNumber string
	Location     string
	Status       Status
}
