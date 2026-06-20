package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/crypto"
	"ra-idp-go/internal/adapters/persistence/memory"
	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/spec"
)

func accountTestDeps(t *testing.T) (context.Context, authusecases.AccountProfileDeps, *spec.User) {
	t.Helper()
	userRepo := memory.NewUserRepository()
	schemaRepo := memory.NewTenantUserAttributeSchemaRepository()
	adminDeps := authusecases.AdminUserDeps{
		UserRepo: userRepo, AttrSchemaRepo: schemaRepo,
		PasswordHasher: crypto.NewArgon2idPasswordHasher(), PasswordHistoryRepo: memory.NewPasswordHistoryRepository(),
		Emit: func(spec.DomainEvent) {},
	}
	ctx := context.Background()
	user, err := authusecases.CreateUser(ctx, adminDeps, authusecases.CreateUserInput{
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
	deps := authusecases.AccountProfileDeps{UserRepo: userRepo, AttrSchemaRepo: schemaRepo, Emit: func(spec.DomainEvent) {}}
	return ctx, deps, user
}

func TestUpdateUserProfileEditsNameAndEditableAttribute(t *testing.T) {
	ctx, deps, user := accountTestDeps(t)
	attrs := map[string]spec.AttributeValue{
		"nickname": {Type: spec.AttributeTypeString, String: strp("davey")},
	}
	updated, _, err := authusecases.UpdateUserProfile(ctx, deps, authusecases.UpdateUserProfileInput{
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
	_, _, err := authusecases.UpdateUserProfile(ctx, deps, authusecases.UpdateUserProfileInput{
		Sub: user.Sub, Attributes: &attrs, Now: time.Now().UTC(),
	})
	if !errors.Is(err, authusecases.ErrAttributeNotEditable) {
		t.Fatalf("expected ErrAttributeNotEditable, got %v", err)
	}
}

func TestUpdateUserProfileRejectsUndefinedAttribute(t *testing.T) {
	ctx, deps, user := accountTestDeps(t)
	attrs := map[string]spec.AttributeValue{
		"not_a_real_attribute": {Type: spec.AttributeTypeString, String: strp("x")},
	}
	_, _, err := authusecases.UpdateUserProfile(ctx, deps, authusecases.UpdateUserProfileInput{
		Sub: user.Sub, Attributes: &attrs, Now: time.Now().UTC(),
	})
	if !errors.Is(err, authusecases.ErrInvalidAttribute) {
		t.Fatalf("expected ErrInvalidAttribute, got %v", err)
	}
}

func TestGetUserProfileHidesAdminOnlyAttributes(t *testing.T) {
	ctx, deps, user := accountTestDeps(t)
	_, defs, err := authusecases.GetUserProfile(ctx, deps, user.Sub)
	if err != nil {
		t.Fatal(err)
	}
	// department は admin_readable なので self には見えない。
	self := authusecases.SelfReadableAttributes(user.Attributes, defs)
	if _, ok := self["department"]; ok {
		t.Fatalf("admin_readable attribute leaked to self view: %+v", self)
	}
}
