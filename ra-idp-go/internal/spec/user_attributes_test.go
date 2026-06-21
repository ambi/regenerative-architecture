package spec

import (
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func TestUserValidateAcceptsAttributes(t *testing.T) {
	u := validUser()
	u.Name = ptr("Alice Q")
	u.Lifecycle = UserLifecycle{
		Status:          UserStatusActive,
		RequiredActions: []RequiredAction{RequiredActionUpdatePassword},
	}
	u.Attributes = map[string]AttributeValue{
		"nickname":     {Type: AttributeTypeString, String: ptr("ally")},
		"region":       {Type: AttributeTypeString, String: ptr("apac")},
		"phone_number": {Type: AttributeTypeString, String: ptr("+819012345678")},
	}
	if err := u.Validate(); err != nil {
		t.Fatalf("expected valid user, got %v", err)
	}
}

func TestUserZeroLifecycleIsActive(t *testing.T) {
	u := validUser() // Lifecycle 未設定
	if !u.IsActive() {
		t.Fatal("zero-value lifecycle must be treated as active")
	}
	if u.IsDeleted() {
		t.Fatal("zero-value lifecycle must not be deleted")
	}
	if err := u.Validate(); err != nil {
		t.Fatalf("zero-value lifecycle must validate, got %v", err)
	}
}

func TestUserStatusReflectsLifecycle(t *testing.T) {
	u := validUser()
	u.Lifecycle.Status = UserStatusDeleted
	if u.IsActive() || !u.IsDeleted() {
		t.Fatal("deleted status must be non-active and deleted")
	}
	u.Lifecycle.Status = UserStatusSuspended
	if u.IsActive() {
		t.Fatal("suspended must be non-active")
	}
}

func TestUserValidateRejectsBadAttributeValue(t *testing.T) {
	u := validUser()
	u.Attributes = map[string]AttributeValue{
		// type と設定フィールドが不一致。
		"region": {Type: AttributeTypeNumber, String: ptr("x")},
	}
	if err := u.Validate(); err == nil {
		t.Fatal("expected error for mismatched attribute value")
	}
}

func TestAttributeValueRequiresSingleMatchingField(t *testing.T) {
	cases := []struct {
		name  string
		value AttributeValue
		valid bool
	}{
		{"string ok", AttributeValue{Type: AttributeTypeString, String: ptr("x")}, true},
		{"number ok", AttributeValue{Type: AttributeTypeNumber, Number: ptr(3.5)}, true},
		{"array ok", AttributeValue{Type: AttributeTypeStringArray, StringArray: []string{"a"}}, true},
		{"type/field mismatch", AttributeValue{Type: AttributeTypeNumber, String: ptr("x")}, false},
		{"two fields", AttributeValue{Type: AttributeTypeString, String: ptr("x"), Boolean: ptr(true)}, false},
		{"no field", AttributeValue{Type: AttributeTypeString}, false},
		{"bad type", AttributeValue{Type: "bogus", String: ptr("x")}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.value.Validate()
			if c.valid && err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
			if !c.valid && err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestBuiltinUserAttributeDefsCoverOIDCAndOrg(t *testing.T) {
	defs := BuiltinUserAttributeDefs()
	byKey := map[string]UserAttributeDef{}
	for _, d := range defs {
		byKey[d.Key] = d
	}
	if _, ok := byKey["nickname"]; !ok {
		t.Fatal("expected builtin nickname")
	}
	if d := byKey["phone_number"]; d.OIDCScope == nil || *d.OIDCScope != "phone" {
		t.Fatal("phone_number must map to phone scope")
	}
	if d := byKey["title"]; d.Visibility != AttrVisibilitySelfReadable || d.EditableByUser {
		t.Fatal("organization title must be self-readable and admin-managed")
	}
	// 返却値の変更がカタログ本体に波及しないこと。
	defs[0].Key = "mutated"
	if BuiltinUserAttributeDefs()[0].Key == "mutated" {
		t.Fatal("BuiltinUserAttributeDefs must return a copy")
	}
}

func sampleSchema() TenantUserAttributeSchema {
	return TenantUserAttributeSchema{
		TenantID: "default",
		Attributes: []UserAttributeDef{
			{Key: "region", Type: AttributeTypeString, Required: true, Visibility: AttrVisibilityClaimExposed, ClaimName: ptr("region"), PII: false},
			{Key: "tags", Type: AttributeTypeStringArray, MultiValued: true, Visibility: AttrVisibilityAdminReadable, PII: true},
		},
		UpdatedAt: time.Now().UTC(),
	}
}

func TestTenantUserAttributeSchemaValidate(t *testing.T) {
	if err := sampleSchema().Validate(); err != nil {
		t.Fatalf("expected valid schema, got %v", err)
	}
}

func TestTenantUserAttributeSchemaRejectsBadKey(t *testing.T) {
	s := sampleSchema()
	s.Attributes[0].Key = "Region-1"
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for non-snake_case key")
	}
}

func TestTenantUserAttributeSchemaRejectsBuiltinCollision(t *testing.T) {
	s := sampleSchema()
	s.Attributes[0].Key = "nickname" // builtin と衝突
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for builtin collision")
	}
}

func TestValidateAttributesEnforcesEffectiveSchema(t *testing.T) {
	s := sampleSchema()
	defs := s.EffectiveDefs()

	ok := map[string]AttributeValue{
		"region":   {Type: AttributeTypeString, String: ptr("apac")},
		"nickname": {Type: AttributeTypeString, String: ptr("ally")}, // builtin
	}
	if err := ValidateAttributes(ok, defs); err != nil {
		t.Fatalf("expected valid values, got %v", err)
	}

	unknown := map[string]AttributeValue{
		"region":  {Type: AttributeTypeString, String: ptr("apac")},
		"unknown": {Type: AttributeTypeString, String: ptr("x")},
	}
	if err := ValidateAttributes(unknown, defs); err == nil {
		t.Fatal("expected error for undefined key")
	}

	if err := ValidateAttributes(map[string]AttributeValue{}, defs); err == nil {
		t.Fatal("expected error for missing required attribute")
	}

	wrongType := map[string]AttributeValue{
		"region": {Type: AttributeTypeNumber, Number: ptr(1.0)},
	}
	if err := ValidateAttributes(wrongType, defs); err == nil {
		t.Fatal("expected error for type mismatch")
	}
}

func TestClaimsForScopesExposesByScope(t *testing.T) {
	u := validUser()
	u.Attributes = map[string]AttributeValue{
		"nickname":     {Type: AttributeTypeString, String: ptr("ally")},
		"phone_number": {Type: AttributeTypeString, String: ptr("+819012345678")},
		"title":        {Type: AttributeTypeString, String: ptr("Engineer")}, // self_readable, never a claim
	}
	defs := BuiltinUserAttributeDefs()

	// profile scope は nickname を開示するが phone scope 無しでは phone_number を出さない。
	claims := ClaimsForScopes(u, defs, []string{"openid", "profile"})
	if claims["nickname"] != "ally" {
		t.Fatalf("nickname not exposed: %#v", claims)
	}
	if _, ok := claims["phone_number"]; ok {
		t.Fatalf("phone_number exposed without phone scope: %#v", claims)
	}
	if _, ok := claims["title"]; ok {
		t.Fatalf("self_readable title must never be a claim: %#v", claims)
	}

	// phone scope を足すと phone_number が出る。
	withPhone := ClaimsForScopes(u, defs, []string{"openid", "profile", "phone"})
	if withPhone["phone_number"] != "+819012345678" {
		t.Fatalf("phone_number not exposed with phone scope: %#v", withPhone)
	}
}

func TestClaimsForScopesReassemblesAddress(t *testing.T) {
	u := validUser()
	u.Attributes = map[string]AttributeValue{
		"address_locality": {Type: AttributeTypeString, String: ptr("Tokyo")},
		"address_country":  {Type: AttributeTypeString, String: ptr("JP")},
	}
	claims := ClaimsForScopes(u, BuiltinUserAttributeDefs(), []string{"openid", "address"})
	addr, ok := claims["address"].(map[string]any)
	if !ok {
		t.Fatalf("address not reassembled into nested object: %#v", claims)
	}
	if addr["locality"] != "Tokyo" || addr["country"] != "JP" {
		t.Fatalf("address fields wrong: %#v", addr)
	}
}

func TestClaimsForScopesCustomAttributeGatedByCustomScope(t *testing.T) {
	u := validUser()
	u.Attributes = map[string]AttributeValue{"region": {Type: AttributeTypeString, String: ptr("apac")}}
	defs := append(BuiltinUserAttributeDefs(), UserAttributeDef{
		Key: "region", Type: AttributeTypeString, Visibility: AttrVisibilityClaimExposed, ClaimName: ptr("region"),
	})

	if c := ClaimsForScopes(u, defs, []string{"openid", "profile"}); c["region"] != nil {
		t.Fatalf("custom attribute exposed without custom_attribute scope: %#v", c)
	}
	c := ClaimsForScopes(u, defs, []string{"openid", "custom_attribute"})
	if c["region"] != "apac" {
		t.Fatalf("custom attribute not exposed with custom_attribute scope: %#v", c)
	}
}

func TestBuiltinUserAttributeDefsHaveLabels(t *testing.T) {
	for _, def := range BuiltinUserAttributeDefs() {
		if def.Label == "" {
			t.Fatalf("builtin attribute %q has no Japanese label", def.Key)
		}
	}
}
