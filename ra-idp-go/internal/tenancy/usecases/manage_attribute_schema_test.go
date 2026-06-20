package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/spec"
)

func strp(s string) *string { return &s }

func TestGetAttributeSchemaReturnsEmptyForUndefinedTenant(t *testing.T) {
	repo := memory.NewTenantAttributeSchemaRepository()
	schema, err := GetAttributeSchema(context.Background(), repo, spec.DefaultTenantID)
	if err != nil {
		t.Fatal(err)
	}
	if schema == nil || schema.TenantID != spec.DefaultTenantID || len(schema.Attributes) != 0 {
		t.Fatalf("expected empty schema, got %#v", schema)
	}
}

func TestUpdateAttributeSchemaPersistsCustomDefs(t *testing.T) {
	repo := memory.NewTenantAttributeSchemaRepository()
	ctx := context.Background()
	defs := []spec.AttributeDef{
		{Key: "region", Type: spec.AttributeTypeString, Visibility: spec.AttrVisibilityClaimExposed, ClaimName: strp("region")},
	}
	saved, err := UpdateAttributeSchema(ctx, repo, spec.DefaultTenantID, defs, time.Now().UTC())
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if len(saved.Attributes) != 1 || saved.Attributes[0].Key != "region" {
		t.Fatalf("unexpected saved schema: %#v", saved)
	}
	reloaded, err := GetAttributeSchema(ctx, repo, spec.DefaultTenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Attributes) != 1 || reloaded.Attributes[0].Key != "region" {
		t.Fatalf("schema not persisted: %#v", reloaded)
	}
}

func TestUpdateAttributeSchemaRejectsBuiltinCollision(t *testing.T) {
	repo := memory.NewTenantAttributeSchemaRepository()
	defs := []spec.AttributeDef{
		{Key: "nickname", Type: spec.AttributeTypeString, Visibility: spec.AttrVisibilityClaimExposed},
	}
	if _, err := UpdateAttributeSchema(
		context.Background(), repo, spec.DefaultTenantID, defs, time.Now().UTC(),
	); !errors.Is(err, ErrInvalidAttributeSchema) {
		t.Fatalf("expected ErrInvalidAttributeSchema, got %v", err)
	}
}

func TestUpdateAttributeSchemaAllowsEmptyClear(t *testing.T) {
	repo := memory.NewTenantAttributeSchemaRepository()
	ctx := context.Background()
	if _, err := UpdateAttributeSchema(ctx, repo, spec.DefaultTenantID,
		[]spec.AttributeDef{{Key: "region", Type: spec.AttributeTypeString, Visibility: spec.AttrVisibilityAdminReadable}},
		time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	cleared, err := UpdateAttributeSchema(ctx, repo, spec.DefaultTenantID, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if len(cleared.Attributes) != 0 {
		t.Fatalf("expected cleared schema, got %#v", cleared)
	}
}
