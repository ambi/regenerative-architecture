package samltoken

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"

	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/wsfederation/domain"
)

// selfSignedCert は署名検証のためのテスト用 X.509 証明書と鍵を生成する。
func selfSignedCert(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "ra-idp federation signing"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(1 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert, key
}

func validateSignature(signedXML []byte, cert *x509.Certificate) error {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(signedXML); err != nil {
		return err
	}
	store := &dsig.MemoryX509CertificateStore{Roots: []*x509.Certificate{cert}}
	vctx := dsig.NewDefaultValidationContext(store)
	vctx.IdAttribute = idAttribute
	_, err := vctx.Validate(doc.Root())
	return err
}

func sampleInput() AssertionInput {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	return AssertionInput{
		Issuer:       "https://idp.example.com/realms/contoso",
		Audience:     "urn:federation:MicrosoftOnline",
		Recipient:    "https://login.microsoftonline.com/login.srf",
		IssueInstant: now,
		NotBefore:    now.Add(-1 * time.Minute),
		NotOnOrAfter: now.Add(5 * time.Minute),
		AuthnInstant: now,
		Result: domain.ClaimIssuanceResult{
			NameIDFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
			NameIDValue:  "AAECAwQFBgc=",
			Claims: []spec.IssuedClaim{
				{ClaimType: "http://schemas.xmlsoap.org/claims/UPN", Values: []string{"alice@contoso.com"}},
				{ClaimType: "http://schemas.xmlsoap.org/claims/Group", Values: []string{"admins", "users"}},
			},
		},
	}
}

func TestIssueSignedAssertion_RoundTrip(t *testing.T) {
	cert, key := selfSignedCert(t)
	signer, err := NewSigner(cert, key)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	signedXML, id, err := IssueSignedAssertion(sampleInput(), signer)
	if err != nil {
		t.Fatalf("issue signed assertion: %v", err)
	}
	if id == "" || id[0] != '_' {
		t.Fatalf("assertion id %q must be an NCName starting with '_'", id)
	}

	if err := validateSignature(signedXML, cert); err != nil {
		t.Fatalf("signature did not validate: %v", err)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(signedXML); err != nil {
		t.Fatalf("parse signed xml: %v", err)
	}
	if got := doc.FindElement("//NameID"); got == nil || got.Text() != "AAECAwQFBgc=" {
		t.Fatalf("NameID not found or wrong value: %+v", got)
	}
	if got := doc.FindElement("//Signature"); got == nil {
		t.Fatal("enveloped signature missing")
	}
	upn := false
	for _, attr := range doc.FindElements("//Attribute") {
		if attr.SelectAttrValue("Name", "") == "http://schemas.xmlsoap.org/claims/UPN" {
			if v := attr.FindElement("AttributeValue"); v != nil && v.Text() == "alice@contoso.com" {
				upn = true
			}
		}
	}
	if !upn {
		t.Fatal("UPN attribute value not found in assertion")
	}
}

func TestSignedAssertion_TamperDetected(t *testing.T) {
	cert, key := selfSignedCert(t)
	signer, err := NewSigner(cert, key)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	signedXML, _, err := IssueSignedAssertion(sampleInput(), signer)
	if err != nil {
		t.Fatalf("issue signed assertion: %v", err)
	}

	// 署名後に NameID を改竄すると検証が失敗しなければならない。
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(signedXML); err != nil {
		t.Fatalf("parse signed xml: %v", err)
	}
	doc.FindElement("//NameID").SetText("attacker@evil.example")
	tampered, err := doc.WriteToBytes()
	if err != nil {
		t.Fatalf("serialize tampered xml: %v", err)
	}
	if err := validateSignature(tampered, cert); err == nil {
		t.Fatal("tampered assertion validated; expected signature failure")
	}
}

func TestIssueSignedAssertion_SAML11RoundTrip(t *testing.T) {
	cert, key := selfSignedCert(t)
	signer, err := NewSigner(cert, key)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	in := sampleInput()
	in.Version = SAML11
	in.AuthnMethod = domain.AuthnPassword
	signedXML, id, err := IssueSignedAssertion(in, signer)
	if err != nil {
		t.Fatalf("issue signed assertion: %v", err)
	}
	if id == "" || id[0] != '_' {
		t.Fatalf("assertion id %q must be an NCName starting with '_'", id)
	}

	// SAML 1.1 は AssertionID を ID 参照属性とする。
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(signedXML); err != nil {
		t.Fatalf("parse signed xml: %v", err)
	}
	store := &dsig.MemoryX509CertificateStore{Roots: []*x509.Certificate{cert}}
	vctx := dsig.NewDefaultValidationContext(store)
	vctx.IdAttribute = idAttribute11
	if _, err := vctx.Validate(doc.Root()); err != nil {
		t.Fatalf("SAML 1.1 signature did not validate: %v", err)
	}

	root := doc.Root()
	if root.SelectAttrValue("MajorVersion", "") != "1" || root.SelectAttrValue("MinorVersion", "") != "1" {
		t.Fatalf("expected MajorVersion/MinorVersion 1/1, got %s/%s",
			root.SelectAttrValue("MajorVersion", ""), root.SelectAttrValue("MinorVersion", ""))
	}
	if root.SelectAttrValue("Issuer", "") != in.Issuer {
		t.Fatalf("Issuer attribute = %q, want %q", root.SelectAttrValue("Issuer", ""), in.Issuer)
	}
	if got := doc.FindElement("//NameIdentifier"); got == nil || got.Text() != "AAECAwQFBgc=" {
		t.Fatalf("SAML 1.1 NameIdentifier missing or wrong: %+v", got)
	}
	if got := doc.FindElement("//AuthenticationStatement"); got == nil ||
		got.SelectAttrValue("AuthenticationMethod", "") != authnMethodPassword {
		t.Fatalf("AuthenticationMethod missing or not password: %+v", got)
	}
	// claim 型 URI は AD FS 流に namespace/name へ分割される。
	upn := false
	for _, attr := range doc.FindElements("//Attribute") {
		if attr.SelectAttrValue("AttributeName", "") == "UPN" &&
			attr.SelectAttrValue("AttributeNamespace", "") == "http://schemas.xmlsoap.org/claims" {
			if v := attr.FindElement("AttributeValue"); v != nil && v.Text() == "alice@contoso.com" {
				upn = true
			}
		}
	}
	if !upn {
		t.Fatal("UPN attribute (split namespace/name) not found in SAML 1.1 assertion")
	}
}

func TestBuildAssertion_InputValidation(t *testing.T) {
	base := sampleInput()
	bad := map[string]func(*AssertionInput){
		"missing issuer":      func(in *AssertionInput) { in.Issuer = "" },
		"missing audience":    func(in *AssertionInput) { in.Audience = "" },
		"missing nameid fmt":  func(in *AssertionInput) { in.Result.NameIDFormat = "" },
		"missing nameid val":  func(in *AssertionInput) { in.Result.NameIDValue = "" },
		"invalid time window": func(in *AssertionInput) { in.NotOnOrAfter = in.NotBefore },
	}
	for name, mutate := range bad {
		t.Run(name, func(t *testing.T) {
			in := base
			in.Result.Claims = append([]spec.IssuedClaim(nil), base.Result.Claims...)
			mutate(&in)
			if _, _, err := BuildAssertion(in); err == nil {
				t.Fatalf("%s: expected error, got nil", name)
			}
		})
	}
}
