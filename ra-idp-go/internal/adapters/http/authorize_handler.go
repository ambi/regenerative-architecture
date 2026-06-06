// /authorize + /consent + /end_session + /login
package http

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	authdomain "ra-idp-go/internal/authentication/domain"
	authusecases "ra-idp-go/internal/authentication/usecases"
	oauthdomain "ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

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
				return renderLogin(c, out.Request.ID, "")
			}
			return d.completeAfterAuthn(c, out.Request, out.Client, authn)
		}
	}
	if out.Request.Prompt != nil && *out.Request.Prompt == "none" {
		return writeOAuthError(c, usecases.NewOAuthError("login_required", "prompt=none では再認証不可"))
	}
	return renderLogin(c, out.Request.ID, "")
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
		return renderStatus(c, http.StatusOK, "signed-out")
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
		return renderLogin(c, requestID, "ユーザー名またはパスワードを確認してください。")
	}
	ok, err := d.PasswordHasher.Verify(password, user.PasswordHash)
	if err != nil || !ok {
		if d.Emit != nil {
			d.Emit(&spec.AuthenticationFailed{At: time.Now().UTC(), Username: username, Reason: "invalid_credentials"})
		}
		return renderLogin(c, requestID, "ユーザー名またはパスワードを確認してください。")
	}
	if r := authusecases.ValidatePassword(password); !r.OK {
		return renderLogin(c, requestID, "パスワードがセキュリティ要件を満たしていません。")
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
