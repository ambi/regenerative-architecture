package http

import (
	"net/url"
	"testing"
)

func TestParseAuthorizeRequestCoercesMaxAge(t *testing.T) {
	input, err := parseAuthorizeRequest(url.Values{
		"client_id":             {"client"},
		"redirect_uri":          {"https://example.com/callback"},
		"response_type":         {"code"},
		"code_challenge":        {"challenge"},
		"code_challenge_method": {"S256"},
		"max_age":               {"60"},
	})
	if err != nil {
		t.Fatalf("parse authorize request: %v", err)
	}
	if input.MaxAge == nil || *input.MaxAge != 60 {
		t.Fatalf("max_age = %v, want 60", input.MaxAge)
	}
}

func TestParseAuthorizeRequestRejectsNegativeMaxAge(t *testing.T) {
	_, err := parseAuthorizeRequest(url.Values{
		"client_id":             {"client"},
		"redirect_uri":          {"https://example.com/callback"},
		"response_type":         {"code"},
		"code_challenge":        {"challenge"},
		"code_challenge_method": {"S256"},
		"max_age":               {"-1"},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateRegisterClientRequestRejectsUnknownGrant(t *testing.T) {
	request := &registerClientRequest{
		RedirectURIs: []string{"https://example.com/callback"},
		GrantTypes:   []string{"password"},
	}
	if err := validateRegisterClientRequest(request); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateRegisterClientRequestRequiresHttpsJwksURI(t *testing.T) {
	request := &registerClientRequest{
		RedirectURIs: []string{"https://example.com/callback"},
		JwksURI:      ptr("http://example.com/jwks.json"),
	}
	if err := validateRegisterClientRequest(request); err == nil {
		t.Fatal("expected jwks_uri validation error")
	}
}

func ptr[T any](v T) *T {
	return &v
}
