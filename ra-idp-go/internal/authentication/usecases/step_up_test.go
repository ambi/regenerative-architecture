package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/platform/crypto"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"
)

func TestStepUpSatisfiedRecencyWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		ctx  *domain.AuthenticationContext
		want bool
	}{
		{"fresh auth", &domain.AuthenticationContext{AuthTime: now.Add(-time.Minute).Unix()}, true},
		{"stale auth", &domain.AuthenticationContext{AuthTime: now.Add(-10 * time.Minute).Unix()}, false},
		{
			"stale auth but recent step-up",
			&domain.AuthenticationContext{
				AuthTime: now.Add(-10 * time.Minute).Unix(), StepUpAt: now.Add(-2 * time.Minute).Unix(),
			},
			true,
		},
		{"boundary 300s", &domain.AuthenticationContext{AuthTime: now.Add(-300 * time.Second).Unix()}, true},
		{"just over 300s", &domain.AuthenticationContext{AuthTime: now.Add(-301 * time.Second).Unix()}, false},
		{"pending never", &domain.AuthenticationContext{AuthTime: now.Unix(), AuthenticationPending: true}, false},
		{"zero times", &domain.AuthenticationContext{}, false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		if got := StepUpSatisfied(tc.ctx, now); got != tc.want {
			t.Errorf("%s: StepUpSatisfied = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestAvailableStepUpMethods(t *testing.T) {
	t.Parallel()
	if got := AvailableStepUpMethods(&spec.User{}); len(got) != 1 || got[0] != StepUpMethodPassword {
		t.Fatalf("no MFA: got %v", got)
	}
	got := AvailableStepUpMethods(&spec.User{MfaEnrolled: true})
	if len(got) != 2 || got[1] != StepUpMethodTOTP {
		t.Fatalf("MFA enrolled: got %v", got)
	}
}

func newStepUpFixture(t *testing.T, now time.Time) (StepUpDeps, *SessionManager, *[]spec.DomainEvent) {
	t.Helper()
	ctx := context.Background()
	userRepo := memory.NewUserRepository()
	hasher := crypto.NewArgon2idPasswordHasher()
	hash, err := hasher.Hash("demo-password-1234")
	if err != nil {
		t.Fatal(err)
	}
	if err := userRepo.Save(ctx, &spec.User{
		Sub: "user-1", PreferredUsername: "alice", PasswordHash: hash, MfaEnrolled: true,
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	mfaRepo := memory.NewMfaFactorRepository()
	if err := mfaRepo.Save(ctx, &spec.MfaFactor{
		Sub: "user-1", Type: spec.MfaFactorTOTP, Secret: &secret, CreatedAt: now.Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	var events []spec.DomainEvent
	// in-memory SessionStore は既定で実時計により期限切れ判定するため、固定 now の
	// テストでセッションが失効しないよう時計を now に固定する。
	sessionStore := memory.NewSessionStore()
	sessionStore.Clock = func() time.Time { return now }
	sm := NewSessionManager(sessionStore)
	deps := StepUpDeps{
		UserRepo: userRepo, PasswordHasher: hasher, MfaFactorRepo: mfaRepo, SessionManager: sm,
		Emit: func(e spec.DomainEvent) { events = append(events, e) },
	}
	return deps, sm, &events
}

func TestCompleteStepUpPasswordRecordsAndEmits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	deps, sm, events := newStepUpFixture(t, now)
	authn, err := sm.CreateWithPending(ctx, "user-1", []string{"pwd"}, now.Add(-time.Hour), false)
	if err != nil {
		t.Fatal(err)
	}

	if err := CompleteStepUp(ctx, deps, CompleteStepUpInput{
		Sub: "user-1", SessionID: authn.SessionID, Method: StepUpMethodPassword,
		Password: "demo-password-1234", Now: now,
	}); err != nil {
		t.Fatalf("complete step-up: %v", err)
	}
	sess, _ := sm.Store.Find(ctx, authn.SessionID)
	if sess.StepUpAt != now.Unix() {
		t.Fatalf("step_up_at=%d, want %d", sess.StepUpAt, now.Unix())
	}
	// 刻んだ後は recency 窓内なので gate を通過する。
	if !StepUpSatisfied(&domain.AuthenticationContext{AuthTime: authn.AuthTime, StepUpAt: sess.StepUpAt}, now) {
		t.Fatal("expected step-up to satisfy gate after completion")
	}
	if len(*events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(*events))
	}
	completed, ok := (*events)[0].(*spec.StepUpCompleted)
	if !ok || completed.Method != "password" || completed.Sub != "user-1" {
		t.Fatalf("unexpected event %#v", (*events)[0])
	}
}

func TestCompleteStepUpWrongPasswordFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	deps, sm, _ := newStepUpFixture(t, now)
	authn, _ := sm.CreateWithPending(ctx, "user-1", []string{"pwd"}, now.Add(-time.Hour), false)

	err := CompleteStepUp(ctx, deps, CompleteStepUpInput{
		Sub: "user-1", SessionID: authn.SessionID, Method: StepUpMethodPassword,
		Password: "wrong-password", Now: now,
	})
	if !errors.Is(err, ErrStepUpFailed) {
		t.Fatalf("err=%v, want ErrStepUpFailed", err)
	}
	sess, _ := sm.Store.Find(ctx, authn.SessionID)
	if sess.StepUpAt != 0 {
		t.Fatal("step_up_at must stay unset on failure")
	}
}

func TestCompleteStepUpTOTPSucceeds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	deps, sm, _ := newStepUpFixture(t, now)
	authn, _ := sm.CreateWithPending(ctx, "user-1", []string{"pwd"}, now.Add(-time.Hour), false)

	factor, err := deps.MfaFactorRepo.Find(ctx, "user-1", spec.MfaFactorTOTP)
	if err != nil {
		t.Fatal(err)
	}
	code, err := GenerateTOTP(*factor.Secret, now.Unix())
	if err != nil {
		t.Fatal(err)
	}
	if err := CompleteStepUp(ctx, deps, CompleteStepUpInput{
		Sub: "user-1", SessionID: authn.SessionID, Method: StepUpMethodTOTP, Code: code, Now: now,
	}); err != nil {
		t.Fatalf("complete TOTP step-up: %v", err)
	}
	sess, _ := sm.Store.Find(ctx, authn.SessionID)
	if sess.StepUpAt != now.Unix() {
		t.Fatal("expected step_up_at recorded for TOTP")
	}
}
