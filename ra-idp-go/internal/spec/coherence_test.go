package spec_test

// SCL ↔ Go バインディングの coherence test。TS 側の invariants.test.ts と同役割。
// 仕様核 (spec/scl.yaml) と Go 実装の双子定義が乖離していないことを検証する。

import (
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
