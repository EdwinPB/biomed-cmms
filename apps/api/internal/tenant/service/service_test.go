package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

type fakeRepo struct {
	createFn func(ctx context.Context, params tenant.CreateParams) (tenant.Tenant, error)
}

func (f *fakeRepo) Create(ctx context.Context, params tenant.CreateParams) (tenant.Tenant, error) {
	if f.createFn != nil {
		return f.createFn(ctx, params)
	}
	return tenant.Tenant{}, errors.New("fakeRepo: Create not configured")
}

func (f *fakeRepo) GetByID(context.Context, uuid.UUID) (tenant.Tenant, error) {
	return tenant.Tenant{}, tenant.ErrNotFound
}

func (f *fakeRepo) GetBySlug(context.Context, string) (tenant.Tenant, error) {
	return tenant.Tenant{}, tenant.ErrNotFound
}

func TestCreateTenantSuccess(t *testing.T) {
	var gotParams tenant.CreateParams
	created := tenant.Tenant{
		ID:        uuid.MustParse("9c5b7f0e-2b2c-4f1e-8a2f-5e0c1a2b3c4d"),
		Slug:      "acme-health",
		Name:      "Acme Health",
		Status:    tenant.StatusActive,
		CreatedAt: mustTime("2026-08-11T00:00:00Z"),
		UpdatedAt: mustTime("2026-08-11T00:00:00Z"),
	}

	fake := &fakeRepo{createFn: func(_ context.Context, params tenant.CreateParams) (tenant.Tenant, error) {
		gotParams = params
		return created, nil
	}}
	svc := New(fake)

	got, err := svc.CreateTenant(context.Background(), tenant.CreateParams{Slug: "acme-health", Name: "Acme Health"})
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if got != created {
		t.Errorf("CreateTenant() = %+v, want %+v", got, created)
	}
	if gotParams.Slug != "acme-health" || gotParams.Name != "Acme Health" {
		t.Errorf("CreateTenant() repo params = %+v", gotParams)
	}
}

func TestCreateTenantMissingSlug(t *testing.T) {
	called := false
	fake := &fakeRepo{createFn: func(context.Context, tenant.CreateParams) (tenant.Tenant, error) {
		called = true
		return tenant.Tenant{}, nil
	}}
	svc := New(fake)

	_, err := svc.CreateTenant(context.Background(), tenant.CreateParams{Slug: "", Name: "Acme Health"})
	if !errors.Is(err, ErrSlugRequired) {
		t.Errorf("CreateTenant() error = %v, want ErrSlugRequired", err)
	}
	if called {
		t.Error("CreateTenant() called repo despite invalid input")
	}
}

func TestCreateTenantMissingName(t *testing.T) {
	called := false
	fake := &fakeRepo{createFn: func(context.Context, tenant.CreateParams) (tenant.Tenant, error) {
		called = true
		return tenant.Tenant{}, nil
	}}
	svc := New(fake)

	_, err := svc.CreateTenant(context.Background(), tenant.CreateParams{Slug: "acme-health", Name: ""})
	if !errors.Is(err, ErrNameRequired) {
		t.Errorf("CreateTenant() error = %v, want ErrNameRequired", err)
	}
	if called {
		t.Error("CreateTenant() called repo despite invalid input")
	}
}

func TestCreateTenantMissingAll(t *testing.T) {
	fake := &fakeRepo{}
	svc := New(fake)

	_, err := svc.CreateTenant(context.Background(), tenant.CreateParams{})
	if !errors.Is(err, ErrSlugRequired) {
		t.Errorf("CreateTenant() error = %v, want ErrSlugRequired", err)
	}
	if !errors.Is(err, ErrNameRequired) {
		t.Errorf("CreateTenant() error = %v, want ErrNameRequired", err)
	}
}

func TestCreateTenantWhitespaceOnlyRejected(t *testing.T) {
	called := false
	fake := &fakeRepo{createFn: func(context.Context, tenant.CreateParams) (tenant.Tenant, error) {
		called = true
		return tenant.Tenant{}, nil
	}}
	svc := New(fake)

	_, err := svc.CreateTenant(context.Background(), tenant.CreateParams{Slug: "   ", Name: "  "})
	if !errors.Is(err, ErrSlugRequired) || !errors.Is(err, ErrNameRequired) {
		t.Errorf("CreateTenant() error = %v, want both validation errors", err)
	}
	if called {
		t.Error("CreateTenant() called repo despite invalid input")
	}
}

func TestCreateTenantPropagatesRepoError(t *testing.T) {
	fake := &fakeRepo{createFn: func(context.Context, tenant.CreateParams) (tenant.Tenant, error) {
		return tenant.Tenant{}, tenant.ErrConflict
	}}
	svc := New(fake)

	_, err := svc.CreateTenant(context.Background(), tenant.CreateParams{Slug: "acme-health", Name: "Acme Health"})
	if !errors.Is(err, tenant.ErrConflict) {
		t.Errorf("CreateTenant() error = %v, want tenant.ErrConflict", err)
	}
}
