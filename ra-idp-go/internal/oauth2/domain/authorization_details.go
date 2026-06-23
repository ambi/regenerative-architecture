// Rich Authorization Requests (RFC 9396) の検証・ダウンスコープ判定 (ADR-050)。
//
// 本ファイルは authorization_details に対する 2 つの fail-closed 判定を担う:
//
//   - ValidateAgainstType: 1 つの detail が登録済み AuthorizationDetailType の
//     スキーマに適合するか (構文・必須・許可値・未登録フィールド拒否)。
//   - DetailsSubsetOf: 要求 detail 群が、基準 (同意済み / 元トークン) detail 群の
//     部分集合か (登録スキーマの半順序による)。
//
// いずれも判定不能なら拒否側へ倒す (fail-closed)。
package domain

import (
	"fmt"
	"slices"
	"strings"

	"ra-idp-go/internal/spec"
)

// standardArrayFields は AuthorizationDetail の標準配列フィールド (RFC 9396) を
// スキーマ規則の name から解決するための対応表。
var standardArrayFields = map[string]func(spec.AuthorizationDetail) []string{
	"locations":  func(d spec.AuthorizationDetail) []string { return d.Locations },
	"actions":    func(d spec.AuthorizationDetail) []string { return d.Actions },
	"datatypes":  func(d spec.AuthorizationDetail) []string { return d.Datatypes },
	"privileges": func(d spec.AuthorizationDetail) []string { return d.Privileges },
}

// presentFieldNames は detail に実際に値が入っているフィールド名の集合を返す。
// 標準フィールド (非空) と Fields マップのキーを統合する。type は対象外。
func presentFieldNames(d spec.AuthorizationDetail) map[string]bool {
	present := map[string]bool{}
	for name, get := range standardArrayFields {
		if len(get(d)) > 0 {
			present[name] = true
		}
	}
	if strings.TrimSpace(d.Identifier) != "" {
		present["identifier"] = true
	}
	for k := range d.Fields {
		present[k] = true
	}
	return present
}

// fieldStrings は name が指すフィールドを文字列スライスとして解決する。
// 標準配列フィールド・identifier・Fields マップ内の文字列/配列に対応する。
// ok=false は値が無い (未設定) ことを表す。
func fieldStrings(d spec.AuthorizationDetail, name string) (values []string, ok bool) {
	if get, isStd := standardArrayFields[name]; isStd {
		v := get(d)
		return v, len(v) > 0
	}
	if name == "identifier" {
		if strings.TrimSpace(d.Identifier) == "" {
			return nil, false
		}
		return []string{d.Identifier}, true
	}
	raw, exists := d.Fields[name]
	if !exists || raw == nil {
		return nil, false
	}
	switch v := raw.(type) {
	case string:
		return []string{v}, true
	case []string:
		return v, len(v) > 0
	case []any:
		out := make([]string, 0, len(v))
		for _, e := range v {
			s, isStr := e.(string)
			if !isStr {
				return nil, false // 文字列以外を含む配列は set として扱えない
			}
			out = append(out, s)
		}
		return out, len(out) > 0
	default:
		return nil, false
	}
}

// fieldNumber は name が指すフィールドを数値として解決する (at_most 用)。
// JSON 由来の float64 / int に対応する。ok=false は値無しまたは数値でないこと。
func fieldNumber(d spec.AuthorizationDetail, name string) (value float64, ok bool) {
	raw, exists := d.Fields[name]
	if !exists || raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

// ValidateAgainstType は detail が登録済み type t のスキーマに適合するかを検証する。
// 不適合・未登録フィールド・必須欠落・許可外値・無効値はすべてエラー (fail-closed)。
func ValidateAgainstType(d spec.AuthorizationDetail, t spec.AuthorizationDetailType) error {
	if d.Type == "" {
		return fmt.Errorf("authorization detail type is required")
	}
	if d.Type != t.Type {
		return fmt.Errorf("authorization detail type %q does not match registered type %q", d.Type, t.Type)
	}
	if t.State != spec.DetailTypeEnabled {
		return fmt.Errorf("authorization detail type %q is not enabled", t.Type)
	}

	ruleByName := make(map[string]spec.AuthorizationDetailFieldRule, len(t.Schema.Rules))
	for _, rule := range t.Schema.Rules {
		ruleByName[rule.Name] = rule
	}

	present := presentFieldNames(d)
	// 未登録フィールドの拒否 (fail-closed): スキーマが扱わないフィールドを許さない。
	for name := range present {
		if _, governed := ruleByName[name]; !governed {
			return fmt.Errorf("authorization detail field %q is not allowed by type %q", name, t.Type)
		}
	}

	for _, rule := range t.Schema.Rules {
		isPresent := present[rule.Name]
		if rule.Required && !isPresent {
			return fmt.Errorf("authorization detail field %q is required by type %q", rule.Name, t.Type)
		}
		if !isPresent {
			continue
		}
		if err := validateFieldValue(d, rule, t.Type); err != nil {
			return err
		}
	}
	return nil
}

func validateFieldValue(d spec.AuthorizationDetail, rule spec.AuthorizationDetailFieldRule, typeName string) error {
	switch rule.Semantics {
	case spec.DetailFieldSet, spec.DetailFieldEnum:
		values, ok := fieldStrings(d, rule.Name)
		if !ok {
			return fmt.Errorf("authorization detail field %q must be a string or string array", rule.Name)
		}
		if rule.Semantics == spec.DetailFieldEnum && len(values) != 1 {
			return fmt.Errorf("authorization detail field %q must be a single value", rule.Name)
		}
		if len(rule.Allowed) > 0 {
			for _, v := range values {
				if !slices.Contains(rule.Allowed, v) {
					return fmt.Errorf("authorization detail field %q value %q is not allowed by type %q", rule.Name, v, typeName)
				}
			}
		}
	case spec.DetailFieldAtMost:
		if _, ok := fieldNumber(d, rule.Name); !ok {
			return fmt.Errorf("authorization detail field %q must be a number", rule.Name)
		}
	case spec.DetailFieldExact:
		if _, ok := fieldStrings(d, rule.Name); !ok {
			return fmt.Errorf("authorization detail field %q must be a scalar value", rule.Name)
		}
	default:
		return fmt.Errorf("authorization detail field %q has unknown semantics %q", rule.Name, rule.Semantics)
	}
	return nil
}

// detailFieldSubset は 1 フィールドについて requested が granted の部分集合かを判定する。
func detailFieldSubset(requested, granted spec.AuthorizationDetail, rule spec.AuthorizationDetailFieldRule) bool {
	switch rule.Semantics {
	case spec.DetailFieldSet, spec.DetailFieldEnum:
		reqValues, reqOK := fieldStrings(requested, rule.Name)
		grantValues, grantOK := fieldStrings(granted, rule.Name)
		if !reqOK {
			return true // 要求がそのフィールドを縛らない = 縮小方向、許容
		}
		if !grantOK {
			return false // 基準に無い権限を要求している = 拡大、拒否
		}
		for _, v := range reqValues {
			if !slices.Contains(grantValues, v) {
				return false
			}
		}
		return true
	case spec.DetailFieldAtMost:
		reqNum, reqOK := fieldNumber(requested, rule.Name)
		grantNum, grantOK := fieldNumber(granted, rule.Name)
		if !reqOK {
			return true
		}
		if !grantOK {
			return false
		}
		return reqNum <= grantNum
	case spec.DetailFieldExact:
		reqValues, reqOK := fieldStrings(requested, rule.Name)
		grantValues, grantOK := fieldStrings(granted, rule.Name)
		if !reqOK {
			return true
		}
		if !grantOK {
			return false
		}
		return slices.Equal(reqValues, grantValues)
	default:
		return false // 未知の半順序は拒否側へ
	}
}

// detailSubsetOf は requested detail が granted detail の部分集合かを type の半順序で判定する。
func detailSubsetOf(requested, granted spec.AuthorizationDetail, t spec.AuthorizationDetailType) bool {
	if requested.Type != granted.Type || requested.Type != t.Type {
		return false
	}
	for _, rule := range t.Schema.Rules {
		if !detailFieldSubset(requested, granted, rule) {
			return false
		}
	}
	return true
}

// DetailsSubsetOf は要求 detail 群 requested が、基準 detail 群 granted の部分集合かを
// 検証する。各 requested detail は、同じ type の granted detail のいずれかに包含されねば
// ならない (fail-closed)。types は要求に現れる type の登録定義。
func DetailsSubsetOf(requested, granted []spec.AuthorizationDetail, types map[string]spec.AuthorizationDetailType) error {
	for _, req := range requested {
		t, registered := types[req.Type]
		if !registered {
			return fmt.Errorf("authorization detail type %q is not registered", req.Type)
		}
		covered := false
		for _, grant := range granted {
			if grant.Type != req.Type {
				continue
			}
			if detailSubsetOf(req, grant, t) {
				covered = true
				break
			}
		}
		if !covered {
			return fmt.Errorf("requested authorization detail of type %q exceeds the consented grant", req.Type)
		}
	}
	return nil
}

// DetailTypes は detail 群に現れる type を重複なく返す (監査イベント用)。
func DetailTypes(details []spec.AuthorizationDetail) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(details))
	for _, d := range details {
		if d.Type == "" || seen[d.Type] {
			continue
		}
		seen[d.Type] = true
		out = append(out, d.Type)
	}
	return out
}
