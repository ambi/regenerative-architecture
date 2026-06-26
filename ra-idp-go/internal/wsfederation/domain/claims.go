// Package domain は WsFederation bounded context のドメインロジックを所有する。
//
// 本ファイルは claim 発行エンジンを担う (ADR-059)。WS-Federation / WS-Trust / SAML が
// 共有する protocol-agnostic な変換で、解決済みの identity 属性と ClaimMappingPolicy から
// 発行 claim と NameID を組み立てる。fail-closed であり、mapping で明示した claim だけを
// 出力し、未マップ属性は決して漏らさない。必須 source が欠けた required rule は発行を拒否する。
package domain

import (
	"fmt"
	"strings"

	"ra-idp-go/internal/spec"
)

// Attributes は解決済みの identity 属性。属性名から値群への対応で、多値属性を表せる。
type Attributes map[string][]string

// ClaimIssuanceResult は claim 発行エンジンの結果。NameID と発行 claim 群を束ねる。
type ClaimIssuanceResult struct {
	NameIDFormat string
	NameIDValue  string
	Claims       []spec.IssuedClaim
}

// IssueClaims は policy と解決済み属性から NameID と claim 群を fail-closed に組み立てる。
//
//   - NameID は subject 識別子として必須。source_attribute が解決できなければ拒否する。
//   - 各 rule は source に応じて値を解決する。値が無いとき required なら拒否、そうでなければ
//     その claim を出力しない (属性最小化)。
//   - policy 自体の不備 (空の claim_type、user_attribute なのに source_key 空、未知の source) は拒否する。
func IssueClaims(policy spec.ClaimMappingPolicy, attrs Attributes) (ClaimIssuanceResult, error) {
	if strings.TrimSpace(policy.NameID.Format) == "" {
		return ClaimIssuanceResult{}, fmt.Errorf("claim issuance: name_id.format is required")
	}
	if strings.TrimSpace(policy.NameID.SourceAttribute) == "" {
		return ClaimIssuanceResult{}, fmt.Errorf("claim issuance: name_id.source_attribute is required")
	}

	nameID, ok := firstNonEmpty(attrs[policy.NameID.SourceAttribute])
	if !ok {
		return ClaimIssuanceResult{}, fmt.Errorf("claim issuance: NameID source attribute %q has no value", policy.NameID.SourceAttribute)
	}

	result := ClaimIssuanceResult{
		NameIDFormat: policy.NameID.Format,
		NameIDValue:  nameID,
	}

	for i, rule := range policy.Rules {
		if strings.TrimSpace(rule.ClaimType) == "" {
			return ClaimIssuanceResult{}, fmt.Errorf("claim issuance: rule %d has empty claim_type", i)
		}
		if !rule.Source.Valid() {
			return ClaimIssuanceResult{}, fmt.Errorf("claim issuance: rule %d (%s) has unknown source %q", i, rule.ClaimType, rule.Source)
		}

		values, err := resolveRule(rule, attrs, nameID)
		if err != nil {
			return ClaimIssuanceResult{}, err
		}
		if len(values) == 0 {
			if rule.Required {
				return ClaimIssuanceResult{}, fmt.Errorf("claim issuance: required claim %q could not be resolved", rule.ClaimType)
			}
			// 属性最小化: 値が無い任意 claim は出力しない (fail-closed)。
			continue
		}
		result.Claims = append(result.Claims, spec.IssuedClaim{ClaimType: rule.ClaimType, Values: values})
	}

	return result, nil
}

// resolveRule は 1 つの rule の出力値群を解決する。値が無ければ空スライスを返す。
func resolveRule(rule spec.ClaimMappingRule, attrs Attributes, nameID string) ([]string, error) {
	switch rule.Source {
	case spec.ClaimSourceUserAttribute:
		if strings.TrimSpace(rule.SourceKey) == "" {
			return nil, fmt.Errorf("claim issuance: claim %q with source user_attribute requires source_key", rule.ClaimType)
		}
		return nonEmpty(attrs[rule.SourceKey]), nil
	case spec.ClaimSourceFixed:
		if v := strings.TrimSpace(rule.FixedValue); v != "" {
			return []string{rule.FixedValue}, nil
		}
		return nil, nil
	case spec.ClaimSourceNameID:
		return []string{nameID}, nil
	default:
		// Valid() を通っているため到達しないが、fail-closed のため明示。
		return nil, fmt.Errorf("claim issuance: claim %q has unknown source %q", rule.ClaimType, rule.Source)
	}
}

// nonEmpty は空白のみの値を除いた値群を返す。
func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

// firstNonEmpty は最初の非空値を返す。
func firstNonEmpty(values []string) (string, bool) {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v, true
		}
	}
	return "", false
}
