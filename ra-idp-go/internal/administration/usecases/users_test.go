package usecases_test

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/crypto"
	"ra-idp-go/internal/adapters/persistence/memory"
	adminusecases "ra-idp-go/internal/administration/usecases"
	"ra-idp-go/internal/spec"
)

func TestCreateUpdateAndDisableUser(t *testing.T) {
	ctx := context.Background()
	userRepo := memory.NewUserRepository()
	historyRepo := memory.NewPasswordHistoryRepository()
	hasher := crypto.NewArgon2idPasswordHasher()
	var events []spec.DomainEvent
	deps := adminusecases.Deps{
		UserRepo: userRepo, PasswordHasher: hasher, PasswordHistoryRepo: historyRepo,
		Emit: func(event spec.DomainEvent) { events = append(events, event) },
	}
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	email := "bob@example.com"
	user, err := adminusecases.CreateUser(ctx, deps, adminusecases.CreateUserInput{
		ActorSub: "admin", PreferredUsername: "bob", Password: "initial-password-9182",
		Email: &email, Roles: []string{"support", "support"}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(user.Roles) != 1 || user.Roles[0] != "support" {
		t.Fatalf("roles=%v", user.Roles)
	}
	if events[0].EventType() != "UserCreated" {
		t.Fatalf("event=%s", events[0].EventType())
	}
	updatedName := "Bob"
	roles := []string{"admin", "support"}
	user, err = adminusecases.UpdateUser(ctx, deps, adminusecases.UpdateUserInput{
		ActorSub: "admin", Sub: user.Sub, Name: &updatedName, Roles: &roles, Now: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.Name == nil || *user.Name != "Bob" || len(user.Roles) != 2 {
		t.Fatalf("updated user=%+v", user)
	}
	user, err = adminusecases.SetUserDisabled(
		ctx, deps, "admin", user.Sub, true, now.Add(2*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if user.DisabledAt == nil {
		t.Fatal("disabled_at was not set")
	}
	if got := events[len(events)-1].EventType(); got != "UserDisabled" {
		t.Fatalf("last event=%s", got)
	}
	user, err = adminusecases.SetUserDisabled(
		ctx, deps, "admin", user.Sub, false, now.Add(3*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if user.DisabledAt != nil {
		t.Fatal("disabled_at was not cleared")
	}
	if got := events[len(events)-1].EventType(); got != "UserEnabled" {
		t.Fatalf("last event=%s", got)
	}
}

func TestCreateUserRejectsDuplicateUsername(t *testing.T) {
	repo := memory.NewUserRepository()
	now := time.Now().UTC()
	repo.Seed(&spec.User{
		Sub: "existing", PreferredUsername: "bob", PasswordHash: "hash",
		CreatedAt: now, UpdatedAt: now,
	})
	_, err := adminusecases.CreateUser(context.Background(), adminusecases.Deps{
		UserRepo: repo, PasswordHasher: crypto.NewArgon2idPasswordHasher(),
		PasswordHistoryRepo: memory.NewPasswordHistoryRepository(),
	}, adminusecases.CreateUserInput{
		PreferredUsername: "bob", Password: "initial-password-9182",
	})
	if err != adminusecases.ErrUsernameConflict {
		t.Fatalf("error=%v, want ErrUsernameConflict", err)
	}
}
