package domain

import (
	"testing"

	"ra-idp-go/internal/shared/spec"
)

func TestNormalizeImmutableID_GUIDUsesMicrosoftByteOrder(t *testing.T) {
	got, err := NormalizeImmutableID("6f9619ff-8b86-d011-b42d-00c04fc964ff")
	if err != nil {
		t.Fatalf("NormalizeImmutableID: %v", err)
	}
	if got != "/xmWb4aLEdC0LQDAT8lk/w==" {
		t.Fatalf("ImmutableID = %q", got)
	}
}

func TestNormalizeImmutableID_AcceptsExistingBase64(t *testing.T) {
	got, err := NormalizeImmutableID("/xmWb4aLEdC0LQDAT8lk/w==")
	if err != nil {
		t.Fatalf("NormalizeImmutableID: %v", err)
	}
	if got != "/xmWb4aLEdC0LQDAT8lk/w==" {
		t.Fatalf("ImmutableID = %q", got)
	}
}

func TestApplyEntraProfileAddsSyntheticImmutableID(t *testing.T) {
	attrs, err := ApplyEntraProfile(Attributes{
		"object_guid": {"6f9619ff-8b86-d011-b42d-00c04fc964ff"},
	}, &spec.EntraFederationProfile{SourceAnchorAttribute: "object_guid"})
	if err != nil {
		t.Fatalf("ApplyEntraProfile: %v", err)
	}
	if got := attrs[EntraImmutableIDAttribute]; len(got) != 1 || got[0] != "/xmWb4aLEdC0LQDAT8lk/w==" {
		t.Fatalf("entra_immutable_id = %#v", got)
	}
}

func TestApplyEntraProfileRejectsMissingSourceAnchor(t *testing.T) {
	if _, err := ApplyEntraProfile(Attributes{}, &spec.EntraFederationProfile{SourceAnchorAttribute: "object_guid"}); err == nil {
		t.Fatal("expected missing sourceAnchor error")
	}
}

func TestBuildEntraClaimPolicy(t *testing.T) {
	policy := BuildEntraClaimPolicy()
	if policy.NameID.Format != EntraPersistentNameIDFormat {
		t.Fatalf("NameID format = %q", policy.NameID.Format)
	}
	if policy.NameID.SourceAttribute != EntraImmutableIDAttribute {
		t.Fatalf("NameID source = %q", policy.NameID.SourceAttribute)
	}
	if len(policy.Rules) != 2 {
		t.Fatalf("rules len = %d", len(policy.Rules))
	}
}
