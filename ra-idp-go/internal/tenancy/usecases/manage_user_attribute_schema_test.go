package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/shared/adapters/persistence/memory"
	"ra-idp-go/internal/shared/spec"
)

func strp(s string) *string { return &s }

func TestGetUserAttributeSchemaReturnsEmptyForUndefinedTenant(t *testing.T) {
	repo := memory.NewTenantUserAttributeSchemaRepository()
	schema, err := GetUserAttributeSchema(context.Background(), repo, spec.DefaultTenantID)
	if err != nil {
		t.Fatal(err)
	}
	if schema == nil || schema.TenantID != spec.DefaultTenantID || len(schema.Attributes) != 0 {
		t.Fatalf("expected empty schema, got %#v", schema)
	}
}

func TestUpdateUserAttributeSchemaPersistsCustomDefs(t *testing.T) {
	repo := memory.NewTenantUserAttributeSchemaRepository()
	ctx := context.Background()
	defs := []spec.UserAttributeDef{
		{Key: "region", Type: spec.AttributeTypeString, Visibility: spec.AttrVisibilityClaimExposed, ClaimName: strp("region")},
	}
	saved, err := UpdateUserAttributeSchema(ctx, repo, spec.DefaultTenantID, defs, time.Now().UTC())
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if len(saved.Attributes) != 1 || saved.Attributes[0].Key != "region" {
		t.Fatalf("unexpected saved schema: %#v", saved)
	}
	reloaded, err := GetUserAttributeSchema(ctx, repo, spec.DefaultTenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Attributes) != 1 || reloaded.Attributes[0].Key != "region" {
		t.Fatalf("schema not persisted: %#v", reloaded)
	}
}

func TestUpdateUserAttributeSchemaRejectsBuiltinCollision(t *testing.T) {
	repo := memory.NewTenantUserAttributeSchemaRepository()
	defs := []spec.UserAttributeDef{
		{Key: "nickname", Type: spec.AttributeTypeString, Visibility: spec.AttrVisibilityClaimExposed},
	}
	if _, err := UpdateUserAttributeSchema(
		context.Background(), repo, spec.DefaultTenantID, defs, time.Now().UTC(),
	); !errors.Is(err, ErrInvalidUserAttributeSchema) {
		t.Fatalf("expected ErrInvalidUserAttributeSchema, got %v", err)
	}
}

func TestUpdateUserAttributeSchemaAllowsEmptyClear(t *testing.T) {
	repo := memory.NewTenantUserAttributeSchemaRepository()
	ctx := context.Background()
	if _, err := UpdateUserAttributeSchema(ctx, repo, spec.DefaultTenantID,
		[]spec.UserAttributeDef{{Key: "region", Type: spec.AttributeTypeString, Visibility: spec.AttrVisibilityAdminReadable}},
		time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	cleared, err := UpdateUserAttributeSchema(ctx, repo, spec.DefaultTenantID, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if len(cleared.Attributes) != 0 {
		t.Fatalf("expected cleared schema, got %#v", cleared)
	}
}
