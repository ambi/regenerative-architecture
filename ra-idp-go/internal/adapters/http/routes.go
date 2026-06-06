// Package http: Echo v5 を用いた HTTP アダプタ。
// TS adapters/http/* に対応。
package http

import (
	"context"
	"encoding/base64"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"ra-idp-go/internal/adapters/crypto"
	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"

	authdomain "ra-idp-go/internal/authentication/domain"
	authports "ra-idp-go/internal/authentication/ports"
	authusecases "ra-idp-go/internal/authentication/usecases"
	oauthdomain "ra-idp-go/internal/oauth2/domain"
	oauthports "ra-idp-go/internal/oauth2/ports"
)

type Deps struct {
	Issuer                     string
	SCL                        *spec.SCL
	ClientRepo                 oauthports.ClientRepository
	UserRepo                   oauthports.UserRepository
	ConsentRepo                oauthports.ConsentRepository
	RequestStore               oauthports.AuthorizationRequestStore
	CodeStore                  oauthports.AuthorizationCodeStore
	PARStore                   oauthports.PARStore
	RefreshStore               oauthports.RefreshTokenStore
	DeviceCodeStore            oauthports.DeviceCodeStore
	DpopReplayStore            oauthports.DpopReplayStore
	ClientAssertionReplayStore oauthports.ClientAssertionReplayStore
	KeyStore                   oauthports.KeyStore
	TokenIssuer                oauthports.TokenIssuer
	TokenIntrospector          oauthports.TokenIntrospector
	Authorizer                 oauthports.Authorizer
	JWKResolver                *crypto.JWKResolver
	PasswordHasher             authports.PasswordHasher
	SessionManager             *authusecases.SessionManager
	AuthnResolver              authdomain.AuthenticationContextResolver
	Emit                       func(spec.DomainEvent)
}

func Register(e *echo.Echo, d Deps) {
	e.GET("/authorize", d.handleAuthorize)
	e.POST("/login", d.handleLogin)
	e.POST("/consent", d.handleConsent)
	e.GET("/end_session", d.handleEndSession)
	e.POST("/end_session", d.handleEndSession)
	e.POST("/token", d.handleToken)
	e.POST("/revoke", d.handleRevoke)
	e.POST("/introspect", d.handleIntrospect)
	e.GET("/userinfo", d.handleUserInfo)
	e.POST("/userinfo", d.handleUserInfo)
	e.POST("/register", d.handleRegisterClient)
	e.POST("/par", d.handlePAR)
	e.POST("/device_authorization", d.handleDeviceAuthorization)
	e.GET("/device", d.handleDeviceVerification)
	e.POST("/device", d.handleDeviceVerification)
	e.GET("/.well-known/openid-configuration", d.handleDiscovery)
	e.GET("/jwks", d.handleJWKS)
}

// =====================================================================
// /authorize + /consent + /end_session
// =====================================================================

func (d Deps) handleAuthorize(c *echo.Context) error {
	q := c.QueryParams()
	parUsed := false
	if requestURI := q.Get("request_uri"); requestURI != "" {
		consumed, err := d.PARStore.Consume(c.Request().Context(), requestURI)
		if err != nil {
			return writeOAuthError(c, err)
		}
		if consumed == nil {
			return writeOAuthError(c, usecases.NewOAuthError("invalid_request_uri", "request_uri 無効または使用済み"))
		}
		if cid := q.Get("client_id"); cid != "" && cid != consumed.ClientID {
			return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "client_id が PAR と不一致"))
		}
		q = url.Values{}
		for k, v := range consumed.Parameters {
			q.Set(k, v)
		}
		q.Set("client_id", consumed.ClientID)
		parUsed = true
	}

	request, err := parseAuthorizeRequest(q)
	if err != nil {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", err.Error()))
	}
	in := usecases.AuthorizeRequestInput{
		ClientID: request.ClientID, RedirectURI: request.RedirectURI,
		ResponseType: request.ResponseType, Scope: request.Scope,
		StateParam: request.StateParam, Nonce: request.Nonce,
		CodeChallenge: request.CodeChallenge, CodeChallengeMethod: request.CodeChallengeMethod,
		Prompt: request.Prompt, MaxAge: request.MaxAge, ParUsed: parUsed,
	}
	if requestURI := c.QueryParam("request_uri"); requestURI != "" {
		in.ParRequestURI = requestURI
	}
	out, err := usecases.Authorize(c.Request().Context(), usecases.AuthorizeDeps{
		ClientRepo:   d.ClientRepo,
		RequestStore: d.RequestStore,
	}, in)
	if err != nil {
		return writeOAuthError(c, err)
	}

	// 既存セッションがあれば取得
	if d.AuthnResolver != nil {
		authn, _ := d.AuthnResolver.Resolve(c.Request().Context(), authdomain.HTTPHeadersAdapter{H: c.Request().Header})
		if authn != nil {
			policy := oauthdomain.ParsePrompt(out.Request)
			if oauthdomain.NeedsReauthentication(
				policy,
				time.Unix(authn.AuthTime, 0),
				time.Now(),
				false,
			) {
				if in.Prompt == "none" {
					return writeOAuthError(c, usecases.NewOAuthError("login_required", "既存セッションが認証要件を満たしません"))
				}
				return renderLogin(c, out.Request.ID)
			}
			return d.completeAfterAuthn(c, out.Request, out.Client, authn)
		}
	}
	if out.Request.Prompt != nil && *out.Request.Prompt == "none" {
		return writeOAuthError(c, usecases.NewOAuthError("login_required", "prompt=none では再認証不可"))
	}
	return renderLogin(c, out.Request.ID)
}

func (d Deps) completeAfterAuthn(c *echo.Context, req *spec.AuthorizationRequest, client *spec.Client, authn *authdomain.AuthenticationContext) error {
	// consent チェック
	if d.ConsentRepo != nil {
		consent, _ := d.ConsentRepo.Find(c.Request().Context(), authn.Sub, client.ClientID)
		covered := consent != nil &&
			consent.State == spec.ConsentGranted &&
			consent.RevokedAt == nil &&
			time.Now().Before(consent.ExpiresAt)
		if covered {
			for _, scope := range strings.Fields(req.Scope) {
				if !containsString(consent.Scopes, scope) {
					covered = false
					break
				}
			}
		}
		if req.Prompt != nil && *req.Prompt == "consent" {
			covered = false
		}
		if !covered {
			if err := d.RequestStore.AttachSubject(c.Request().Context(), req.ID, authn.Sub, authn.AuthTime); err != nil {
				return writeOAuthError(c, err)
			}
			req.Sub = &authn.Sub
			req.AuthTime = &authn.AuthTime
			return renderConsent(c, req, client)
		}
	}
	return d.issueCodeAndRedirect(c, req, authn.Sub, time.Unix(authn.AuthTime, 0))
}

func (d Deps) issueCodeAndRedirect(c *echo.Context, req *spec.AuthorizationRequest, sub string, authTime time.Time) error {
	out, err := usecases.CompleteLogin(c.Request().Context(), usecases.CompleteLoginDeps{
		RequestStore: d.RequestStore,
		CodeStore:    d.CodeStore,
	}, usecases.CompleteLoginInput{RequestID: req.ID, Sub: sub, AuthTime: authTime})
	if err != nil {
		return writeOAuthError(c, err)
	}
	if d.Emit != nil {
		d.Emit(&spec.AuthorizationCodeIssued{At: time.Now().UTC(), ClientID: req.ClientID, Sub: sub, Scopes: out.Code.Scopes, CodeChallengeMethod: req.CodeChallengeMethod})
	}
	u, _ := url.Parse(out.Request.RedirectURI)
	qry := u.Query()
	qry.Set("code", out.Code.Code)
	if out.Request.StateParam != nil {
		qry.Set("state", *out.Request.StateParam)
	}
	u.RawQuery = qry.Encode()
	return c.Redirect(http.StatusFound, u.String())
}

func (d Deps) handleConsent(c *echo.Context) error {
	if err := c.Request().ParseForm(); err != nil {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "form body parse failed"))
	}
	reqID := c.Request().PostFormValue("request_id")
	action := c.Request().PostFormValue("action")
	req, err := d.RequestStore.Find(c.Request().Context(), reqID)
	if err != nil {
		return writeOAuthError(c, err)
	}
	if req == nil {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "未知の authorization_request"))
	}
	if action != "allow" {
		u, _ := url.Parse(req.RedirectURI)
		q := u.Query()
		q.Set("error", "access_denied")
		if req.StateParam != nil {
			q.Set("state", *req.StateParam)
		}
		u.RawQuery = q.Encode()
		return c.Redirect(http.StatusFound, u.String())
	}
	if req.Sub == nil {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "consent 前にログインが必要"))
	}
	scopes := strings.Fields(req.Scope)
	if d.ConsentRepo != nil {
		now := time.Now().UTC()
		_ = d.ConsentRepo.Save(c.Request().Context(), &spec.Consent{
			Sub: *req.Sub, ClientID: req.ClientID, Scopes: scopes, State: spec.ConsentGranted,
			GrantedAt: now, ExpiresAt: now.Add(365 * 24 * time.Hour),
		})
		if d.Emit != nil {
			d.Emit(&spec.ConsentGrantedEvent{At: now, Sub: *req.Sub, ClientID: req.ClientID, Scopes: scopes})
		}
	}
	authTime := time.Now().UTC()
	if req.AuthTime != nil {
		authTime = time.Unix(*req.AuthTime, 0).UTC()
	}
	return d.issueCodeAndRedirect(c, req, *req.Sub, authTime)
}

func (d Deps) handleEndSession(c *echo.Context) error {
	if d.SessionManager != nil {
		_ = d.SessionManager.Revoke(c.Request().Context(), c.Request().Header.Get("Cookie"))
	}
	post := c.QueryParam("post_logout_redirect_uri")
	if post == "" {
		post = c.Request().PostFormValue("post_logout_redirect_uri")
	}
	if post == "" {
		c.Response().Header().Set("Content-Type", "text/html; charset=UTF-8")
		return c.HTML(http.StatusOK, "<!doctype html><h1>ログアウトしました</h1>")
	}
	clientID := c.QueryParam("client_id")
	if clientID == "" {
		clientID = c.Request().PostFormValue("client_id")
	}
	if clientID == "" {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "client_id が必要"))
	}
	client, err := d.ClientRepo.FindByID(c.Request().Context(), clientID)
	if err != nil {
		return writeOAuthError(c, err)
	}
	if client == nil {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "未知の client_id"))
	}
	if !containsString(client.RedirectURIs, post) {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "post_logout_redirect_uri が未登録"))
	}
	u, _ := url.Parse(post)
	q := u.Query()
	if state := c.QueryParam("state"); state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	return c.Redirect(http.StatusFound, u.String())
}

// =====================================================================
// /login
// =====================================================================

func (d Deps) handleLogin(c *echo.Context) error {
	input, err := parseLoginRequest(c.Request())
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	requestID := input.RequestID
	username := input.Username
	password := input.Password
	user, err := d.UserRepo.FindByUsername(c.Request().Context(), username)
	if err != nil {
		return err
	}
	if user == nil {
		if d.Emit != nil {
			d.Emit(&spec.AuthenticationFailed{At: time.Now().UTC(), Username: username, Reason: "user_not_found"})
		}
		return renderLogin(c, requestID)
	}
	ok, err := d.PasswordHasher.Verify(password, user.PasswordHash)
	if err != nil || !ok {
		if d.Emit != nil {
			d.Emit(&spec.AuthenticationFailed{At: time.Now().UTC(), Username: username, Reason: "invalid_credentials"})
		}
		return renderLogin(c, requestID)
	}
	if r := authusecases.ValidatePassword(password); !r.OK {
		return renderLogin(c, requestID)
	}

	// セッション作成
	var sessionID string
	if d.SessionManager != nil {
		authn, err := d.SessionManager.Create(c.Request().Context(), user.Sub, []string{"pwd"}, time.Now().UTC())
		if err != nil {
			return err
		}
		sessionID = authn.SessionID
	}
	if sessionID != "" {
		// Secure 属性は issuer が https の場合のみ付与する。HTTP では Secure を立てると
		// ブラウザが cookie を捨てて挙動が壊れるためデモ用途では条件付きにする。
		// 本番では常に https を強制する想定（ADR-021 の HSTS 設定と組み合わせる）。
		secure := strings.HasPrefix(d.Issuer, "https://")
		c.SetCookie(&http.Cookie{ //nolint:gosec // Secure is set conditionally; see above
			Name: authusecases.SessionCookie, Value: sessionID, Path: "/",
			HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
			MaxAge: authusecases.SessionTTLSeconds,
		})
	}
	if d.Emit != nil {
		d.Emit(&spec.UserAuthenticated{At: time.Now().UTC(), Sub: user.Sub, AMR: []string{"pwd"}})
	}

	// authorization request を読み出して継続
	req, err := d.RequestStore.Find(c.Request().Context(), requestID)
	if err != nil {
		return writeOAuthError(c, err)
	}
	if req == nil {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "未知の authorization_request"))
	}
	client, err := d.ClientRepo.FindByID(c.Request().Context(), req.ClientID)
	if err != nil || client == nil {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "client missing"))
	}
	// 認証直後は authTime = 今
	if err := d.RequestStore.AttachSubject(c.Request().Context(), req.ID, user.Sub, time.Now().Unix()); err != nil {
		return writeOAuthError(c, err)
	}
	req.Sub = &user.Sub
	t := time.Now().Unix()
	req.AuthTime = &t
	return d.completeAfterAuthn(c, req, client, &authdomain.AuthenticationContext{Sub: user.Sub, AuthTime: t})
}

// =====================================================================
// /token (4 grant types)
// =====================================================================

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
	client, err := d.ClientRepo.FindByID(c.Request().Context(), clientStub.ID)
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
		htu := d.Issuer + "/token"
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
			Emit: d.Emit,
		}, usecases.ExchangeCodeInput{
			ClientID: clientStub.ID, Code: c.Request().PostFormValue("code"),
			CodeVerifier: c.Request().PostFormValue("code_verifier"),
			RedirectURI:  c.Request().PostFormValue("redirect_uri"),
			DpopJKT:      dpopJKT,
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
		}, usecases.RefreshInput{ClientID: clientStub.ID, RefreshToken: rt, ProofJKT: dpopJKT}, now)
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
		}
		token, jti, err := d.TokenIssuer.SignAccessToken(ctx, oauthports.AccessTokenInput{
			Client: client, Sub: client.ClientID, Scopes: scopes,
			SenderConstraint: sc, AuthTime: now.Unix(),
		})
		if err != nil {
			return writeOAuthError(c, err)
		}
		if d.Emit != nil {
			tag := "none"
			if sc != nil {
				tag = string(sc.Type)
			}
			d.Emit(&spec.AccessTokenIssued{At: now, JTI: jti, ClientID: client.ClientID, Sub: client.ClientID, Scopes: scopes, SenderConstraint: tag})
		}
		tokenType := "Bearer"
		if sc != nil {
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
		}, usecases.ExchangeDeviceCodeInput{ClientID: clientStub.ID, DeviceCode: dc, ProofJKT: dpopJKT}, now)
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
	}
	return writeOAuthError(c, usecases.NewOAuthError("unsupported_grant_type", "未対応 grant_type: "+grantType))
}

// =====================================================================
// /revoke
// =====================================================================

func (d Deps) handleRevoke(c *echo.Context) error {
	if err := c.Request().ParseForm(); err != nil {
		return c.JSON(http.StatusBadRequest, oauthErrorBody("invalid_request", "form parse"))
	}
	if _, err := d.authenticateTokenClient(c); err != nil {
		return writeOAuthError(c, err)
	}
	if err := usecases.RevokeToken(c.Request().Context(), usecases.RevokeDeps{
		RefreshStore: d.RefreshStore, Emit: d.Emit,
	}, c.Request().PostFormValue("token"), time.Now().UTC()); err != nil {
		return writeOAuthError(c, err)
	}
	return c.NoContent(http.StatusOK)
}

// =====================================================================
// /introspect
// =====================================================================

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
	}, usecases.IntrospectInput{
		Token:         c.Request().PostFormValue("token"),
		TokenTypeHint: c.Request().PostFormValue("token_type_hint"),
	}, time.Now().UTC())
	if err != nil {
		return writeOAuthError(c, err)
	}
	if d.Emit != nil {
		d.Emit(&spec.TokenIntrospected{At: time.Now().UTC(), RSClientID: clientStub.ID, TokenID: resp.JTI, Active: resp.Active})
	}
	return c.JSON(http.StatusOK, resp)
}

// =====================================================================
// /userinfo
// =====================================================================

func (d Deps) handleUserInfo(c *echo.Context) error {
	auth := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_token", "Bearer token が必要"))
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	intro, err := d.TokenIntrospector.IntrospectAccessToken(c.Request().Context(), token)
	if err != nil {
		return writeOAuthError(c, err)
	}
	if !intro.Active {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_token", "トークンが無効"))
	}
	res, err := usecases.UserInfo(c.Request().Context(), d.UserRepo, d.Authorizer, usecases.UserInfoInput{
		Scopes: strings.Fields(intro.Scope), Sub: intro.Sub, Active: intro.Active, ClientID: intro.ClientID,
	})
	if err != nil {
		return writeOAuthError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

// =====================================================================
// /register
// =====================================================================

func (d Deps) handleRegisterClient(c *echo.Context) error {
	var req registerClientRequest
	if err := c.Bind(&req); err != nil {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", err.Error()))
	}
	if err := validateRegisterClientRequest(&req); err != nil {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_client_metadata", err.Error()))
	}
	if req.JwksURI != nil {
		if err := crypto.ValidateJWKSURI(*req.JwksURI); err != nil {
			return writeOAuthError(c, usecases.NewOAuthError("invalid_client_metadata", err.Error()))
		}
	}
	in := usecases.RegisterClientInput{
		ClientName:              req.ClientName,
		ClientType:              spec.ClientType(req.ClientType),
		RedirectURIs:            req.RedirectURIs,
		TokenEndpointAuthMethod: spec.TokenEndpointAuthMethod(req.TokenEndpointAuthMethod),
		Scope:                   req.Scope,
		JWKS:                    req.JWKS,
		JwksURI:                 req.JwksURI,
		RequirePAR:              req.RequirePAR,
		DpopBoundAccessTokens:   req.DpopBoundAccessTokens,
		FapiProfile:             spec.FapiProfile(req.FapiProfile),
	}
	for _, g := range req.GrantTypes {
		in.GrantTypes = append(in.GrantTypes, spec.GrantType(g))
	}
	for _, r := range req.ResponseTypes {
		in.ResponseTypes = append(in.ResponseTypes, spec.ResponseType(r))
	}
	result, err := usecases.RegisterClient(c.Request().Context(), usecases.RegisterClientDeps{
		ClientRepo: d.ClientRepo, Emit: d.Emit,
	}, in, time.Now().UTC())
	if err != nil {
		return writeOAuthError(c, err)
	}
	resp := map[string]any{
		"client_id":                             result.Client.ClientID,
		"client_type":                           result.Client.ClientType,
		"redirect_uris":                         result.Client.RedirectURIs,
		"grant_types":                           result.Client.GrantTypes,
		"response_types":                        result.Client.ResponseTypes,
		"token_endpoint_auth_method":            result.Client.TokenEndpointAuthMethod,
		"scope":                                 result.Client.Scope,
		"require_pushed_authorization_requests": result.Client.RequirePushedAuthorizationRequests,
		"dpop_bound_access_tokens":              result.Client.DpopBoundAccessTokens,
		"fapi_profile":                          result.Client.FapiProfile,
	}
	if result.Client.JWKS != nil {
		resp["jwks"] = result.Client.JWKS
	}
	if result.Client.JwksURI != nil {
		resp["jwks_uri"] = *result.Client.JwksURI
	}
	if result.ClientSecret != "" {
		resp["client_secret"] = result.ClientSecret
	}
	return c.JSON(http.StatusCreated, resp)
}

// =====================================================================
// /par
// =====================================================================

func (d Deps) handlePAR(c *echo.Context) error {
	if err := c.Request().ParseForm(); err != nil {
		return c.JSON(http.StatusBadRequest, oauthErrorBody("invalid_request", "form parse"))
	}
	clientStub, err := d.authenticateTokenClient(c)
	if err != nil {
		return writeOAuthError(c, err)
	}
	params := map[string]string{}
	for k, v := range c.Request().PostForm {
		if k == "client_id" || k == "client_secret" ||
			k == "client_assertion" || k == "client_assertion_type" {
			continue
		}
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	res, err := usecases.PushAuthorizationRequest(c.Request().Context(), usecases.PARDeps{
		ClientRepo: d.ClientRepo, Store: d.PARStore, Emit: d.Emit,
	}, usecases.PARInput{ClientID: clientStub.ID, Parameters: params}, time.Now().UTC())
	if err != nil {
		return writeOAuthError(c, err)
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"request_uri": res.RequestURI, "expires_in": res.ExpiresIn,
	})
}

// =====================================================================
// /device_authorization
// =====================================================================

func (d Deps) handleDeviceAuthorization(c *echo.Context) error {
	if err := c.Request().ParseForm(); err != nil {
		return c.JSON(http.StatusBadRequest, oauthErrorBody("invalid_request", "form parse"))
	}
	client, err := d.authenticateTokenClient(c)
	if err != nil {
		return writeOAuthError(c, err)
	}
	in := usecases.DeviceAuthorizationInput{
		ClientID: client.ID,
		Scope:    c.Request().PostFormValue("scope"),
	}
	res, err := usecases.RequestDeviceAuthorization(c.Request().Context(), usecases.DeviceAuthorizationDeps{
		ClientRepo: d.ClientRepo, DeviceCodeStore: d.DeviceCodeStore,
		BaseVerification: d.Issuer + "/device", Emit: d.Emit,
	}, in, time.Now().UTC())
	if err != nil {
		return writeOAuthError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

// =====================================================================
// /device (user code entry / approval UI)
// =====================================================================

func (d Deps) handleDeviceVerification(c *echo.Context) error {
	if c.Request().Method == http.MethodGet {
		return c.HTML(http.StatusOK, deviceVerificationForm(c.QueryParam("user_code")))
	}
	if err := c.Request().ParseForm(); err != nil {
		return c.String(http.StatusBadRequest, "invalid form")
	}
	userCode := c.Request().PostFormValue("user_code")
	action := c.Request().PostFormValue("action")
	authn, _ := d.AuthnResolver.Resolve(c.Request().Context(), authdomain.HTTPHeadersAdapter{H: c.Request().Header})
	if authn == nil {
		return c.HTML(http.StatusUnauthorized, "<!doctype html><h1>ログインが必要です</h1>")
	}
	if action == "deny" {
		_ = usecases.DenyUserCode(c.Request().Context(), usecases.VerifyUserCodeDeps{DeviceCodeStore: d.DeviceCodeStore, Emit: d.Emit}, userCode, authn.Sub, time.Now().UTC())
		return c.HTML(http.StatusOK, "<!doctype html><h1>拒否しました</h1>")
	}
	if err := usecases.ApproveUserCode(c.Request().Context(), usecases.VerifyUserCodeDeps{DeviceCodeStore: d.DeviceCodeStore, Emit: d.Emit}, userCode, authn.Sub, time.Now().UTC()); err != nil {
		return writeOAuthError(c, err)
	}
	return c.HTML(http.StatusOK, "<!doctype html><h1>承認しました</h1>")
}

// =====================================================================
// /.well-known/openid-configuration
// =====================================================================

func (d Deps) handleDiscovery(c *echo.Context) error {
	if d.SCL == nil {
		return writeOAuthError(c, usecases.NewOAuthError("server_error", "SCL not loaded"))
	}
	doc, err := d.SCL.BuildDiscoveryDocument(d.Issuer)
	if err != nil {
		return writeOAuthError(c, err)
	}
	return c.JSON(http.StatusOK, doc)
}

// =====================================================================
// /jwks
// =====================================================================

func (d Deps) handleJWKS(c *echo.Context) error {
	keys, err := d.KeyStore.GetAllKeys(c.Request().Context())
	if err != nil {
		return writeOAuthError(c, err)
	}
	jwks := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		jwks = append(jwks, k.PublicJWK)
	}
	return c.JSON(http.StatusOK, map[string]any{"keys": jwks})
}

// =====================================================================
// クライアント認証 (client_secret_basic / client_secret_post / private_key_jwt)
// =====================================================================

type authedClient struct{ ID string }

func (d Deps) authenticateTokenClient(c *echo.Context) (authedClient, error) {
	basicAuth := c.Request().Header.Get("Authorization")
	hasBasic := strings.HasPrefix(basicAuth, "Basic ")
	hasSecret := c.Request().PostFormValue("client_secret") != ""
	hasAssertion := c.Request().PostFormValue("client_assertion") != "" ||
		c.Request().PostFormValue("client_assertion_type") != ""
	methods := 0
	for _, present := range []bool{hasBasic, hasSecret, hasAssertion} {
		if present {
			methods++
		}
	}
	if methods > 1 {
		return authedClient{}, usecases.NewOAuthError("invalid_request", "複数のクライアント認証方式が混在しています")
	}

	// 1. client_assertion (private_key_jwt)
	if hasAssertion {
		const assertionType = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
		if c.Request().PostFormValue("client_assertion_type") != assertionType {
			return authedClient{}, usecases.NewOAuthError("invalid_request", "未対応の client_assertion_type です")
		}
		a := c.Request().PostFormValue("client_assertion")
		if a == "" {
			return authedClient{}, usecases.NewOAuthError("invalid_request", "client_assertion が必要です")
		}
		clientID := c.Request().PostFormValue("client_id")
		client, err := d.ClientRepo.FindByID(c.Request().Context(), clientID)
		if err != nil || client == nil || client.TokenEndpointAuthMethod != spec.AuthMethodPrivateKeyJwt {
			return authedClient{}, usecases.NewOAuthError("invalid_client", "クライアント認証に失敗しました")
		}
		resolver := d.JWKResolver
		if resolver == nil {
			resolver = crypto.NewJWKResolver()
		}
		_, err = crypto.VerifyClientAssertion(
			c.Request().Context(), a, clientID,
			acceptableClientAssertionAudiences(d.Issuer, c.Request()),
			func(ctx context.Context, cid string) ([]map[string]any, error) {
				cl, err := d.ClientRepo.FindByID(ctx, cid)
				if err != nil {
					return nil, err
				}
				if cl == nil {
					return nil, usecases.NewOAuthError("invalid_client", "client not found")
				}
				return resolver.Resolve(ctx, cl)
			},
			d.ClientAssertionReplayStore, time.Now().UTC(), nil,
		)
		if err != nil {
			return authedClient{}, usecases.NewOAuthError("invalid_client", err.Error())
		}
		return authedClient{ID: clientID}, nil
	}

	// 2. client_secret_basic / client_secret_post
	var clientID, secret string
	method := spec.AuthMethodNone
	switch {
	case hasBasic:
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(basicAuth, "Basic "))
		if err != nil {
			return authedClient{}, usecases.NewOAuthError("invalid_client", "Basic 復号失敗")
		}
		parts := strings.SplitN(string(raw), ":", 2)
		if len(parts) != 2 {
			return authedClient{}, usecases.NewOAuthError("invalid_client", "Basic 形式不正")
		}
		clientID, err = url.QueryUnescape(parts[0])
		if err != nil {
			return authedClient{}, usecases.NewOAuthError("invalid_client", "client_id の形式不正")
		}
		secret, err = url.QueryUnescape(parts[1])
		if err != nil {
			return authedClient{}, usecases.NewOAuthError("invalid_client", "client_secret の形式不正")
		}
		method = spec.AuthMethodClientSecretBasic
	case hasSecret:
		clientID = c.Request().PostFormValue("client_id")
		secret = c.Request().PostFormValue("client_secret")
		method = spec.AuthMethodClientSecretPost
	default:
		clientID = c.Request().PostFormValue("client_id")
	}
	client, err := d.ClientRepo.FindByID(c.Request().Context(), clientID)
	if err != nil || client == nil {
		return authedClient{}, usecases.NewOAuthError("invalid_client", "未知の client_id")
	}
	if client.TokenEndpointAuthMethod != method {
		return authedClient{}, usecases.NewOAuthError("invalid_client", "宣言されたクライアント認証方式と一致しません")
	}
	if method == spec.AuthMethodNone {
		return authedClient{ID: clientID}, nil
	}
	if client.ClientSecretHash == nil || !oauthdomain.VerifyClientSecret(secret, *client.ClientSecretHash) {
		return authedClient{}, usecases.NewOAuthError("invalid_client", "client_secret 不一致")
	}
	return authedClient{ID: clientID}, nil
}

func acceptableClientAssertionAudiences(issuer string, req *http.Request) []string {
	base := strings.TrimSuffix(issuer, "/")
	values := []string{base, base + "/token", base + "/par", base + "/introspect", base + "/revoke"}
	if req != nil {
		values = append(values, base+req.URL.Path)
	}
	return values
}

// =====================================================================
// 表示・エラー応答ヘルパ
// =====================================================================

var loginTmpl = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>ログイン</title></head>
<body>
<h1>ログインが必要です</h1>
<form method="POST" action="/login">
  <input type="hidden" name="request_id" value="{{.RequestID}}">
  <label>ユーザー名 <input name="username" autocomplete="username" required></label>
  <label>パスワード <input name="password" type="password" autocomplete="current-password" required></label>
  <button type="submit">ログイン</button>
</form>
</body></html>`))

var consentTmpl = template.Must(template.New("consent").Parse(`<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>同意</title></head>
<body>
<h1>{{.ClientName}} がアクセスを要求しています</h1>
<p>要求スコープ: <code>{{.Scope}}</code></p>
<form method="POST" action="/consent">
  <input type="hidden" name="request_id" value="{{.RequestID}}">
  <button type="submit" name="action" value="allow">許可</button>
  <button type="submit" name="action" value="deny">拒否</button>
</form>
</body></html>`))

func renderLogin(c *echo.Context, requestID string) error {
	c.Response().Header().Set("Content-Type", "text/html; charset=UTF-8")
	c.Response().WriteHeader(http.StatusUnauthorized)
	return loginTmpl.Execute(c.Response(), struct{ RequestID string }{RequestID: requestID})
}

func renderConsent(c *echo.Context, req *spec.AuthorizationRequest, client *spec.Client) error {
	c.Response().Header().Set("Content-Type", "text/html; charset=UTF-8")
	c.Response().WriteHeader(http.StatusOK)
	name := client.ClientID
	if client.ClientName != nil {
		name = *client.ClientName
	}
	return consentTmpl.Execute(c.Response(), struct {
		ClientName, Scope, RequestID string
	}{ClientName: name, Scope: req.Scope, RequestID: req.ID})
}

func deviceVerificationForm(userCode string) string {
	return `<!doctype html><html lang="ja"><head><meta charset="utf-8"><title>デバイス承認</title></head>
<body>
<h1>デバイス承認</h1>
<form method="POST" action="/device">
  <label>ユーザーコード <input name="user_code" value="` + template.HTMLEscapeString(userCode) + `" required></label>
  <button type="submit" name="action" value="allow">承認</button>
  <button type="submit" name="action" value="deny">拒否</button>
</form>
</body></html>`
}

func writeOAuthError(c *echo.Context, err error) error {
	var oe *usecases.OAuthError
	if !errors.As(err, &oe) {
		return c.JSON(http.StatusInternalServerError, oauthErrorBody("server_error", err.Error()))
	}
	status := http.StatusBadRequest
	switch oe.Code {
	case "invalid_client":
		status = http.StatusUnauthorized
	case "server_error":
		status = http.StatusInternalServerError
	}
	return c.JSON(status, oauthErrorBody(oe.Code, oe.Description))
}

func oauthErrorBody(code, description string) map[string]string {
	return map[string]string{"error": code, "error_description": description}
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
