// Package service implements tenant application/use-case logic.
//
// It depends only on the tenant domain boundary (tenant.Repository), never on
// PostgreSQL or any infrastructure package.
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

var (
	ErrSlugRequired = errors.New("slug is required")
	ErrNameRequired = errors.New("name is required")
)

type Service struct {
	repo tenant.Repository
}

func New(repo tenant.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateTenant(ctx context.Context, params tenant.CreateParams) (tenant.Tenant, error) {
	var errs []error
	if strings.TrimSpace(params.Slug) == "" {
		errs = append(errs, ErrSlugRequired)
	}
	if strings.TrimSpace(params.Name) == "" {
		errs = append(errs, ErrNameRequired)
	}
	if len(errs) > 0 {
		return tenant.Tenant{}, errors.Join(errs...)
	}

	return s.repo.Create(ctx, params)
}
