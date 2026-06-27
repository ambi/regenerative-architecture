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
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/spec"

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
	// service (M2M / client_credentials)
	Scope string `json:"scope"`
	// WS-Federation
	Wtrealm      string   `json:"wtrealm"`
	ReplyURLs    []string `json:"reply_urls"`
	NameIDFormat string   `json:"name_id_format"`
	NameIDSource string   `json:"name_id_source"`
}

// oidcConfig / wsfedConfig はアプリ詳細に解決して返す protocol 設定。
type oidcConfig struct {
	ClientID     string   `json:"client_id"`
	RedirectURIs []string `json:"redirect_uris"`
	Scope        string   `json:"scope"`
}

type wsfedConfig struct {
	Wtrealm      string   `json:"wtrealm"`
	ReplyURLs    []string `json:"reply_urls"`
	NameIDFormat string   `json:"name_id_format"`
	NameIDSource string   `json:"name_id_source"`
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
	if err := core.DecodeJSON(c.Request(), &req); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
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
		return core.NoStoreJSON(c, http.StatusCreated, map[string]any{"application": toApplicationResponse(app)})

	case "oidc":
		if len(req.RedirectURIs) == 0 {
			return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "リダイレクト URI を 1 つ以上指定してください")
		}
		result, err := oauthusecases.CreateClient(ctx, oauthusecases.ClientDeps{ClientRepo: d.ClientRepo, Emit: d.Emit}, oauthusecases.CreateClientInput{
			ActorSub: actor.Sub,
			Registration: oauthusecases.RegisterClientInput{
				ClientName: req.Name, ClientType: spec.ClientConfidential, RedirectURIs: req.RedirectURIs,
				GrantTypes:              []spec.GrantType{spec.GrantAuthorizationCode, spec.GrantRefreshToken},
				ResponseTypes:           []spec.ResponseType{spec.ResponseTypeCode},
				TokenEndpointAuthMethod: spec.AuthMethodClientSecretBasic, Scope: defaultOIDCScope,
			},
			Now: now,
		})
		if err != nil {
			return d.writeApplicationError(c, err)
		}
		app, err := d.createCatalogApp(ctx, actor.Sub, req, now, spec.ApplicationFederated,
			spec.ProtocolBinding{Type: spec.ProtocolBindingOIDC, ClientID: result.Client.ClientID})
		if err != nil {
			return d.writeApplicationError(c, err)
		}
		return core.NoStoreJSON(c, http.StatusCreated, map[string]any{
			"application": toApplicationResponse(app), "client_id": result.Client.ClientID, "client_secret": result.ClientSecret,
		})

	case "service":
		// M2M / サービスクライアント (client_credentials)。redirect を持たず、ポータルにも
		// 出さない service kind の Application として登録する (Okta の API Services 相当)。
		result, err := oauthusecases.CreateClient(ctx, oauthusecases.ClientDeps{ClientRepo: d.ClientRepo, Emit: d.Emit}, oauthusecases.CreateClientInput{
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
		return core.NoStoreJSON(c, http.StatusCreated, map[string]any{
			"application": toApplicationResponse(app), "client_id": result.Client.ClientID, "client_secret": result.ClientSecret,
		})

	case "wsfed":
		if strings.TrimSpace(req.Wtrealm) == "" || len(req.ReplyURLs) == 0 {
			return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "wtrealm と reply URL を指定してください")
		}
		rp := &spec.WsFedRelyingParty{
			TenantID: core.RequestTenantID(c), Wtrealm: req.Wtrealm, DisplayName: req.Name, ReplyURLs: req.ReplyURLs,
			ClaimPolicy: spec.ClaimMappingPolicy{NameID: spec.NameIdConfiguration{
				Format: nonEmpty(req.NameIDFormat, defaultNameIDFormat), SourceAttribute: nonEmpty(req.NameIDSource, defaultNameIDSource),
			}},
			CreatedAt: now,
		}
		if d.WsFedRPRepo == nil {
			return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "WS-Federation は利用できません")
		}
		if err := d.WsFedRPRepo.Save(ctx, rp); err != nil {
			return err
		}
		app, err := d.createCatalogApp(ctx, actor.Sub, req, now, spec.ApplicationFederated,
			spec.ProtocolBinding{Type: spec.ProtocolBindingWsFed, Wtrealm: req.Wtrealm})
		if err != nil {
			return d.writeApplicationError(c, err)
		}
		return core.NoStoreJSON(c, http.StatusCreated, map[string]any{"application": toApplicationResponse(app)})

	default:
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "種別は oidc / wsfed / weblink のいずれかです")
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
func (d Deps) resolveProtocolConfig(c *echo.Context, app *spec.Application) (*oidcConfig, *wsfedConfig) {
	ctx := c.Request().Context()
	tenantID := core.RequestTenantID(c)
	var oidc *oidcConfig
	var wsfed *wsfedConfig
	for _, binding := range app.Bindings {
		switch binding.Type {
		case spec.ProtocolBindingOIDC:
			if d.ClientRepo == nil {
				continue
			}
			if client, err := d.ClientRepo.FindByID(ctx, tenantID, binding.ClientID); err == nil && client != nil {
				oidc = &oidcConfig{ClientID: client.ClientID, RedirectURIs: client.RedirectURIs, Scope: client.Scope}
			}
		case spec.ProtocolBindingWsFed:
			if d.WsFedRPRepo == nil {
				continue
			}
			if rp, err := d.WsFedRPRepo.FindByWtrealm(ctx, tenantID, binding.Wtrealm); err == nil && rp != nil {
				wsfed = &wsfedConfig{
					Wtrealm: rp.Wtrealm, ReplyURLs: rp.ReplyURLs,
					NameIDFormat: rp.ClaimPolicy.NameID.Format, NameIDSource: rp.ClaimPolicy.NameID.SourceAttribute,
				}
			}
		}
	}
	return oidc, wsfed
}

type updateOIDCRequest struct {
	RedirectURIs *[]string `json:"redirect_uris"`
	Scope        *string   `json:"scope"`
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
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "OIDC バインディングがありません")
	}
	var req updateOIDCRequest
	if err := core.DecodeJSON(c.Request(), &req); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if _, err := oauthusecases.UpdateClient(c.Request().Context(), oauthusecases.ClientDeps{ClientRepo: d.ClientRepo, Emit: d.Emit}, oauthusecases.UpdateClientInput{
		ActorSub: actor.Sub, ClientID: clientID, RedirectURIs: req.RedirectURIs, Scope: req.Scope, Now: time.Now().UTC(),
	}); err != nil {
		return d.writeApplicationError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

type updateWsFedRequest struct {
	ReplyURLs    *[]string `json:"reply_urls"`
	NameIDFormat *string   `json:"name_id_format"`
	NameIDSource *string   `json:"name_id_source"`
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
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "WS-Federation バインディングがありません")
	}
	ctx := c.Request().Context()
	rp, err := d.WsFedRPRepo.FindByWtrealm(ctx, core.RequestTenantID(c), wtrealm)
	if err != nil || rp == nil {
		return core.WriteBrowserError(c, http.StatusNotFound, "not_found", "relying party が存在しません")
	}
	var req updateWsFedRequest
	if err := core.DecodeJSON(c.Request(), &req); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if req.ReplyURLs != nil {
		rp.ReplyURLs = *req.ReplyURLs
	}
	if req.NameIDFormat != nil {
		rp.ClaimPolicy.NameID.Format = *req.NameIDFormat
	}
	if req.NameIDSource != nil {
		rp.ClaimPolicy.NameID.SourceAttribute = *req.NameIDSource
	}
	now := time.Now().UTC()
	rp.UpdatedAt = &now
	if err := d.WsFedRPRepo.Save(ctx, rp); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) requireApp(c *echo.Context) (*spec.Application, error) {
	app, err := d.ApplicationRepo.FindByID(c.Request().Context(), core.RequestTenantID(c), c.Param("application_id"))
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
			if bindingType == spec.ProtocolBindingWsFed {
				return b.Wtrealm
			}
			return b.ClientID
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
