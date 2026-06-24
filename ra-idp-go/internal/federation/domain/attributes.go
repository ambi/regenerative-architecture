// identity 属性の解決 (wi-61)。
//
// claim 発行エンジン (ADR-059) は protocol-agnostic な属性マップ (Attributes) を入力に取る。
// 本ファイルは User 集約 (標準フィールド + sparse な custom 属性, ADR-040) を、その属性マップへ
// 平坦化する純粋関数を提供する。WS-Fed / WS-Trust / SAML が同じ解決を共有する。
package domain

import (
	"strconv"
	"strings"

	"ra-idp-go/internal/spec"
)

// 標準 User フィールドを指す属性キー。ClaimMappingRule.source_key から参照する。
const (
	AttrSub               = "sub"
	AttrPreferredUsername = "preferred_username"
	AttrEmail             = "email"
	AttrEmailVerified     = "email_verified"
	AttrName              = "name"
	AttrGivenName         = "given_name"
	AttrFamilyName        = "family_name"
	AttrRoles             = "roles"
)

// ResolveUserAttributes は User を claim 発行エンジン向けの属性マップへ平坦化する。
// 標準フィールドを先に入れ、tenant 定義の custom 属性 (ADR-040) で上書きする。
// 値が無いフィールドはキーごと省略する (claim 発行側の属性最小化と整合)。
func ResolveUserAttributes(u spec.User) Attributes {
	attrs := Attributes{}

	put := func(key string, values ...string) {
		filtered := make([]string, 0, len(values))
		for _, v := range values {
			if strings.TrimSpace(v) != "" {
				filtered = append(filtered, v)
			}
		}
		if len(filtered) > 0 {
			attrs[key] = filtered
		}
	}

	put(AttrSub, u.Sub)
	put(AttrPreferredUsername, u.PreferredUsername)
	put(AttrEmailVerified, strconv.FormatBool(u.EmailVerified))
	put(AttrRoles, u.Roles...)
	if u.Email != nil {
		put(AttrEmail, *u.Email)
	}
	if u.Name != nil {
		put(AttrName, *u.Name)
	}
	if u.GivenName != nil {
		put(AttrGivenName, *u.GivenName)
	}
	if u.FamilyName != nil {
		put(AttrFamilyName, *u.FamilyName)
	}

	for key, value := range u.Attributes {
		if values := attributeValueStrings(value); len(values) > 0 {
			attrs[key] = values
		}
	}

	return attrs
}

// attributeValueStrings は AttributeValue (sum type, ADR-040) を文字列スライスへ変換する。
func attributeValueStrings(v spec.AttributeValue) []string {
	switch v.Type {
	case spec.AttributeTypeString:
		if v.String != nil && strings.TrimSpace(*v.String) != "" {
			return []string{*v.String}
		}
	case spec.AttributeTypeStringArray:
		out := make([]string, 0, len(v.StringArray))
		for _, s := range v.StringArray {
			if strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case spec.AttributeTypeNumber:
		if v.Number != nil {
			return []string{strconv.FormatFloat(*v.Number, 'f', -1, 64)}
		}
	case spec.AttributeTypeBoolean:
		if v.Boolean != nil {
			return []string{strconv.FormatBool(*v.Boolean)}
		}
	case spec.AttributeTypeDate:
		if v.Date != nil && strings.TrimSpace(*v.Date) != "" {
			return []string{*v.Date}
		}
	}
	return nil
}
