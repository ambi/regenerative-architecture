package domain

import (
	"testing"

	"ra-idp-go/internal/shared/spec"
)

func strptr(s string) *string { return &s }

func TestResolveUserAttributes_StandardFields(t *testing.T) {
	u := spec.User{
		Sub:               "user-1",
		PreferredUsername: "alice",
		Email:             strptr("alice@contoso.com"),
		EmailVerified:     true,
		Name:              strptr("Alice Example"),
		Roles:             []string{"admin", "user"},
	}
	attrs := ResolveUserAttributes(u)

	cases := map[string][]string{
		AttrSub:               {"user-1"},
		AttrPreferredUsername: {"alice"},
		AttrEmail:             {"alice@contoso.com"},
		AttrEmailVerified:     {"true"},
		AttrName:              {"Alice Example"},
		AttrRoles:             {"admin", "user"},
	}
	for key, want := range cases {
		got, ok := attrs[key]
		if !ok {
			t.Fatalf("attribute %q missing", key)
		}
		if len(got) != len(want) {
			t.Fatalf("attribute %q = %v, want %v", key, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("attribute %q = %v, want %v", key, got, want)
			}
		}
	}
	// 未設定の任意フィールドはキーごと省略される。
	if _, ok := attrs[AttrGivenName]; ok {
		t.Fatal("given_name should be omitted when unset")
	}
}

func TestResolveUserAttributes_CustomAttributes(t *testing.T) {
	num := 42.0
	flag := true
	u := spec.User{
		Sub: "user-2",
		Attributes: map[string]spec.AttributeValue{
			"object_guid": {Type: spec.AttributeTypeString, String: strptr("AAECAwQFBgc=")},
			"groups":      {Type: spec.AttributeTypeStringArray, StringArray: []string{"g1", "g2"}},
			"level":       {Type: spec.AttributeTypeNumber, Number: &num},
			"vip":         {Type: spec.AttributeTypeBoolean, Boolean: &flag},
			"blank":       {Type: spec.AttributeTypeString, String: strptr("  ")},
		},
	}
	attrs := ResolveUserAttributes(u)

	if got := attrs["object_guid"]; len(got) != 1 || got[0] != "AAECAwQFBgc=" {
		t.Fatalf("object_guid = %v", got)
	}
	if got := attrs["groups"]; len(got) != 2 {
		t.Fatalf("groups = %v", got)
	}
	if got := attrs["level"]; len(got) != 1 || got[0] != "42" {
		t.Fatalf("level = %v", got)
	}
	if got := attrs["vip"]; len(got) != 1 || got[0] != "true" {
		t.Fatalf("vip = %v", got)
	}
	if _, ok := attrs["blank"]; ok {
		t.Fatal("blank string attribute should be omitted")
	}
}
