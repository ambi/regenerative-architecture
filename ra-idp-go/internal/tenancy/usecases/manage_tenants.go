package usecases

import (
	"context"
	"errors"
	"strings"
	"time"

	"ra-idp-go/internal/spec"
	tenantports "ra-idp-go/internal/tenancy/ports"
)

var (
	ErrTenantNotFound   = errors.New("tenant not found")
	ErrTenantConflict   = errors.New("tenant already exists")
	ErrInvalidTenantID  = errors.New("invalid tenant id")
	ErrDefaultTenant    = errors.New("default tenant cannot be disabled")
	ErrDisplayNameEmpty = errors.New("display name is required")
)

func EnsureDefault(ctx context.Context, repo tenantports.TenantRepository, now time.Time) error {
	tenant, err := repo.FindByID(ctx, spec.DefaultTenantID)
	if err != nil {
		return err
	}
	if tenant != nil {
		return nil
	}
	now = normalizeNow(now)
	return repo.Save(ctx, &spec.Tenant{
		ID: spec.DefaultTenantID, DisplayName: "Default", Status: spec.TenantStatusActive, CreatedAt: now,
	})
}

func Create(ctx context.Context, repo tenantports.TenantRepository, id, displayName string, now time.Time) (*spec.Tenant, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, ErrDisplayNameEmpty
	}
	tenant := &spec.Tenant{
		ID: strings.TrimSpace(id), DisplayName: displayName, Status: spec.TenantStatusActive,
		CreatedAt: normalizeNow(now),
	}
	if err := tenant.Validate(); err != nil {
		return nil, ErrInvalidTenantID
	}
	existing, err := repo.FindByID(ctx, tenant.ID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrTenantConflict
	}
	if err := repo.Save(ctx, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

func Update(ctx context.Context, repo tenantports.TenantRepository, id, displayName string, now time.Time) (*spec.Tenant, error) {
	tenant, err := repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, ErrTenantNotFound
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, ErrDisplayNameEmpty
	}
	updated := *tenant
	updated.DisplayName = displayName
	t := normalizeNow(now)
	updated.UpdatedAt = &t
	if err := repo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

func SetDisabled(ctx context.Context, repo tenantports.TenantRepository, id string, disabled bool, now time.Time) (*spec.Tenant, error) {
	if id == spec.DefaultTenantID && disabled {
		return nil, ErrDefaultTenant
	}
	tenant, err := repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, ErrTenantNotFound
	}
	updated := *tenant
	t := normalizeNow(now)
	updated.UpdatedAt = &t
	if disabled {
		updated.Status = spec.TenantStatusDisabled
		updated.DisabledAt = &t
	} else {
		updated.Status = spec.TenantStatusActive
		updated.DisabledAt = nil
	}
	if err := repo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	return &updated, nil
}

func normalizeNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}
