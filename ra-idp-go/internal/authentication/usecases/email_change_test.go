package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/platform/notification"
	"ra-idp-go/internal/spec"
)

func TestRequestEmailChangeSendsLinkToNewAddress(t *testing.T) {
	ctx := context.Background()
	userRepo := memory.NewUserRepository()
	tokenStore := memory.NewEmailChangeTokenStore()
	sender := &notification.NoopEmailSender{}
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	current := "old@example.com"
	userRepo.Seed(&spec.User{
		Sub: "user-alice", PreferredUsername: "alice", PasswordHash: "unused",
		Email: &current, EmailVerified: true, CreatedAt: now, UpdatedAt: now,
	})
	var events []spec.DomainEvent
	if err := usecases.RequestEmailChange(ctx, usecases.RequestEmailChangeDeps{
		UserRepo: userRepo, TokenStore: tokenStore, EmailSender: sender,
		Emit:   func(e spec.DomainEvent) { events = append(events, e) },
		Issuer: "http://idp.test",
	}, usecases.RequestEmailChangeInput{Sub: "user-alice", NewEmail: " NEW@Example.COM ", Now: now}); err != nil {
		t.Fatal(err)
	}
	if len(sender.Sent) != 1 || sender.Sent[0].To != "new@example.com" {
		t.Fatalf("unexpected sent emails: %#v", sender.Sent)
	}
	if len(events) != 2 || events[0].EventType() != "EmailChangeRequested" ||
		events[1].EventType() != "EmailSent" {
		t.Fatalf("unexpected events: %#v", events)
	}
	// 起票だけでは User.email は変わらない。
	stored, _ := userRepo.FindBySub(ctx, "user-alice")
	if stored.Email == nil || *stored.Email != current {
		t.Fatalf("email changed before confirmation: %#v", stored.Email)
	}
}

func TestConfirmEmailChangeAppliesEmailAndClearsVerifyAction(t *testing.T) {
	ctx := context.Background()
	userRepo := memory.NewUserRepository()
	tokenStore := memory.NewEmailChangeTokenStore()
	sender := &notification.NoopEmailSender{}
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	current := "old@example.com"
	userRepo.Seed(&spec.User{
		Sub: "user-alice", PreferredUsername: "alice", PasswordHash: "unused",
		Email: &current, EmailVerified: false, CreatedAt: now, UpdatedAt: now,
		Lifecycle: spec.UserLifecycle{
			Status:          spec.UserStatusActive,
			RequiredActions: []spec.RequiredAction{spec.RequiredActionVerifyEmail},
		},
	})
	if err := usecases.RequestEmailChange(ctx, usecases.RequestEmailChangeDeps{
		UserRepo: userRepo, TokenStore: tokenStore, EmailSender: sender, Issuer: "http://idp.test",
	}, usecases.RequestEmailChangeInput{Sub: "user-alice", NewEmail: "new@example.com", Now: now}); err != nil {
		t.Fatal(err)
	}
	token := tokenFromMessage(t, sender.Sent[0].Text)

	var events []spec.DomainEvent
	updated, err := usecases.ConfirmEmailChange(ctx, usecases.ConfirmEmailChangeDeps{
		UserRepo: userRepo, TokenStore: tokenStore,
		Emit: func(e spec.DomainEvent) { events = append(events, e) },
	}, usecases.ConfirmEmailChangeInput{Token: token, Now: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Email == nil || *updated.Email != "new@example.com" || !updated.EmailVerified {
		t.Fatalf("email not applied: email=%v verified=%v", updated.Email, updated.EmailVerified)
	}
	for _, a := range updated.Lifecycle.RequiredActions {
		if a == spec.RequiredActionVerifyEmail {
			t.Fatal("verify_email required action was not cleared")
		}
	}
	if len(events) != 2 || events[0].EventType() != "EmailChanged" ||
		events[1].EventType() != "UserRequiredActionCleared" {
		t.Fatalf("unexpected events: %#v", events)
	}

	// トークンは単発消費。
	if _, err := usecases.ConfirmEmailChange(ctx, usecases.ConfirmEmailChangeDeps{
		UserRepo: userRepo, TokenStore: tokenStore,
	}, usecases.ConfirmEmailChangeInput{Token: token, Now: now.Add(2 * time.Minute)}); !errors.Is(err, usecases.ErrInvalidEmailChangeToken) {
		t.Fatalf("reused token error=%v, want ErrInvalidEmailChangeToken", err)
	}
}

func TestRequestEmailChangeRejectsAddressTakenByAnotherUser(t *testing.T) {
	ctx := context.Background()
	userRepo := memory.NewUserRepository()
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	mine := "mine@example.com"
	taken := "taken@example.com"
	userRepo.Seed(&spec.User{
		Sub: "user-alice", PreferredUsername: "alice", PasswordHash: "unused",
		Email: &mine, EmailVerified: true, CreatedAt: now, UpdatedAt: now,
	})
	userRepo.Seed(&spec.User{
		Sub: "user-bob", PreferredUsername: "bob", PasswordHash: "unused",
		Email: &taken, EmailVerified: true, CreatedAt: now, UpdatedAt: now,
	})
	err := usecases.RequestEmailChange(ctx, usecases.RequestEmailChangeDeps{
		UserRepo: userRepo, TokenStore: memory.NewEmailChangeTokenStore(),
		EmailSender: &notification.NoopEmailSender{}, Issuer: "http://idp.test",
	}, usecases.RequestEmailChangeInput{Sub: "user-alice", NewEmail: taken, Now: now})
	if !errors.Is(err, usecases.ErrEmailTaken) {
		t.Fatalf("error=%v, want ErrEmailTaken", err)
	}
}
