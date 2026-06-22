// /token (5 grant types) + /revoke + /introspect
package http

import (
	"net/http"
	"slices"
	"strings"
	"time"

	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/platform/crypto"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func (d Deps) handleToken(c *echo.Context) error {
	if err := c.Request().ParseForm(); err != nil {
		return c.JSON(http.StatusBadRequest, oauthErrorBody("invalid_request", "form parse"))
	}
	clientStub, err := d.authenticateTokenClient(c)
	if err != nil {
		return writeOAuthError(c, err)
	}
	grantType := c.Request().PostFormValue("grant_type")
	if grantType == "" {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "grant_type が必要です"))
	}
	if !spec.GrantType(grantType).Valid() {
		return writeOAuthError(c, usecases.NewOAuthError("unsupported_grant_type", "未対応 grant_type: "+grantType))
	}
	client, err := d.ClientRepo.FindByID(c.Request().Context(), requestTenantID(c), clientStub.ID)
	if err != nil {
		return writeOAuthError(c, err)
	}
	if client == nil {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_client", "未知の client_id"))
	}
	if !slices.Contains(client.GrantTypes, spec.GrantType(grantType)) {
		return writeOAuthError(c, usecases.NewOAuthError("unauthorized_client", "宣言外の grant_type です"))
	}

	// DPoP 検証 (任意)
	var dpopJKT string
	if proof := c.Request().Header.Get("DPoP"); proof != "" && d.DpopReplayStore != nil {
		htu := requestHTU(c, d.Issuer)
		r, err := crypto.VerifyDPoP(c.Request().Context(), proof, "POST", htu, d.DpopReplayStore, time.Now().UTC())
		if err != nil {
			return writeOAuthError(c, usecases.NewOAuthError("invalid_dpop_proof", err.Error()))
		}
		if r != nil {
			dpopJKT = r.JKT
		}
	}

	ctx := c.Request().Context()
	now := time.Now().UTC()

	switch grantType {
	case "authorization_code":
		out, err := usecases.ExchangeCodeForToken(ctx, usecases.ExchangeCodeDeps{
			ClientRepo: d.ClientRepo, UserRepo: d.UserRepo,
			RequestStore: d.RequestStore, CodeStore: d.CodeStore,
			RefreshStore: d.RefreshStore, TokenIssuer: d.TokenIssuer,
			Emit: d.Emit, ResolveAttributeDefs: d.effectiveUserAttributeDefs,
		}, usecases.ExchangeCodeInput{
			ClientID: clientStub.ID, Code: c.Request().PostFormValue("code"),
			CodeVerifier: c.Request().PostFormValue("code_verifier"),
			RedirectURI:  c.Request().PostFormValue("redirect_uri"),
			DpopJKT:      dpopJKT,
			MTLSX5TS256:  clientStub.MTLSThumbprintS256,
		})
		if err != nil {
			return writeOAuthError(c, err)
		}
		body := map[string]any{
			"access_token": out.AccessToken,
			"token_type":   out.TokenType, "expires_in": out.ExpiresIn, "scope": out.Scope,
		}
		if out.IDToken != "" {
			body["id_token"] = out.IDToken
		}
		if out.RefreshToken != "" {
			body["refresh_token"] = out.RefreshToken
		}
		return c.JSON(http.StatusOK, body)

	case "refresh_token":
		rt := c.Request().PostFormValue("refresh_token")
		if rt == "" {
			return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "refresh_token が必要"))
		}
		res, err := usecases.RefreshTokens(ctx, usecases.RefreshDeps{
			ClientRepo: d.ClientRepo, UserRepo: d.UserRepo,
			RefreshStore: d.RefreshStore, TokenIssuer: d.TokenIssuer,
			Authorizer: d.Authorizer, Emit: d.Emit,
		}, usecases.RefreshInput{
			ClientID: clientStub.ID, RefreshToken: rt,
			ProofJKT: dpopJKT, ProofX5TS256: clientStub.MTLSThumbprintS256,
		}, now)
		if err != nil {
			return writeOAuthError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"access_token": res.AccessToken, "refresh_token": res.RefreshToken,
			"token_type": res.TokenType, "expires_in": res.ExpiresIn, "scope": res.Scope,
		})

	case "client_credentials":
		if client.ClientType != spec.ClientConfidential {
			return writeOAuthError(c, usecases.NewOAuthError("unauthorized_client", "public client は不可"))
		}
		scope := c.Request().PostFormValue("scope")
		if scope == "" {
			scope = client.Scope
		}
		scopes := strings.Fields(scope)
		declared := map[string]bool{}
		for _, s := range strings.Fields(client.Scope) {
			declared[s] = true
		}
		for _, s := range scopes {
			if !declared[s] {
				return writeOAuthError(c, usecases.NewOAuthError("invalid_scope", "宣言外のスコープ"))
			}
		}
		var sc *spec.SenderConstraint
		if dpopJKT != "" {
			sc = &spec.SenderConstraint{Type: spec.SenderConstraintDPoP, JKT: dpopJKT}
		} else if clientStub.MTLSThumbprintS256 != "" {
			sc = &spec.SenderConstraint{Type: spec.SenderConstraintMTLS, X5TS256: clientStub.MTLSThumbprintS256}
		}
		// ADR-048: client に Agent が束縛されている場合、Active 以外 (Disabled / Killed)
		// なら新規トークンを発行しない (fail-closed)。束縛があれば agent_id を token に載せる。
		var agentID string
		if d.AgentRepo != nil {
			agent, err := d.AgentRepo.FindByClientID(ctx, requestTenantID(c), client.ClientID)
			if err != nil {
				return writeOAuthError(c, err)
			}
			if agent != nil {
				if !agent.IsActive() {
					return writeOAuthError(c, usecases.NewOAuthError("invalid_client", "agent is disabled or killed"))
				}
				agentID = agent.ID
			}
		}
		token, jti, err := d.TokenIssuer.SignAccessToken(ctx, oauthports.AccessTokenInput{
			Client: client, Sub: client.ClientID, Scopes: scopes,
			SenderConstraint: sc, AuthTime: now.Unix(), AgentID: agentID,
		})
		if err != nil {
			return writeOAuthError(c, err)
		}
		if d.Emit != nil {
			tag := "none"
			if sc != nil {
				tag = string(sc.Type)
			}
			d.Emit(&spec.AccessTokenIssued{At: now, TenantID: requestTenantID(c), JTI: jti, ClientID: client.ClientID, Sub: client.ClientID, Scopes: scopes, SenderConstraint: tag})
		}
		tokenType := "Bearer"
		if sc != nil && sc.Type == spec.SenderConstraintDPoP {
			tokenType = "DPoP"
		}
		return c.JSON(http.StatusOK, map[string]any{
			"access_token": token, "token_type": tokenType,
			"expires_in": d.TokenIssuer.AccessTokenTTLSeconds(), "scope": strings.Join(scopes, " "),
		})

	case "urn:ietf:params:oauth:grant-type:device_code":
		dc := c.Request().PostFormValue("device_code")
		if dc == "" {
			return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "device_code が必要"))
		}
		res, err := usecases.ExchangeDeviceCode(ctx, usecases.ExchangeDeviceCodeDeps{
			ClientRepo: d.ClientRepo, UserRepo: d.UserRepo,
			DeviceCodeStore: d.DeviceCodeStore, RefreshStore: d.RefreshStore,
			TokenIssuer: d.TokenIssuer, Emit: d.Emit,
			ResolveAttributeDefs: d.effectiveUserAttributeDefs,
		}, usecases.ExchangeDeviceCodeInput{
			ClientID: clientStub.ID, DeviceCode: dc,
			ProofJKT: dpopJKT, ProofX5TS256: clientStub.MTLSThumbprintS256,
		}, now)
		if err != nil {
			return writeOAuthError(c, err)
		}
		body := map[string]any{
			"access_token": res.AccessToken, "token_type": res.TokenType,
			"expires_in": res.ExpiresIn, "scope": res.Scope,
		}
		if res.RefreshToken != "" {
			body["refresh_token"] = res.RefreshToken
		}
		if res.IDToken != "" {
			body["id_token"] = res.IDToken
		}
		return c.JSON(http.StatusOK, body)

	case "urn:ietf:params:oauth:grant-type:token-exchange":
		res, err := usecases.ExchangeToken(ctx, usecases.ExchangeTokenDeps{
			ClientRepo: d.ClientRepo, Introspector: d.TokenIntrospector,
			TokenIssuer: d.TokenIssuer, Emit: d.Emit,
		}, usecases.ExchangeTokenInput{
			ClientID:           clientStub.ID,
			SubjectToken:       c.Request().PostFormValue("subject_token"),
			SubjectTokenType:   c.Request().PostFormValue("subject_token_type"),
			ActorToken:         c.Request().PostFormValue("actor_token"),
			ActorTokenType:     c.Request().PostFormValue("actor_token_type"),
			Resource:           c.Request().PostForm["resource"],
			Scope:              c.Request().PostFormValue("scope"),
			RequestedTokenType: c.Request().PostFormValue("requested_token_type"),
			ProofJKT:           dpopJKT,
			ProofX5TS256:       clientStub.MTLSThumbprintS256,
		}, now)
		if err != nil {
			return writeOAuthError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"access_token":      res.AccessToken,
			"issued_token_type": res.IssuedTokenType,
			"token_type":        res.TokenType,
			"expires_in":        res.ExpiresIn,
			"scope":             res.Scope,
		})
	}
	return writeOAuthError(c, usecases.NewOAuthError("unsupported_grant_type", "未対応 grant_type: "+grantType))
}

func (d Deps) handleRevoke(c *echo.Context) error {
	if err := c.Request().ParseForm(); err != nil {
		return c.JSON(http.StatusBadRequest, oauthErrorBody("invalid_request", "form parse"))
	}
	client, err := d.authenticateTokenClient(c)
	if err != nil {
		return writeOAuthError(c, err)
	}
	if err := usecases.RevokeToken(c.Request().Context(), usecases.RevokeDeps{
		RefreshStore: d.RefreshStore, Introspector: d.TokenIntrospector,
		AccessTokenDenylist: d.AccessTokenDenylist, Emit: d.Emit,
	}, client.ID, c.Request().PostFormValue("token"), time.Now().UTC()); err != nil {
		return writeOAuthError(c, err)
	}
	return c.NoContent(http.StatusOK)
}

func (d Deps) handleIntrospect(c *echo.Context) error {
	if err := c.Request().ParseForm(); err != nil {
		return c.JSON(http.StatusBadRequest, oauthErrorBody("invalid_request", "form parse"))
	}
	clientStub, err := d.authenticateTokenClient(c)
	if err != nil {
		return writeOAuthError(c, err)
	}
	resp, err := usecases.IntrospectToken(c.Request().Context(), usecases.IntrospectDeps{
		Introspector: d.TokenIntrospector, RefreshStore: d.RefreshStore,
		AccessTokenDenylist: d.AccessTokenDenylist,
	}, usecases.IntrospectInput{
		Token:         c.Request().PostFormValue("token"),
		TokenTypeHint: c.Request().PostFormValue("token_type_hint"),
	}, time.Now().UTC())
	if err != nil {
		return writeOAuthError(c, err)
	}
	if d.Emit != nil {
		d.Emit(&spec.TokenIntrospected{At: time.Now().UTC(), TenantID: requestTenantID(c), RSClientID: clientStub.ID, TokenID: resp.JTI, Active: resp.Active})
	}
	return c.JSON(http.StatusOK, resp)
}
