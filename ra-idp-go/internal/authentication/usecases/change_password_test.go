package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/crypto"
	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/spec"
)

func TestChangePasswordUpdatesHashAndEmitsEvent(t *testing.T) {
	t.Parallel()

	userRepo := memory.NewUserRepository()
	historyRepo := memory.NewPasswordHistoryRepository()
	hasher := crypto.NewArgon2idPasswordHasher()
	hash, err := hasher.Hash("demo-password-1234")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC)
	user := &spec.User{
		Sub: "user-1", PreferredUsername: "alice", PasswordHash: hash,
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	if err := userRepo.Save(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	var events []spec.DomainEvent

	updated, err := ChangePassword(context.Background(), ChangePasswordDeps{
		UserRepo:            userRepo,
		PasswordHasher:      hasher,
		PasswordHistoryRepo: historyRepo,
		Emit: func(event spec.DomainEvent) {
			events = append(events, event)
		},
	}, ChangePasswordInput{
		Sub:             user.Sub,
		CurrentPassword: "demo-password-1234",
		NewPassword:     "fresh-pass-9182",
		Now:             now,
	})
	if err != nil {
		t.Fatalf("change password: %v", err)
	}
	if updated.UpdatedAt != now {
		t.Fatalf("updated_at=%s, want %s", updated.UpdatedAt, now)
	}
	ok, err := hasher.Verify("fresh-pass-9182", updated.PasswordHash)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("updated hash does not verify new password")
	}
	recent, err := historyRepo.Recent(context.Background(), user.Sub, PasswordPolicyHistoryDepth)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 {
		t.Fatalf("history entries=%d, want 1", len(recent))
	}
	ok, err = hasher.Verify("fresh-pass-9182", recent[0].Encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("history hash does not verify new password")
	}
	if len(events) != 1 {
		t.Fatalf("events=%d, want 1", len(events))
	}
	if _, ok := events[0].(*spec.PasswordChanged); !ok {
		t.Fatalf("event type=%T, want *spec.PasswordChanged", events[0])
	}
}

func TestChangePasswordRejectsCurrentPasswordMismatch(t *testing.T) {
	t.Parallel()

	userRepo := memory.NewUserRepository()
	historyRepo := memory.NewPasswordHistoryRepository()
	hasher := crypto.NewArgon2idPasswordHasher()
	hash, err := hasher.Hash("demo-password-1234")
	if err != nil {
		t.Fatal(err)
	}
	if err := userRepo.Save(context.Background(), &spec.User{
		Sub: "user-1", PreferredUsername: "alice", PasswordHash: hash,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	_, err = ChangePassword(context.Background(), ChangePasswordDeps{
		UserRepo:            userRepo,
		PasswordHasher:      hasher,
		PasswordHistoryRepo: historyRepo,
	}, ChangePasswordInput{
		Sub:             "user-1",
		CurrentPassword: "wrong-password",
		NewPassword:     "fresh-pass-9182",
	})
	if !errors.Is(err, ErrCurrentPasswordMismatch) {
		t.Fatalf("err=%v, want current password mismatch", err)
	}
}

func TestChangePasswordHonorsTenantOverridePolicy(t *testing.T) {
	t.Parallel()

	userRepo := memory.NewUserRepository()
	historyRepo := memory.NewPasswordHistoryRepository()
	hasher := crypto.NewArgon2idPasswordHasher()
	hash, err := hasher.Hash("demo-password-1234")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC)
	if err := userRepo.Save(context.Background(), &spec.User{
		Sub: "user-1", PreferredUsername: "alice", PasswordHash: hash,
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	strict := PasswordPolicySnapshot{MinLength: 24, MaxLength: 128, HistoryDepth: 5}
	_, err = ChangePassword(context.Background(), ChangePasswordDeps{
		UserRepo:            userRepo,
		PasswordHasher:      hasher,
		PasswordHistoryRepo: historyRepo,
		Policy:              strict,
	}, ChangePasswordInput{
		Sub:             "user-1",
		CurrentPassword: "demo-password-1234",
		NewPassword:     "fresh-pass-9182",
		Now:             now,
	})
	var policyErr *PasswordPolicyError
	if !errors.As(err, &policyErr) {
		t.Fatalf("err=%v, want PasswordPolicyError under tenant strict policy", err)
	}
	if len(policyErr.Violations) == 0 || policyErr.Violations[0] != ViolationTooShort {
		t.Fatalf("violations=%v, want first too_short", policyErr.Violations)
	}
}

func TestChangePasswordRejectsPasswordReuse(t *testing.T) {
	t.Parallel()

	userRepo := memory.NewUserRepository()
	historyRepo := memory.NewPasswordHistoryRepository()
	hasher := crypto.NewArgon2idPasswordHasher()
	initialHash, err := hasher.Hash("demo-password-1234")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := userRepo.Save(context.Background(), &spec.User{
		Sub: "user-1", PreferredUsername: "alice", PasswordHash: initialHash,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	reusedHash, err := hasher.Hash("pw-history-aaaa-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := historyRepo.Add(context.Background(), "user-1", reusedHash, now); err != nil {
		t.Fatal(err)
	}

	_, err = ChangePassword(context.Background(), ChangePasswordDeps{
		UserRepo:            userRepo,
		PasswordHasher:      hasher,
		PasswordHistoryRepo: historyRepo,
	}, ChangePasswordInput{
		Sub:             "user-1",
		CurrentPassword: "demo-password-1234",
		NewPassword:     "pw-history-aaaa-1",
	})
	if !errors.Is(err, ErrPasswordReused) {
		t.Fatalf("err=%v, want password reused", err)
	}
}
