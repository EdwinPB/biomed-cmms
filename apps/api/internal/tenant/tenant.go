// Package tenant defines the Tenant domain entity and the repository boundary.
//
// This package is database-agnostic: it must not import pgx, contain SQL, or
// depend on any infrastructure package.
package tenant

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
	StatusArchived  Status = "archived"
)

var (
	ErrNotFound = errors.New("tenant: not found")
	ErrConflict = errors.New("tenant: already exists")
)

type Tenant struct {
	ID        uuid.UUID
	Slug      string
	Name      string
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateParams struct {
	Slug string
	Name string
}
