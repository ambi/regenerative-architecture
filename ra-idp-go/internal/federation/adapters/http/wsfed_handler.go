package http

import (
	"net/http"
	"net/url"
	"time"

	authdomain "ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/federation/adapters/samltoken"
	"ra-idp-go/internal/federation/adapters/wsfed"
	feddomain "ra-idp-go/internal/federation/domain"
	"ra-idp-go/internal/platform/http/core"

	"github.com/labstack/echo/v5"
)

// assertionLifetime は発行 assertion / RSTR の有効期間。
const assertionLifetime = 5 * time.Minute

// handleWsFedSignIn は WS-Federation passive sign-in を処理する。
//
// 未認証ならログイン UI に return_to つきで誘導し、認証済みなら relying party の
// claim policy で claim を発行し、署名済み SAML assertion を RSTR に包んで自動 POST する。
func (d Deps) handleWsFedSignIn(c *echo.Context) error {
	ctx := c.Request().Context()
	req := feddomain.ParseSignInRequest(c.QueryParam)

	if !req.IsSignIn() {
		// sign-out (wsignout1.0) は後続スライス。
		return c.String(http.StatusBadRequest, "unsupported wa")
	}
	if req.Wtrealm == "" {
		return c.String(http.StatusBadRequest, "wtrealm is required")
	}

	tenantID := core.RequestTenantID(c)
	rp, err := d.WsFedRPRepo.FindByWtrealm(ctx, tenantID, req.Wtrealm)
	if err != nil {
		return err
	}
	if rp == nil {
		return c.String(http.StatusBadRequest, "unknown relying party")
	}

	validated, err := feddomain.ValidateSignIn(req, *rp)
	if err != nil {
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

	result, err := feddomain.IssueClaims(rp.ClaimPolicy, feddomain.ResolveUserAttributes(*user))
	if err != nil {
		return c.String(http.StatusInternalServerError, "claim issuance failed")
	}

	now := time.Now().UTC()
	assertion, _, err := samltoken.BuildAssertion(samltoken.AssertionInput{
		Issuer:       core.RequestIssuer(c, d.Issuer),
		Audience:     rp.EffectiveAudience(),
		Recipient:    validated.ReplyURL,
		IssueInstant: now,
		NotBefore:    now.Add(-1 * time.Minute),
		NotOnOrAfter: now.Add(assertionLifetime),
		AuthnInstant: time.Unix(authn.AuthTime, 0).UTC(),
		Result:       result,
	})
	if err != nil {
		return c.String(http.StatusInternalServerError, "assertion build failed")
	}

	signed, err := d.FederationSigner.Sign(assertion)
	if err != nil {
		return c.String(http.StatusInternalServerError, "assertion signing failed")
	}

	rstr, err := wsfed.BuildRSTR(signed, rp.Wtrealm, now, now.Add(assertionLifetime))
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

	c.Response().Header().Set("Cache-Control", "no-store")
	return c.HTML(http.StatusOK, string(formHTML))
}

// loginRedirect はログイン UI への誘導 URL を、認証後に現在の WS-Fed 要求へ戻す
// return_to つきで組み立てる (同一オリジンの相対パスのみ)。
func loginRedirect(c *echo.Context) string {
	returnTo := c.Request().URL.RequestURI()
	return core.TenantRoute(c, "/login") + "?return_to=" + url.QueryEscape(returnTo)
}
