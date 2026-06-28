package usecases_test

import (
	"context"
	"slices"
	"testing"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"
	idmusecases "ra-idp-go/internal/identitymanagement/usecases"
	"ra-idp-go/internal/infrastructure/crypto"
	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/spec"
)

func newRequiredActionFixture(t *testing.T) (context.Context, idmusecases.AdminUserDeps, *[]spec.DomainEvent, *spec.User) {
	t.Helper()
	ctx := context.Background()
	userRepo := memory.NewUserRepository()
	historyRepo := memory.NewPasswordHistoryRepository()
	hasher := crypto.NewArgon2idPasswordHasher()
	events := &[]spec.DomainEvent{}
	deps := idmusecases.AdminUserDeps{
		UserRepo: userRepo, PasswordHasher: hasher, PasswordHistoryRepo: historyRepo,
		Emit: func(event spec.DomainEvent) { *events = append(*events, event) },
	}
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	email := "carol@example.com"
	user, err := idmusecases.CreateUser(ctx, deps, idmusecases.CreateUserInput{
		ActorSub: "admin", PreferredUsername: "carol", Password: "initial-password-9182",
		Email: &email, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	*events = (*events)[:0]
	return ctx, deps, events, user
}

func TestSetAndClearUserRequiredAction(t *testing.T) {
	ctx, deps, events, user := newRequiredActionFixture(t)
	now := time.Date(2026, 6, 20, 13, 0, 0, 0, time.UTC)

	updated, err := idmusecases.SetUserRequiredAction(
		ctx, deps, "admin", user.Sub, spec.RequiredActionUpdatePassword, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(updated.Lifecycle.RequiredActions, spec.RequiredActionUpdatePassword) {
		t.Fatalf("required actions=%v, want update_password", updated.Lifecycle.RequiredActions)
	}
	if got := (*events)[len(*events)-1]; got.EventType() != "UserRequiredActionSet" {
		t.Fatalf("event=%s, want UserRequiredActionSet", got.EventType())
	}

	// 冪等: 二重付与してもイベントを増やさず単一のまま。
	before := len(*events)
	updated, err = idmusecases.SetUserRequiredAction(
		ctx, deps, "admin", user.Sub, spec.RequiredActionUpdatePassword, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Lifecycle.RequiredActions) != 1 {
		t.Fatalf("required actions=%v, want single", updated.Lifecycle.RequiredActions)
	}
	if len(*events) != before {
		t.Fatalf("idempotent set emitted extra events: %d", len(*events)-before)
	}

	updated, err = idmusecases.ClearUserRequiredAction(
		ctx, deps, "admin", user.Sub, spec.RequiredActionUpdatePassword, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Lifecycle.RequiredActions) != 0 {
		t.Fatalf("required actions=%v, want empty", updated.Lifecycle.RequiredActions)
	}
	if got := (*events)[len(*events)-1]; got.EventType() != "UserRequiredActionCleared" {
		t.Fatalf("event=%s, want UserRequiredActionCleared", got.EventType())
	}
}

func TestSetUserRequiredActionRejectsUnknownAction(t *testing.T) {
	ctx, deps, _, user := newRequiredActionFixture(t)
	_, err := idmusecases.SetUserRequiredAction(
		ctx, deps, "admin", user.Sub, spec.RequiredAction("teleport"), time.Now().UTC(),
	)
	if err == nil {
		t.Fatal("expected error for unknown required action")
	}
}

func TestChangePasswordAutoClearsUpdatePasswordAction(t *testing.T) {
	ctx, deps, events, user := newRequiredActionFixture(t)
	now := time.Date(2026, 6, 20, 14, 0, 0, 0, time.UTC)
	if _, err := idmusecases.SetUserRequiredAction(
		ctx, deps, "admin", user.Sub, spec.RequiredActionUpdatePassword, now,
	); err != nil {
		t.Fatal(err)
	}
	*events = (*events)[:0]

	updated, err := authusecases.ChangePassword(ctx, authusecases.ChangePasswordDeps{
		UserRepo:            deps.UserRepo,
		PasswordHasher:      deps.PasswordHasher,
		PasswordHistoryRepo: deps.PasswordHistoryRepo,
		Emit:                deps.Emit,
	}, authusecases.ChangePasswordInput{
		Sub:             user.Sub,
		CurrentPassword: "initial-password-9182",
		NewPassword:     "fresh-pass-77182",
		Now:             now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("change password: %v", err)
	}
	if len(updated.Lifecycle.RequiredActions) != 0 {
		t.Fatalf("update_password should be auto-cleared, got %v", updated.Lifecycle.RequiredActions)
	}
	if updated.Lifecycle.PasswordChangedAt == nil {
		t.Fatal("password_changed_at was not set")
	}
	var sawCleared bool
	for _, e := range *events {
		if cleared, ok := e.(*spec.UserRequiredActionCleared); ok {
			sawCleared = true
			if cleared.ActorSub != user.Sub {
				t.Fatalf("auto-clear actorSub=%s, want self %s", cleared.ActorSub, user.Sub)
			}
		}
	}
	if !sawCleared {
		t.Fatal("UserRequiredActionCleared was not emitted on auto-clear")
	}
}
