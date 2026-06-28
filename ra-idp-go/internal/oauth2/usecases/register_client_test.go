package usecases

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/spec"
)

func TestRegisterClientHashesSecret(t *testing.T) {
	repo := memory.NewClientRepository()
	result, err := RegisterClient(context.Background(), RegisterClientDeps{ClientRepo: repo}, RegisterClientInput{
		ClientType:              spec.ClientConfidential,
		RedirectURIs:            []string{"https://client.example/cb"},
		TokenEndpointAuthMethod: spec.AuthMethodClientSecretBasic,
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if result.ClientSecret == "" || result.Client.ClientSecretHash == nil {
		t.Fatal("client secret was not issued")
	}
	if *result.Client.ClientSecretHash == result.ClientSecret {
		t.Fatal("client secret was stored in plaintext")
	}
	if !domain.VerifyClientSecret(result.ClientSecret, *result.Client.ClientSecretHash) {
		t.Fatal("stored client secret hash does not verify")
	}
}

func TestRegisterPrivateKeyJWTRequiresInlineJWKS(t *testing.T) {
	repo := memory.NewClientRepository()
	_, err := RegisterClient(context.Background(), RegisterClientDeps{ClientRepo: repo}, RegisterClientInput{
		ClientType:              spec.ClientConfidential,
		RedirectURIs:            []string{"https://client.example/cb"},
		TokenEndpointAuthMethod: spec.AuthMethodPrivateKeyJwt,
	}, time.Now())
	if err == nil {
		t.Fatal("expected missing jwks rejection")
	}
}
