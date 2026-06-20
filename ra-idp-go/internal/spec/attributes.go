package spec

// 組み込み属性カタログ (wi-19 / ADR-039 / ADR-040)。
//
// core (User の型付きフィールド: sub / preferred_username / name / given_name /
// family_name / email / email_verified / roles / lifecycle) 以外の OIDC §5.1
// optional claim と SCIM enterprise:User 拡張相当の組織属性を、全テナント共通の
// 組み込み属性として定義する。User.Attributes には実際に値を入れた key だけが
// sparse に入るため、使わない属性は保存領域を消費しない。
//
// OIDC `address` Claim (§5.1.1) は構造体ではなく address_* のフラット key に
// 分解して保持し、UserInfo / ID Token 生成時に address オブジェクトへ再構成する
// (claim 生成は後続 PR)。

func sp(s string) *string { return &s }

// builtinDefs は副作用のない不変カタログ。BuiltinAttributeDefs() がコピーを返す。
var builtinDefs = func() []AttributeDef {
	// OIDC `profile` scope で開示する標準クレーム (core にある分を除く)。
	profile := func(key string, t AttributeType) AttributeDef {
		return AttributeDef{
			Key: key, Type: t, EditableByUser: true,
			ClaimName: sp(key), OIDCScope: sp("profile"),
			Visibility: AttrVisibilityClaimExposed, PII: true,
		}
	}
	// OIDC `address` scope。claim 名は親 "address"、key は address_* に分解。
	address := func(key string) AttributeDef {
		return AttributeDef{
			Key: key, Type: AttributeTypeString, EditableByUser: true,
			ClaimName: sp("address"), OIDCScope: sp("address"),
			Visibility: AttrVisibilityClaimExposed, PII: true,
		}
	}
	// SCIM enterprise:User 拡張相当の組織属性。OIDC claim ではなく admin 管理。
	org := func(key string, t AttributeType) AttributeDef {
		return AttributeDef{
			Key: key, Type: t, EditableByUser: false,
			Visibility: AttrVisibilityAdminReadable, PII: false,
		}
	}

	return []AttributeDef{
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

// BuiltinAttributeDefs は全テナント共通の組み込み属性定義のコピーを返す。
func BuiltinAttributeDefs() []AttributeDef {
	out := make([]AttributeDef, len(builtinDefs))
	copy(out, builtinDefs)
	return out
}
