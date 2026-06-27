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
	httpadapter "ra-idp-go/internal/platform/http"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/platform/persistence/memory"
	samldomain "ra-idp-go/internal/saml/domain"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/wsfederation/adapters/samltoken"

	"github.com/labstack/echo/v5"
)

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
		Subject:      pkix.Name{CommonName: "test saml signing"},
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

	spRepo := memory.NewSamlServiceProviderRepository()
	spRepo.Seed(&spec.SamlServiceProvider{
		EntityID:      "https://sp.example.com",
		ACSURLs:       []string{"https://sp.example.com/acs"},
		SignAssertion: true,
		ClaimPolicy: spec.ClaimMappingPolicy{
			NameID: spec.NameIdConfiguration{
				Format:          spec.SamlNameIDFormatPersistent,
				SourceAttribute: "sub",
			},
		},
	})

	userRepo := memory.NewUserRepository()
	userRepo.Seed(&spec.User{Sub: "user-1", PreferredUsername: "alice"})

	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer:           "https://idp.example",
		SCL:              spec.MustLoadSCL(),
		SamlSPRepo:       spRepo,
		UserRepo:         userRepo,
		FederationSigner: devSigner(t),
		AuthnResolver:    stubResolver{ctx: authn},
		Emit:             func(ev spec.DomainEvent) { *captured = append(*captured, ev) },
	})
	return e, captured
}

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

// authnRequestRedirect は HTTP-Redirect binding 用の SAMLRequest を組み立てる。
func authnRequestRedirect(t *testing.T, issuer, acsURL string) string {
	t.Helper()
	xml := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ` +
		`xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_req-1" Version="2.0" ` +
		`Destination="https://idp.example/saml/sso"`
	if acsURL != "" {
		xml += ` AssertionConsumerServiceURL="` + acsURL + `"`
	}
	xml += `><saml:Issuer>` + issuer + `</saml:Issuer></samlp:AuthnRequest>`
	encoded, err := samldomain.EncodeRedirect([]byte(xml))
	if err != nil {
		t.Fatalf("encode redirect: %v", err)
	}
	return encoded
}

func TestSamlSSO_SPInitiatedAuthenticatedIssuesPostForm(t *testing.T) {
	e, events := newServer(t, &authdomain.AuthenticationContext{Sub: "user-1", AuthTime: time.Now().Unix(), AMR: []string{"pwd"}})

	samlReq := authnRequestRedirect(t, "https://sp.example.com", "https://sp.example.com/acs")
	rec := get(e, "/saml/sso?SAMLRequest="+url.QueryEscape(samlReq)+"&RelayState=state-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `action="https://sp.example.com/acs"`) {
		t.Fatalf("form action missing: %s", body)
	}
	if !strings.Contains(body, `name="SAMLResponse"`) {
		t.Fatal("SAMLResponse hidden input missing")
	}
	if !strings.Contains(body, "state-1") {
		t.Fatal("RelayState not echoed")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control=%q, want no-store", rec.Header().Get("Cache-Control"))
	}
	if !hasEvent(*events, "SamlSignInIssued") {
		t.Fatal("SamlSignInIssued not emitted")
	}
}

func TestSamlSSO_IdPInitiatedIssuesPostForm(t *testing.T) {
	e, events := newServer(t, &authdomain.AuthenticationContext{Sub: "user-1", AuthTime: time.Now().Unix()})

	rec := get(e, "/saml/sso?entityID="+url.QueryEscape("https://sp.example.com"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `action="https://sp.example.com/acs"`) {
		t.Fatalf("form action missing: %s", rec.Body.String())
	}
	if !hasEvent(*events, "SamlSignInIssued") {
		t.Fatal("SamlSignInIssued not emitted")
	}
}

func TestSamlSSO_UnauthenticatedRedirectsToLogin(t *testing.T) {
	e, _ := newServer(t, nil)

	samlReq := authnRequestRedirect(t, "https://sp.example.com", "")
	rec := get(e, "/saml/sso?SAMLRequest="+url.QueryEscape(samlReq))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login") || !strings.Contains(loc, "return_to=") {
		t.Fatalf("Location=%q, want /login with return_to", loc)
	}
	decoded, err := url.QueryUnescape(loc[strings.Index(loc, "return_to=")+len("return_to="):])
	if err != nil {
		t.Fatalf("unescape return_to: %v", err)
	}
	if !strings.Contains(decoded, "/saml/sso") {
		t.Fatalf("return_to does not point back to /saml/sso: %q", loc)
	}
}

func TestSamlSSO_UnknownServiceProviderRejected(t *testing.T) {
	e, events := newServer(t, &authdomain.AuthenticationContext{Sub: "user-1"})
	samlReq := authnRequestRedirect(t, "https://evil.example.com", "")
	if rec := get(e, "/saml/sso?SAMLRequest="+url.QueryEscape(samlReq)); rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
	if !hasEvent(*events, "SamlSignInRejected") {
		t.Fatal("SamlSignInRejected not emitted")
	}
}

func TestSamlSSO_DisallowedACSRejected(t *testing.T) {
	e, _ := newServer(t, &authdomain.AuthenticationContext{Sub: "user-1"})
	samlReq := authnRequestRedirect(t, "https://sp.example.com", "https://evil.example.com/steal")
	rec := get(e, "/saml/sso?SAMLRequest="+url.QueryEscape(samlReq))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (open redirect prevention)", rec.Code)
	}
}

func TestSamlSLO_RedirectsToRegisteredSLOURL(t *testing.T) {
	captured := &[]spec.DomainEvent{}
	spRepo := memory.NewSamlServiceProviderRepository()
	spRepo.Seed(&spec.SamlServiceProvider{
		EntityID: "https://sp.example.com",
		ACSURLs:  []string{"https://sp.example.com/acs"},
		SLOURL:   "https://sp.example.com/saml/slo",
		ClaimPolicy: spec.ClaimMappingPolicy{
			NameID: spec.NameIdConfiguration{Format: spec.SamlNameIDFormatPersistent, SourceAttribute: "sub"},
		},
	})
	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer:           "https://idp.example",
		SCL:              spec.MustLoadSCL(),
		SamlSPRepo:       spRepo,
		UserRepo:         memory.NewUserRepository(),
		FederationSigner: devSigner(t),
		AuthnResolver:    stubResolver{ctx: nil},
		Emit:             func(ev spec.DomainEvent) { *captured = append(*captured, ev) },
	})

	rec := get(e, "/saml/slo?entityID="+url.QueryEscape("https://sp.example.com")+"&RelayState=s1")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://sp.example.com/saml/slo?RelayState=s1" {
		t.Fatalf("Location=%q, want registered SLO URL", loc)
	}
	if !strings.Contains(rec.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("session cookie not cleared: %q", rec.Header().Get("Set-Cookie"))
	}
	if !hasEvent(*captured, "SamlLogout") {
		t.Fatal("SamlLogout not emitted")
	}
}

func TestSamlSLO_UnknownSPReturns200(t *testing.T) {
	e, _ := newServer(t, nil)
	// seed した SP は SLO URL を持たないため、リダイレクトせず 200 を返す (open redirect 防止)。
	rec := get(e, "/saml/slo?entityID="+url.QueryEscape("https://sp.example.com"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
}

func TestSamlMetadata_Published(t *testing.T) {
	e, _ := newServer(t, nil)
	rec := get(e, "/saml/metadata")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"IDPSSODescriptor",
		"SingleSignOnService",
		"X509Certificate",
		"https://idp.example/realms/default/saml/sso",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metadata missing %q:\n%s", want, body)
		}
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/xml") {
		t.Fatalf("Content-Type=%q, want application/xml", ct)
	}
}

func newAdminServer(t *testing.T) *echo.Echo {
	t.Helper()
	userRepo := memory.NewUserRepository()
	userRepo.Seed(&spec.User{
		Sub:               "admin-1",
		TenantID:          spec.DefaultTenantID,
		PreferredUsername: "admin@example.com",
		Roles:             []string{"admin"},
	})
	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer:        "https://idp.example",
		SCL:           spec.MustLoadSCL(),
		SamlSPRepo:    memory.NewSamlServiceProviderRepository(),
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

func TestAdminServiceProvider_CRUD(t *testing.T) {
	e := newAdminServer(t)
	const path = "/api/admin/saml/service-providers"
	body := `{"entity_id":"https://sp.example.com","acs_urls":["https://sp.example.com/acs"],` +
		`"claim_policy":{"name_id":{"format":"urn:oasis:names:tc:SAML:2.0:nameid-format:persistent","source_attribute":"sub"}}}`

	if rec := doJSON(e, http.MethodPost, path, body); rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(e, http.MethodPost, path, body); rec.Code != http.StatusOK {
		t.Fatalf("update status=%d, want 200", rec.Code)
	}
	if rec := get(e, path); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "https://sp.example.com") {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(e, http.MethodDelete, path+"?entity_id="+url.QueryEscape("https://sp.example.com"), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d, want 204", rec.Code)
	}
	if rec := get(e, path); strings.Contains(rec.Body.String(), "https://sp.example.com") {
		t.Fatalf("SP still present after delete: %s", rec.Body.String())
	}
}

func TestAdminServiceProvider_RejectsInvalid(t *testing.T) {
	e := newAdminServer(t)
	// acs_urls 欠落。
	body := `{"entity_id":"https://sp.example.com","claim_policy":{"name_id":{"format":"f","source_attribute":"sub"}}}`
	if rec := doJSON(e, http.MethodPost, "/api/admin/saml/service-providers", body); rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestAdminServiceProvider_ForbiddenForNonAdmin(t *testing.T) {
	e, _ := newServer(t, &authdomain.AuthenticationContext{Sub: "user-1"}) // 非 admin
	if rec := get(e, "/api/admin/saml/service-providers"); rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
}
