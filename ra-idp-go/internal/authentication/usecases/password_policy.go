// Package usecases: Layer 3 - Application Logic（パスワードポリシー）
//
// 仕様核は spec/scl.yaml `annotations.password_policy`。本ファイルはその
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
)

const (
	PasswordPolicyMinLength = 12
	PasswordPolicyMaxLength = 128
)

type PasswordPolicyResult struct {
	OK         bool
	Violations []PasswordPolicyViolation
}

var passwordSchema = z.String().
	Required(z.Message(string(ViolationTooShort))).
	TestFunc(
		func(value *string, _ z.Ctx) bool {
			return utf8.RuneCountInString(*value) >= PasswordPolicyMinLength
		},
		z.Message(string(ViolationTooShort)),
	).
	TestFunc(
		func(value *string, _ z.Ctx) bool {
			return utf8.RuneCountInString(*value) <= PasswordPolicyMaxLength
		},
		z.Message(string(ViolationTooLong)),
	)

// ValidatePassword は SCL の password_policy 制約を適用する。
// 文字数は UTF-8 コードポイント単位 (rune) で数え、surrogate サイズの差で
// TS 実装 (UTF-16 code units) と細部が食い違うため、ASCII デモパスワード
// については両者一致する想定。
func ValidatePassword(plain string) PasswordPolicyResult {
	var violations []PasswordPolicyViolation
	for _, issue := range passwordSchema.Validate(&plain) {
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
