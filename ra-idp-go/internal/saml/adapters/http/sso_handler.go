package http

import (
	"net/http"
	"net/url"
	"slices"
	"time"

	authdomain "ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/saml/adapters/samlresponse"
	samldomain "ra-idp-go/internal/saml/domain"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/wsfederation/adapters/samltoken"
	feddomain "ra-idp-go/internal/wsfederation/domain"

	"github.com/beevik/etree"
	"github.com/labstack/echo/v5"
)

// assertionLifetime は発行 assertion / SAMLResponse の有効期間。
const assertionLifetime = 5 * time.Minute

// handleSamlSSORedirect は HTTP-Redirect binding と IdP-initiated SSO を処理する。
// SAMLRequest があれば SP-initiated、無ければ entityID クエリで IdP-initiated とする。
func (d Deps) handleSamlSSORedirect(c *echo.Context) error {
	samlRequest := c.QueryParam("SAMLRequest")
	relayState := c.QueryParam("RelayState")
	if samlRequest == "" {
		return d.handleIdPInitiated(c, relayState)
	}
	xml, err := samldomain.DecodeRedirect(samlRequest)
	if err != nil {
		return d.rejectSSO(c, "", "decode redirect AuthnRequest", err)
	}
	return d.processAuthnRequest(c, xml, relayState)
}

// handleSamlSSOPost は HTTP-POST binding の SP-initiated SSO を処理する。
func (d Deps) handleSamlSSOPost(c *echo.Context) error {
	samlRequest := c.FormValue("SAMLRequest")
	relayState := c.FormValue("RelayState")
	if samlRequest == "" {
		return d.rejectSSO(c, "", "missing SAMLRequest", nil)
	}
	xml, err := samldomain.DecodePost(samlRequest)
	if err != nil {
		return d.rejectSSO(c, "", "decode POST AuthnRequest", err)
	}
	return d.processAuthnRequest(c, xml, relayState)
}

// processAuthnRequest は復号済み AuthnRequest を解析・検証し、SAMLResponse を発行する。
// 未認証時のログイン往復をまたいで要求を保つため、redirect binding に符号化した resume URL を渡す。
func (d Deps) processAuthnRequest(c *echo.Context, xml []byte, relayState string) error {
	req, err := samldomain.ParseAuthnRequest(xml)
	if err != nil {
		return d.rejectSSO(c, "", "parse AuthnRequest", err)
	}
	encoded, err := samldomain.EncodeRedirect(xml)
	if err != nil {
		return d.rejectSSO(c, req.Issuer, "encode resume request", err)
	}
	resumeURL := core.TenantRoute(c, "/saml/sso") + "?SAMLRequest=" + url.QueryEscape(encoded)
	if relayState != "" {
		resumeURL += "&RelayState=" + url.QueryEscape(relayState)
	}
	return d.issueForRequest(c, req, relayState, resumeURL)
}

// handleIdPInitiated は entityID 指定の IdP-initiated SSO を処理する。AuthnRequest を伴わないため、
// SP の既定 ACS へ InResponseTo 無しの SAMLResponse を発行する。
func (d Deps) handleIdPInitiated(c *echo.Context, relayState string) error {
	entityID := c.QueryParam("entityID")
	if entityID == "" {
		entityID = c.QueryParam("sp")
	}
	if entityID == "" {
		return d.rejectSSO(c, "", "missing SAMLRequest or entityID", nil)
	}
	req := samldomain.AuthnRequest{Issuer: entityID}
	return d.issueForRequest(c, req, relayState, c.Request().URL.RequestURI())
}

// issueForRequest は要求を SP に解決・検証し、認証ゲートを適用して SAMLResponse を発行する。
func (d Deps) issueForRequest(c *echo.Context, req samldomain.AuthnRequest, relayState, resumeURL string) error {
	ctx := c.Request().Context()
	tenantID := core.RequestTenantID(c)

	if d.SamlSPRepo == nil {
		return c.String(http.StatusBadRequest, "SAML is not available")
	}
	sp, err := d.SamlSPRepo.FindByEntityID(ctx, tenantID, req.Issuer)
	if err != nil {
		return err
	}
	if sp == nil {
		return d.rejectSSO(c, req.Issuer, "unknown service provider", nil)
	}
	validated, err := samldomain.ValidateSignIn(req, *sp)
	if err != nil {
		return d.rejectSSO(c, req.Issuer, err.Error(), nil)
	}

	authn, _ := d.AuthnResolver.Resolve(ctx, authdomain.HTTPHeadersAdapter{H: c.Request().Header})
	if authn == nil || authn.Sub == "" || authn.AuthenticationPending {
		return c.Redirect(http.StatusSeeOther, loginRedirect(c, resumeURL))
	}
	user, err := d.UserRepo.FindBySub(ctx, authn.Sub)
	if err != nil {
		return err
	}
	if user == nil || !user.IsActive() {
		return c.Redirect(http.StatusSeeOther, loginRedirect(c, resumeURL))
	}

	now := time.Now().UTC()

	// 割当ゲート: SP が Application binding に属する場合、未割当 subject には発行しない (fail-closed)。
	allowed, err := d.ApplicationAccessAllowed(ctx, tenantID, spec.ProtocolBindingSAML, sp.EntityID, authn.Sub)
	if err != nil {
		return err
	}
	if !allowed {
		d.emit(&spec.SamlSignInRejected{At: now, TenantID: tenantID, EntityID: sp.EntityID, Reason: "subject not assigned to application"})
		return c.String(http.StatusForbidden, "この利用者はアプリケーションに割り当てられていません")
	}

	result, err := feddomain.IssueClaims(sp.ClaimPolicy, feddomain.ResolveUserAttributes(*user))
	if err != nil {
		return d.rejectSSO(c, sp.EntityID, "claim issuance failed", err)
	}
	if validated.NameIDFormat != "" {
		result.NameIDFormat = validated.NameIDFormat
	}

	assertion, err := d.buildAssertion(c, *sp, validated, result, authn, now)
	if err != nil {
		return d.rejectSSO(c, sp.EntityID, "assertion build failed", err)
	}

	responseXML, err := samlresponse.BuildResponse(samlresponse.ResponseInput{
		Issuer:       core.RequestIssuer(c, d.Issuer),
		Destination:  validated.ACSURL,
		InResponseTo: validated.InResponseTo,
		IssueInstant: now,
		Assertion:    assertion,
		SignResponse: sp.SignResponse,
	}, d.FederationSigner)
	if err != nil {
		return d.rejectSSO(c, sp.EntityID, "response build failed", err)
	}
	formHTML, err := samlresponse.EncodePostForm(responseXML, validated.ACSURL, relayState)
	if err != nil {
		return d.rejectSSO(c, sp.EntityID, "form render failed", err)
	}

	d.emit(&spec.SamlSignInIssued{At: now, TenantID: tenantID, EntityID: sp.EntityID, Sub: authn.Sub})
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.HTML(http.StatusOK, string(formHTML))
}

// buildAssertion は claim 発行結果から SAML 2.0 assertion を組み立て、SP 設定に従って署名する。
func (d Deps) buildAssertion(c *echo.Context, sp spec.SamlServiceProvider, validated samldomain.ValidatedSignIn, result feddomain.ClaimIssuanceResult, authn *authdomain.AuthenticationContext, now time.Time) (*etree.Element, error) {
	authnMethod := feddomain.AuthnUnspecified
	if slices.Contains(authn.AMR, "pwd") {
		authnMethod = feddomain.AuthnPassword
	}
	in := samltoken.AssertionInput{
		Version:      samltoken.SAML20,
		Issuer:       core.RequestIssuer(c, d.Issuer),
		Audience:     sp.EffectiveAudience(),
		Recipient:    validated.ACSURL,
		InResponseTo: validated.InResponseTo,
		IssueInstant: now,
		NotBefore:    now.Add(-1 * time.Minute),
		NotOnOrAfter: now.Add(assertionLifetime),
		AuthnInstant: time.Unix(authn.AuthTime, 0).UTC(),
		AuthnMethod:  authnMethod,
		Result:       result,
	}
	if sp.SignAssertion {
		signed, _, err := samltoken.BuildSignedAssertion(in, d.FederationSigner)
		return signed, err
	}
	assertion, _, err := samltoken.BuildAssertion(in)
	return assertion, err
}

func (d Deps) rejectSSO(c *echo.Context, entityID, reason string, cause error) error {
	msg := reason
	if cause != nil {
		msg = reason + ": " + cause.Error()
	}
	d.emit(&spec.SamlSignInRejected{At: time.Now().UTC(), TenantID: core.RequestTenantID(c), EntityID: entityID, Reason: msg})
	return c.String(http.StatusBadRequest, reason)
}

func (d Deps) emit(event spec.DomainEvent) {
	if d.Emit != nil {
		d.Emit(event)
	}
}

// loginRedirect はログイン UI への誘導 URL を、認証後に SAML 要求へ戻す return_to つきで組み立てる
// (同一オリジンの相対パスのみ)。
func loginRedirect(c *echo.Context, returnTo string) string {
	return core.TenantRoute(c, "/login") + "?return_to=" + url.QueryEscape(returnTo)
}
