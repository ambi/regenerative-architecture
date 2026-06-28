package http_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	authdomain "ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/infrastructure/crypto"
	httpadapter "ra-idp-go/internal/infrastructure/http"
	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/wsfederation/adapters/samltoken"

	"github.com/labstack/echo/v5"
)

// stubResolver は固定の認証コンテキストを返す AuthnResolver。
type stubResolver struct {
	ctx *authdomain.AuthenticationContext
}

func (s stubResolver) Resolve(context.Context, authdomain.Headers) (*authdomain.AuthenticationContext, error) {
	return s.ctx, nil
}

func devSigner(t *testing.T) *samltoken.Signer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test fed signing"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	signer, err := samltoken.NewSigner(cert, key)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return signer
}

func newServer(t *testing.T, authn *authdomain.AuthenticationContext) (*echo.Echo, *[]spec.DomainEvent) {
	t.Helper()

	captured := &[]spec.DomainEvent{}

	rpRepo := memory.NewWsFedRelyingPartyRepository()
	rpRepo.Seed(&spec.WsFedRelyingParty{
		Wtrealm:   "urn:ra-idp:demo-rp",
		ReplyURLs: []string{"https://rp.example/wsfed"},
		ClaimPolicy: spec.ClaimMappingPolicy{
			NameID: spec.NameIdConfiguration{
				Format:          "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
				SourceAttribute: "sub",
			},
			Rules: []spec.ClaimMappingRule{
				{ClaimType: "http://schemas.xmlsoap.org/claims/UPN", Source: spec.ClaimSourceUserAttribute, SourceKey: "preferred_username", Required: true},
			},
		},
	})

	userRepo := memory.NewUserRepository()
	hasher := crypto.NewArgon2idPasswordHasher()
	passwordHash, err := hasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	sentinel, err := hasher.Hash("sentinel-password")
	if err != nil {
		t.Fatalf("hash sentinel: %v", err)
	}
	userRepo.Seed(&spec.User{Sub: "user-1", PreferredUsername: "alice", PasswordHash: passwordHash})

	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer:                     "https://idp.example",
		SCL:                        spec.MustLoadSCL(),
		WsFedRPRepo:                rpRepo,
		UserRepo:                   userRepo,
		PasswordHasher:             hasher,
		SentinelPasswordHash:       sentinel,
		ClientAssertionReplayStore: memory.NewClientAssertionReplayStore(),
		FederationSigner:           devSigner(t),
		AuthnResolver:              stubResolver{ctx: authn},
		Emit:                       func(ev spec.DomainEvent) { *captured = append(*captured, ev) },
	})
	return e, captured
}

// hasEvent は指定 EventType の event が捕捉されたかを返す。
func hasEvent(events []spec.DomainEvent, eventType string) bool {
	for _, ev := range events {
		if ev.EventType() == eventType {
			return true
		}
	}
	return false
}

func get(e *echo.Echo, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestWsFedSignIn_AuthenticatedIssuesPassiveForm(t *testing.T) {
	e, events := newServer(t, &authdomain.AuthenticationContext{Sub: "user-1", AuthTime: time.Now().Unix()})

	rec := get(e, "/wsfed?wa=wsignin1.0&wtrealm=urn:ra-idp:demo-rp&wctx=ctx-42")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `action="https://rp.example/wsfed"`) {
		t.Fatalf("form action missing: %s", body)
	}
	if !strings.Contains(body, `value="wsignin1.0"`) {
		t.Fatal("wa hidden input missing")
	}
	if !strings.Contains(body, "RequestSecurityTokenResponse") {
		t.Fatal("RSTR not present in wresult")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}
	if !hasEvent(*events, "WsFedSignInIssued") {
		t.Fatal("WsFedSignInIssued not emitted")
	}
}

func TestWsFedSignIn_DefaultsToSAML11Token(t *testing.T) {
	e, _ := newServer(t, &authdomain.AuthenticationContext{Sub: "user-1", AuthTime: time.Now().Unix(), AMR: []string{"pwd"}})

	rec := get(e, "/wsfed?wa=wsignin1.0&wtrealm=urn:ra-idp:demo-rp")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// 既定 token type は Entra 互換の SAML 1.1。
	if !strings.Contains(body, "urn:oasis:names:tc:SAML:1.0:assertion") {
		t.Fatalf("RSTR TokenType is not SAML 1.1: %s", body)
	}
	// SAML 1.1 assertion は MajorVersion/MinorVersion と AuthenticationStatement を持つ。
	if !strings.Contains(body, "MajorVersion=&#34;1&#34;") && !strings.Contains(body, `MajorVersion="1"`) {
		t.Fatalf("SAML 1.1 MajorVersion not present: %s", body)
	}
}

func TestWsFedSignIn_StaleSessionWithWfreshRedirectsToLogin(t *testing.T) {
	// 認証から十分時間が経過したセッションに wfresh=0 を要求すると再認証へ誘導される。
	staleAuthTime := time.Now().Add(-30 * time.Minute).Unix()
	e, _ := newServer(t, &authdomain.AuthenticationContext{Sub: "user-1", AuthTime: staleAuthTime, AMR: []string{"pwd"}})

	rec := get(e, "/wsfed?wa=wsignin1.0&wtrealm=urn:ra-idp:demo-rp&wfresh=0")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303 (wfresh forces re-auth)", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/login") {
		t.Fatalf("Location = %q, want /login", loc)
	}
}

func TestWsFedSignIn_UnsupportedWauthRejected(t *testing.T) {
	e, events := newServer(t, &authdomain.AuthenticationContext{Sub: "user-1", AuthTime: time.Now().Unix(), AMR: []string{"pwd"}})

	rec := get(e, "/wsfed?wa=wsignin1.0&wtrealm=urn:ra-idp:demo-rp&wauth=urn:federation:authentication:windows")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (integrated Windows auth unsupported)", rec.Code)
	}
	if !hasEvent(*events, "WsFedSignInRejected") {
		t.Fatal("WsFedSignInRejected not emitted")
	}
}

func TestWsFedSignIn_UnauthenticatedRedirectsToLogin(t *testing.T) {
	e, _ := newServer(t, nil) // resolver returns no session

	rec := get(e, "/wsfed?wa=wsignin1.0&wtrealm=urn:ra-idp:demo-rp")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login") || !strings.Contains(loc, "return_to=") {
		t.Fatalf("Location = %q, want /login with return_to", loc)
	}
	// return_to は元の WS-Fed 要求へ戻ること。
	if decoded := loc[strings.Index(loc, "return_to=")+len("return_to="):]; !strings.Contains(mustUnescape(t, decoded), "/wsfed") {
		t.Fatalf("return_to does not point back to /wsfed: %q", loc)
	}
}

func TestWsFedSignIn_UnknownRelyingParty(t *testing.T) {
	e, events := newServer(t, &authdomain.AuthenticationContext{Sub: "user-1"})
	if rec := get(e, "/wsfed?wa=wsignin1.0&wtrealm=urn:unknown"); rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
	if !hasEvent(*events, "WsFedSignInRejected") {
		t.Fatal("WsFedSignInRejected not emitted")
	}
}

func TestWsFedSignIn_DisallowedWreplyRejected(t *testing.T) {
	e, _ := newServer(t, &authdomain.AuthenticationContext{Sub: "user-1"})
	rec := get(e, "/wsfed?wa=wsignin1.0&wtrealm=urn:ra-idp:demo-rp&wreply=https://evil.example/steal")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (open redirect prevention)", rec.Code)
	}
}

func TestWsFedSignOut_RedirectsToAllowedWreply(t *testing.T) {
	e, events := newServer(t, nil)
	rec := get(e, "/wsfed?wa=wsignout1.0&wtrealm=urn:ra-idp:demo-rp&wreply=https://rp.example/wsfed")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://rp.example/wsfed" {
		t.Fatalf("Location = %q, want allowed wreply", loc)
	}
	// セッション cookie が失効される。
	if !strings.Contains(rec.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("session cookie not cleared: %q", rec.Header().Get("Set-Cookie"))
	}
	if !hasEvent(*events, "WsFedSignOut") {
		t.Fatal("WsFedSignOut not emitted")
	}
}

func TestWsFedSignOut_DisallowedWreplyNoRedirect(t *testing.T) {
	e, _ := newServer(t, nil)
	rec := get(e, "/wsfed?wa=wsignout1.0&wtrealm=urn:ra-idp:demo-rp&wreply=https://evil.example/x")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (no open redirect)", rec.Code)
	}
}

func TestWsFedSignOutCleanup_ClearsAndReturns200(t *testing.T) {
	e, _ := newServer(t, nil)
	rec := get(e, "/wsfed?wa=wsignoutcleanup1.0")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
}

func TestFederationMetadata_Published(t *testing.T) {
	e, _ := newServer(t, nil)
	rec := get(e, "/federationmetadata/2007-06/federationmetadata.xml")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`entityID="https://idp.example/realms/default"`,
		"fed:PassiveRequestorEndpoint",
		"https://idp.example/realms/default/wsfed",
		"https://idp.example/realms/default/trust/usernamemixed",
		"ds:X509Certificate",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metadata missing %q:\n%s", want, body)
		}
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/xml") {
		t.Fatalf("Content-Type=%q, want application/xml", ct)
	}
}

func TestTrustMEX_Published(t *testing.T) {
	e, _ := newServer(t, nil)
	rec := get(e, "/trust/mex")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"mex:Metadata",
		"UserNameWSTrustBinding_IWSTrust13Sync",
		"https://idp.example/realms/default/trust/usernamemixed",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("MEX missing %q:\n%s", want, body)
		}
	}
}

func TestWsTrustUsernameMixed_IssuesRSTR(t *testing.T) {
	e, events := newServer(t, nil)
	rec := postWsTrustSOAP(e, wsTrustRST(time.Now().UTC(), "urn:uuid:issue-1", "urn:ra-idp:demo-rp"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"RequestSecurityTokenResponseCollection",
		"RequestedSecurityToken",
		"urn:oasis:names:tc:SAML:1.0:assertion",
		"urn:ra-idp:demo-rp",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("RSTR missing %q:\n%s", want, body)
		}
	}
	if !hasEvent(*events, "WsTrustTokenIssued") {
		t.Fatal("WsTrustTokenIssued not emitted")
	}
}

func TestWsTrustUsernameMixed_RejectsUnknownAppliesTo(t *testing.T) {
	e, events := newServer(t, nil)
	rec := postWsTrustSOAP(e, wsTrustRST(time.Now().UTC(), "urn:uuid:issue-2", "urn:unknown"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 body=%s", rec.Code, rec.Body.String())
	}
	if !hasEvent(*events, "WsTrustTokenRejected") {
		t.Fatal("WsTrustTokenRejected not emitted")
	}
}

func TestWsTrustUsernameMixed_RejectsExpiredTimestampAndReplay(t *testing.T) {
	e, _ := newServer(t, nil)
	expired := time.Now().UTC().Add(-10 * time.Minute)
	if rec := postWsTrustSOAP(e, wsTrustRST(expired, "urn:uuid:expired", "urn:ra-idp:demo-rp")); rec.Code != http.StatusBadRequest {
		t.Fatalf("expired status=%d, want 400", rec.Code)
	}
	first := postWsTrustSOAP(e, wsTrustRST(time.Now().UTC(), "urn:uuid:replay", "urn:ra-idp:demo-rp"))
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	second := postWsTrustSOAP(e, wsTrustRST(time.Now().UTC(), "urn:uuid:replay", "urn:ra-idp:demo-rp"))
	if second.Code != http.StatusBadRequest {
		t.Fatalf("replay status=%d, want 400", second.Code)
	}
}

func TestWsTrustUsernameMixed_RejectsMismatchedTo(t *testing.T) {
	e, _ := newServer(t, nil)
	body := strings.Replace(
		wsTrustRST(time.Now().UTC(), "urn:uuid:mismatched-to", "urn:ra-idp:demo-rp"),
		"https://idp.example/realms/default/trust/usernamemixed",
		"https://evil.example/trust/usernamemixed",
		1,
	)
	if rec := postWsTrustSOAP(e, body); rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestWsTrustUsernameMixed_RejectsNonBearerKeyType(t *testing.T) {
	e, _ := newServer(t, nil)
	body := strings.Replace(
		wsTrustRST(time.Now().UTC(), "urn:uuid:public-key", "urn:ra-idp:demo-rp"),
		"<wsp:AppliesTo>",
		"<t:KeyType>http://docs.oasis-open.org/ws-sx/ws-trust/200512/PublicKey</t:KeyType><wsp:AppliesTo>",
		1,
	)
	if rec := postWsTrustSOAP(e, body); rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func postWsTrustSOAP(e *echo.Echo, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/trust/usernamemixed", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/soap+xml")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func wsTrustRST(now time.Time, messageID, appliesTo string) string {
	return `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
  xmlns:a="http://www.w3.org/2005/08/addressing"
  xmlns:o="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd"
  xmlns:u="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"
  xmlns:t="http://docs.oasis-open.org/ws-sx/ws-trust/200512"
  xmlns:wsp="http://schemas.xmlsoap.org/ws/2004/09/policy">
  <s:Header>
    <a:Action>http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue</a:Action>
    <a:MessageID>` + messageID + `</a:MessageID>
    <a:To>https://idp.example/realms/default/trust/usernamemixed</a:To>
    <o:Security>
      <u:Timestamp>
        <u:Created>` + now.Format(time.RFC3339) + `</u:Created>
        <u:Expires>` + now.Add(5*time.Minute).Format(time.RFC3339) + `</u:Expires>
      </u:Timestamp>
      <o:UsernameToken>
        <o:Username>alice</o:Username>
        <o:Password>correct-password</o:Password>
      </o:UsernameToken>
    </o:Security>
  </s:Header>
  <s:Body>
    <t:RequestSecurityToken>
      <t:RequestType>http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue</t:RequestType>
      <wsp:AppliesTo>
        <a:EndpointReference>
          <a:Address>` + appliesTo + `</a:Address>
        </a:EndpointReference>
      </wsp:AppliesTo>
    </t:RequestSecurityToken>
  </s:Body>
</s:Envelope>`
}

func newAdminServer(t *testing.T) *echo.Echo {
	t.Helper()
	userRepo := memory.NewUserRepository()
	objectGUID := "6f9619ff-8b86-d011-b42d-00c04fc964ff"
	userRepo.Seed(&spec.User{
		Sub:               "admin-1",
		TenantID:          spec.DefaultTenantID,
		PreferredUsername: "admin@contoso.com",
		Roles:             []string{"admin"},
		Attributes: map[string]spec.AttributeValue{
			"object_guid": {Type: spec.AttributeTypeString, String: &objectGUID},
		},
	})
	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer:        "https://idp.example",
		SCL:           spec.MustLoadSCL(),
		WsFedRPRepo:   memory.NewWsFedRelyingPartyRepository(),
		UserRepo:      userRepo,
		AuthnResolver: stubResolver{ctx: &authdomain.AuthenticationContext{Sub: "admin-1"}},
	})
	return e
}

func doJSON(e *echo.Echo, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestAdminRelyingParty_CRUD(t *testing.T) {
	e := newAdminServer(t)
	const path = "/api/admin/wsfed/relying-parties"
	body := `{"wtrealm":"urn:rp:a","reply_urls":["https://a.example/acs"],"claim_policy":{"name_id":{"format":"urn:oasis:names:tc:SAML:2.0:nameid-format:persistent","source_attribute":"sub"}}}`

	if rec := doJSON(e, http.MethodPost, path, body); rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(e, http.MethodPost, path, body); rec.Code != http.StatusOK {
		t.Fatalf("update status=%d, want 200", rec.Code)
	}
	if rec := get(e, path); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "urn:rp:a") {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(e, http.MethodDelete, path+"?wtrealm=urn:rp:a", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d, want 204", rec.Code)
	}
	if rec := get(e, path); strings.Contains(rec.Body.String(), "urn:rp:a") {
		t.Fatalf("RP still present after delete: %s", rec.Body.String())
	}
}

func TestAdminRelyingParty_RejectsInvalid(t *testing.T) {
	e := newAdminServer(t)
	// reply_urls 欠落。
	body := `{"wtrealm":"urn:rp:b","claim_policy":{"name_id":{"format":"f","source_attribute":"sub"}}}`
	if rec := doJSON(e, http.MethodPost, "/api/admin/wsfed/relying-parties", body); rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestAdminRelyingParty_ForbiddenForNonAdmin(t *testing.T) {
	e, _ := newServer(t, &authdomain.AuthenticationContext{Sub: "user-1"}) // 非 admin
	if rec := get(e, "/api/admin/wsfed/relying-parties"); rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
}

func TestAdminConfigureEntraFederation_CreatesPresetRelyingParty(t *testing.T) {
	e := newAdminServer(t)
	body := `{"domain":"contoso.com","source_anchor_attribute":"object_guid"}`
	rec := doJSON(e, http.MethodPost, "/api/admin/wsfed/entra-federation", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"issuer_uri":"urn:ra-idp:entra:contoso.com"`,
		`"source_anchor_attribute":"object_guid"`,
		`"claim_type":"http://schemas.xmlsoap.org/claims/UPN"`,
		`"claim_type":"http://schemas.xmlsoap.org/claims/nameidentifier"`,
		`"ActiveLogOnUri":"https://idp.example/realms/default/trust/usernamemixed"`,
		"Hybrid Azure AD Join device registration is not provided",
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("response missing %q:\n%s", want, rec.Body.String())
		}
	}
	list := get(e, "/api/admin/wsfed/relying-parties")
	if !strings.Contains(list.Body.String(), `"entra_profile"`) {
		t.Fatalf("configured RP missing entra_profile: %s", list.Body.String())
	}
}

func TestAdminConfigureEntraFederation_RejectsMissingSourceAnchor(t *testing.T) {
	e := newAdminServer(t)
	body := `{"domain":"contoso.com","source_anchor_attribute":"missing_anchor"}`
	rec := doJSON(e, http.MethodPost, "/api/admin/wsfed/entra-federation", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "sourceAnchor validation failed") {
		t.Fatalf("missing sourceAnchor error not returned: %s", rec.Body.String())
	}
}

func mustUnescape(t *testing.T, s string) string {
	t.Helper()
	out, err := url.QueryUnescape(s)
	if err != nil {
		t.Fatalf("unescape: %v", err)
	}
	return out
}
