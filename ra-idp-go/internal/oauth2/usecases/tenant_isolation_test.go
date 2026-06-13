package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

func tenantContext(id string) context.Context {
	return tenancy.WithTenant(context.Background(), &spec.Tenant{
		ID: id, DisplayName: id, Status: spec.TenantStatusActive, CreatedAt: time.Now().UTC(),
	}, "https://idp.example/realms/"+id, "/realms/"+id)
}

func TestAuthorizeCannotResolveAnotherTenantClient(t *testing.T) {
	clients := memory.NewClientRepository()
	clients.Seed(&spec.Client{
		TenantID: spec.DefaultTenantID, ClientID: "web-app", ClientType: spec.ClientPublic,
		RedirectURIs:            []string{"https://app.example/callback"},
		GrantTypes:              []spec.GrantType{spec.GrantAuthorizationCode},
		ResponseTypes:           []spec.ResponseType{spec.ResponseTypeCode},
		TokenEndpointAuthMethod: spec.AuthMethodNone, Scope: "openid",
		IDTokenSignedResponseAlg: spec.SigAlgPS256, FapiProfile: spec.FapiNone,
		CreatedAt: time.Now().UTC(),
	})
	_, err := Authorize(tenantContext("acme"), AuthorizeDeps{
		ClientRepo: clients, RequestStore: memory.NewAuthorizationRequestStore(),
	}, AuthorizeRequestInput{
		ClientID: "web-app", RedirectURI: "https://app.example/callback",
		ResponseType: string(spec.ResponseTypeCode), Scope: "openid",
		CodeChallenge: "challenge", CodeChallengeMethod: string(spec.CodeChallengeMethodS256),
	})
	assertOAuthErrorCode(t, err, "invalid_client")
}

func TestAuthorizationCodeCannotCrossTenantBoundary(t *testing.T) {
	codes := memory.NewAuthorizationCodeStore()
	if err := codes.Save(context.Background(), &spec.AuthorizationCodeRecord{
		Code: "AC1", TenantID: "acme", AuthorizationRequestID: "7856cb4e-7405-4d24-9c04-475cbb13f6f1",
		ClientID: "web-app", Sub: "user", RedirectURI: "https://app.example/callback",
		CodeChallenge: "challenge", CodeChallengeMethod: spec.CodeChallengeMethodS256,
		State: spec.AuthCodeRecordIssued, IssuedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	_, err := ExchangeCodeForToken(tenantContext(spec.DefaultTenantID), ExchangeCodeDeps{
		CodeStore: codes,
	}, ExchangeCodeInput{
		ClientID: "web-app", Code: "AC1", CodeVerifier: "verifier",
		RedirectURI: "https://app.example/callback",
	})
	assertOAuthErrorCode(t, err, "invalid_grant")
}

func assertOAuthErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	oauthErr := &OAuthError{}
	ok := errors.As(err, &oauthErr)
	if !ok || oauthErr.Code != code {
		t.Fatalf("error = %#v, want OAuth code %s", err, code)
	}
}
