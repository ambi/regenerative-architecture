package http

import (
	"errors"
	"net/http"
	"slices"
	"time"

	oauthusecases "ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/shared/adapters/crypto"
	"ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/shared/spec"

	"github.com/labstack/echo/v5"
)

type adminClientUpdateRequest struct {
	ClientName      *string              `json:"client_name"`
	RedirectURIs    *[]string            `json:"redirect_uris"`
	GrantTypes      *[]spec.GrantType    `json:"grant_types"`
	ResponseTypes   *[]spec.ResponseType `json:"response_types"`
	Scope           *string              `json:"scope"`
	RequirePAR      *bool                `json:"require_pushed_authorization_requests"`
	DpopBoundTokens *bool                `json:"dpop_bound_access_tokens"`
}

type adminClientResponse struct {
	TenantID                           string                       `json:"tenant_id"`
	ClientID                           string                       `json:"client_id"`
	ClientName                         *string                      `json:"client_name,omitempty"`
	ClientType                         spec.ClientType              `json:"client_type"`
	RedirectURIs                       []string                     `json:"redirect_uris"`
	GrantTypes                         []spec.GrantType             `json:"grant_types"`
	ResponseTypes                      []spec.ResponseType          `json:"response_types"`
	TokenEndpointAuthMethod            spec.TokenEndpointAuthMethod `json:"token_endpoint_auth_method"`
	Scope                              string                       `json:"scope"`
	JWKS                               map[string]any               `json:"jwks,omitempty"`
	JwksURI                            *string                      `json:"jwks_uri,omitempty"`
	TlsClientAuthSubjectDN             *string                      `json:"tls_client_auth_subject_dn,omitempty"`
	IDTokenSignedResponseAlg           spec.SignatureAlgorithm      `json:"id_token_signed_response_alg"`
	RequirePushedAuthorizationRequests bool                         `json:"require_pushed_authorization_requests"`
	DpopBoundAccessTokens              bool                         `json:"dpop_bound_access_tokens"`
	FapiProfile                        spec.FapiProfile             `json:"fapi_profile"`
	CreatedAt                          time.Time                    `json:"created_at"`
}

func (d Deps) handleListAdminOAuth2Clients(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	clients, err := d.ClientRepo.FindAll(c.Request().Context(), support.RequestTenantID(c))
	if err != nil {
		return err
	}
	slices.SortFunc(clients, func(a, b *spec.OAuth2Client) int {
		if a.ClientID < b.ClientID {
			return -1
		}
		if a.ClientID > b.ClientID {
			return 1
		}
		return 0
	})
	response := make([]adminClientResponse, len(clients))
	for i, client := range clients {
		response[i] = toAdminOAuth2ClientResponse(client)
	}
	return support.NoStoreJSON(c, http.StatusOK, map[string]any{"clients": response})
}

func (d Deps) handleGetAdminOAuth2Client(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	client, err := d.ClientRepo.FindByID(
		c.Request().Context(), support.RequestTenantID(c), c.Param("client_id"),
	)
	if err != nil {
		return err
	}
	if client == nil {
		return d.writeAdminOAuth2ClientError(c, oauthusecases.ErrClientNotFound)
	}
	return support.NoStoreJSON(c, http.StatusOK, toAdminOAuth2ClientResponse(client))
}

func (d Deps) handleCreateAdminOAuth2Client(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var req registerClientRequest
	if err := support.DecodeJSON(c.Request(), &req); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if err := validateRegisterClientRequest(&req); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_client_metadata", err.Error())
	}
	if req.JwksURI != nil {
		if err := crypto.ValidateJWKSURI(*req.JwksURI); err != nil {
			return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_client_metadata", err.Error())
		}
	}
	registration := oauthusecases.RegisterClientInput{
		ClientName: req.ClientName, ClientType: spec.ClientType(req.ClientType),
		RedirectURIs: req.RedirectURIs, TokenEndpointAuthMethod: spec.TokenEndpointAuthMethod(req.TokenEndpointAuthMethod),
		Scope: req.Scope, JWKS: req.JWKS, JwksURI: req.JwksURI,
		TlsClientAuthSubjectDN: req.TlsClientAuthSubjectDN, RequirePAR: req.RequirePAR,
		DpopBoundAccessTokens: req.DpopBoundAccessTokens, FapiProfile: spec.FapiProfile(req.FapiProfile),
	}
	for _, grant := range req.GrantTypes {
		registration.GrantTypes = append(registration.GrantTypes, spec.GrantType(grant))
	}
	for _, responseType := range req.ResponseTypes {
		registration.ResponseTypes = append(registration.ResponseTypes, spec.ResponseType(responseType))
	}
	result, err := oauthusecases.CreateAdminOAuth2Client(c.Request().Context(), d.adminClientDeps(), oauthusecases.CreateAdminOAuth2ClientInput{
		ActorSub: actor.Sub, Registration: registration, Now: time.Now().UTC(),
	})
	if err != nil {
		return d.writeAdminOAuth2ClientError(c, err)
	}
	response := map[string]any{"client": toAdminOAuth2ClientResponse(result.Client)}
	if result.ClientSecret != "" {
		response["client_secret"] = result.ClientSecret
	}
	return support.NoStoreJSON(c, http.StatusCreated, response)
}

func (d Deps) handleUpdateAdminOAuth2Client(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var req adminClientUpdateRequest
	if err := support.DecodeJSON(c.Request(), &req); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	client, err := oauthusecases.UpdateAdminOAuth2Client(c.Request().Context(), d.adminClientDeps(), oauthusecases.UpdateAdminOAuth2ClientInput{
		ActorSub: actor.Sub, ClientID: c.Param("client_id"), ClientName: req.ClientName,
		RedirectURIs: req.RedirectURIs, GrantTypes: req.GrantTypes, ResponseTypes: req.ResponseTypes,
		Scope: req.Scope, RequirePAR: req.RequirePAR, DpopBoundTokens: req.DpopBoundTokens,
		Now: time.Now().UTC(),
	})
	if err != nil {
		return d.writeAdminOAuth2ClientError(c, err)
	}
	return support.NoStoreJSON(c, http.StatusOK, toAdminOAuth2ClientResponse(client))
}

func (d Deps) handleDeleteAdminOAuth2Client(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if err := oauthusecases.DeleteAdminOAuth2Client(
		c.Request().Context(), d.adminClientDeps(), actor.Sub, c.Param("client_id"), time.Now().UTC(),
	); err != nil {
		return d.writeAdminOAuth2ClientError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) adminClientDeps() oauthusecases.AdminOAuth2ClientDeps {
	return oauthusecases.AdminOAuth2ClientDeps{ClientRepo: d.ClientRepo, Emit: d.Emit}
}

func (d Deps) writeAdminOAuth2ClientError(c *echo.Context, err error) error {
	if errors.Is(err, oauthusecases.ErrClientNotFound) {
		return support.WriteBrowserError(c, http.StatusNotFound, "client_not_found", "クライアントが存在しません")
	}
	var oauthErr *oauthusecases.OAuthError
	if errors.As(err, &oauthErr) {
		return support.WriteBrowserError(c, http.StatusBadRequest, oauthErr.Code, oauthErr.Description)
	}
	return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_client_metadata", err.Error())
}

func toAdminOAuth2ClientResponse(client *spec.OAuth2Client) adminClientResponse {
	return adminClientResponse{
		TenantID: client.TenantID, ClientID: client.ClientID, ClientName: client.ClientName,
		ClientType: client.ClientType, RedirectURIs: slices.Clone(client.RedirectURIs),
		GrantTypes: slices.Clone(client.GrantTypes), ResponseTypes: slices.Clone(client.ResponseTypes),
		TokenEndpointAuthMethod: client.TokenEndpointAuthMethod, Scope: client.Scope,
		JWKS: client.JWKS, JwksURI: client.JwksURI, TlsClientAuthSubjectDN: client.TlsClientAuthSubjectDN,
		IDTokenSignedResponseAlg:           client.IDTokenSignedResponseAlg,
		RequirePushedAuthorizationRequests: client.RequirePushedAuthorizationRequests,
		DpopBoundAccessTokens:              client.DpopBoundAccessTokens, FapiProfile: client.FapiProfile,
		CreatedAt: client.CreatedAt,
	}
}
