package spec

import (
	"slices"
	"strings"
)

// 組み込み属性カタログ (wi-19 / ADR-039 / ADR-040)。
//
// core (User の型付きフィールド: sub / preferred_username / name / given_name /
// family_name / email / email_verified / roles / lifecycle) 以外の OIDC §5.1
// optional claim と SCIM enterprise:User 拡張相当の組織属性を、全テナント共通の
// 組み込み属性として定義する。User.Attributes には実際に値を入れた key だけが
// sparse に入るため、使わない属性は保存領域を消費しない。
//
// OIDC `address` Claim (§5.1.1) は構造体ではなく address_* のフラット key に
// 分解して保持し、claim 生成時 (ClaimsForScopes) に address オブジェクトへ
// 再構成する。UserInfo は実装済み、ID Token への展開は後続 PR。

func sp(s string) *string { return &s }

// builtinDefs は副作用のない不変カタログ。BuiltinUserAttributeDefs() がコピーを返す。
var builtinDefs = func() []UserAttributeDef {
	// OIDC `profile` scope で開示する標準クレーム (core にある分を除く)。
	profile := func(key string, t AttributeType) UserAttributeDef {
		return UserAttributeDef{
			Key: key, Type: t, EditableByUser: true,
			ClaimName: sp(key), OIDCScope: sp("profile"),
			Visibility: AttrVisibilityClaimExposed, PII: true,
		}
	}
	// OIDC `address` scope。claim 名は親 "address"、key は address_* に分解。
	address := func(key string) UserAttributeDef {
		return UserAttributeDef{
			Key: key, Type: AttributeTypeString, EditableByUser: true,
			ClaimName: sp("address"), OIDCScope: sp("address"),
			Visibility: AttrVisibilityClaimExposed, PII: true,
		}
	}
	// SCIM enterprise:User 拡張相当の組織属性。OIDC claim ではなく admin 管理。
	org := func(key string, t AttributeType) UserAttributeDef {
		return UserAttributeDef{
			Key: key, Type: t, EditableByUser: false,
			Visibility: AttrVisibilityAdminReadable, PII: false,
		}
	}

	return []UserAttributeDef{
		profile("middle_name", AttributeTypeString),
		profile("nickname", AttributeTypeString),
		profile("profile", AttributeTypeString),
		profile("picture", AttributeTypeString),
		profile("website", AttributeTypeString),
		profile("gender", AttributeTypeString),
		profile("birthdate", AttributeTypeDate),
		profile("zoneinfo", AttributeTypeString),
		profile("locale", AttributeTypeString),
		{
			Key: "phone_number", Type: AttributeTypeString, EditableByUser: true,
			ClaimName: sp("phone_number"), OIDCScope: sp("phone"),
			Visibility: AttrVisibilityClaimExposed, PII: true,
		},
		{
			Key: "phone_number_verified", Type: AttributeTypeBoolean, EditableByUser: false,
			ClaimName: sp("phone_number_verified"), OIDCScope: sp("phone"),
			Visibility: AttrVisibilityClaimExposed, PII: false,
		},
		address("address_formatted"),
		address("address_street_address"),
		address("address_locality"),
		address("address_region"),
		address("address_postal_code"),
		address("address_country"),
		org("title", AttributeTypeString),
		org("department", AttributeTypeString),
		org("division", AttributeTypeString),
		org("organization_name", AttributeTypeString),
		org("employee_number", AttributeTypeString),
		org("cost_center", AttributeTypeString),
		org("manager_sub", AttributeTypeString),
		org("hire_date", AttributeTypeDate),
		org("employment_type", AttributeTypeString),
	}
}()

// BuiltinUserAttributeDefs は全テナント共通の組み込み属性定義のコピーを返す。
func BuiltinUserAttributeDefs() []UserAttributeDef {
	out := make([]UserAttributeDef, len(builtinDefs))
	copy(out, builtinDefs)
	return out
}

// ClaimsForScopes は visibility=claim_exposed の属性のうち、要求 scope で開示が
// 許可されたものを OIDC claim の map に変換する (OIDC Core §5.4 / wi-19)。core の
// 型付きフィールド (name / email など) は呼び出し側が別途扱い、本関数は
// User.Attributes に入った sparse 属性だけを対象とする。address_* キーは §5.1.1 の
// 入れ子 address オブジェクトへ再構成する。値が無い属性は出力しない。
func ClaimsForScopes(u User, defs []UserAttributeDef, scopes []string) map[string]any {
	if len(u.Attributes) == 0 {
		return nil
	}
	out := map[string]any{}
	var address map[string]any
	for _, def := range defs {
		if def.Visibility != AttrVisibilityClaimExposed || !scopeExposesDef(def, scopes) {
			continue
		}
		value, ok := u.Attributes[def.Key]
		if !ok {
			continue
		}
		jv := value.JSONValue()
		if jv == nil {
			continue
		}
		if claimName(def) == "address" && strings.HasPrefix(def.Key, "address_") {
			if address == nil {
				address = map[string]any{}
			}
			address[strings.TrimPrefix(def.Key, "address_")] = jv
			continue
		}
		out[claimName(def)] = jv
	}
	if address != nil {
		out["address"] = address
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// scopeExposesDef は def の開示を要求 scope が解禁するかを判定する。OIDC scope を
// 持つ組み込み属性 (profile / phone / address) はその scope、scope を持たない
// tenant custom 属性は custom_attribute scope で解禁する。
func scopeExposesDef(def UserAttributeDef, scopes []string) bool {
	if def.OIDCScope != nil && *def.OIDCScope != "" {
		return slices.Contains(scopes, *def.OIDCScope)
	}
	return slices.Contains(scopes, "custom_attribute")
}

func claimName(def UserAttributeDef) string {
	if def.ClaimName != nil && *def.ClaimName != "" {
		return *def.ClaimName
	}
	return def.Key
}
