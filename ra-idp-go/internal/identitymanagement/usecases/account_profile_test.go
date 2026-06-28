package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	idmusecases "ra-idp-go/internal/identitymanagement/usecases"
	"ra-idp-go/internal/infrastructure/crypto"
	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/spec"
)

func accountTestDeps(t *testing.T) (context.Context, idmusecases.AccountProfileDeps, *spec.User) {
	t.Helper()
	userRepo := memory.NewUserRepository()
	schemaRepo := memory.NewTenantUserAttributeSchemaRepository()
	adminDeps := idmusecases.AdminUserDeps{
		UserRepo: userRepo, AttrSchemaRepo: schemaRepo,
		PasswordHasher: crypto.NewArgon2idPasswordHasher(), PasswordHistoryRepo: memory.NewPasswordHistoryRepository(),
		Emit: func(spec.DomainEvent) {},
	}
	ctx := context.Background()
	user, err := idmusecases.CreateUser(ctx, adminDeps, idmusecases.CreateUserInput{
		ActorSub: "admin", PreferredUsername: "dave", Password: "initial-password-9182", Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	// admin 管理属性 (organization, editable_by_user=false) を事前に入れておく。
	user.Attributes = map[string]spec.AttributeValue{
		"department": {Type: spec.AttributeTypeString, String: strp("Platform")},
	}
	if err := userRepo.Save(ctx, user); err != nil {
		t.Fatal(err)
	}
	deps := idmusecases.AccountProfileDeps{UserRepo: userRepo, AttrSchemaRepo: schemaRepo, Emit: func(spec.DomainEvent) {}}
	return ctx, deps, user
}

func TestUpdateUserProfileEditsNameAndEditableAttribute(t *testing.T) {
	ctx, deps, user := accountTestDeps(t)
	attrs := map[string]spec.AttributeValue{
		"nickname": {Type: spec.AttributeTypeString, String: strp("davey")},
	}
	updated, _, err := idmusecases.UpdateUserProfile(ctx, deps, idmusecases.UpdateUserProfileInput{
		Sub: user.Sub, GivenName: strp("Dave"), Attributes: &attrs, Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.GivenName == nil || *updated.GivenName != "Dave" {
		t.Fatalf("given_name not updated: %+v", updated.GivenName)
	}
	if v := updated.Attributes["nickname"]; v.String == nil || *v.String != "davey" {
		t.Fatalf("nickname not stored: %+v", updated.Attributes)
	}
	// admin 管理属性 (department) は merge で保持される。
	if v := updated.Attributes["department"]; v.String == nil || *v.String != "Platform" {
		t.Fatalf("admin-managed attribute lost on self merge: %+v", updated.Attributes)
	}
}

func TestUpdateUserProfileRejectsAdminManagedAttribute(t *testing.T) {
	ctx, deps, user := accountTestDeps(t)
	attrs := map[string]spec.AttributeValue{
		"department": {Type: spec.AttributeTypeString, String: strp("Sales")}, // editable_by_user=false
	}
	_, _, err := idmusecases.UpdateUserProfile(ctx, deps, idmusecases.UpdateUserProfileInput{
		Sub: user.Sub, Attributes: &attrs, Now: time.Now().UTC(),
	})
	if !errors.Is(err, idmusecases.ErrAttributeNotEditable) {
		t.Fatalf("expected ErrAttributeNotEditable, got %v", err)
	}
}

func TestUpdateUserProfileRejectsUndefinedAttribute(t *testing.T) {
	ctx, deps, user := accountTestDeps(t)
	attrs := map[string]spec.AttributeValue{
		"not_a_real_attribute": {Type: spec.AttributeTypeString, String: strp("x")},
	}
	_, _, err := idmusecases.UpdateUserProfile(ctx, deps, idmusecases.UpdateUserProfileInput{
		Sub: user.Sub, Attributes: &attrs, Now: time.Now().UTC(),
	})
	if !errors.Is(err, idmusecases.ErrInvalidAttribute) {
		t.Fatalf("expected ErrInvalidAttribute, got %v", err)
	}
}

func TestGetUserProfileShowsReadOnlyOrganizationAttributes(t *testing.T) {
	ctx, deps, user := accountTestDeps(t)
	_, defs, err := idmusecases.GetUserProfile(ctx, deps, user.Sub)
	if err != nil {
		t.Fatal(err)
	}
	// department は本人が参照できるが、self-service では編集できない。
	self := idmusecases.SelfReadableAttributes(user.Attributes, defs)
	if v, ok := self["department"]; !ok || v.String == nil || *v.String != "Platform" {
		t.Fatalf("self_readable organization attribute missing: %+v", self)
	}
}
