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
	"ra-idp-go/internal/federation/adapters/samltoken"
	httpadapter "ra-idp-go/internal/platform/http"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"

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
	userRepo.Seed(&spec.User{Sub: "user-1", PreferredUsername: "alice"})

	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer:           "https://idp.example",
		SCL:              spec.MustLoadSCL(),
		WsFedRPRepo:      rpRepo,
		UserRepo:         userRepo,
		FederationSigner: devSigner(t),
		AuthnResolver:    stubResolver{ctx: authn},
		Emit:             func(ev spec.DomainEvent) { *captured = append(*captured, ev) },
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

func newAdminServer(t *testing.T) *echo.Echo {
	t.Helper()
	userRepo := memory.NewUserRepository()
	userRepo.Seed(&spec.User{Sub: "admin-1", TenantID: spec.DefaultTenantID, PreferredUsername: "admin", Roles: []string{"admin"}})
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

func mustUnescape(t *testing.T, s string) string {
	t.Helper()
	out, err := url.QueryUnescape(s)
	if err != nil {
		t.Fatalf("unescape: %v", err)
	}
	return out
}
