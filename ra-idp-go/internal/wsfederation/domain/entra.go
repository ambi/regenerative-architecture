package domain

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"ra-idp-go/internal/shared/spec"
)

const (
	EntraUPNClaim                = "http://schemas.xmlsoap.org/claims/UPN"
	EntraNameIdentifierClaim     = "http://schemas.xmlsoap.org/claims/nameidentifier"
	EntraPersistentNameIDFormat  = "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"
	EntraImmutableIDAttribute    = "entra_immutable_id"
	DefaultEntraSourceAnchorAttr = "object_guid"
	DefaultEntraUPNAttribute     = "preferred_username"
)

// BuildEntraClaimPolicy は Microsoft Entra domain federation 用 claim preset を返す。
func BuildEntraClaimPolicy() spec.ClaimMappingPolicy {
	return spec.ClaimMappingPolicy{
		NameID: spec.NameIdConfiguration{
			Format:          EntraPersistentNameIDFormat,
			SourceAttribute: EntraImmutableIDAttribute,
		},
		Rules: []spec.ClaimMappingRule{
			{ClaimType: EntraUPNClaim, Source: spec.ClaimSourceUserAttribute, SourceKey: DefaultEntraUPNAttribute, Required: true},
			{ClaimType: EntraNameIdentifierClaim, Source: spec.ClaimSourceNameID, Required: true},
		},
	}
}

// ApplyEntraProfile derives the ImmutableID synthetic attribute required by Entra.
func ApplyEntraProfile(attrs Attributes, profile *spec.EntraFederationProfile) (Attributes, error) {
	if profile == nil {
		return attrs, nil
	}
	sourceAttr := strings.TrimSpace(profile.SourceAnchorAttribute)
	if sourceAttr == "" {
		sourceAttr = DefaultEntraSourceAnchorAttr
	}
	raw, ok := firstNonEmpty(attrs[sourceAttr])
	if !ok {
		return nil, fmt.Errorf("entra federation: sourceAnchor attribute %q has no value", sourceAttr)
	}
	immutableID, err := NormalizeImmutableID(raw)
	if err != nil {
		return nil, fmt.Errorf("entra federation: sourceAnchor attribute %q: %w", sourceAttr, err)
	}
	targetAttr := strings.TrimSpace(profile.ImmutableIDAttribute)
	if targetAttr == "" {
		targetAttr = EntraImmutableIDAttribute
	}
	out := make(Attributes, len(attrs)+1)
	for key, values := range attrs {
		out[key] = append([]string(nil), values...)
	}
	out[targetAttr] = []string{immutableID}
	return out, nil
}

// NormalizeImmutableID returns the Azure AD ImmutableID base64 value for a sourceAnchor.
func NormalizeImmutableID(value string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", fmt.Errorf("empty sourceAnchor")
	}
	if guidBytes, ok := parseGUIDToMicrosoftBytes(v); ok {
		return base64.StdEncoding.EncodeToString(guidBytes), nil
	}
	decoded, err := base64.StdEncoding.DecodeString(v)
	if err == nil && len(decoded) > 0 {
		return v, nil
	}
	return "", fmt.Errorf("must be a GUID or base64 ImmutableID")
}

func parseGUIDToMicrosoftBytes(value string) ([]byte, bool) {
	compact := strings.ReplaceAll(strings.TrimSpace(value), "-", "")
	if len(compact) != 32 {
		return nil, false
	}
	raw, err := hex.DecodeString(compact)
	if err != nil || len(raw) != 16 {
		return nil, false
	}
	return []byte{
		raw[3], raw[2], raw[1], raw[0],
		raw[5], raw[4],
		raw[7], raw[6],
		raw[8], raw[9],
		raw[10], raw[11], raw[12], raw[13], raw[14], raw[15],
	}, true
}
