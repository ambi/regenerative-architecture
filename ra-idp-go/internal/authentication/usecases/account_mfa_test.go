package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/spec"
)

func newMfaDeps(t *testing.T) (usecases.AccountMfaDeps, *memory.UserRepository, *[]spec.DomainEvent) {
	t.Helper()
	userRepo := memory.NewUserRepository()
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	userRepo.Seed(&spec.User{
		Sub: "user-alice", PreferredUsername: "alice", PasswordHash: "unused",
		CreatedAt: now, UpdatedAt: now,
	})
	var events []spec.DomainEvent
	deps := usecases.AccountMfaDeps{
		UserRepo: userRepo, MfaFactorRepo: memory.NewMfaFactorRepository(),
		Emit:   func(e spec.DomainEvent) { events = append(events, e) },
		Issuer: "http://idp.test",
	}
	return deps, userRepo, &events
}

func TestTOTPEnrollmentConfirmPersistsFactorAndFlag(t *testing.T) {
	ctx := context.Background()
	deps, userRepo, events := newMfaDeps(t)
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)

	start, err := usecases.StartTOTPEnrollment(ctx, deps, "user-alice")
	if err != nil {
		t.Fatal(err)
	}
	if start.Secret == "" || start.OTPAuthURI == "" {
		t.Fatalf("incomplete enrollment start: %#v", start)
	}

	code, err := usecases.GenerateTOTP(start.Secret, now.Unix())
	if err != nil {
		t.Fatal(err)
	}
	if err := usecases.ConfirmTOTPEnrollment(ctx, deps, usecases.ConfirmTOTPEnrollmentInput{
		Sub: "user-alice", Secret: start.Secret, Code: code, Now: now,
	}); err != nil {
		t.Fatal(err)
	}

	factor, _ := deps.MfaFactorRepo.Find(ctx, "user-alice", spec.MfaFactorTOTP)
	if factor == nil || factor.Secret == nil || *factor.Secret != start.Secret {
		t.Fatalf("factor not persisted: %#v", factor)
	}
	stored, _ := userRepo.FindBySub(ctx, "user-alice")
	if !stored.MfaEnrolled {
		t.Fatal("MfaEnrolled flag not set")
	}
	if len(*events) != 1 || (*events)[0].EventType() != "MfaFactorEnrolled" {
		t.Fatalf("unexpected events: %#v", *events)
	}
}

func TestTOTPEnrollmentConfirmRejectsWrongCode(t *testing.T) {
	ctx := context.Background()
	deps, _, _ := newMfaDeps(t)
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	start, err := usecases.StartTOTPEnrollment(ctx, deps, "user-alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := usecases.ConfirmTOTPEnrollment(ctx, deps, usecases.ConfirmTOTPEnrollmentInput{
		Sub: "user-alice", Secret: start.Secret, Code: "000000", Now: now,
	}); !errors.Is(err, usecases.ErrInvalidTOTPCode) {
		t.Fatalf("error=%v, want ErrInvalidTOTPCode", err)
	}
	factor, _ := deps.MfaFactorRepo.Find(ctx, "user-alice", spec.MfaFactorTOTP)
	if factor != nil {
		t.Fatal("factor persisted despite invalid code")
	}
}

func TestTOTPEnrollmentStartRejectsWhenAlreadyEnrolled(t *testing.T) {
	ctx := context.Background()
	deps, _, _ := newMfaDeps(t)
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	start, _ := usecases.StartTOTPEnrollment(ctx, deps, "user-alice")
	code, _ := usecases.GenerateTOTP(start.Secret, now.Unix())
	if err := usecases.ConfirmTOTPEnrollment(ctx, deps, usecases.ConfirmTOTPEnrollmentInput{
		Sub: "user-alice", Secret: start.Secret, Code: code, Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := usecases.StartTOTPEnrollment(ctx, deps, "user-alice"); !errors.Is(err, usecases.ErrMfaAlreadyEnrolled) {
		t.Fatalf("error=%v, want ErrMfaAlreadyEnrolled", err)
	}
}

func TestRemoveTOTPFactorRequiresValidCode(t *testing.T) {
	ctx := context.Background()
	deps, userRepo, _ := newMfaDeps(t)
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	start, _ := usecases.StartTOTPEnrollment(ctx, deps, "user-alice")
	code, _ := usecases.GenerateTOTP(start.Secret, now.Unix())
	if err := usecases.ConfirmTOTPEnrollment(ctx, deps, usecases.ConfirmTOTPEnrollmentInput{
		Sub: "user-alice", Secret: start.Secret, Code: code, Now: now,
	}); err != nil {
		t.Fatal(err)
	}

	// wrong code keeps the factor.
	if err := usecases.RemoveTOTPFactor(ctx, deps, usecases.RemoveTOTPFactorInput{
		Sub: "user-alice", Code: "000000", Now: now,
	}); !errors.Is(err, usecases.ErrInvalidTOTPCode) {
		t.Fatalf("error=%v, want ErrInvalidTOTPCode", err)
	}
	if factor, _ := deps.MfaFactorRepo.Find(ctx, "user-alice", spec.MfaFactorTOTP); factor == nil {
		t.Fatal("factor removed despite invalid code")
	}

	// valid code removes the factor and clears the flag.
	removeCode, _ := usecases.GenerateTOTP(start.Secret, now.Unix())
	if err := usecases.RemoveTOTPFactor(ctx, deps, usecases.RemoveTOTPFactorInput{
		Sub: "user-alice", Code: removeCode, Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	if factor, _ := deps.MfaFactorRepo.Find(ctx, "user-alice", spec.MfaFactorTOTP); factor != nil {
		t.Fatal("factor not removed")
	}
	stored, _ := userRepo.FindBySub(ctx, "user-alice")
	if stored.MfaEnrolled {
		t.Fatal("MfaEnrolled flag not cleared")
	}
}

func TestRemoveTOTPFactorWhenNoneEnrolled(t *testing.T) {
	ctx := context.Background()
	deps, _, _ := newMfaDeps(t)
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	if err := usecases.RemoveTOTPFactor(ctx, deps, usecases.RemoveTOTPFactorInput{
		Sub: "user-alice", Code: "000000", Now: now,
	}); !errors.Is(err, usecases.ErrMfaNotEnrolled) {
		t.Fatalf("error=%v, want ErrMfaNotEnrolled", err)
	}
}
