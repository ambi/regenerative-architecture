package usecases

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/spec"
)

func TestVerifyTOTPFactorReturnsNoFactor(t *testing.T) {
	repo := memory.NewMfaFactorRepository()
	result, err := VerifyTOTPFactor(context.Background(), repo, "user-1", "123456", time.Unix(59, 0))
	if err != nil {
		t.Fatalf("verify totp factor: %v", err)
	}
	if result.OK || result.Reason != "no_factor" {
		t.Fatalf("result=%+v, want no_factor", result)
	}
}

func TestVerifyTOTPFactorUpdatesLastUsedAt(t *testing.T) {
	repo := memory.NewMfaFactorRepository()
	secret := rfc6238SHA1SecretBase32
	created := time.Unix(1, 0).UTC()
	now := time.Unix(59, 0).UTC()
	if err := repo.Save(context.Background(), &spec.MfaFactor{
		Sub: "user-1", Type: spec.MfaFactorTOTP, Secret: &secret, CreatedAt: created,
	}); err != nil {
		t.Fatalf("save factor: %v", err)
	}
	code, err := GenerateTOTP(secret, now.Unix())
	if err != nil {
		t.Fatalf("generate totp: %v", err)
	}
	result, err := VerifyTOTPFactor(context.Background(), repo, "user-1", code, now)
	if err != nil {
		t.Fatalf("verify totp factor: %v", err)
	}
	if !result.OK {
		t.Fatalf("result=%+v, want ok", result)
	}
	stored, err := repo.Find(context.Background(), "user-1", spec.MfaFactorTOTP)
	if err != nil {
		t.Fatalf("find factor: %v", err)
	}
	if stored.LastUsedAt == nil || !stored.LastUsedAt.Equal(now) {
		t.Fatalf("last_used_at=%v, want %v", stored.LastUsedAt, now)
	}
}

func TestVerifyTOTPFactorRejectsInvalidCode(t *testing.T) {
	repo := memory.NewMfaFactorRepository()
	secret := rfc6238SHA1SecretBase32
	if err := repo.Save(context.Background(), &spec.MfaFactor{
		Sub: "user-1", Type: spec.MfaFactorTOTP, Secret: &secret, CreatedAt: time.Unix(1, 0).UTC(),
	}); err != nil {
		t.Fatalf("save factor: %v", err)
	}
	result, err := VerifyTOTPFactor(context.Background(), repo, "user-1", "000000", time.Unix(59, 0))
	if err != nil {
		t.Fatalf("verify totp factor: %v", err)
	}
	if result.OK || result.Reason != "invalid_code" {
		t.Fatalf("result=%+v, want invalid_code", result)
	}
}
