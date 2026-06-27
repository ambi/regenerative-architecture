package samlresponse_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"

	"ra-idp-go/internal/saml/adapters/samlresponse"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/wsfederation/adapters/samltoken"
	"ra-idp-go/internal/wsfederation/domain"
)

func newSigner(t *testing.T) (*samltoken.Signer, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "ra-idp saml signing"},
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
	signer, err := samltoken.NewSigner(cert, key)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	return signer, cert
}

func sampleAssertion(t *testing.T) *etree.Element {
	t.Helper()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	assertion, _, err := samltoken.BuildAssertion(samltoken.AssertionInput{
		Version:      samltoken.SAML20,
		Issuer:       "https://idp.example.com/saml",
		Audience:     "https://sp.example.com",
		Recipient:    "https://sp.example.com/acs",
		InResponseTo: "_req-1",
		IssueInstant: now,
		NotBefore:    now.Add(-1 * time.Minute),
		NotOnOrAfter: now.Add(5 * time.Minute),
		AuthnInstant: now,
		Result: domain.ClaimIssuanceResult{
			NameIDFormat: spec.SamlNameIDFormatPersistent,
			NameIDValue:  "alice",
			Claims: []spec.IssuedClaim{
				{ClaimType: "email", Values: []string{"alice@example.com"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("build assertion: %v", err)
	}
	return assertion
}

func TestBuildResponseWrapsAssertion(t *testing.T) {
	out, err := samlresponse.BuildResponse(samlresponse.ResponseInput{
		Issuer:       "https://idp.example.com/saml",
		Destination:  "https://sp.example.com/acs",
		InResponseTo: "_req-1",
		IssueInstant: time.Now(),
		Assertion:    sampleAssertion(t),
	}, nil)
	if err != nil {
		t.Fatalf("build response: %v", err)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(out); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	root := doc.Root()
	if root == nil || root.Tag != "Response" {
		t.Fatalf("root=%v", root)
	}
	if got := root.SelectAttrValue("InResponseTo", ""); got != "_req-1" {
		t.Errorf("InResponseTo=%q", got)
	}
	if got := root.SelectAttrValue("Destination", ""); got != "https://sp.example.com/acs" {
		t.Errorf("Destination=%q", got)
	}
	if doc.FindElement("//Status/StatusCode") == nil {
		t.Error("StatusCode missing")
	}
	if doc.FindElement("//Assertion") == nil {
		t.Error("Assertion missing")
	}
}

func TestBuildResponseSignsResponse(t *testing.T) {
	signer, cert := newSigner(t)
	out, err := samlresponse.BuildResponse(samlresponse.ResponseInput{
		Issuer:       "https://idp.example.com/saml",
		Destination:  "https://sp.example.com/acs",
		IssueInstant: time.Now(),
		Assertion:    sampleAssertion(t),
		SignResponse: true,
	}, signer)
	if err != nil {
		t.Fatalf("build response: %v", err)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(out); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if doc.FindElement("//Signature") == nil {
		t.Fatal("Response signature missing")
	}

	store := &dsig.MemoryX509CertificateStore{Roots: []*x509.Certificate{cert}}
	vctx := dsig.NewDefaultValidationContext(store)
	vctx.IdAttribute = "ID"
	if _, err := vctx.Validate(doc.Root()); err != nil {
		t.Fatalf("response signature did not validate: %v", err)
	}
}

func TestBuildResponseRequiresSignerWhenSigning(t *testing.T) {
	_, err := samlresponse.BuildResponse(samlresponse.ResponseInput{
		Issuer:       "https://idp.example.com/saml",
		Destination:  "https://sp.example.com/acs",
		IssueInstant: time.Now(),
		Assertion:    sampleAssertion(t),
		SignResponse: true,
	}, nil)
	if err == nil {
		t.Fatal("expected missing signer to be rejected")
	}
}

func TestBuildResponseValidatesInput(t *testing.T) {
	assertion := sampleAssertion(t)
	cases := []samlresponse.ResponseInput{
		{Destination: "https://sp.example.com/acs", Assertion: assertion},
		{Issuer: "https://idp.example.com/saml", Assertion: assertion},
		{Issuer: "https://idp.example.com/saml", Destination: "https://sp.example.com/acs"},
	}
	for i, in := range cases {
		if _, err := samlresponse.BuildResponse(in, nil); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
}

func TestEncodePostForm(t *testing.T) {
	responseXML := []byte(`<samlp:Response/>`)
	out, err := samlresponse.EncodePostForm(responseXML, "https://sp.example.com/acs", "state-123")
	if err != nil {
		t.Fatalf("encode post form: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(responseXML)
	if !bytes.Contains(out, []byte(encoded)) {
		t.Error("form does not contain base64 SAMLResponse")
	}
	if !bytes.Contains(out, []byte("state-123")) {
		t.Error("form does not contain RelayState")
	}
	if !bytes.Contains(out, []byte(`action="https://sp.example.com/acs"`)) {
		t.Error("form does not POST to ACS URL")
	}
}

func TestEncodePostFormRequiresACS(t *testing.T) {
	if _, err := samlresponse.EncodePostForm([]byte("<x/>"), "", ""); err == nil {
		t.Fatal("expected missing ACS URL to be rejected")
	}
}
