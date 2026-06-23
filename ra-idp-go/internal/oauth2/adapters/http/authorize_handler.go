// /authorize + browser authentication APIs + /end_session
package http

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	authdomain "ra-idp-go/internal/authentication/domain"
	authports "ra-idp-go/internal/authentication/ports"
	authusecases "ra-idp-go/internal/authentication/usecases"
	oauthdomain "ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

const (
	authorizationTransactionCookie = "ra_idp_transaction"
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

type loginAPIRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	ReturnTo string `json:"return_to,omitempty"`
}

type consentAPIRequest struct {
	Action string `json:"action"`
}

type totpAPIRequest struct {
	Code     string `json:"code"`
	ReturnTo string `json:"return_to,omitempty"`
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
		if consumed.TenantID != core.RequestTenantID(c) {
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
	details, err := usecases.ParseAuthorizationDetails(q.Get("authorization_details"))
	if err != nil {
		return writeOAuthError(c, err)
	}
	in := usecases.AuthorizeRequestInput{
		ClientID: request.ClientID, RedirectURI: request.RedirectURI,
		ResponseType: request.ResponseType, Scope: request.Scope,
		StateParam: request.StateParam, Nonce: request.Nonce,
		CodeChallenge: request.CodeChallenge, CodeChallengeMethod: request.CodeChallengeMethod,
		Prompt: request.Prompt, MaxAge: request.MaxAge, ACRValues: request.AcrValues, ParUsed: parUsed,
		AuthorizationDetails: details,
	}
	if requestURI := c.QueryParam("request_uri"); requestURI != "" {
		in.ParRequestURI = requestURI
	}
	out, err := usecases.Authorize(c.Request().Context(), usecases.AuthorizeDeps{
		ClientRepo:          d.ClientRepo,
		RequestStore:        d.RequestStore,
		AuthzDetailTypeRepo: d.AuthzDetailTypeRepo,
	}, in)
	if err != nil {
		return writeOAuthError(c, err)
	}
	if len(details) > 0 && d.Emit != nil {
		d.Emit(&spec.AuthorizationDetailsRequested{
			At: time.Now().UTC(), TenantID: core.RequestTenantID(c), ClientID: out.Request.ClientID,
			DetailTypes: oauthdomain.DetailTypes(details),
		})
	}

	d.setTransactionCookie(c, out.Request.ID)
	if d.AuthnResolver != nil {
		authn, _ := d.AuthnResolver.Resolve(c.Request().Context(), authdomain.HTTPHeadersAdapter{H: c.Request().Header})
		if authn != nil {
			if authn.AuthenticationPending {
				if in.Prompt == "none" {
					return writeOAuthError(c, usecases.NewOAuthError("login_required", "追加factor検証が必要です"))
				}
				return c.Redirect(http.StatusSeeOther, core.TenantRoute(c, "/totp"))
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
					return c.Redirect(http.StatusSeeOther, core.TenantRoute(c, "/totp"))
				}
				return c.Redirect(http.StatusSeeOther, core.TenantRoute(c, "/login"))
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
	return c.Redirect(http.StatusSeeOther, core.TenantRoute(c, "/login"))
}

func (d Deps) handleTransaction(c *echo.Context) error {
	req, err := d.transactionRequest(c)
	if err != nil {
		if returnTo := c.QueryParam("return_to"); returnTo != "" {
			if !validAdminReturnTo(c, returnTo) {
				return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "return_to が不正です")
			}
			csrf, csrfErr := d.EnsureCSRFCookie(c)
			if csrfErr != nil {
				return csrfErr
			}
			authn, _ := d.ResolveAuthentication(c)
			if authn != nil && authn.AuthenticationPending {
				return core.NoStoreJSON(c, http.StatusOK, transactionResponse{Kind: "totp", CSRFToken: csrf})
			}
			return core.NoStoreJSON(c, http.StatusOK, transactionResponse{Kind: "login", CSRFToken: csrf})
		}
		return core.WriteBrowserError(c, http.StatusUnauthorized, "transaction_unavailable", err.Error())
	}
	csrf, err := d.EnsureCSRFCookie(c)
	if err != nil {
		return err
	}
	if req.Sub == nil {
		authn, _ := d.ResolveAuthentication(c)
		if authn != nil && authn.AuthenticationPending {
			return core.NoStoreJSON(c, http.StatusOK, transactionResponse{Kind: "totp", CSRFToken: csrf})
		}
		return core.NoStoreJSON(c, http.StatusOK, transactionResponse{Kind: "login", CSRFToken: csrf})
	}
	authn, _ := d.ResolveAuthentication(c)
	if authn != nil && authn.AuthenticationPending {
		return core.NoStoreJSON(c, http.StatusOK, transactionResponse{Kind: "totp", CSRFToken: csrf})
	}
	if authn == nil || authn.Sub != *req.Sub {
		return core.WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証セッションが一致しません")
	}
	client, err := d.ClientRepo.FindByID(c.Request().Context(), core.RequestTenantID(c), req.ClientID)
	if err != nil {
		return err
	}
	if client == nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_transaction", "クライアントが存在しません")
	}
	name := client.ClientID
	if client.ClientName != nil {
		name = *client.ClientName
	}
	return core.NoStoreJSON(c, http.StatusOK, transactionResponse{
		Kind: "consent", CSRFToken: csrf, ClientName: name, Scopes: strings.Fields(req.Scope),
	})
}

func (d Deps) handleLoginAPI(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	var input loginAPIRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if strings.TrimSpace(input.Username) == "" || input.Password == "" {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "ユーザー名とパスワードが必要です")
	}
	req, transactionErr := d.transactionRequest(c)
	directAdminLogin := transactionErr != nil && input.ReturnTo != ""
	if directAdminLogin {
		if !validAdminReturnTo(c, input.ReturnTo) {
			return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "return_to が不正です")
		}
	} else if transactionErr != nil {
		return core.WriteBrowserError(c, http.StatusUnauthorized, "transaction_unavailable", transactionErr.Error())
	}

	normalizedUsername := strings.ToLower(input.Username)
	clientIP := extractClientIP(c.Request(), d.TrustedForwardedHops)
	if result, err := d.acquireLoginThrottle(c, authports.LoginThrottleAccount, normalizedUsername); err != nil {
		return err
	} else if !result.Allowed {
		return writeLoginThrottled(c, result.RetryAfterSeconds)
	}
	if clientIP != "" {
		if result, err := d.acquireLoginThrottle(c, authports.LoginThrottleIP, clientIP); err != nil {
			return err
		} else if !result.Allowed {
			return writeLoginThrottled(c, result.RetryAfterSeconds)
		}
	}

	user, err := d.UserRepo.FindByUsername(c.Request().Context(), core.RequestTenantID(c), input.Username)
	if err != nil {
		return err
	}
	hashToVerify := d.SentinelPasswordHash
	if user != nil {
		hashToVerify = user.PasswordHash
	}
	ok := false
	if hashToVerify != "" {
		ok, err = d.PasswordHasher.Verify(input.Password, hashToVerify)
	}
	if user == nil || err != nil || !ok {
		aggregated, ferr := d.recordLoginFailure(c, normalizedUsername, clientIP)
		if ferr != nil {
			return ferr
		}
		// 閾値超過後は AuthenticationEventAggregated に集約し、個別行を出さない (爆発抑制)。
		if !aggregated {
			d.emitAuthenticationFailure(c, input.Username, "invalid_credentials")
		}
		return core.WriteBrowserError(c, http.StatusUnauthorized, "invalid_credentials", "ユーザー名またはパスワードを確認してください。")
	}
	if !user.IsActive() {
		d.emitAuthenticationFailure(c, input.Username, "account_disabled")
		return core.WriteBrowserError(c, http.StatusUnauthorized, "invalid_credentials", "ユーザー名またはパスワードを確認してください。")
	}
	if result := authusecases.ValidatePassword(input.Password); !result.OK {
		return core.WriteBrowserError(c, http.StatusUnauthorized, "password_policy", "パスワードがセキュリティ要件を満たしていません。")
	}
	if d.LoginAttemptThrottle != nil {
		if err := d.LoginAttemptThrottle.RecordSuccess(
			c.Request().Context(), authports.LoginThrottleAccount, normalizedUsername,
		); err != nil {
			return err
		}
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
		d.Emit(&spec.UserAuthenticated{At: authTime, TenantID: user.TenantID, Sub: user.Sub, AMR: []string{"pwd"}})
	}
	if user.MfaEnrolled {
		next := core.TenantRoute(c, "/totp")
		if directAdminLogin {
			next += "?return_to=" + url.QueryEscape(input.ReturnTo)
		}
		return core.NoStoreJSON(c, http.StatusOK, browserFlowResponse{Next: next})
	}
	// full authentication 完了 (pwd-only)。last_login_at 記録 + required action gate。
	gateNext, err := d.recordLoginAndRequiredAction(c, user, authTime)
	if err != nil {
		return err
	}
	if gateNext != "" {
		return core.NoStoreJSON(c, http.StatusOK, browserFlowResponse{Next: gateNext})
	}
	if directAdminLogin {
		return core.NoStoreJSON(c, http.StatusOK, browserFlowResponse{RedirectTo: input.ReturnTo})
	}
	if err := d.RequestStore.AttachAuthentication(
		c.Request().Context(), req.ID, user.Sub, authn.AuthTime, authn.AMR, authn.ACR,
	); err != nil {
		return writeOAuthError(c, err)
	}
	req.Sub, req.AuthTime = &user.Sub, &authn.AuthTime
	client, err := d.ClientRepo.FindByID(c.Request().Context(), core.RequestTenantID(c), req.ClientID)
	if err != nil || client == nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_transaction", "クライアントが存在しません")
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

// recordLoginAndRequiredAction は full authentication 完了時に last_login_at を
// 記録し (wi-19)、未充足の required action があればログイン後の強制誘導先を返す。
// 返り値 gateNext が非空なら OAuth フローを完了させず、その画面へ誘導する。現状
// 専用画面のある update_password のみ change-password へ gate する (UI 拡張は wi-21)。
func (d Deps) recordLoginAndRequiredAction(c *echo.Context, user *spec.User, now time.Time) (string, error) {
	updated := *user
	updated.Lifecycle.LastLoginAt = &now
	if err := d.UserRepo.Save(c.Request().Context(), &updated); err != nil {
		return "", err
	}
	*user = updated
	if slices.Contains(updated.Lifecycle.RequiredActions, spec.RequiredActionUpdatePassword) {
		return core.TenantRoute(c, "/change_password"), nil
	}
	return "", nil
}

func (d Deps) handleTOTPAPI(c *echo.Context) error {
	if d.MfaFactorRepo == nil {
		return core.WriteBrowserError(c, http.StatusServiceUnavailable, "mfa_unavailable", "MFA factor store is unavailable")
	}
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	authn, _ := d.ResolveAuthentication(c)
	if authn == nil || authn.SessionID == "" {
		return core.WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "TOTP検証セッションがありません")
	}
	if containsString(authn.AMR, "otp") && !authn.AuthenticationPending {
		return core.WriteBrowserError(c, http.StatusForbidden, "access_denied", "TOTPは既に検証済みです")
	}
	var input totpAPIRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	req, transactionErr := d.transactionRequest(c)
	directAdminLogin := transactionErr != nil && input.ReturnTo != ""
	if directAdminLogin {
		if !validAdminReturnTo(c, input.ReturnTo) {
			return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "return_to が不正です")
		}
	} else if transactionErr != nil {
		return core.WriteBrowserError(c, http.StatusUnauthorized, "transaction_unavailable", transactionErr.Error())
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
		d.emitAuthenticationFailure(c, authn.Sub, result.Reason)
		return core.WriteBrowserError(c, http.StatusUnauthorized, "invalid_totp", "TOTPコードを確認してください。")
	}
	completed, err := d.SessionManager.CompleteFactor(c.Request().Context(), authn.SessionID, []string{"otp"})
	if err != nil {
		return err
	}
	if completed == nil {
		return core.WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "セッションが失効しました")
	}
	d.setSessionCookie(c, completed.SessionID)
	if d.Emit != nil {
		d.Emit(&spec.UserAuthenticated{At: time.Now().UTC(), Sub: completed.Sub, AMR: completed.AMR})
	}
	// full authentication 完了 (pwd + otp)。last_login_at 記録 + required action gate。
	if user, err := d.UserRepo.FindBySub(c.Request().Context(), completed.Sub); err != nil {
		return err
	} else if user != nil {
		gateNext, err := d.recordLoginAndRequiredAction(c, user, time.Now().UTC())
		if err != nil {
			return err
		}
		if gateNext != "" {
			return core.NoStoreJSON(c, http.StatusOK, browserFlowResponse{Next: gateNext})
		}
	}
	if directAdminLogin {
		return core.NoStoreJSON(c, http.StatusOK, browserFlowResponse{RedirectTo: input.ReturnTo})
	}
	if err := d.RequestStore.AttachAuthentication(
		c.Request().Context(), req.ID, completed.Sub, completed.AuthTime, completed.AMR, completed.ACR,
	); err != nil {
		return writeOAuthError(c, err)
	}
	req.Sub, req.AuthTime, req.AMR, req.ACR = &completed.Sub, &completed.AuthTime, completed.AMR, &completed.ACR
	client, err := d.ClientRepo.FindByID(c.Request().Context(), core.RequestTenantID(c), req.ClientID)
	if err != nil || client == nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_transaction", "クライアントが存在しません")
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

func (d Deps) handleConsentAPI(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	req, err := d.transactionRequest(c)
	if err != nil {
		return core.WriteBrowserError(c, http.StatusUnauthorized, "transaction_unavailable", err.Error())
	}
	authn, _ := d.ResolveAuthentication(c)
	if authn == nil || req.Sub == nil || authn.Sub != *req.Sub {
		return core.WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証セッションが一致しません")
	}
	var input consentAPIRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if input.Action != "allow" {
		_ = d.RequestStore.UpdateState(c.Request().Context(), req.ID, spec.AuthFlowRejected)
		d.clearTransactionCookie(c)
		return core.NoStoreJSON(c, http.StatusOK, browserFlowResponse{RedirectTo: authorizationErrorURL(req, core.RequestIssuer(c, d.Issuer), "access_denied", "")})
	}

	scopes := strings.Fields(req.Scope)
	if d.ConsentRepo != nil {
		now := time.Now().UTC()
		if err := d.ConsentRepo.Save(c.Request().Context(), &spec.Consent{
			TenantID: core.RequestTenantID(c), Sub: authn.Sub, ClientID: req.ClientID,
			Scopes: scopes, State: spec.ConsentGranted,
			GrantedAt: now, ExpiresAt: now.Add(365 * 24 * time.Hour),
			AuthorizationDetails: req.AuthorizationDetails,
		}); err != nil {
			return err
		}
		if d.Emit != nil {
			d.Emit(&spec.ConsentGrantedEvent{At: now, TenantID: core.RequestTenantID(c), Sub: authn.Sub, ClientID: req.ClientID, Scopes: scopes})
			if len(req.AuthorizationDetails) > 0 {
				d.Emit(&spec.AuthorizationDetailsConsented{
					At: now, TenantID: core.RequestTenantID(c), Sub: authn.Sub, ClientID: req.ClientID,
					DetailTypes: oauthdomain.DetailTypes(req.AuthorizationDetails),
				})
			}
		}
	}
	redirectTo, err := d.issueCodeURL(c, req, authn.Sub, time.Unix(authn.AuthTime, 0))
	if err != nil {
		return err
	}
	d.clearTransactionCookie(c)
	return core.NoStoreJSON(c, http.StatusOK, browserFlowResponse{RedirectTo: redirectTo})
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
		return authorizationNext{Path: core.TenantRoute(c, "/totp")}, nil
	}
	if d.ConsentRepo != nil {
		consent, _ := d.ConsentRepo.Find(
			c.Request().Context(), core.RequestTenantID(c), authn.Sub, client.ClientID,
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
		// RFC 9396 — 構造化された authorization_details は粗い scope 同意では代替できない。
		// 明示同意を要求し、過去 scope 同意での自動スキップを許さない (fail-closed, ADR-050)。
		if len(req.AuthorizationDetails) > 0 {
			covered = false
		}
		if !covered {
			if err := d.RequestStore.AttachAuthentication(
				c.Request().Context(), req.ID, authn.Sub, authn.AuthTime, authn.AMR, authn.ACR,
			); err != nil {
				return authorizationNext{}, err
			}
			req.Sub, req.AuthTime = &authn.Sub, &authn.AuthTime
			return authorizationNext{Path: core.TenantRoute(c, "/consent")}, nil
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
	iss := core.RequestIssuer(c, d.Issuer)
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
			return authorizationErrorURL(req, iss, oauthErr.Code, oauthErr.Description), nil
		}
		return "", err
	}
	if d.Emit != nil {
		d.Emit(&spec.AuthorizationCodeIssued{
			At: time.Now().UTC(), TenantID: core.RequestTenantID(c), ClientID: req.ClientID, Sub: sub,
			Scopes: out.Code.Scopes, CodeChallengeMethod: req.CodeChallengeMethod,
		})
	}
	u, _ := url.Parse(out.Request.RedirectURI)
	query := u.Query()
	query.Set("code", out.Code.Code)
	if out.Request.StateParam != nil {
		query.Set("state", *out.Request.StateParam)
	}
	// RFC 9207 §2: Authorization Server Issuer Identification (mix-up 攻撃対策)。
	query.Set("iss", iss)
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func authorizationErrorURL(req *spec.AuthorizationRequest, iss, code, description string) string {
	u, _ := url.Parse(req.RedirectURI)
	query := u.Query()
	query.Set("error", code)
	if description != "" {
		query.Set("error_description", description)
	}
	if req.StateParam != nil {
		query.Set("state", *req.StateParam)
	}
	// RFC 9207 §2: error response も含めて iss を必須にする。
	if iss != "" {
		query.Set("iss", iss)
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
		return core.NoStoreJSON(c, http.StatusOK, browserFlowResponse{RedirectTo: next.RedirectTo})
	}
	return core.NoStoreJSON(c, http.StatusOK, browserFlowResponse{Next: next.Path})
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
	client, err := d.ClientRepo.FindByID(c.Request().Context(), core.RequestTenantID(c), clientID)
	if err != nil {
		return writeOAuthError(c, err)
	}
	if client == nil {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "post_logout_redirect_uri が未登録"))
	}
	// Redirect only to a URI from the client's registered allowlist. Selecting the
	// stored value (rather than reusing the request parameter) keeps the redirect
	// target server-controlled and avoids open-redirect via user input.
	registered := ""
	for _, uri := range client.RedirectURIs {
		if uri == post {
			registered = uri
			break
		}
	}
	if registered == "" {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "post_logout_redirect_uri が未登録"))
	}
	u, err := url.Parse(registered)
	if err != nil {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_request", "post_logout_redirect_uri が不正"))
	}
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
	if req == nil || req.TenantID != core.RequestTenantID(c) ||
		time.Now().After(req.ExpiresAt) || req.State != spec.AuthFlowReceived {
		return nil, errors.New("認可トランザクションが無効または期限切れです")
	}
	return req, nil
}

func validAdminReturnTo(c *echo.Context, returnTo string) bool {
	if strings.Contains(returnTo, "\\") {
		return false
	}
	parsed, err := url.Parse(returnTo)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.Fragment != "" {
		return false
	}
	if path.Clean(parsed.Path) != parsed.Path {
		return false
	}
	adminRoot := core.TenantRoute(c, "/admin")
	return parsed.Path == adminRoot || strings.HasPrefix(parsed.Path, adminRoot+"/")
}

func (d Deps) setTransactionCookie(c *echo.Context, requestID string) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // Secure is enabled for HTTPS issuers; local HTTP development intentionally disables it.
		Name: authorizationTransactionCookie, Value: requestID, Path: core.TenantCookiePath(c),
		Secure: d.SecureCookies(), HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: 600,
	})
}

func (d Deps) clearTransactionCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // Secure is enabled for HTTPS issuers; local HTTP development intentionally disables it.
		Name: authorizationTransactionCookie, Path: core.TenantCookiePath(c),
		Secure: d.SecureCookies(), HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: -1,
	})
}

func (d Deps) setSessionCookie(c *echo.Context, sessionID string) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // Secure is enabled for HTTPS issuers; local HTTP development intentionally disables it.
		Name: authusecases.SessionCookie, Value: sessionID, Path: core.TenantCookiePath(c),
		Secure: d.SecureCookies(), HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: authusecases.SessionTTLSeconds,
	})
}

func (d Deps) clearSessionCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // Secure is enabled for HTTPS issuers; local HTTP development intentionally disables it.
		Name: authusecases.SessionCookie, Path: core.TenantCookiePath(c),
		Secure: d.SecureCookies(), HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: -1,
	})
}

func (d Deps) emitAuthenticationFailure(c *echo.Context, username, reason string) {
	if d.Emit != nil {
		d.Emit(&spec.AuthenticationFailed{
			At: time.Now().UTC(), TenantID: core.RequestTenantID(c), Username: username, Reason: reason,
		})
	}
}

func (d Deps) acquireLoginThrottle(
	c *echo.Context,
	kind authports.LoginThrottleKind,
	key string,
) (authports.LoginThrottleResult, error) {
	if d.LoginAttemptThrottle == nil {
		return authports.LoginThrottleResult{Allowed: true}, nil
	}
	return d.LoginAttemptThrottle.TryAcquire(c.Request().Context(), kind, key, time.Now().UTC())
}

// recordLoginFailure は失敗を throttle に記録し、閾値超過 (Locked) の key については
// LoginThrottled を emit したうえで失敗を集約 bucket に積む。集約に切り替わった場合は
// aggregated=true を返し、呼び出し側は個別の AuthenticationFailed を抑制する
// (これが攻撃時の行爆発を止める要点 / wi-20 スライス 3)。
func (d Deps) recordLoginFailure(c *echo.Context, username, clientIP string) (bool, error) {
	if d.LoginAttemptThrottle == nil {
		return false, nil
	}
	now := time.Now().UTC()
	aggregated := false
	for _, attempt := range []struct {
		kind authports.LoginThrottleKind
		key  string
	}{
		{authports.LoginThrottleAccount, username},
		{authports.LoginThrottleIP, clientIP},
	} {
		if attempt.key == "" {
			continue
		}
		result, err := d.LoginAttemptThrottle.RecordFailure(
			c.Request().Context(), attempt.kind, attempt.key, now,
		)
		if err != nil {
			return aggregated, err
		}
		if !result.Locked {
			continue
		}
		keyHash := hashThrottleKey(attempt.key)
		if d.Emit != nil {
			d.Emit(&spec.LoginThrottled{
				At: now, TenantID: core.RequestTenantID(c), Kind: string(attempt.kind),
				KeyHash:           keyHash,
				RetryAfterSeconds: result.RetryAfterSeconds,
			})
		}
		if d.recordFailedLoginBucket(c, keyHash, now) {
			aggregated = true
		}
	}
	return aggregated, nil
}

func hashThrottleKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// recordFailedLoginBucket は閾値超過後の失敗を 5 分窓の bucket に積み、その窓で最初の
// 記録だったときだけ AuthenticationEventAggregated を 1 件 emit する。bucket store が
// 無い構成では集約せず false を返し、呼び出し側は従来どおり個別イベントを残す。
func (d Deps) recordFailedLoginBucket(c *echo.Context, keyHash string, now time.Time) bool {
	if d.AuthEventBucketStore == nil {
		return false
	}
	result, err := d.AuthEventBucketStore.Record(
		c.Request().Context(), authports.AuthEventBucketFailedLogin, core.RequestTenantID(c), keyHash, now,
	)
	if err != nil {
		return false
	}
	if result.FirstInWindow && d.Emit != nil {
		bucket := result.Bucket
		d.Emit(&spec.AuthenticationEventAggregated{
			At: now, TenantID: bucket.TenantID, Kind: string(bucket.Kind),
			BucketKey: failedLoginBucketKey(bucket),
			KeyHash:   bucket.KeyHash, Count: bucket.Count,
			FirstSeen: bucket.FirstSeen, LastSeen: bucket.LastSeen,
			TopKeys: []string{bucket.KeyHash},
		})
	}
	return true
}

func failedLoginBucketKey(bucket authports.AuthEventBucket) string {
	return string(bucket.Kind) + ":" + bucket.KeyHash + ":" +
		strconv.FormatInt(bucket.WindowStart.Unix(), 10)
}

func extractClientIP(request *http.Request, trustedHops int) string {
	if request == nil || trustedHops <= 0 {
		return ""
	}
	parts := strings.Split(request.Header.Get("X-Forwarded-For"), ",")
	ips := make([]string, 0, len(parts))
	for _, part := range parts {
		if ip := strings.TrimSpace(part); ip != "" {
			ips = append(ips, ip)
		}
	}
	index := len(ips) - 1 - trustedHops
	if index < 0 || index >= len(ips) {
		return ""
	}
	return ips[index]
}

func writeLoginThrottled(c *echo.Context, retryAfterSeconds int) error {
	c.Response().Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	return core.NoStoreJSON(c, http.StatusTooManyRequests, map[string]any{
		"error": "too_many_requests", "retry_after_seconds": retryAfterSeconds,
	})
}
