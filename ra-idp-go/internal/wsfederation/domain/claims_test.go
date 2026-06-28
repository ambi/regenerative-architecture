package domain

import (
	"testing"

	"ra-idp-go/internal/shared/spec"
)

const (
	upnClaim   = "http://schemas.xmlsoap.org/claims/UPN"
	emailClaim = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
	groupClaim = "http://schemas.xmlsoap.org/claims/Group"
	tenantClm  = "https://ra-idp/claims/tenant"
	nameIDClm  = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/nameidentifier"
	persistent = "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"
)

func nameIDOnly() spec.ClaimMappingPolicy {
	return spec.ClaimMappingPolicy{
		NameID: spec.NameIdConfiguration{Format: persistent, SourceAttribute: "object_guid"},
	}
}

func TestIssueClaims_HappyPath(t *testing.T) {
	policy := spec.ClaimMappingPolicy{
		NameID: spec.NameIdConfiguration{Format: persistent, SourceAttribute: "object_guid"},
		Rules: []spec.ClaimMappingRule{
			{ClaimType: upnClaim, Source: spec.ClaimSourceUserAttribute, SourceKey: "upn", Required: true},
			{ClaimType: groupClaim, Source: spec.ClaimSourceUserAttribute, SourceKey: "groups"},
			{ClaimType: tenantClm, Source: spec.ClaimSourceFixed, FixedValue: "contoso"},
			{ClaimType: nameIDClm, Source: spec.ClaimSourceNameID},
		},
	}
	attrs := Attributes{
		"object_guid": {"AAECAwQFBgc="},
		"upn":         {"alice@contoso.com"},
		"groups":      {"admins", "users"},
		// 未マップ属性は出力されないことの検証用。
		"phone": {"+1-555-0100"},
	}

	got, err := IssueClaims(policy, attrs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.NameIDFormat != persistent || got.NameIDValue != "AAECAwQFBgc=" {
		t.Fatalf("NameID = %q/%q, want %q/%q", got.NameIDFormat, got.NameIDValue, persistent, "AAECAwQFBgc=")
	}

	want := map[string][]string{
		upnClaim:   {"alice@contoso.com"},
		groupClaim: {"admins", "users"},
		tenantClm:  {"contoso"},
		nameIDClm:  {"AAECAwQFBgc="},
	}
	if len(got.Claims) != len(want) {
		t.Fatalf("emitted %d claims, want %d: %+v", len(got.Claims), len(want), got.Claims)
	}
	for _, c := range got.Claims {
		w, ok := want[c.ClaimType]
		if !ok {
			t.Fatalf("unexpected claim %q (unmapped attribute leaked?)", c.ClaimType)
		}
		if !equalSlices(c.Values, w) {
			t.Fatalf("claim %q values = %v, want %v", c.ClaimType, c.Values, w)
		}
	}
}

func TestIssueClaims_OptionalMissingIsSkipped(t *testing.T) {
	policy := nameIDOnly()
	policy.Rules = []spec.ClaimMappingRule{
		{ClaimType: emailClaim, Source: spec.ClaimSourceUserAttribute, SourceKey: "email"}, // optional, missing
	}
	attrs := Attributes{"object_guid": {"id-1"}}

	got, err := IssueClaims(policy, attrs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Claims) != 0 {
		t.Fatalf("expected no claims for missing optional source, got %+v", got.Claims)
	}
}

func TestIssueClaims_RequiredMissingIsRejected(t *testing.T) {
	policy := nameIDOnly()
	policy.Rules = []spec.ClaimMappingRule{
		{ClaimType: upnClaim, Source: spec.ClaimSourceUserAttribute, SourceKey: "upn", Required: true},
	}
	attrs := Attributes{"object_guid": {"id-1"}} // upn 欠落

	if _, err := IssueClaims(policy, attrs); err == nil {
		t.Fatal("expected error for missing required claim, got nil")
	}
}

func TestIssueClaims_NameIDSourceMissingIsRejected(t *testing.T) {
	attrs := Attributes{"upn": {"alice@contoso.com"}} // object_guid 欠落
	if _, err := IssueClaims(nameIDOnly(), attrs); err == nil {
		t.Fatal("expected error for missing NameID source, got nil")
	}
}

func TestIssueClaims_EmptyValuesTreatedAsMissing(t *testing.T) {
	policy := nameIDOnly()
	policy.Rules = []spec.ClaimMappingRule{
		{ClaimType: upnClaim, Source: spec.ClaimSourceUserAttribute, SourceKey: "upn"},
	}
	attrs := Attributes{
		"object_guid": {"id-1"},
		"upn":         {"  ", ""}, // 空白のみ → 値なし扱い
	}
	got, err := IssueClaims(policy, attrs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Claims) != 0 {
		t.Fatalf("expected blank values to be dropped, got %+v", got.Claims)
	}
}

func TestIssueClaims_PolicyValidation(t *testing.T) {
	tests := map[string]spec.ClaimMappingPolicy{
		"empty name_id format": {
			NameID: spec.NameIdConfiguration{SourceAttribute: "object_guid"},
		},
		"empty name_id source": {
			NameID: spec.NameIdConfiguration{Format: persistent},
		},
		"empty claim_type": {
			NameID: spec.NameIdConfiguration{Format: persistent, SourceAttribute: "object_guid"},
			Rules:  []spec.ClaimMappingRule{{Source: spec.ClaimSourceFixed, FixedValue: "x"}},
		},
		"unknown source": {
			NameID: spec.NameIdConfiguration{Format: persistent, SourceAttribute: "object_guid"},
			Rules:  []spec.ClaimMappingRule{{ClaimType: upnClaim, Source: "ldap_lookup"}},
		},
		"user_attribute without source_key": {
			NameID: spec.NameIdConfiguration{Format: persistent, SourceAttribute: "object_guid"},
			Rules:  []spec.ClaimMappingRule{{ClaimType: upnClaim, Source: spec.ClaimSourceUserAttribute}},
		},
	}
	attrs := Attributes{"object_guid": {"id-1"}}
	for name, policy := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := IssueClaims(policy, attrs); err == nil {
				t.Fatalf("%s: expected error, got nil", name)
			}
		})
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
