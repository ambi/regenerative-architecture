package spec_test

// SCL ↔ Go バインディングの coherence test。TS 側の invariants.test.ts と同役割。
// 仕様核 (spec/scl.yaml) と Go 実装の双子定義が乖離していないことを検証する。

import (
	"slices"
	"testing"

	"ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/spec"
)

func TestPasswordPolicyMinLengthMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	if got, want := s.Annotations.PasswordPolicy.MinLength, usecases.PasswordPolicyMinLength; got != want {
		t.Fatalf("scl.annotations.password_policy.min_length=%d, Go PasswordPolicyMinLength=%d", got, want)
	}
}

func TestPasswordPolicyMaxLengthMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	if got, want := s.Annotations.PasswordPolicy.MaxLength, usecases.PasswordPolicyMaxLength; got != want {
		t.Fatalf("scl.annotations.password_policy.max_length=%d, Go PasswordPolicyMaxLength=%d", got, want)
	}
}

func TestPasswordPolicyHistoryDepthMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	if got, want := s.Annotations.PasswordPolicy.HistoryDepth, usecases.PasswordPolicyHistoryDepth; got != want {
		t.Fatalf("scl.annotations.password_policy.history_depth=%d, Go PasswordPolicyHistoryDepth=%d", got, want)
	}
}

func TestPasswordResetTokenTTLMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.Annotations.PasswordResetPolicy.TokenTTLSeconds, usecases.PasswordResetTokenTTLSeconds; got != want {
		t.Fatalf("scl.annotations.password_reset_policy.token_ttl_seconds=%d, Go PasswordResetTokenTTLSeconds=%d", got, want)
	}
}

func TestMfaFactorTypeMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	got, err := s.EnumWireValues("MfaFactorType")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		string(spec.MfaFactorTOTP),
		string(spec.MfaFactorWebAuthn),
		string(spec.MfaFactorHWK),
		string(spec.MfaFactorSWK),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("SCL MfaFactorType=%v, Go=%v", got, want)
	}
}

func TestTOTPPolicyMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	policy := s.Annotations.TOTPPolicy
	if got, want := policy.Algorithm, "SHA1"; got != want {
		t.Fatalf("totp algorithm=%q, want %q", got, want)
	}
	if got, want := policy.StepSeconds, usecases.TOTPStepSeconds; got != want {
		t.Fatalf("totp step_seconds=%d, want %d", got, want)
	}
	if got, want := policy.Digits, usecases.TOTPDigits; got != want {
		t.Fatalf("totp digits=%d, want %d", got, want)
	}
	if got, want := policy.Window, usecases.TOTPWindow; got != want {
		t.Fatalf("totp window=%d, want %d", got, want)
	}
	if got, want := policy.SecretBytes, usecases.TOTPSecretBytes; got != want {
		t.Fatalf("totp secret_bytes=%d, want %d", got, want)
	}
}

func TestStandardsAndUserExperienceLoadFromSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	rfc9700, ok := s.Standards["RFC9700"]
	if !ok {
		t.Fatal("standards.RFC9700 is missing")
	}
	if len(rfc9700.Requirements) == 0 {
		t.Fatal("standards.RFC9700.requirements is empty")
	}
	if got := s.UserExperience.Accessibility["standard"]; got != "WCAG22" {
		t.Fatalf("user_experience.accessibility.standard=%q, want WCAG22", got)
	}
	if _, ok := s.UserExperience.Screens["Login"]; !ok {
		t.Fatal("user_experience.screens.Login is missing")
	}
}
