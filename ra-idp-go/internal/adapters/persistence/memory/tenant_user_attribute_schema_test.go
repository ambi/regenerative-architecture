package memory

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/spec"
	tenantports "ra-idp-go/internal/tenancy/ports"
)

// 実装がポートを満たすことをコンパイル時に保証する。
var _ tenantports.TenantUserAttributeSchemaRepository = (*TenantUserAttributeSchemaRepository)(nil)

func TestTenantUserAttributeSchemaRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := NewTenantUserAttributeSchemaRepository()

	if got, err := repo.FindByTenant(ctx, "acme"); err != nil || got != nil {
		t.Fatalf("expected nil schema for unknown tenant, got %v, %v", got, err)
	}

	claim := "region"
	schema := &spec.TenantUserAttributeSchema{
		TenantID: "acme",
		Attributes: []spec.UserAttributeDef{
			{Key: "region", Type: spec.AttributeTypeString, Required: true, Visibility: spec.AttrVisibilityClaimExposed, ClaimName: &claim},
		},
		UpdatedAt: time.Now().UTC(),
	}
	if err := repo.Save(ctx, schema); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	got, err := repo.FindByTenant(ctx, "acme")
	if err != nil || got == nil {
		t.Fatalf("expected stored schema, got %v, %v", got, err)
	}
	if len(got.Attributes) != 1 || got.Attributes[0].Key != "region" {
		t.Fatalf("unexpected schema: %+v", got)
	}

	// 返却値の変更が保管値に波及しないこと (深いコピー)。
	got.Attributes[0].Key = "mutated"
	again, _ := repo.FindByTenant(ctx, "acme")
	if again.Attributes[0].Key != "region" {
		t.Fatal("stored schema must not alias returned slice")
	}

	if err := repo.Delete(ctx, "acme"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if got, _ := repo.FindByTenant(ctx, "acme"); got != nil {
		t.Fatal("expected schema removed after delete")
	}
}

func TestTenantUserAttributeSchemaRepositoryDefaultsTenant(t *testing.T) {
	ctx := context.Background()
	repo := NewTenantUserAttributeSchemaRepository()
	if err := repo.Save(ctx, &spec.TenantUserAttributeSchema{UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := repo.FindByTenant(ctx, "")
	if err != nil || got == nil {
		t.Fatalf("expected default-tenant schema, got %v, %v", got, err)
	}
	if got.TenantID != spec.DefaultTenantID {
		t.Fatalf("expected tenant_id %q, got %q", spec.DefaultTenantID, got.TenantID)
	}
}
