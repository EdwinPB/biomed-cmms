package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
	"github.com/edwinpolo/biomed-cmms/api/internal/equipment"
)

type fakeRepo struct {
	listByTenantFn func(ctx context.Context, tenantID uuid.UUID) ([]equipment.Equipment, error)
	lastTenantID   uuid.UUID
}

func (f *fakeRepo) Create(context.Context, equipment.CreateParams) (equipment.Equipment, error) {
	return equipment.Equipment{}, errors.New("fakeRepo: Create not configured")
}

func (f *fakeRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (equipment.Equipment, error) {
	return equipment.Equipment{}, errors.New("fakeRepo: GetByID not configured")
}

func (f *fakeRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]equipment.Equipment, error) {
	f.lastTenantID = tenantID
	if f.listByTenantFn != nil {
		return f.listByTenantFn(ctx, tenantID)
	}
	return nil, errors.New("fakeRepo: ListByTenant not configured")
}

func TestListEquipmentForwardsTenant(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	want := []equipment.Equipment{
		{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), TenantID: tenantID, AssetTag: "DEV-001", Name: "Infusion Pump"},
		{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), TenantID: tenantID, AssetTag: "DEV-002", Name: "MRI Scanner"},
	}
	fake := &fakeRepo{listByTenantFn: func(_ context.Context, tenantID uuid.UUID) ([]equipment.Equipment, error) {
		return want, nil
	}}
	svc := New(fake)

	got, err := svc.ListEquipment(context.Background(), tenantID, auth.RoleAdmin)
	if err != nil {
		t.Fatalf("ListEquipment() error = %v", err)
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ListEquipment() = %+v, want %+v", got, want)
	}
	if fake.lastTenantID != tenantID {
		t.Errorf("ListByTenant() tenant = %v, want %v", fake.lastTenantID, tenantID)
	}
}

func TestListEquipmentForwardsEmptySlice(t *testing.T) {
	fake := &fakeRepo{listByTenantFn: func(context.Context, uuid.UUID) ([]equipment.Equipment, error) {
		return []equipment.Equipment{}, nil
	}}
	svc := New(fake)

	got, err := svc.ListEquipment(context.Background(), uuid.New(), auth.RoleBiomedic)
	if err != nil {
		t.Fatalf("ListEquipment() error = %v", err)
	}
	if got == nil {
		t.Error("ListEquipment() returned nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("ListEquipment() returned %d items, want 0", len(got))
	}
}

func TestListEquipmentRepoErrorPropagated(t *testing.T) {
	wantErr := errors.New("connection reset")
	fake := &fakeRepo{listByTenantFn: func(context.Context, uuid.UUID) ([]equipment.Equipment, error) {
		return nil, wantErr
	}}
	svc := New(fake)

	_, err := svc.ListEquipment(context.Background(), uuid.New(), auth.RoleAdmin)
	if !errors.Is(err, wantErr) {
		t.Errorf("ListEquipment() error = %v, want %v", err, wantErr)
	}
}

func TestListEquipmentRequesterForbidden(t *testing.T) {
	fake := &fakeRepo{
		listByTenantFn: func(context.Context, uuid.UUID) ([]equipment.Equipment, error) {
			t.Fatal("ListByTenant() must not be called for requester")
			return nil, nil
		},
	}
	svc := New(fake)

	_, err := svc.ListEquipment(context.Background(), uuid.New(), auth.RoleRequester)
	if !errors.Is(err, equipment.ErrForbidden) {
		t.Fatalf("ListEquipment() error = %v, want ErrForbidden", err)
	}
	if fake.lastTenantID != uuid.Nil {
		t.Errorf("ListByTenant() called with tenant %v, want no call", fake.lastTenantID)
	}
}

func TestListEquipmentAdminAndBiomedicAllowed(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleAdmin, auth.RoleBiomedic} {
		t.Run(string(role), func(t *testing.T) {
			fake := &fakeRepo{listByTenantFn: func(context.Context, uuid.UUID) ([]equipment.Equipment, error) {
				return []equipment.Equipment{{ID: uuid.New(), TenantID: uuid.New()}}, nil
			}}
			svc := New(fake)

			got, err := svc.ListEquipment(context.Background(), uuid.New(), role)
			if err != nil {
				t.Fatalf("ListEquipment() error = %v, want nil", err)
			}
			if len(got) != 1 {
				t.Errorf("ListEquipment() returned %d items, want 1", len(got))
			}
		})
	}
}

func TestListSelectableRequesterAllowed(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	want := []equipment.Equipment{
		{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), TenantID: tenantID, AssetTag: "DEV-001", Name: "Infusion Pump"},
		{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), TenantID: tenantID, AssetTag: "DEV-002", Name: "MRI Scanner"},
	}
	fake := &fakeRepo{listByTenantFn: func(_ context.Context, tenantID uuid.UUID) ([]equipment.Equipment, error) {
		return want, nil
	}}
	svc := New(fake)

	got, err := svc.ListSelectable(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("ListSelectable() error = %v", err)
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ListSelectable() = %+v, want %+v", got, want)
	}
	if fake.lastTenantID != tenantID {
		t.Errorf("ListByTenant() tenant = %v, want %v", fake.lastTenantID, tenantID)
	}
}

func TestListSelectableForwardsEmptySlice(t *testing.T) {
	fake := &fakeRepo{listByTenantFn: func(context.Context, uuid.UUID) ([]equipment.Equipment, error) {
		return []equipment.Equipment{}, nil
	}}
	svc := New(fake)

	got, err := svc.ListSelectable(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("ListSelectable() error = %v", err)
	}
	if got == nil {
		t.Error("ListSelectable() returned nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("ListSelectable() returned %d items, want 0", len(got))
	}
}

func TestListSelectableRepoErrorPropagated(t *testing.T) {
	wantErr := errors.New("connection reset")
	fake := &fakeRepo{listByTenantFn: func(context.Context, uuid.UUID) ([]equipment.Equipment, error) {
		return nil, wantErr
	}}
	svc := New(fake)

	_, err := svc.ListSelectable(context.Background(), uuid.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("ListSelectable() error = %v, want %v", err, wantErr)
	}
}
