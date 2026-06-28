// Package usecases: Layer 3 - Application Logic（パスワードポリシー）
//
// 仕様核は spec/scl.yaml `objectives.PasswordPolicy`。本ファイルはその
// 双子定義。SCL と本ファイルの値が乖離すると spec↔impl drift になるため、
// coherence test (spec_bindings) で突き合わせる。
package usecases

import (
	"fmt"
	"strings"
	"unicode/utf8"

	z "github.com/Oudwins/zog"
)

type PasswordPolicyViolation string

const (
	ViolationTooShort PasswordPolicyViolation = "too_short"
	ViolationTooLong  PasswordPolicyViolation = "too_long"
	ViolationBreached PasswordPolicyViolation = "breached"
)

const (
	PasswordPolicyMinLength    = 12
	PasswordPolicyMaxLength    = 128
	PasswordPolicyHistoryDepth = 5
)

// PasswordPolicyBreachedCheckEnabled は breached password check の宣言的既定値
// (spec/scl.yaml objectives.PasswordPolicy.value.breached_password_check_enabled の双子)。
// 既定は false で NoopBreachedPasswordChecker を使う。実運用では
// BREACHED_PASSWORD_CHECKER=hibp で adapter を差し替えて有効化する。テナント別の
// opt-out は Phase 4 (ADR-028 §6)。
const PasswordPolicyBreachedCheckEnabled = false

type PasswordPolicyResult struct {
	OK         bool
	Violations []PasswordPolicyViolation
}

// PasswordPolicySnapshot は実評価に使うしきい値。テナント解決後の値を渡す想定で、
// Phase 4 のテナント別ポリシー (spec.ResolvePasswordPolicy) と統合する。
// spec.PasswordPolicySnapshot と同じ shape を持ち、authentication 層が
// 単独で評価できる形にしてある。
type PasswordPolicySnapshot struct {
	MinLength    int
	MaxLength    int
	HistoryDepth int
}

func defaultPasswordPolicySnapshot() PasswordPolicySnapshot {
	return PasswordPolicySnapshot{
		MinLength:    PasswordPolicyMinLength,
		MaxLength:    PasswordPolicyMaxLength,
		HistoryDepth: PasswordPolicyHistoryDepth,
	}
}

// resolveSnapshot は spec 解決済みの snapshot を最優先で受け取り、
// 旧経路（HistoryDepth のみ別個に渡す形）からの呼び出しもサポートする。
// snap が完全なゼロ値なら global default、legacyDepth が指定されていれば
// HistoryDepth のみ上書きする。
func resolveSnapshot(snap PasswordPolicySnapshot, legacyDepth int) PasswordPolicySnapshot {
	result := snap
	if result.MinLength == 0 {
		result.MinLength = PasswordPolicyMinLength
	}
	if result.MaxLength == 0 {
		result.MaxLength = PasswordPolicyMaxLength
	}
	if result.HistoryDepth == 0 {
		if legacyDepth > 0 {
			result.HistoryDepth = legacyDepth
		} else {
			result.HistoryDepth = PasswordPolicyHistoryDepth
		}
	}
	return result
}

func passwordSchemaFor(snap PasswordPolicySnapshot) *z.StringSchema[string] {
	return z.String().
		Required(z.Message(string(ViolationTooShort))).
		TestFunc(
			func(value *string, _ z.Ctx) bool {
				return utf8.RuneCountInString(*value) >= snap.MinLength
			},
			z.Message(string(ViolationTooShort)),
		).
		TestFunc(
			func(value *string, _ z.Ctx) bool {
				return utf8.RuneCountInString(*value) <= snap.MaxLength
			},
			z.Message(string(ViolationTooLong)),
		)
}

// ValidatePassword は global default のしきい値で評価する。テナント別ポリシーが
// 不要な経路 (ログイン経路の弱い再評価など) で使う。
//
// 文字数は UTF-8 コードポイント単位 (rune) で数え、surrogate サイズの差で
// TS 実装 (UTF-16 code units) と細部が食い違うため、ASCII デモパスワード
// については両者一致する想定。
func ValidatePassword(plain string) PasswordPolicyResult {
	return ValidatePasswordWith(plain, defaultPasswordPolicySnapshot())
}

// ValidatePasswordWith はテナント解決済みのしきい値で評価する。change-password /
// reset-password のように本格的なポリシー適用が必要な経路で使う。
func ValidatePasswordWith(plain string, snap PasswordPolicySnapshot) PasswordPolicyResult {
	var violations []PasswordPolicyViolation
	for _, issue := range passwordSchemaFor(snap).Validate(&plain) {
		switch PasswordPolicyViolation(issue.Message) {
		case ViolationTooShort:
			violations = append(violations, ViolationTooShort)
		case ViolationTooLong:
			violations = append(violations, ViolationTooLong)
		}
	}
	return PasswordPolicyResult{OK: len(violations) == 0, Violations: violations}
}

type PasswordPolicyError struct {
	Violations []PasswordPolicyViolation
}

func (e *PasswordPolicyError) Error() string {
	parts := make([]string, len(e.Violations))
	for i, v := range e.Violations {
		parts[i] = string(v)
	}
	return fmt.Sprintf("password policy violated: %s", strings.Join(parts, ", "))
}
