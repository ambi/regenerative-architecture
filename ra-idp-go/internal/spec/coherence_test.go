package spec_test

// SCL ↔ Go バインディングの coherence test。TS 側の invariants.test.ts と同役割。
// 仕様核 (spec/scl.yaml) と Go 実装の双子定義が乖離していないことを検証する。

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/spec"
)

func TestPasswordPolicyMinLengthMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	if got, want := objectiveInt(t, s, "PasswordPolicy", "min_length"), usecases.PasswordPolicyMinLength; got != want {
		t.Fatalf("objectives.PasswordPolicy.value.min_length=%d, Go PasswordPolicyMinLength=%d", got, want)
	}
}

func TestPasswordPolicyMaxLengthMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	if got, want := objectiveInt(t, s, "PasswordPolicy", "max_length"), usecases.PasswordPolicyMaxLength; got != want {
		t.Fatalf("objectives.PasswordPolicy.value.max_length=%d, Go PasswordPolicyMaxLength=%d", got, want)
	}
}

func TestPasswordPolicyHistoryDepthMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	if got, want := objectiveInt(t, s, "PasswordPolicy", "history_depth"), usecases.PasswordPolicyHistoryDepth; got != want {
		t.Fatalf("objectives.PasswordPolicy.value.history_depth=%d, Go PasswordPolicyHistoryDepth=%d", got, want)
	}
}

func TestPasswordPolicyBreachedCheckEnabledMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	got, ok := s.ObjectiveBool("PasswordPolicy", "breached_password_check_enabled")
	if !ok {
		t.Fatal("objectives.PasswordPolicy.value.breached_password_check_enabled missing or not a bool")
	}
	if got != usecases.PasswordPolicyBreachedCheckEnabled {
		t.Fatalf("objectives.PasswordPolicy.value.breached_password_check_enabled=%v, Go PasswordPolicyBreachedCheckEnabled=%v", got, usecases.PasswordPolicyBreachedCheckEnabled)
	}
}

func TestPasswordResetTokenTTLMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatal(err)
	}
	ttl, err := time.ParseDuration(s.Objectives["PasswordResetTokenLifetime"].TTL)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := int(ttl.Seconds()), usecases.PasswordResetTokenTTLSeconds; got != want {
		t.Fatalf("objectives.PasswordResetTokenLifetime.ttl=%d, Go PasswordResetTokenTTLSeconds=%d", got, want)
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
	policy := objectiveValue(t, s, "TotpPolicy")
	if got, want := policy["algorithm"], "SHA1"; got != want {
		t.Fatalf("totp algorithm=%q, want %q", got, want)
	}
	if got, want := objectiveInt(t, s, "TotpPolicy", "step_seconds"), int(usecases.TOTPStepSeconds); got != want {
		t.Fatalf("totp step_seconds=%d, want %d", got, want)
	}
	if got, want := objectiveInt(t, s, "TotpPolicy", "digits"), usecases.TOTPDigits; got != want {
		t.Fatalf("totp digits=%d, want %d", got, want)
	}
	if got, want := objectiveInt(t, s, "TotpPolicy", "window"), usecases.TOTPWindow; got != want {
		t.Fatalf("totp window=%d, want %d", got, want)
	}
	if got, want := objectiveInt(t, s, "TotpPolicy", "secret_bytes"), usecases.TOTPSecretBytes; got != want {
		t.Fatalf("totp secret_bytes=%d, want %d", got, want)
	}
}

func TestAgentStatusMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	got, err := s.EnumWireValues("AgentStatus")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		string(spec.AgentStatusActive),
		string(spec.AgentStatusDisabled),
		string(spec.AgentStatusKilled),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("SCL AgentStatus=%v, Go=%v", got, want)
	}
}

func TestAgentKindMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	got, err := s.EnumWireValues("AgentKind")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		string(spec.AgentKindAutonomous),
		string(spec.AgentKindSupervised),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("SCL AgentKind=%v, Go=%v", got, want)
	}
}

func TestGrantTypeTokenExchangeMatchesSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	got, err := s.EnumWireValues("GrantType")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got, string(spec.GrantTokenExchange)) {
		t.Fatalf("SCL GrantType=%v は Go の GrantTokenExchange=%q を含みません", got, spec.GrantTokenExchange)
	}
	if !spec.GrantTokenExchange.Valid() {
		t.Fatal("GrantTokenExchange.Valid() が false です")
	}
}

func TestLoginThrottlePolicyLoadsFromSCL(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	policy := objectiveValue(t, s, "LoginThrottlePolicy")
	account := nestedMap(t, policy, "per_account")
	ip := nestedMap(t, policy, "per_ip")
	if intValue(t, account, "max_failures") != 10 ||
		intValue(t, account, "window_seconds") != 900 ||
		intValue(t, account, "lockout_seconds") != 900 {
		t.Fatalf("unexpected per-account policy: %+v", account)
	}
	if intValue(t, ip, "max_failures") != 30 ||
		intValue(t, ip, "window_seconds") != 900 ||
		intValue(t, ip, "lockout_seconds") != 900 {
		t.Fatalf("unexpected per-IP policy: %+v", ip)
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

func TestCurrentSCLLoadsAllNormativeSections(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	sections := []struct {
		name string
		size int
	}{
		{"context_map", len(s.ContextMap)},
		{"standards", len(s.Standards)},
		{"vocabulary", len(s.Vocabulary)},
		{"models", len(s.Models)},
		{"interfaces", len(s.Interfaces)},
		{"states", len(s.States)},
		{"invariants", len(s.Invariants)},
		{"scenarios", len(s.Scenarios)},
		{"permissions", len(s.Permissions)},
		{"objectives", len(s.Objectives)},
		{"user_experience.screens", len(s.UserExperience.Screens)},
	}
	for _, section := range sections {
		if section.size == 0 {
			t.Errorf("%s was not loaded", section.name)
		}
	}
	if len(s.Annotations) != 0 {
		t.Errorf("top-level annotations must remain non-normative and are currently unexpected: %v", s.Annotations)
	}
}

func TestCurrentSCLIsInternallyCoherent(t *testing.T) {
	s, err := spec.LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	if err := s.ValidateCoherence(); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeSCLRejectsUnknownFields(t *testing.T) {
	_, err := spec.DecodeSCL([]byte(`
system: example
spec_version: "1.0"
unknown_section: {}
`))
	if err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
}

func TestAssuranceEvidenceHasExecutableBindings(t *testing.T) {
	root := repositoryRoot(t)
	for evidenceID, verifications := range spec.AssuranceManifest {
		for _, verification := range verifications {
			content, err := os.ReadFile(filepath.Join(root, verification.File))
			if err != nil {
				t.Errorf("%s: read %s: %v", evidenceID, verification.File, err)
				continue
			}
			if !strings.Contains(string(content), verification.Check) {
				t.Errorf("%s: %s does not contain check %q", evidenceID, verification.File, verification.Check)
			}
		}
	}
}

func objectiveValue(t *testing.T, s *spec.SCL, name string) map[string]any {
	t.Helper()
	value, ok := s.Objectives[name].Value.(map[string]any)
	if !ok {
		t.Fatalf("objectives.%s.value is not a map: %T", name, s.Objectives[name].Value)
	}
	return value
}

func objectiveInt(t *testing.T, s *spec.SCL, objective, key string) int {
	t.Helper()
	return intValue(t, objectiveValue(t, s, objective), key)
}

func nestedMap(t *testing.T, values map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := values[key].(map[string]any)
	if !ok {
		t.Fatalf("%s is not a map: %T", key, values[key])
	}
	return value
}

func intValue(t *testing.T, values map[string]any, key string) int {
	t.Helper()
	switch value := values[key].(type) {
	case int:
		return value
	case uint64:
		return int(value)
	default:
		t.Fatalf("%s is not an integer: %T", key, values[key])
		return 0
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}
