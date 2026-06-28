package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	idmusecases "ra-idp-go/internal/identitymanagement/usecases"
	"ra-idp-go/internal/shared/adapters/crypto"
	"ra-idp-go/internal/shared/adapters/persistence/memory"
	"ra-idp-go/internal/shared/spec"
)

func strp(s string) *string { return &s }

func attrTestDeps(t *testing.T) (context.Context, idmusecases.AdminUserDeps, *memory.TenantUserAttributeSchemaRepository) {
	t.Helper()
	schemaRepo := memory.NewTenantUserAttributeSchemaRepository()
	deps := idmusecases.AdminUserDeps{
		UserRepo:            memory.NewUserRepository(),
		AttrSchemaRepo:      schemaRepo,
		PasswordHasher:      crypto.NewArgon2idPasswordHasher(),
		PasswordHistoryRepo: memory.NewPasswordHistoryRepository(),
		Emit:                func(spec.DomainEvent) {},
	}
	return context.Background(), deps, schemaRepo
}

func createAttrUser(ctx context.Context, t *testing.T, deps idmusecases.AdminUserDeps) *spec.User {
	t.Helper()
	user, err := idmusecases.CreateUser(ctx, deps, idmusecases.CreateUserInput{
		ActorSub: "admin", PreferredUsername: "carol", Password: "initial-password-9182",
		Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func TestUpdateUserAcceptsBuiltinAttribute(t *testing.T) {
	ctx, deps, _ := attrTestDeps(t)
	user := createAttrUser(ctx, t, deps)

	attrs := map[string]spec.AttributeValue{
		"nickname":     {Type: spec.AttributeTypeString, String: strp("cici")},
		"phone_number": {Type: spec.AttributeTypeString, String: strp("+819012345678")},
	}
	updated, err := idmusecases.UpdateUser(ctx, deps, idmusecases.UpdateUserInput{
		ActorSub: "admin", Sub: user.Sub, GivenName: strp("Carol"), Attributes: &attrs,
		Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.GivenName == nil || *updated.GivenName != "Carol" {
		t.Fatalf("given_name not updated: %+v", updated.GivenName)
	}
	if v := updated.Attributes["nickname"]; v.String == nil || *v.String != "cici" {
		t.Fatalf("nickname not stored: %+v", updated.Attributes)
	}
}

func TestUpdateUserRejectsUndefinedAttribute(t *testing.T) {
	ctx, deps, _ := attrTestDeps(t)
	user := createAttrUser(ctx, t, deps)

	attrs := map[string]spec.AttributeValue{
		"not_a_real_attribute": {Type: spec.AttributeTypeString, String: strp("x")},
	}
	_, err := idmusecases.UpdateUser(ctx, deps, idmusecases.UpdateUserInput{
		ActorSub: "admin", Sub: user.Sub, Attributes: &attrs, Now: time.Now().UTC(),
	})
	if !errors.Is(err, idmusecases.ErrInvalidAttribute) {
		t.Fatalf("expected ErrInvalidAttribute, got %v", err)
	}
}

func TestUpdateUserAcceptsTenantCustomAttribute(t *testing.T) {
	ctx, deps, schemaRepo := attrTestDeps(t)
	if err := schemaRepo.Save(ctx, &spec.TenantUserAttributeSchema{
		TenantID: spec.DefaultTenantID,
		Attributes: []spec.UserAttributeDef{
			{Key: "region", Type: spec.AttributeTypeString, Visibility: spec.AttrVisibilityClaimExposed, ClaimName: strp("region")},
		},
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	user := createAttrUser(ctx, t, deps)

	attrs := map[string]spec.AttributeValue{
		"region": {Type: spec.AttributeTypeString, String: strp("apac")},
	}
	updated, err := idmusecases.UpdateUser(ctx, deps, idmusecases.UpdateUserInput{
		ActorSub: "admin", Sub: user.Sub, Attributes: &attrs, Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if v := updated.Attributes["region"]; v.String == nil || *v.String != "apac" {
		t.Fatalf("region not stored: %+v", updated.Attributes)
	}

	// schema 未定義の custom key は拒否される。
	bad := map[string]spec.AttributeValue{"zone": {Type: spec.AttributeTypeString, String: strp("z")}}
	if _, err := idmusecases.UpdateUser(ctx, deps, idmusecases.UpdateUserInput{
		ActorSub: "admin", Sub: user.Sub, Attributes: &bad, Now: time.Now().UTC(),
	}); !errors.Is(err, idmusecases.ErrInvalidAttribute) {
		t.Fatalf("expected ErrInvalidAttribute for undefined custom key, got %v", err)
	}
}
