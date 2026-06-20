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

func TestBuiltinAttributeDefsCoverOIDCAndOrg(t *testing.T) {
	defs := BuiltinAttributeDefs()
	byKey := map[string]AttributeDef{}
	for _, d := range defs {
		byKey[d.Key] = d
	}
	if _, ok := byKey["nickname"]; !ok {
		t.Fatal("expected builtin nickname")
	}
	if d := byKey["phone_number"]; d.OIDCScope == nil || *d.OIDCScope != "phone" {
		t.Fatal("phone_number must map to phone scope")
	}
	if d := byKey["title"]; d.Visibility != AttrVisibilityAdminReadable || d.EditableByUser {
		t.Fatal("organization title must be admin-managed")
	}
	// 返却値の変更がカタログ本体に波及しないこと。
	defs[0].Key = "mutated"
	if BuiltinAttributeDefs()[0].Key == "mutated" {
		t.Fatal("BuiltinAttributeDefs must return a copy")
	}
}

func sampleSchema() TenantAttributeSchema {
	return TenantAttributeSchema{
		TenantID: "default",
		Attributes: []AttributeDef{
			{Key: "region", Type: AttributeTypeString, Required: true, Visibility: AttrVisibilityClaimExposed, ClaimName: ptr("region"), PII: false},
			{Key: "tags", Type: AttributeTypeStringArray, MultiValued: true, Visibility: AttrVisibilityAdminReadable, PII: true},
		},
		UpdatedAt: time.Now().UTC(),
	}
}

func TestTenantAttributeSchemaValidate(t *testing.T) {
	if err := sampleSchema().Validate(); err != nil {
		t.Fatalf("expected valid schema, got %v", err)
	}
}

func TestTenantAttributeSchemaRejectsBadKey(t *testing.T) {
	s := sampleSchema()
	s.Attributes[0].Key = "Region-1"
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for non-snake_case key")
	}
}

func TestTenantAttributeSchemaRejectsBuiltinCollision(t *testing.T) {
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
