// アプリケーションの一括プロビジョニング (wi-69)。Okta / Entra のように、種別を選んで
// プロトコル設定もまとめて入力すると、backend が OAuth2 client / WS-Fed RP を作成し、
// Application と protocol binding を一括で作る。OAuth2/WS-Fed の wire 設定は各 protocol
// context が所有し、本ハンドラは adapter として両者を合成する。
package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	appusecases "ra-idp-go/internal/application/usecases"
	oauthusecases "ra-idp-go/internal/oauth2/usecases"
	samldomain "ra-idp-go/internal/saml/domain"
	"ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/shared/spec"

	"github.com/labstack/echo/v5"
)

const (
	defaultOIDCScope    = "openid profile email"
	defaultServiceScope = "openid"
	defaultNameIDFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified"
	defaultNameIDSource = "sub"
)

type createApplicationRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // oidc | wsfed | weblink | service
	IconURL   string `json:"icon_url"`
	LaunchURL string `json:"launch_url"`
	// OIDC
	RedirectURIs []string `json:"redirect_uris"`
	// OIDC / service の生成 client 設定。auth 方式は作成時に確定し以後不変。
	Scope                   string                       `json:"scope"`
	ClientType              spec.ClientType              `json:"client_type"`
	TokenEndpointAuthMethod spec.TokenEndpointAuthMethod `json:"token_endpoint_auth_method"`
	JwksURI                 string                       `json:"jwks_uri"`
	TLSClientAuthSubjectDN  string                       `json:"tls_client_auth_subject_dn"`
	// WS-Federation
	Wtrealm      string   `json:"wtrealm"`
	ReplyURLs    []string `json:"reply_urls"`
	NameIDFormat string   `json:"name_id_format"`
	NameIDSource string   `json:"name_id_source"`
	// SAML 2.0
	EntityID                          string   `json:"entity_id"`
	ACSURLs                           []string `json:"acs_urls"`
	SLOURL                            string   `json:"slo_url"`
	SignResponse                      bool     `json:"sign_response"`
	WantAuthnRequestsSigned           bool     `json:"want_authn_requests_signed"`
	AuthnRequestSigningCertificatePEM string   `json:"authn_request_signing_certificate_pem"`
}

// oidcConfig / wsfedConfig はアプリ詳細に解決して返す protocol 設定。
// advanced 項目を含めてアプリ編集画面に集約する (wi-76, ADR-066)。
// ClientType / TokenEndpointAuthMethod / FapiProfile は更新契約上の不変項目で表示専用。
type oidcConfig struct {
	ClientID                string                       `json:"client_id"`
	ClientType              spec.ClientType              `json:"client_type"`
	RedirectURIs            []string                     `json:"redirect_uris"`
	GrantTypes              []spec.GrantType             `json:"grant_types"`
	ResponseTypes           []spec.ResponseType          `json:"response_types"`
	TokenEndpointAuthMethod spec.TokenEndpointAuthMethod `json:"token_endpoint_auth_method"`
	Scope                   string                       `json:"scope"`
	RequirePAR              bool                         `json:"require_pushed_authorization_requests"`
	DpopBoundAccessTokens   bool                         `json:"dpop_bound_access_tokens"`
	FapiProfile             spec.FapiProfile             `json:"fapi_profile"`
}

type wsfedConfig struct {
	Wtrealm      string                  `json:"wtrealm"`
	ReplyURLs    []string                `json:"reply_urls"`
	Audience     string                  `json:"audience"`
	TokenType    spec.WsFedTokenType     `json:"token_type"`
	NameIDFormat string                  `json:"name_id_format"`
	NameIDSource string                  `json:"name_id_source"`
	Rules        []spec.ClaimMappingRule `json:"rules"`
}

type samlConfig struct {
	EntityID                          string                  `json:"entity_id"`
	ACSURLs                           []string                `json:"acs_urls"`
	SLOURL                            string                  `json:"slo_url"`
	Audience                          string                  `json:"audience"`
	NameIDFormat                      string                  `json:"name_id_format"`
	NameIDSource                      string                  `json:"name_id_source"`
	SignAssertion                     bool                    `json:"sign_assertion"`
	SignResponse                      bool                    `json:"sign_response"`
	WantAuthnRequestsSigned           bool                    `json:"want_authn_requests_signed"`
	AuthnRequestSigningCertificatePEM string                  `json:"authn_request_signing_certificate_pem"`
	Rules                             []spec.ClaimMappingRule `json:"rules"`
}

// nonNilRules は nil スライスを空スライスに正規化する。claim 規則を持たない RP/SP の
// JSON が null ではなく [] になり、UI 側の .length 参照が安全になる。
func nonNilRules(rules []spec.ClaimMappingRule) []spec.ClaimMappingRule {
	if rules == nil {
		return []spec.ClaimMappingRule{}
	}
	return rules
}

func (d Deps) handleCreateApplication(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var req createApplicationRequest
	if err := support.DecodeJSON(c.Request(), &req); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	ctx := c.Request().Context()
	now := time.Now().UTC()

	switch req.Type {
	case "weblink":
		app, err := appusecases.CreateApplication(ctx, d.applicationDeps(), appusecases.CreateApplicationInput{
			ActorSub: actor.Sub, Name: req.Name, Kind: spec.ApplicationWeblink,
			IconURL: req.IconURL, LaunchURL: req.LaunchURL, Now: now,
		})
		if err != nil {
			return d.writeApplicationError(c, err)
		}
		return support.NoStoreJSON(c, http.StatusCreated, map[string]any{"application": toApplicationResponse(app)})

	case "oidc":
		if len(req.RedirectURIs) == 0 {
			return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "リダイレクト URI を 1 つ以上指定してください")
		}
		registration := oauthusecases.RegisterClientInput{
			ClientName: req.Name, ClientType: req.ClientType, RedirectURIs: req.RedirectURIs,
			GrantTypes:              []spec.GrantType{spec.GrantAuthorizationCode, spec.GrantRefreshToken},
			ResponseTypes:           []spec.ResponseType{spec.ResponseTypeCode},
			TokenEndpointAuthMethod: req.TokenEndpointAuthMethod, Scope: nonEmpty(req.Scope, defaultOIDCScope),
		}
		if dn := strings.TrimSpace(req.TLSClientAuthSubjectDN); dn != "" {
			registration.TlsClientAuthSubjectDN = &dn
		}
		if uri := strings.TrimSpace(req.JwksURI); uri != "" {
			registration.JwksURI = &uri
		}
		result, err := oauthusecases.CreateAdminOAuth2Client(ctx, oauthusecases.AdminOAuth2ClientDeps{ClientRepo: d.ClientRepo, Emit: d.Emit}, oauthusecases.CreateAdminOAuth2ClientInput{
			ActorSub:     actor.Sub,
			Registration: registration,
			Now:          now,
		})
		if err != nil {
			return d.writeApplicationError(c, err)
		}
		app, err := d.createCatalogApp(ctx, actor.Sub, req, now, spec.ApplicationFederated,
			spec.ProtocolBinding{Type: spec.ProtocolBindingOIDC, ClientID: result.Client.ClientID})
		if err != nil {
			return d.writeApplicationError(c, err)
		}
		return support.NoStoreJSON(c, http.StatusCreated, map[string]any{
			"application": toApplicationResponse(app), "client_id": result.Client.ClientID, "client_secret": result.ClientSecret,
		})

	case "service":
		// M2M / サービスクライアント (client_credentials)。redirect を持たず、ポータルにも
		// 出さない service kind の Application として登録する (Okta の API Services 相当)。
		result, err := oauthusecases.CreateAdminOAuth2Client(ctx, oauthusecases.AdminOAuth2ClientDeps{ClientRepo: d.ClientRepo, Emit: d.Emit}, oauthusecases.CreateAdminOAuth2ClientInput{
			ActorSub: actor.Sub,
			Registration: oauthusecases.RegisterClientInput{
				ClientName: req.Name, ClientType: spec.ClientConfidential,
				GrantTypes:              []spec.GrantType{spec.GrantClientCredentials},
				TokenEndpointAuthMethod: spec.AuthMethodClientSecretBasic, Scope: nonEmpty(req.Scope, defaultServiceScope),
			},
			Now: now,
		})
		if err != nil {
			return d.writeApplicationError(c, err)
		}
		app, err := d.createCatalogApp(ctx, actor.Sub, req, now, spec.ApplicationService,
			spec.ProtocolBinding{Type: spec.ProtocolBindingOIDC, ClientID: result.Client.ClientID})
		if err != nil {
			return d.writeApplicationError(c, err)
		}
		return support.NoStoreJSON(c, http.StatusCreated, map[string]any{
			"application": toApplicationResponse(app), "client_id": result.Client.ClientID, "client_secret": result.ClientSecret,
		})

	case "wsfed":
		if strings.TrimSpace(req.Wtrealm) == "" || len(req.ReplyURLs) == 0 {
			return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "wtrealm と reply URL を指定してください")
		}
		rp := &spec.WsFedRelyingParty{
			TenantID: support.RequestTenantID(c), Wtrealm: req.Wtrealm, DisplayName: req.Name, ReplyURLs: req.ReplyURLs,
			ClaimPolicy: spec.ClaimMappingPolicy{NameID: spec.NameIdConfiguration{
				Format: nonEmpty(req.NameIDFormat, defaultNameIDFormat), SourceAttribute: nonEmpty(req.NameIDSource, defaultNameIDSource),
			}},
			CreatedAt: now,
		}
		if d.WsFedRPRepo == nil {
			return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "WS-Federation は利用できません")
		}
		if err := d.WsFedRPRepo.Save(ctx, rp); err != nil {
			return err
		}
		app, err := d.createCatalogApp(ctx, actor.Sub, req, now, spec.ApplicationFederated,
			spec.ProtocolBinding{Type: spec.ProtocolBindingWsFed, Wtrealm: req.Wtrealm})
		if err != nil {
			return d.writeApplicationError(c, err)
		}
		return support.NoStoreJSON(c, http.StatusCreated, map[string]any{"application": toApplicationResponse(app)})

	case "saml":
		if strings.TrimSpace(req.EntityID) == "" || len(req.ACSURLs) == 0 {
			return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "entity ID と ACS URL を指定してください")
		}
		if d.SamlSPRepo == nil {
			return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "SAML は利用できません")
		}
		if req.WantAuthnRequestsSigned {
			if _, err := samldomain.ParseCertificatePEM(req.AuthnRequestSigningCertificatePEM); err != nil {
				return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "AuthnRequest 署名検証用証明書を指定してください")
			}
		}
		sp := &spec.SamlServiceProvider{
			TenantID: support.RequestTenantID(c), EntityID: req.EntityID, DisplayName: req.Name,
			ACSURLs: req.ACSURLs, SLOURL: strings.TrimSpace(req.SLOURL),
			ClaimPolicy: spec.ClaimMappingPolicy{NameID: spec.NameIdConfiguration{
				Format: nonEmpty(req.NameIDFormat, spec.SamlNameIDFormatPersistent), SourceAttribute: nonEmpty(req.NameIDSource, defaultNameIDSource),
			}},
			SignAssertion: true, SignResponse: req.SignResponse,
			WantAuthnRequestsSigned:           req.WantAuthnRequestsSigned,
			AuthnRequestSigningCertificatePEM: strings.TrimSpace(req.AuthnRequestSigningCertificatePEM),
			CreatedAt:                         now,
		}
		if err := d.SamlSPRepo.Save(ctx, sp); err != nil {
			return err
		}
		app, err := d.createCatalogApp(ctx, actor.Sub, req, now, spec.ApplicationFederated,
			spec.ProtocolBinding{Type: spec.ProtocolBindingSAML, EntityID: req.EntityID})
		if err != nil {
			return d.writeApplicationError(c, err)
		}
		return support.NoStoreJSON(c, http.StatusCreated, map[string]any{"application": toApplicationResponse(app)})

	default:
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "種別は oidc / wsfed / saml / weblink のいずれかです")
	}
}

// createCatalogApp は指定 kind の Application を作成し、protocol binding を接続する。
func (d Deps) createCatalogApp(ctx context.Context, actorSub string, req createApplicationRequest, now time.Time, kind spec.ApplicationKind, binding spec.ProtocolBinding) (*spec.Application, error) {
	app, err := appusecases.CreateApplication(ctx, d.applicationDeps(), appusecases.CreateApplicationInput{
		ActorSub: actorSub, Name: req.Name, Kind: kind, IconURL: req.IconURL, LaunchURL: req.LaunchURL, Now: now,
	})
	if err != nil {
		return nil, err
	}
	return appusecases.AttachBinding(ctx, d.applicationDeps(), appusecases.AttachBindingInput{
		ActorSub: actorSub, ApplicationID: app.ApplicationID, Binding: binding, Now: now,
	})
}

// resolveProtocolConfig は Application の binding から OAuth2 client / WS-Fed RP の
// 実設定を解決して返す (アプリ詳細表示用)。
func (d Deps) resolveProtocolConfig(c *echo.Context, app *spec.Application) (*oidcConfig, *wsfedConfig, *samlConfig) {
	ctx := c.Request().Context()
	tenantID := support.RequestTenantID(c)
	var oidc *oidcConfig
	var wsfed *wsfedConfig
	var saml *samlConfig
	for _, binding := range app.Bindings {
		switch binding.Type {
		case spec.ProtocolBindingOIDC:
			if d.ClientRepo == nil {
				continue
			}
			if client, err := d.ClientRepo.FindByID(ctx, tenantID, binding.ClientID); err == nil && client != nil {
				oidc = &oidcConfig{
					ClientID: client.ClientID, ClientType: client.ClientType, RedirectURIs: client.RedirectURIs,
					GrantTypes: client.GrantTypes, ResponseTypes: client.ResponseTypes,
					TokenEndpointAuthMethod: client.TokenEndpointAuthMethod, Scope: client.Scope,
					RequirePAR:            client.RequirePushedAuthorizationRequests,
					DpopBoundAccessTokens: client.DpopBoundAccessTokens, FapiProfile: client.FapiProfile,
				}
			}
		case spec.ProtocolBindingWsFed:
			if d.WsFedRPRepo == nil {
				continue
			}
			if rp, err := d.WsFedRPRepo.FindByWtrealm(ctx, tenantID, binding.Wtrealm); err == nil && rp != nil {
				wsfed = &wsfedConfig{
					Wtrealm: rp.Wtrealm, ReplyURLs: rp.ReplyURLs,
					Audience: rp.Audience, TokenType: rp.EffectiveTokenType(),
					NameIDFormat: rp.ClaimPolicy.NameID.Format, NameIDSource: rp.ClaimPolicy.NameID.SourceAttribute,
					Rules: nonNilRules(rp.ClaimPolicy.Rules),
				}
			}
		case spec.ProtocolBindingSAML:
			if d.SamlSPRepo == nil {
				continue
			}
			if sp, err := d.SamlSPRepo.FindByEntityID(ctx, tenantID, binding.EntityID); err == nil && sp != nil {
				saml = &samlConfig{
					EntityID: sp.EntityID, ACSURLs: sp.ACSURLs, SLOURL: sp.SLOURL,
					Audience: sp.Audience, NameIDFormat: sp.ClaimPolicy.NameID.Format,
					NameIDSource:  sp.ClaimPolicy.NameID.SourceAttribute,
					SignAssertion: sp.SignAssertion, SignResponse: sp.SignResponse,
					WantAuthnRequestsSigned:           sp.WantAuthnRequestsSigned,
					AuthnRequestSigningCertificatePEM: sp.AuthnRequestSigningCertificatePEM,
					Rules:                             nonNilRules(sp.ClaimPolicy.Rules),
				}
			}
		}
	}
	return oidc, wsfed, saml
}

type updateOIDCRequest struct {
	RedirectURIs    *[]string            `json:"redirect_uris"`
	GrantTypes      *[]spec.GrantType    `json:"grant_types"`
	ResponseTypes   *[]spec.ResponseType `json:"response_types"`
	Scope           *string              `json:"scope"`
	RequirePAR      *bool                `json:"require_pushed_authorization_requests"`
	DpopBoundTokens *bool                `json:"dpop_bound_access_tokens"`
}

func (d Deps) handleUpdateOIDCConfig(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	app, err := d.requireApp(c)
	if err != nil {
		return d.writeApplicationError(c, err)
	}
	clientID := bindingKeyOf(app, spec.ProtocolBindingOIDC)
	if clientID == "" {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "OIDC バインディングがありません")
	}
	var req updateOIDCRequest
	if err := support.DecodeJSON(c.Request(), &req); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if _, err := oauthusecases.UpdateAdminOAuth2Client(c.Request().Context(), oauthusecases.AdminOAuth2ClientDeps{ClientRepo: d.ClientRepo, Emit: d.Emit}, oauthusecases.UpdateAdminOAuth2ClientInput{
		ActorSub: actor.Sub, ClientID: clientID,
		RedirectURIs: req.RedirectURIs, GrantTypes: req.GrantTypes, ResponseTypes: req.ResponseTypes,
		Scope: req.Scope, RequirePAR: req.RequirePAR, DpopBoundTokens: req.DpopBoundTokens,
		Now: time.Now().UTC(),
	}); err != nil {
		return d.writeApplicationError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

type updateWsFedRequest struct {
	ReplyURLs    *[]string                `json:"reply_urls"`
	Audience     *string                  `json:"audience"`
	TokenType    *spec.WsFedTokenType     `json:"token_type"`
	NameIDFormat *string                  `json:"name_id_format"`
	NameIDSource *string                  `json:"name_id_source"`
	Rules        *[]spec.ClaimMappingRule `json:"rules"`
}

func (d Deps) handleUpdateWsFedConfig(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	app, err := d.requireApp(c)
	if err != nil {
		return d.writeApplicationError(c, err)
	}
	wtrealm := bindingKeyOf(app, spec.ProtocolBindingWsFed)
	if wtrealm == "" || d.WsFedRPRepo == nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "WS-Federation バインディングがありません")
	}
	ctx := c.Request().Context()
	rp, err := d.WsFedRPRepo.FindByWtrealm(ctx, support.RequestTenantID(c), wtrealm)
	if err != nil || rp == nil {
		return support.WriteBrowserError(c, http.StatusNotFound, "not_found", "relying party が存在しません")
	}
	var req updateWsFedRequest
	if err := support.DecodeJSON(c.Request(), &req); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if req.ReplyURLs != nil {
		rp.ReplyURLs = *req.ReplyURLs
	}
	if req.Audience != nil {
		rp.Audience = strings.TrimSpace(*req.Audience)
	}
	if req.TokenType != nil {
		if *req.TokenType != "" && !req.TokenType.Valid() {
			return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "token_type が不正です")
		}
		rp.TokenType = *req.TokenType
	}
	if req.NameIDFormat != nil {
		rp.ClaimPolicy.NameID.Format = *req.NameIDFormat
	}
	if req.NameIDSource != nil {
		rp.ClaimPolicy.NameID.SourceAttribute = *req.NameIDSource
	}
	if req.Rules != nil {
		rp.ClaimPolicy.Rules = *req.Rules
	}
	now := time.Now().UTC()
	rp.UpdatedAt = &now
	if err := d.WsFedRPRepo.Save(ctx, rp); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type updateSamlRequest struct {
	ACSURLs                           *[]string                `json:"acs_urls"`
	SLOURL                            *string                  `json:"slo_url"`
	Audience                          *string                  `json:"audience"`
	NameIDFormat                      *string                  `json:"name_id_format"`
	NameIDSource                      *string                  `json:"name_id_source"`
	SignAssertion                     *bool                    `json:"sign_assertion"`
	SignResponse                      *bool                    `json:"sign_response"`
	WantAuthnRequestsSigned           *bool                    `json:"want_authn_requests_signed"`
	AuthnRequestSigningCertificatePEM *string                  `json:"authn_request_signing_certificate_pem"`
	Rules                             *[]spec.ClaimMappingRule `json:"rules"`
}

func (d Deps) handleUpdateSamlConfig(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	app, err := d.requireApp(c)
	if err != nil {
		return d.writeApplicationError(c, err)
	}
	entityID := bindingKeyOf(app, spec.ProtocolBindingSAML)
	if entityID == "" || d.SamlSPRepo == nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "SAML バインディングがありません")
	}
	ctx := c.Request().Context()
	sp, err := d.SamlSPRepo.FindByEntityID(ctx, support.RequestTenantID(c), entityID)
	if err != nil || sp == nil {
		return support.WriteBrowserError(c, http.StatusNotFound, "not_found", "service provider が存在しません")
	}
	var req updateSamlRequest
	if err := support.DecodeJSON(c.Request(), &req); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if req.ACSURLs != nil {
		sp.ACSURLs = *req.ACSURLs
	}
	if req.SLOURL != nil {
		sp.SLOURL = strings.TrimSpace(*req.SLOURL)
	}
	if req.Audience != nil {
		sp.Audience = strings.TrimSpace(*req.Audience)
	}
	if req.NameIDFormat != nil {
		sp.ClaimPolicy.NameID.Format = *req.NameIDFormat
	}
	if req.NameIDSource != nil {
		sp.ClaimPolicy.NameID.SourceAttribute = *req.NameIDSource
	}
	if req.SignAssertion != nil {
		sp.SignAssertion = *req.SignAssertion
	}
	if req.SignResponse != nil {
		sp.SignResponse = *req.SignResponse
	}
	if req.WantAuthnRequestsSigned != nil {
		sp.WantAuthnRequestsSigned = *req.WantAuthnRequestsSigned
	}
	if req.AuthnRequestSigningCertificatePEM != nil {
		sp.AuthnRequestSigningCertificatePEM = strings.TrimSpace(*req.AuthnRequestSigningCertificatePEM)
	}
	if sp.WantAuthnRequestsSigned {
		if _, err := samldomain.ParseCertificatePEM(sp.AuthnRequestSigningCertificatePEM); err != nil {
			return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "AuthnRequest 署名検証用証明書を指定してください")
		}
	}
	if req.Rules != nil {
		sp.ClaimPolicy.Rules = *req.Rules
	}
	now := time.Now().UTC()
	sp.UpdatedAt = &now
	if err := d.SamlSPRepo.Save(ctx, sp); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) requireApp(c *echo.Context) (*spec.Application, error) {
	app, err := d.ApplicationRepo.FindByID(c.Request().Context(), support.RequestTenantID(c), c.Param("application_id"))
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, appusecases.ErrApplicationNotFound
	}
	return app, nil
}

func bindingKeyOf(app *spec.Application, bindingType spec.ProtocolBindingType) string {
	for _, b := range app.Bindings {
		if b.Type == bindingType {
			switch bindingType {
			case spec.ProtocolBindingWsFed:
				return b.Wtrealm
			case spec.ProtocolBindingSAML:
				return b.EntityID
			default:
				return b.ClientID
			}
		}
	}
	return ""
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
