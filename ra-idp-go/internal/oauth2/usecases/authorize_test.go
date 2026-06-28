package usecases

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/spec"
)

func newAuthorizeDeps(requirePAR bool) AuthorizeDeps {
	repo := memory.NewClientRepository()
	repo.Seed(&spec.OAuth2Client{
		ClientID: "client", ClientType: spec.ClientPublic,
		RedirectURIs: []string{"https://client.example/cb"},
		GrantTypes:   []spec.GrantType{spec.GrantAuthorizationCode},
		ResponseTypes: []spec.ResponseType{
			spec.ResponseTypeCode,
		},
		TokenEndpointAuthMethod:            spec.AuthMethodNone,
		Scope:                              "openid profile",
		IDTokenSignedResponseAlg:           spec.SigAlgPS256,
		RequirePushedAuthorizationRequests: requirePAR,
		FapiProfile:                        spec.FapiNone,
		CreatedAt:                          time.Now(),
	})
	return AuthorizeDeps{
		ClientRepo: repo, RequestStore: memory.NewAuthorizationRequestStore(),
	}
}

func validAuthorizeInput() AuthorizeRequestInput {
	return AuthorizeRequestInput{
		ClientID: "client", RedirectURI: "https://client.example/cb",
		ResponseType: "code", Scope: "openid",
		CodeChallenge: "challenge", CodeChallengeMethod: "S256",
	}
}

func TestAuthorizeRejectsUndeclaredScope(t *testing.T) {
	in := validAuthorizeInput()
	in.Scope = "openid admin"
	if _, err := Authorize(context.Background(), newAuthorizeDeps(false), in); err == nil {
		t.Fatal("expected invalid_scope")
	}
}

func TestAuthorizeRequiresPARWhenConfigured(t *testing.T) {
	in := validAuthorizeInput()
	if _, err := Authorize(context.Background(), newAuthorizeDeps(true), in); err == nil {
		t.Fatal("expected PAR requirement rejection")
	}
	in.ParUsed = true
	in.ParRequestURI = "urn:ietf:params:oauth:request_uri:test"
	if _, err := Authorize(context.Background(), newAuthorizeDeps(true), in); err != nil {
		t.Fatalf("PAR request rejected: %v", err)
	}
}

func TestAuthorizePersistsPromptAndMaxAge(t *testing.T) {
	in := validAuthorizeInput()
	in.Prompt = "login"
	maxAge := 30
	in.MaxAge = &maxAge
	out, err := Authorize(context.Background(), newAuthorizeDeps(false), in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Request.Prompt == nil || *out.Request.Prompt != "login" {
		t.Fatal("prompt was not persisted")
	}
	if out.Request.MaxAge == nil || *out.Request.MaxAge != maxAge {
		t.Fatal("max_age was not persisted")
	}
}
