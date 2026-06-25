package http

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"time"

	authdomain "ra-idp-go/internal/authentication/domain"
	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/federation/adapters/samltoken"
	"ra-idp-go/internal/federation/adapters/wsfed"
	feddomain "ra-idp-go/internal/federation/domain"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

// assertionLifetime は発行 assertion / RSTR の有効期間。
const assertionLifetime = 5 * time.Minute

// samlVersion は RP の token type を samltoken の SAML バージョンへ写す。
func samlVersion(t spec.WsFedTokenType) samltoken.SAMLVersion {
	if t == spec.TokenTypeSAML20 {
		return samltoken.SAML20
	}
	return samltoken.SAML11
}

// handleWsFed は WS-Federation passive エンドポイントを wa で分岐する。
func (d Deps) handleWsFed(c *echo.Context) error {
	req := feddomain.ParseSignInRequest(c.QueryParam)
	switch req.Wa {
	case feddomain.WaSignIn:
		return d.handleWsFedSignIn(c, req)
	case feddomain.WaSignOut, feddomain.WaSignOutCleanup:
		return d.handleWsFedSignOut(c, req)
	default:
		return c.String(http.StatusBadRequest, "unsupported wa")
	}
}

// handleWsFedSignIn は passive sign-in を処理する。未認証ならログイン UI に return_to つきで
// 誘導し、認証済みなら relying party の claim policy で claim を発行し、署名済み SAML assertion を
// RSTR に包んで自動 POST する。
func (d Deps) handleWsFedSignIn(c *echo.Context, req feddomain.WsFedSignInRequest) error {
	ctx := c.Request().Context()
	tenantID := core.RequestTenantID(c)

	if req.Wtrealm == "" {
		d.emit(&spec.WsFedSignInRejected{At: time.Now().UTC(), TenantID: tenantID, Reason: "wtrealm is required"})
		return c.String(http.StatusBadRequest, "wtrealm is required")
	}

	rp, err := d.WsFedRPRepo.FindByWtrealm(ctx, tenantID, req.Wtrealm)
	if err != nil {
		return err
	}
	if rp == nil {
		d.emit(&spec.WsFedSignInRejected{At: time.Now().UTC(), TenantID: tenantID, Wtrealm: req.Wtrealm, Reason: "unknown relying party"})
		return c.String(http.StatusBadRequest, "unknown relying party")
	}

	validated, err := feddomain.ValidateSignIn(req, *rp)
	if err != nil {
		d.emit(&spec.WsFedSignInRejected{At: time.Now().UTC(), TenantID: tenantID, Wtrealm: req.Wtrealm, Reason: err.Error()})
		return c.String(http.StatusBadRequest, err.Error())
	}

	// セッション解決。未認証ならログインへ誘導し、認証後に同じ URL へ戻す。
	authn, _ := d.AuthnResolver.Resolve(ctx, authdomain.HTTPHeadersAdapter{H: c.Request().Header})
	if authn == nil || authn.Sub == "" || authn.AuthenticationPending {
		return c.Redirect(http.StatusSeeOther, loginRedirect(c))
	}

	user, err := d.UserRepo.FindBySub(ctx, authn.Sub)
	if err != nil {
		return err
	}
	if user == nil || !user.IsActive() {
		return c.Redirect(http.StatusSeeOther, loginRedirect(c))
	}

	now := time.Now().UTC()

	// wfresh: 認証が古すぎれば再認証のためログインへ誘導する。
	if feddomain.RequiresFreshAuth(req.Wfresh, time.Unix(authn.AuthTime, 0), now) {
		return c.Redirect(http.StatusSeeOther, loginRedirect(c))
	}
	// wauth: 要求された認証方式を尊重する。満たせない方式 (統合 Windows 等) は拒否する。
	authnMethod, err := feddomain.ResolveAuthnMethod(req.Wauth, authn.AMR)
	if err != nil {
		d.emit(&spec.WsFedSignInRejected{At: now, TenantID: tenantID, Wtrealm: rp.Wtrealm, Reason: err.Error()})
		return c.String(http.StatusBadRequest, err.Error())
	}

	result, err := feddomain.IssueClaims(rp.ClaimPolicy, feddomain.ResolveUserAttributes(*user))
	if err != nil {
		d.emit(&spec.WsFedSignInRejected{At: now, TenantID: tenantID, Wtrealm: rp.Wtrealm, Reason: "claim issuance failed"})
		return c.String(http.StatusInternalServerError, "claim issuance failed")
	}

	tokenType := rp.EffectiveTokenType()
	signed, _, err := samltoken.BuildSignedAssertion(samltoken.AssertionInput{
		Version:      samlVersion(tokenType),
		Issuer:       core.RequestIssuer(c, d.Issuer),
		Audience:     rp.EffectiveAudience(),
		Recipient:    validated.ReplyURL,
		IssueInstant: now,
		NotBefore:    now.Add(-1 * time.Minute),
		NotOnOrAfter: now.Add(assertionLifetime),
		AuthnInstant: time.Unix(authn.AuthTime, 0).UTC(),
		AuthnMethod:  authnMethod,
		Result:       result,
	}, d.FederationSigner)
	if err != nil {
		return c.String(http.StatusInternalServerError, "assertion build failed")
	}

	rstr, err := wsfed.BuildRSTR(signed, rp.Wtrealm, string(tokenType), now, now.Add(assertionLifetime))
	if err != nil {
		return c.String(http.StatusInternalServerError, "rstr build failed")
	}
	wresult, err := wsfed.SerializeRSTR(rstr)
	if err != nil {
		return c.String(http.StatusInternalServerError, "rstr serialize failed")
	}
	formHTML, err := wsfed.RenderPassiveForm(validated.ReplyURL, wresult, validated.Wctx)
	if err != nil {
		return c.String(http.StatusInternalServerError, "form render failed")
	}

	d.emit(&spec.WsFedSignInIssued{At: now, TenantID: tenantID, Wtrealm: rp.Wtrealm, Sub: authn.Sub})
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.HTML(http.StatusOK, string(formHTML))
}

// handleWsFedSignOut はローカルセッションを破棄する。wsignout1.0 は許可済み wreply への
// リダイレクトまで行い、wsignoutcleanup1.0 は破棄のみで 200 を返す。
func (d Deps) handleWsFedSignOut(c *echo.Context, req feddomain.WsFedSignInRequest) error {
	ctx := c.Request().Context()
	tenantID := core.RequestTenantID(c)

	if d.SessionManager != nil {
		_ = d.SessionManager.Revoke(ctx, c.Request().Header.Get("Cookie"))
	}
	d.clearSessionCookie(c)
	d.emit(&spec.WsFedSignOut{At: time.Now().UTC(), TenantID: tenantID, Wtrealm: req.Wtrealm})

	if req.Wa == feddomain.WaSignOut {
		if target := d.resolveSignOutReply(ctx, tenantID, req); target != "" {
			return c.Redirect(http.StatusSeeOther, target)
		}
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.String(http.StatusOK, "signed out")
}

// resolveSignOutReply は wreply を、wtrealm で解決した RP の許可集合に対して検証する。
// 検証を通らない (または wtrealm/wreply 不在) なら空文字を返し、リダイレクトしない (open redirect 防止)。
func (d Deps) resolveSignOutReply(ctx context.Context, tenantID string, req feddomain.WsFedSignInRequest) string {
	if req.Wtrealm == "" || req.Wreply == "" {
		return ""
	}
	rp, err := d.WsFedRPRepo.FindByWtrealm(ctx, tenantID, req.Wtrealm)
	if err != nil || rp == nil {
		return ""
	}
	if slices.Contains(rp.ReplyURLs, req.Wreply) {
		return req.Wreply
	}
	return ""
}

func (d Deps) emit(event spec.DomainEvent) {
	if d.Emit != nil {
		d.Emit(event)
	}
}

func (d Deps) clearSessionCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // Secure は HTTPS issuer で有効化、ローカル HTTP 開発では意図的に無効。
		Name: authusecases.SessionCookie, Path: core.TenantCookiePath(c),
		Secure: d.SecureCookies(), HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: -1,
	})
}

// loginRedirect はログイン UI への誘導 URL を、認証後に現在の WS-Fed 要求へ戻す
// return_to つきで組み立てる (同一オリジンの相対パスのみ)。
func loginRedirect(c *echo.Context) string {
	returnTo := c.Request().URL.RequestURI()
	return core.TenantRoute(c, "/login") + "?return_to=" + url.QueryEscape(returnTo)
}
