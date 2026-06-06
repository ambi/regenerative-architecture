// /register (RFC 7591 Dynamic Client Registration)
package usecases

import (
	"context"
	"time"

	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

type RegisterClientInput struct {
	ClientName              string
	ClientType              spec.ClientType
	RedirectURIs            []string
	GrantTypes              []spec.GrantType
	ResponseTypes           []spec.ResponseType
	TokenEndpointAuthMethod spec.TokenEndpointAuthMethod
	Scope                   string
	JWKS                    map[string]any
	JwksURI                 *string
	RequirePAR              bool
	DpopBoundAccessTokens   bool
	FapiProfile             spec.FapiProfile
}

type RegisterClientResult struct {
	Client       *spec.Client
	ClientSecret string // 平文。出力後は再表示されない (RFC 7591 §3.2.1)
}

type RegisterClientDeps struct {
	ClientRepo ports.ClientRepository
	Emit       func(spec.DomainEvent)
}

func RegisterClient(ctx context.Context, deps RegisterClientDeps, in RegisterClientInput, now time.Time) (*RegisterClientResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if in.ClientType == "" {
		in.ClientType = spec.ClientConfidential
	}
	if len(in.RedirectURIs) == 0 {
		return nil, NewOAuthError("invalid_redirect_uri", "redirect_uris is required")
	}
	if len(in.GrantTypes) == 0 {
		in.GrantTypes = []spec.GrantType{spec.GrantAuthorizationCode}
	}
	if len(in.ResponseTypes) == 0 {
		in.ResponseTypes = []spec.ResponseType{spec.ResponseTypeCode}
	}
	if in.TokenEndpointAuthMethod == "" {
		if in.ClientType == spec.ClientPublic {
			in.TokenEndpointAuthMethod = spec.AuthMethodNone
		} else {
			in.TokenEndpointAuthMethod = spec.AuthMethodClientSecretBasic
		}
	}
	if in.TokenEndpointAuthMethod == spec.AuthMethodPrivateKeyJwt {
		candidate := spec.Client{
			ClientID: "validation", ClientType: in.ClientType,
			RedirectURIs:             []string{"https://validation.invalid/callback"},
			GrantTypes:               []spec.GrantType{spec.GrantClientCredentials},
			TokenEndpointAuthMethod:  spec.AuthMethodPrivateKeyJwt,
			JWKS:                     in.JWKS,
			JwksURI:                  in.JwksURI,
			IDTokenSignedResponseAlg: spec.SigAlgPS256,
			FapiProfile:              spec.FapiNone,
			CreatedAt:                now,
		}
		if err := candidate.Validate(); err != nil {
			return nil, NewOAuthError("invalid_client_metadata", "private_key_jwt requires non-empty inline jwks")
		}
	}
	id, err := generateOpaqueToken(16)
	if err != nil {
		return nil, err
	}
	clientID := "c_" + id
	var secret string
	var secretHash *string
	usesSecret := in.TokenEndpointAuthMethod == spec.AuthMethodClientSecretBasic ||
		in.TokenEndpointAuthMethod == spec.AuthMethodClientSecretPost
	if in.ClientType == spec.ClientConfidential && usesSecret {
		s, err := generateOpaqueToken(32)
		if err != nil {
			return nil, err
		}
		secret = s
		ss := domain.HashClientSecret(s)
		secretHash = &ss
	}
	fapiProfile := in.FapiProfile
	if fapiProfile == "" {
		fapiProfile = spec.FapiNone
	}
	scope := in.Scope
	if scope == "" {
		scope = "openid profile email"
	}
	c := &spec.Client{
		ClientID:                           clientID,
		ClientSecretHash:                   secretHash,
		ClientType:                         in.ClientType,
		RedirectURIs:                       in.RedirectURIs,
		GrantTypes:                         in.GrantTypes,
		ResponseTypes:                      in.ResponseTypes,
		TokenEndpointAuthMethod:            in.TokenEndpointAuthMethod,
		Scope:                              scope,
		JWKS:                               in.JWKS,
		JwksURI:                            in.JwksURI,
		IDTokenSignedResponseAlg:           spec.SigAlgPS256,
		RequirePushedAuthorizationRequests: in.RequirePAR,
		DpopBoundAccessTokens:              in.DpopBoundAccessTokens,
		FapiProfile:                        fapiProfile,
		CreatedAt:                          now,
	}
	if in.ClientName != "" {
		name := in.ClientName
		c.ClientName = &name
	}
	if err := c.Validate(); err != nil {
		return nil, NewOAuthError("invalid_client_metadata", err.Error())
	}
	if err := deps.ClientRepo.Save(ctx, c); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.ClientRegistered{At: now, ClientID: clientID, ClientType: in.ClientType})
	return &RegisterClientResult{Client: c, ClientSecret: secret}, nil
}
