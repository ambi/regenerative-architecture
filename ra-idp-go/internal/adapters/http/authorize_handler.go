// /authorize + browser authentication APIs + /end_session
package http

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
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

const (
	authorizationTransactionCookie = "ra_idp_transaction"
	csrfCookie                     = "ra_idp_csrf"
	csrfHeader                     = "X-CSRF-Token"
)

type browserFlowResponse struct {
	Next       string `json:"next,omitempty"`
	RedirectTo string `json:"redirect_to,omitempty"`
}

type transactionResponse struct {
	Kind       string   `json:"kind"`
	CSRFToken  string   `json:"csrf_token"`
	ClientName string   `json:"client_name,omitempty"`
	Scopes     []string `json:"scopes,omitempty"`
}

type accountContextResponse struct {
	CSRFToken         string `json:"csrf_token"`
	Sub               string `json:"sub"`
	PreferredUsername string `json:"preferred_username,omitempty"`
}

type loginAPIRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type consentAPIRequest struct {
	Action string `json:"action"`
}

type totpAPIRequest struct {
	Code string `json:"code"`
}

type changePasswordAPIRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

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
		if consumed.TenantID != requestTenantID(c) {
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
		Prompt: request.Prompt, MaxAge: request.MaxAge, ACRValues: request.AcrValues, ParUsed: parUsed,
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

	d.setTransactionCookie(c, out.Request.ID)
	if d.AuthnResolver != nil {
		authn, _ := d.AuthnResolver.Resolve(c.Request().Context(), authdomain.HTTPHeadersAdapter{H: c.Request().Header})
		if authn != nil {
			if authn.AuthenticationPending {
				if in.Prompt == "none" {
					return writeOAuthError(c, usecases.NewOAuthError("login_required", "追加factor検証が必要です"))
				}
				return c.Redirect(http.StatusSeeOther, tenantRoute(c, "/totp"))
			}
			policy := oauthdomain.ParsePrompt(out.Request)
			needsStepUp := out.Request.ACRValues != nil &&
				!authusecases.ACRSatisfies(authn.ACR, *out.Request.ACRValues)
			if oauthdomain.NeedsReauthentication(policy, time.Unix(authn.AuthTime, 0), time.Now(), false) ||
				needsStepUp {
				if in.Prompt == "none" {
					return writeOAuthError(c, usecases.NewOAuthError("login_required", "既存セッションが認証要件を満たしません"))
				}
				if needsStepUp && d.canUseTOTP(c, authn.Sub) {
					pending, err := d.SessionManager.CreateWithPending(
						c.Request().Context(),
						authn.Sub,
						authn.AMR,
						time.Now().UTC(),
						true,
					)
					if err != nil {
						return err
					}
					d.setSessionCookie(c, pending.SessionID)
					return c.Redirect(http.StatusSeeOther, tenantRoute(c, "/totp"))
				}
				return c.Redirect(http.StatusSeeOther, tenantRoute(c, "/login"))
			}
			next, err := d.completeAfterAuthn(c, out.Request, out.Client, authn)
			if err != nil {
				return err
			}
			if next.RedirectTo != "" {
				d.clearTransactionCookie(c)
			}
			return redirectAuthorizationNext(c, next)
		}
	}
	if out.Request.Prompt != nil && *out.Request.Prompt == "none" {
		return writeOAuthError(c, usecases.NewOAuthError("login_required", "prompt=none では再認証不可"))
	}
	return c.Redirect(http.StatusSeeOther, tenantRoute(c, "/login"))
}

func (d Deps) handleTransaction(c *echo.Context) error {
	req, err := d.transactionRequest(c)
	if err != nil {
		return writeBrowserError(c, http.StatusUnauthorized, "transaction_unavailable", err.Error())
	}
	csrf, err := d.ensureCSRFCookie(c)
	if err != nil {
		return err
	}
	if req.Sub == nil {
		authn, _ := d.resolveAuthentication(c)
		if authn != nil && authn.AuthenticationPending {
			return noStoreJSON(c, http.StatusOK, transactionResponse{Kind: "totp", CSRFToken: csrf})
		}
		return noStoreJSON(c, http.StatusOK, transactionResponse{Kind: "login", CSRFToken: csrf})
	}
	authn, _ := d.resolveAuthentication(c)
	if authn != nil && authn.AuthenticationPending {
		return noStoreJSON(c, http.StatusOK, transactionResponse{Kind: "totp", CSRFToken: csrf})
	}
	if authn == nil || authn.Sub != *req.Sub {
		return writeBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証セッションが一致しません")
	}
	client, err := d.ClientRepo.FindByID(c.Request().Context(), requestTenantID(c), req.ClientID)
	if err != nil {
		return err
	}
	if client == nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_transaction", "クライアントが存在しません")
	}
	name := client.ClientID
	if client.ClientName != nil {
		name = *client.ClientName
	}
	return noStoreJSON(c, http.StatusOK, transactionResponse{
		Kind: "consent", CSRFToken: csrf, ClientName: name, Scopes: strings.Fields(req.Scope),
	})
}

// handleAccountContext は認証済みセッションに対して CSRF cookie を発行し、SPA が
// /account/password 等の認証必須画面で X-CSRF-Token を構築するためのコンテキストを返す。
// OAuth 認可トランザクションは要求しない (handleTransaction との分岐点)。
func (d Deps) handleAccountContext(c *echo.Context) error {
	authn, err := d.resolveAuthentication(c)
	if err != nil {
		return err
	}
	if authn == nil || authn.AuthenticationPending {
		return writeBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証済みセッションが必要です")
	}
	csrf, err := d.ensureCSRFCookie(c)
	if err != nil {
		return err
	}
	resp := accountContextResponse{CSRFToken: csrf, Sub: authn.Sub}
	if d.UserRepo != nil {
		if user, _ := d.UserRepo.FindBySub(c.Request().Context(), authn.Sub); user != nil {
			resp.PreferredUsername = user.PreferredUsername
		}
	}
	return noStoreJSON(c, http.StatusOK, resp)
}

func (d Deps) handleLoginAPI(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	req, err := d.transactionRequest(c)
	if err != nil {
		return writeBrowserError(c, http.StatusUnauthorized, "transaction_unavailable", err.Error())
	}
	var input loginAPIRequest
	if err := decodeJSON(c.Request(), &input); err != nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if strings.TrimSpace(input.Username) == "" || input.Password == "" {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "ユーザー名とパスワードが必要です")
	}

	user, err := d.UserRepo.FindByUsername(c.Request().Context(), requestTenantID(c), input.Username)
	if err != nil {
		return err
	}
	if user == nil {
		d.emitAuthenticationFailure(input.Username, "user_not_found")
		return writeBrowserError(c, http.StatusUnauthorized, "invalid_credentials", "ユーザー名またはパスワードを確認してください。")
	}
	ok, err := d.PasswordHasher.Verify(input.Password, user.PasswordHash)
	if err != nil || !ok {
		d.emitAuthenticationFailure(input.Username, "invalid_credentials")
		return writeBrowserError(c, http.StatusUnauthorized, "invalid_credentials", "ユーザー名またはパスワードを確認してください。")
	}
	if user.DisabledAt != nil {
		d.emitAuthenticationFailure(input.Username, "account_disabled")
		return writeBrowserError(c, http.StatusUnauthorized, "invalid_credentials", "ユーザー名またはパスワードを確認してください。")
	}
	if result := authusecases.ValidatePassword(input.Password); !result.OK {
		return writeBrowserError(c, http.StatusUnauthorized, "password_policy", "パスワードがセキュリティ要件を満たしていません。")
	}

	authTime := time.Now().UTC()
	authn, err := d.SessionManager.CreateWithPending(
		c.Request().Context(),
		user.Sub,
		[]string{"pwd"},
		authTime,
		user.MfaEnrolled,
	)
	if err != nil {
		return err
	}
	d.setSessionCookie(c, authn.SessionID)
	if d.Emit != nil {
		d.Emit(&spec.UserAuthenticated{At: authTime, Sub: user.Sub, AMR: []string{"pwd"}})
	}
	if user.MfaEnrolled {
		return noStoreJSON(c, http.StatusOK, browserFlowResponse{Next: tenantRoute(c, "/totp")})
	}
	if err := d.RequestStore.AttachAuthentication(
		c.Request().Context(), req.ID, user.Sub, authn.AuthTime, authn.AMR, authn.ACR,
	); err != nil {
		return writeOAuthError(c, err)
	}
	req.Sub, req.AuthTime = &user.Sub, &authn.AuthTime
	client, err := d.ClientRepo.FindByID(c.Request().Context(), requestTenantID(c), req.ClientID)
	if err != nil || client == nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_transaction", "クライアントが存在しません")
	}
	next, err := d.completeAfterAuthn(c, req, client, authn)
	if err != nil {
		return err
	}
	if next.RedirectTo != "" {
		d.clearTransactionCookie(c)
	}
	return writeAuthorizationNext(c, next)
}

func (d Deps) handleTOTPAPI(c *echo.Context) error {
	if d.MfaFactorRepo == nil {
		return writeBrowserError(c, http.StatusServiceUnavailable, "mfa_unavailable", "MFA factor store is unavailable")
	}
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	req, err := d.transactionRequest(c)
	if err != nil {
		return writeBrowserError(c, http.StatusUnauthorized, "transaction_unavailable", err.Error())
	}
	authn, _ := d.resolveAuthentication(c)
	if authn == nil || authn.SessionID == "" {
		return writeBrowserError(c, http.StatusUnauthorized, "authentication_required", "TOTP検証セッションがありません")
	}
	if containsString(authn.AMR, "otp") && !authn.AuthenticationPending {
		return writeBrowserError(c, http.StatusForbidden, "access_denied", "TOTPは既に検証済みです")
	}
	var input totpAPIRequest
	if err := decodeJSON(c.Request(), &input); err != nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	result, err := authusecases.VerifyTOTPFactor(
		c.Request().Context(),
		d.MfaFactorRepo,
		authn.Sub,
		input.Code,
		time.Now().UTC(),
	)
	if err != nil {
		return err
	}
	if !result.OK {
		d.emitAuthenticationFailure(authn.Sub, result.Reason)
		return writeBrowserError(c, http.StatusUnauthorized, "invalid_totp", "TOTPコードを確認してください。")
	}
	completed, err := d.SessionManager.CompleteFactor(c.Request().Context(), authn.SessionID, []string{"otp"})
	if err != nil {
		return err
	}
	if completed == nil {
		return writeBrowserError(c, http.StatusUnauthorized, "authentication_required", "セッションが失効しました")
	}
	d.setSessionCookie(c, completed.SessionID)
	if d.Emit != nil {
		d.Emit(&spec.UserAuthenticated{At: time.Now().UTC(), Sub: completed.Sub, AMR: completed.AMR})
	}
	if err := d.RequestStore.AttachAuthentication(
		c.Request().Context(), req.ID, completed.Sub, completed.AuthTime, completed.AMR, completed.ACR,
	); err != nil {
		return writeOAuthError(c, err)
	}
	req.Sub, req.AuthTime, req.AMR, req.ACR = &completed.Sub, &completed.AuthTime, completed.AMR, &completed.ACR
	client, err := d.ClientRepo.FindByID(c.Request().Context(), requestTenantID(c), req.ClientID)
	if err != nil || client == nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_transaction", "クライアントが存在しません")
	}
	next, err := d.completeAfterAuthn(c, req, client, completed)
	if err != nil {
		return err
	}
	if next.RedirectTo != "" {
		d.clearTransactionCookie(c)
	}
	return writeAuthorizationNext(c, next)
}

func (d Deps) handleChangePasswordAPI(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	authn, err := d.resolveAuthentication(c)
	if err != nil {
		return err
	}
	if authn == nil || authn.AuthenticationPending {
		return writeBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証済みセッションが必要です")
	}
	var input changePasswordAPIRequest
	if err := decodeJSON(c.Request(), &input); err != nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if input.CurrentPassword == "" || input.NewPassword == "" {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "現在と新しいパスワードが必要です")
	}

	historyDepth := authusecases.PasswordPolicyHistoryDepth
	if d.SCL != nil && d.SCL.Annotations.PasswordPolicy.HistoryDepth > 0 {
		historyDepth = d.SCL.Annotations.PasswordPolicy.HistoryDepth
	}

	_, err = authusecases.ChangePassword(c.Request().Context(), authusecases.ChangePasswordDeps{
		UserRepo:            d.UserRepo,
		PasswordHasher:      d.PasswordHasher,
		PasswordHistoryRepo: d.PasswordHistoryRepo,
		Emit:                d.Emit,
		HistoryDepth:        historyDepth,
	}, authusecases.ChangePasswordInput{
		Sub:             authn.Sub,
		CurrentPassword: input.CurrentPassword,
		NewPassword:     input.NewPassword,
		Now:             time.Now().UTC(),
	})
	switch {
	case err == nil:
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.NoContent(http.StatusNoContent)
	case errors.Is(err, authusecases.ErrUserNotFound):
		return writeBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証済みセッションが無効です")
	case errors.Is(err, authusecases.ErrCurrentPasswordMismatch):
		return writeBrowserError(c, http.StatusForbidden, "access_denied", "現在のパスワードが一致しません")
	case errors.Is(err, authusecases.ErrPasswordReused):
		return writeBrowserError(c, http.StatusBadRequest, "password_reuse", "新しいパスワードは最近使用したものを再利用できません")
	default:
		var policyErr *authusecases.PasswordPolicyError
		if errors.As(err, &policyErr) {
			violations := make([]string, len(policyErr.Violations))
			for i, v := range policyErr.Violations {
				violations[i] = string(v)
			}
			return noStoreJSON(c, http.StatusBadRequest, map[string]any{
				"error":      "password_policy",
				"message":    "パスワードがセキュリティ要件を満たしていません。",
				"violations": violations,
			})
		}
		return err
	}
}

func (d Deps) handleConsentAPI(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	req, err := d.transactionRequest(c)
	if err != nil {
		return writeBrowserError(c, http.StatusUnauthorized, "transaction_unavailable", err.Error())
	}
	authn, _ := d.resolveAuthentication(c)
	if authn == nil || req.Sub == nil || authn.Sub != *req.Sub {
		return writeBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証セッションが一致しません")
	}
	var input consentAPIRequest
	if err := decodeJSON(c.Request(), &input); err != nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if input.Action != "allow" {
		_ = d.RequestStore.UpdateState(c.Request().Context(), req.ID, spec.AuthFlowRejected)
		d.clearTransactionCookie(c)
		return noStoreJSON(c, http.StatusOK, browserFlowResponse{RedirectTo: authorizationErrorURL(req, "access_denied", "")})
	}

	scopes := strings.Fields(req.Scope)
	if d.ConsentRepo != nil {
		now := time.Now().UTC()
		if err := d.ConsentRepo.Save(c.Request().Context(), &spec.Consent{
			TenantID: requestTenantID(c), Sub: authn.Sub, ClientID: req.ClientID,
			Scopes: scopes, State: spec.ConsentGranted,
			GrantedAt: now, ExpiresAt: now.Add(365 * 24 * time.Hour),
		}); err != nil {
			return err
		}
		if d.Emit != nil {
			d.Emit(&spec.ConsentGrantedEvent{At: now, Sub: authn.Sub, ClientID: req.ClientID, Scopes: scopes})
		}
	}
	redirectTo, err := d.issueCodeURL(c, req, authn.Sub, time.Unix(authn.AuthTime, 0))
	if err != nil {
		return err
	}
	d.clearTransactionCookie(c)
	return noStoreJSON(c, http.StatusOK, browserFlowResponse{RedirectTo: redirectTo})
}

type authorizationNext struct {
	Path       string
	RedirectTo string
}

func (d Deps) completeAfterAuthn(
	c *echo.Context,
	req *spec.AuthorizationRequest,
	client *spec.Client,
	authn *authdomain.AuthenticationContext,
) (authorizationNext, error) {
	if authn.AuthenticationPending {
		return authorizationNext{Path: tenantRoute(c, "/totp")}, nil
	}
	if d.ConsentRepo != nil {
		consent, _ := d.ConsentRepo.Find(
			c.Request().Context(), requestTenantID(c), authn.Sub, client.ClientID,
		)
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
			if err := d.RequestStore.AttachAuthentication(
				c.Request().Context(), req.ID, authn.Sub, authn.AuthTime, authn.AMR, authn.ACR,
			); err != nil {
				return authorizationNext{}, err
			}
			req.Sub, req.AuthTime = &authn.Sub, &authn.AuthTime
			return authorizationNext{Path: tenantRoute(c, "/consent")}, nil
		}
	}
	redirectTo, err := d.issueCodeURL(c, req, authn.Sub, time.Unix(authn.AuthTime, 0))
	return authorizationNext{RedirectTo: redirectTo}, err
}

func (d Deps) canUseTOTP(c *echo.Context, sub string) bool {
	if d.MfaFactorRepo == nil {
		return false
	}
	factor, err := d.MfaFactorRepo.Find(c.Request().Context(), sub, spec.MfaFactorTOTP)
	return err == nil && factor != nil && factor.Secret != nil && *factor.Secret != ""
}

func (d Deps) issueCodeURL(
	c *echo.Context,
	req *spec.AuthorizationRequest,
	sub string,
	authTime time.Time,
) (string, error) {
	out, err := usecases.CompleteLogin(c.Request().Context(), usecases.CompleteLoginDeps{
		RequestStore: d.RequestStore,
		CodeStore:    d.CodeStore,
	}, usecases.CompleteLoginInput{
		RequestID: req.ID,
		Sub:       sub,
		AuthTime:  authTime,
		AMR:       req.AMR,
		ACR:       stringValue(req.ACR),
	})
	if err != nil {
		var oauthErr *usecases.OAuthError
		if errors.As(err, &oauthErr) {
			return authorizationErrorURL(req, oauthErr.Code, oauthErr.Description), nil
		}
		return "", err
	}
	if d.Emit != nil {
		d.Emit(&spec.AuthorizationCodeIssued{
			At: time.Now().UTC(), ClientID: req.ClientID, Sub: sub,
			Scopes: out.Code.Scopes, CodeChallengeMethod: req.CodeChallengeMethod,
		})
	}
	u, _ := url.Parse(out.Request.RedirectURI)
	query := u.Query()
	query.Set("code", out.Code.Code)
	if out.Request.StateParam != nil {
		query.Set("state", *out.Request.StateParam)
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func authorizationErrorURL(req *spec.AuthorizationRequest, code, description string) string {
	u, _ := url.Parse(req.RedirectURI)
	query := u.Query()
	query.Set("error", code)
	if description != "" {
		query.Set("error_description", description)
	}
	if req.StateParam != nil {
		query.Set("state", *req.StateParam)
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func redirectAuthorizationNext(c *echo.Context, next authorizationNext) error {
	if next.RedirectTo != "" {
		return c.Redirect(http.StatusFound, next.RedirectTo)
	}
	return c.Redirect(http.StatusSeeOther, next.Path)
}

func writeAuthorizationNext(c *echo.Context, next authorizationNext) error {
	if next.RedirectTo != "" {
		return noStoreJSON(c, http.StatusOK, browserFlowResponse{RedirectTo: next.RedirectTo})
	}
	return noStoreJSON(c, http.StatusOK, browserFlowResponse{Next: next.Path})
}

func (d Deps) handleEndSession(c *echo.Context) error {
	if d.SessionManager != nil {
		_ = d.SessionManager.Revoke(c.Request().Context(), c.Request().Header.Get("Cookie"))
		d.clearSessionCookie(c)
	}
	post := c.QueryParam("post_logout_redirect_uri")
	if post == "" {
		post = c.Request().PostFormValue("post_logout_redirect_uri")
	}
	if post == "" {
		return c.Redirect(http.StatusSeeOther, "/status?state=signed-out")
	}
	clientID := c.QueryParam("client_id")
	if clientID == "" {
		clientID = c.Request().PostFormValue("client_id")
	}
	if clientID == "" {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "client_id が必要"))
	}
	client, err := d.ClientRepo.FindByID(c.Request().Context(), requestTenantID(c), clientID)
	if err != nil {
		return writeOAuthError(c, err)
	}
	if client == nil || !containsString(client.RedirectURIs, post) {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "post_logout_redirect_uri が未登録"))
	}
	u, _ := url.Parse(post)
	query := u.Query()
	if state := c.QueryParam("state"); state != "" {
		query.Set("state", state)
	}
	u.RawQuery = query.Encode()
	return c.Redirect(http.StatusFound, u.String())
}

func (d Deps) transactionRequest(c *echo.Context) (*spec.AuthorizationRequest, error) {
	cookie, err := c.Cookie(authorizationTransactionCookie)
	if err != nil || cookie.Value == "" {
		return nil, errors.New("認可トランザクションがありません")
	}
	req, err := d.RequestStore.Find(c.Request().Context(), cookie.Value)
	if err != nil {
		return nil, err
	}
	if req == nil || req.TenantID != requestTenantID(c) ||
		time.Now().After(req.ExpiresAt) || req.State != spec.AuthFlowReceived {
		return nil, errors.New("認可トランザクションが無効または期限切れです")
	}
	return req, nil
}

func (d Deps) resolveAuthentication(c *echo.Context) (*authdomain.AuthenticationContext, error) {
	if d.AuthnResolver == nil {
		return nil, nil
	}
	authn, err := d.AuthnResolver.Resolve(
		c.Request().Context(),
		authdomain.HTTPHeadersAdapter{H: c.Request().Header},
	)
	if err != nil || authn == nil || d.UserRepo == nil {
		return authn, err
	}
	user, err := d.UserRepo.FindBySub(c.Request().Context(), authn.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil || user.DisabledAt != nil {
		if d.SessionManager != nil && authn.SessionID != "" {
			_ = d.SessionManager.Store.Delete(c.Request().Context(), authn.SessionID)
		}
		return nil, nil
	}
	return authn, nil
}

func (d Deps) verifyBrowserRequest(c *echo.Context) error {
	origin := c.Request().Header.Get("Origin")
	issuer, err := url.Parse(d.Issuer)
	if err != nil || origin == "" || origin != issuer.Scheme+"://"+issuer.Host {
		return writeBrowserError(c, http.StatusForbidden, "invalid_origin", "Originが一致しません")
	}
	cookie, err := c.Cookie(csrfCookie)
	header := c.Request().Header.Get(csrfHeader)
	if err != nil || cookie.Value == "" || header == "" ||
		len(cookie.Value) != len(header) ||
		subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
		return writeBrowserError(c, http.StatusForbidden, "csrf_failed", "CSRF検証に失敗しました")
	}
	return nil
}

func (d Deps) ensureCSRFCookie(c *echo.Context) (string, error) {
	if cookie, err := c.Cookie(csrfCookie); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}
	value, err := randomToken(32)
	if err != nil {
		return "", err
	}
	c.SetCookie(&http.Cookie{ //nolint:gosec // Secure is enabled for HTTPS issuers; local HTTP development intentionally disables it.
		Name: csrfCookie, Value: value, Path: tenantCookiePath(c),
		Secure: d.secureCookies(), HttpOnly: false, SameSite: http.SameSiteStrictMode,
		MaxAge: 600,
	})
	return value, nil
}

func (d Deps) setTransactionCookie(c *echo.Context, requestID string) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // Secure is enabled for HTTPS issuers; local HTTP development intentionally disables it.
		Name: authorizationTransactionCookie, Value: requestID, Path: tenantCookiePath(c),
		Secure: d.secureCookies(), HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: 600,
	})
}

func (d Deps) clearTransactionCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // Secure is enabled for HTTPS issuers; local HTTP development intentionally disables it.
		Name: authorizationTransactionCookie, Path: tenantCookiePath(c),
		Secure: d.secureCookies(), HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: -1,
	})
}

func (d Deps) setSessionCookie(c *echo.Context, sessionID string) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // Secure is enabled for HTTPS issuers; local HTTP development intentionally disables it.
		Name: authusecases.SessionCookie, Value: sessionID, Path: tenantCookiePath(c),
		Secure: d.secureCookies(), HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: authusecases.SessionTTLSeconds,
	})
}

func (d Deps) clearSessionCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // Secure is enabled for HTTPS issuers; local HTTP development intentionally disables it.
		Name: authusecases.SessionCookie, Path: tenantCookiePath(c),
		Secure: d.secureCookies(), HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: -1,
	})
}

func (d Deps) secureCookies() bool {
	return strings.HasPrefix(d.Issuer, "https://")
}

func (d Deps) emitAuthenticationFailure(username, reason string) {
	if d.Emit != nil {
		d.Emit(&spec.AuthenticationFailed{At: time.Now().UTC(), Username: username, Reason: reason})
	}
}

func decodeJSON(request *http.Request, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func noStoreJSON(c *echo.Context, status int, body any) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(status, body)
}

func writeBrowserError(c *echo.Context, status int, code, message string) error {
	return noStoreJSON(c, status, map[string]string{"error": code, "message": message})
}
