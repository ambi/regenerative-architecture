package domain_test

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	samldomain "ra-idp-go/internal/saml/domain"
	"ra-idp-go/internal/shared/spec"
)

const sampleAuthnRequest = `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ` +
	`xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_req-1" Version="2.0" ` +
	`Destination="https://idp.example.com/saml/sso" ` +
	`AssertionConsumerServiceURL="https://sp.example.com/acs" ForceAuthn="true">` +
	`<saml:Issuer>https://sp.example.com</saml:Issuer>` +
	`<samlp:NameIDPolicy Format="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"/>` +
	`</samlp:AuthnRequest>`

func sampleServiceProvider() spec.SamlServiceProvider {
	return spec.SamlServiceProvider{
		TenantID: "acme",
		EntityID: "https://sp.example.com",
		ACSURLs:  []string{"https://sp.example.com/acs", "https://sp.example.com/acs2"},
		ClaimPolicy: spec.ClaimMappingPolicy{
			NameID: spec.NameIdConfiguration{Format: spec.SamlNameIDFormatPersistent},
		},
	}
}

func TestEncodeDecodeRedirectRoundTrip(t *testing.T) {
	encoded, err := samldomain.EncodeRedirect([]byte(sampleAuthnRequest))
	if err != nil {
		t.Fatalf("encode redirect: %v", err)
	}
	decoded, err := samldomain.DecodeRedirect(encoded)
	if err != nil {
		t.Fatalf("decode redirect: %v", err)
	}
	if string(decoded) != sampleAuthnRequest {
		t.Fatalf("round trip mismatch:\n got %s\nwant %s", decoded, sampleAuthnRequest)
	}
}

func TestDecodeRedirectRejectsInvalidBase64(t *testing.T) {
	if _, err := samldomain.DecodeRedirect("not base64!!!"); err == nil {
		t.Fatal("expected base64 decode error")
	}
}

func TestParseAuthnRequestExtractsFields(t *testing.T) {
	req, err := samldomain.ParseAuthnRequest([]byte(sampleAuthnRequest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.ID != "_req-1" {
		t.Errorf("ID=%q", req.ID)
	}
	if req.Issuer != "https://sp.example.com" {
		t.Errorf("Issuer=%q", req.Issuer)
	}
	if req.ACSURL != "https://sp.example.com/acs" {
		t.Errorf("ACSURL=%q", req.ACSURL)
	}
	if req.Destination != "https://idp.example.com/saml/sso" {
		t.Errorf("Destination=%q", req.Destination)
	}
	if req.NameIDFormat != spec.SamlNameIDFormatEmailAddress {
		t.Errorf("NameIDFormat=%q", req.NameIDFormat)
	}
	if !req.ForceAuthn {
		t.Error("ForceAuthn=false, want true")
	}
}

func TestParseAuthnRequestRejectsNonAuthnRequestRoot(t *testing.T) {
	xml := `<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol"/>`
	if _, err := samldomain.ParseAuthnRequest([]byte(xml)); err == nil {
		t.Fatal("expected non-AuthnRequest root to be rejected")
	}
}

func TestParseAuthnRequestRejectsMissingIssuer(t *testing.T) {
	xml := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ID="_x" Version="2.0"/>`
	if _, err := samldomain.ParseAuthnRequest([]byte(xml)); err == nil {
		t.Fatal("expected missing Issuer to be rejected")
	}
}

func TestParseAuthnRequestRejectsMissingID(t *testing.T) {
	xml := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" ` +
		`xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" Version="2.0">` +
		`<saml:Issuer>https://sp.example.com</saml:Issuer></samlp:AuthnRequest>`
	if _, err := samldomain.ParseAuthnRequest([]byte(xml)); err == nil {
		t.Fatal("expected missing ID to be rejected")
	}
}

func TestValidateSignInResolvesRequestedACS(t *testing.T) {
	req, err := samldomain.ParseAuthnRequest([]byte(sampleAuthnRequest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := samldomain.ValidateSignIn(req, sampleServiceProvider(), "https://idp.example.com/saml/sso")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if out.ACSURL != "https://sp.example.com/acs" {
		t.Errorf("ACSURL=%q", out.ACSURL)
	}
	if out.InResponseTo != "_req-1" {
		t.Errorf("InResponseTo=%q", out.InResponseTo)
	}
	// 要求の NameIDPolicy が SP 既定より優先される。
	if out.NameIDFormat != spec.SamlNameIDFormatEmailAddress {
		t.Errorf("NameIDFormat=%q", out.NameIDFormat)
	}
}

func TestValidateSignInRejectsIssuerMismatch(t *testing.T) {
	req := samldomain.AuthnRequest{ID: "_x", Issuer: "https://evil.example.com"}
	if _, err := samldomain.ValidateSignIn(req, sampleServiceProvider(), "https://idp.example.com/saml/sso"); err == nil {
		t.Fatal("expected issuer mismatch to be rejected")
	}
}

func TestValidateSignInRejectsUnregisteredACS(t *testing.T) {
	req := samldomain.AuthnRequest{
		ID:     "_x",
		Issuer: "https://sp.example.com",
		ACSURL: "https://evil.example.com/acs",
	}
	if _, err := samldomain.ValidateSignIn(req, sampleServiceProvider(), "https://idp.example.com/saml/sso"); err == nil {
		t.Fatal("expected unregistered ACS URL to be rejected (open redirect)")
	}
}

func TestValidateSignInRejectsDestinationMismatch(t *testing.T) {
	req := samldomain.AuthnRequest{
		ID:          "_x",
		Issuer:      "https://sp.example.com",
		Destination: "https://other-idp.example.com/saml/sso",
	}
	if _, err := samldomain.ValidateSignIn(req, sampleServiceProvider(), "https://idp.example.com/saml/sso"); err == nil {
		t.Fatal("expected mismatched Destination to be rejected")
	}
}

func TestValidateRequestSignatureRequiresCertificate(t *testing.T) {
	sp := sampleServiceProvider()
	sp.WantAuthnRequestsSigned = true
	if err := samldomain.ValidateRequestSignature(samldomain.BindingRedirect, []byte(sampleAuthnRequest), "SAMLRequest=x", sp); err == nil {
		t.Fatal("expected missing signing certificate to be rejected")
	}
}

func TestValidateSignInFallsBackToDefaultACS(t *testing.T) {
	req := samldomain.AuthnRequest{ID: "_x", Issuer: "https://sp.example.com"}
	out, err := samldomain.ValidateSignIn(req, sampleServiceProvider(), "https://idp.example.com/saml/sso")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if out.ACSURL != "https://sp.example.com/acs" {
		t.Errorf("ACSURL=%q, want default", out.ACSURL)
	}
	// 要求が NameID format を持たないので SP 既定が用いられる。
	if out.NameIDFormat != spec.SamlNameIDFormatPersistent {
		t.Errorf("NameIDFormat=%q, want SP default", out.NameIDFormat)
	}
}

func TestValidateSignInUnspecifiedFormatUsesSPDefault(t *testing.T) {
	req := samldomain.AuthnRequest{
		ID:           "_x",
		Issuer:       "https://sp.example.com",
		NameIDFormat: spec.SamlNameIDFormatUnspecified,
	}
	out, err := samldomain.ValidateSignIn(req, sampleServiceProvider(), "https://idp.example.com/saml/sso")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if out.NameIDFormat != spec.SamlNameIDFormatPersistent {
		t.Errorf("NameIDFormat=%q, want SP default", out.NameIDFormat)
	}
}

func TestRequiresFreshAuth(t *testing.T) {
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	if samldomain.RequiresFreshAuth(false, now.Add(-time.Hour), now) {
		t.Fatal("ForceAuthn=false should not require fresh authentication")
	}
	if samldomain.RequiresFreshAuth(true, now.Add(-10*time.Second), now) {
		t.Fatal("recent authentication should satisfy ForceAuthn")
	}
	if !samldomain.RequiresFreshAuth(true, now.Add(-time.Minute), now) {
		t.Fatal("stale authentication should require fresh authentication")
	}
}

func TestDecodePostRejectsOversizedRequest(t *testing.T) {
	// base64 で 256KiB 超の生 XML を作って上限を超えさせる。
	oversized := strings.Repeat("A", 256*1024+1)
	encoded := base64.StdEncoding.EncodeToString([]byte(oversized))
	if _, err := samldomain.DecodePost(encoded); err == nil {
		t.Fatal("expected oversized POST request to be rejected")
	}
}
