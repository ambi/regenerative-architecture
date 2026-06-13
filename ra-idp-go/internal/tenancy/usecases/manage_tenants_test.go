package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/spec"
)

func TestEnsureDefaultAndRejectDefaultDisable(t *testing.T) {
	repo := memory.NewTenantRepository()
	now := time.Now().UTC()
	if err := EnsureDefault(context.Background(), repo, now); err != nil {
		t.Fatal(err)
	}
	tenant, err := repo.FindByID(context.Background(), spec.DefaultTenantID)
	if err != nil {
		t.Fatal(err)
	}
	if tenant == nil || tenant.Status != spec.TenantStatusActive {
		t.Fatalf("default tenant = %#v", tenant)
	}
	if _, err := SetDisabled(
		context.Background(), repo, spec.DefaultTenantID, true, now,
	); !errors.Is(err, ErrDefaultTenant) {
		t.Fatalf("disable default error = %v", err)
	}
}

func TestTenantLifecycle(t *testing.T) {
	repo := memory.NewTenantRepository()
	tenant, err := Create(context.Background(), repo, "acme", "Acme", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if tenant.Status != spec.TenantStatusActive {
		t.Fatalf("status = %s", tenant.Status)
	}
	tenant, err = SetDisabled(context.Background(), repo, "acme", true, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if tenant.Status != spec.TenantStatusDisabled || tenant.DisabledAt == nil {
		t.Fatalf("disabled tenant = %#v", tenant)
	}
	tenant, err = SetDisabled(context.Background(), repo, "acme", false, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if tenant.Status != spec.TenantStatusActive || tenant.DisabledAt != nil {
		t.Fatalf("enabled tenant = %#v", tenant)
	}
}
