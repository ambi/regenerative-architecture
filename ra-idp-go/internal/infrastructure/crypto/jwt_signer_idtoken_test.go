package crypto

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

func idTokenClaims(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("malformed jwt: %q", token)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatal(err)
	}
	return claims
}

func idTokenTestUser() *spec.User {
	name := "Carol Q"
	nick := "cici"
	phone := "+819012345678"
	return &spec.User{
		Sub: "user-1", TenantID: spec.DefaultTenantID, PreferredUsername: "carol", Name: &name,
		Lifecycle: spec.UserLifecycle{Status: spec.UserStatusActive},
		Attributes: map[string]spec.AttributeValue{
			"nickname":     {Type: spec.AttributeTypeString, String: &nick},
			"phone_number": {Type: spec.AttributeTypeString, String: &phone},
		},
	}
}

func TestSignIDTokenIncludesAttributeClaimsByScope(t *testing.T) {
	ks, err := NewInMemoryKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	signer := NewJWTSigner("https://idp.test", ks)
	resolve := func(_ context.Context, _ string) ([]spec.UserAttributeDef, error) {
		return spec.BuiltinUserAttributeDefs(), nil
	}

	token, err := signer.SignIDToken(context.Background(), ports.IDTokenInput{
		Client: &spec.Client{ClientID: "c1"}, User: idTokenTestUser(),
		Scopes: []string{"openid", "profile", "phone"}, ResolveAttributeDefs: resolve,
	})
	if err != nil {
		t.Fatal(err)
	}
	claims := idTokenClaims(t, token)
	if claims["name"] != "Carol Q" {
		t.Fatalf("standard profile claim missing: %#v", claims)
	}
	if claims["nickname"] != "cici" {
		t.Fatalf("nickname attribute claim missing: %#v", claims)
	}
	if claims["phone_number"] != "+819012345678" {
		t.Fatalf("phone_number attribute claim missing: %#v", claims)
	}
}

func TestSignIDTokenOmitsAttributeClaimsWithoutScope(t *testing.T) {
	ks, err := NewInMemoryKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	signer := NewJWTSigner("https://idp.test", ks)
	resolve := func(_ context.Context, _ string) ([]spec.UserAttributeDef, error) {
		return spec.BuiltinUserAttributeDefs(), nil
	}

	token, err := signer.SignIDToken(context.Background(), ports.IDTokenInput{
		Client: &spec.Client{ClientID: "c1"}, User: idTokenTestUser(),
		Scopes: []string{"openid"}, ResolveAttributeDefs: resolve,
	})
	if err != nil {
		t.Fatal(err)
	}
	claims := idTokenClaims(t, token)
	if _, ok := claims["nickname"]; ok {
		t.Fatalf("nickname leaked without profile scope: %#v", claims)
	}
	if _, ok := claims["phone_number"]; ok {
		t.Fatalf("phone_number leaked without phone scope: %#v", claims)
	}
}
