package crypto

import (
	"context"
	"testing"

	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

func TestSignAccessTokenIncludesAuthorizationDetails(t *testing.T) {
	ks, err := NewInMemoryKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	signer := NewJWTSigner("https://idp.test", ks)

	token, _, err := signer.SignAccessToken(context.Background(), ports.AccessTokenInput{
		Client: &spec.Client{ClientID: "c1"}, Sub: "user-1", Scopes: []string{"openid"},
		AuthorizationDetails: []spec.AuthorizationDetail{
			{Type: "payment_initiation", Actions: []string{"initiate"}, Fields: map[string]any{"instructedAmount": float64(100)}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	claims := idTokenClaims(t, token)
	details, ok := claims["authorization_details"].([]any)
	if !ok || len(details) != 1 {
		t.Fatalf("authorization_details claim missing or malformed: %#v", claims["authorization_details"])
	}
	first, _ := details[0].(map[string]any)
	if first["type"] != "payment_initiation" {
		t.Fatalf("unexpected detail type: %#v", first)
	}
}

func TestSignAccessTokenOmitsAuthorizationDetailsWhenAbsent(t *testing.T) {
	ks, err := NewInMemoryKeyStore()
	if err != nil {
		t.Fatal(err)
	}
	signer := NewJWTSigner("https://idp.test", ks)
	token, _, err := signer.SignAccessToken(context.Background(), ports.AccessTokenInput{
		Client: &spec.Client{ClientID: "c1"}, Sub: "user-1", Scopes: []string{"openid"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, present := idTokenClaims(t, token)["authorization_details"]; present {
		t.Fatal("authorization_details claim must be omitted when no details are bound")
	}
}
