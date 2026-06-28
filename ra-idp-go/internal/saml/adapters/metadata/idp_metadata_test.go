package metadata_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"slices"
	"testing"
	"time"

	"github.com/beevik/etree"

	"ra-idp-go/internal/saml/adapters/metadata"
	"ra-idp-go/internal/shared/spec"
)

func selfSignedCert(t *testing.T) *x509.Certificate {
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
	return cert
}

func TestBuildIDPMetadataAdvertisesEndpoints(t *testing.T) {
	cert := selfSignedCert(t)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	out, err := metadata.BuildIDPMetadata(
		"https://idp.example.com/saml",
		cert,
		metadata.Endpoints{
			SSOURL: "https://idp.example.com/saml/sso",
			SLOURL: "https://idp.example.com/saml/slo",
		},
		now,
	)
	if err != nil {
		t.Fatalf("build metadata: %v", err)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(out); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	root := doc.Root()
	if root == nil || root.Tag != "EntityDescriptor" {
		t.Fatalf("root=%v", root)
	}
	if got := root.SelectAttrValue("entityID", ""); got != "https://idp.example.com/saml" {
		t.Errorf("entityID=%q", got)
	}
	if doc.FindElement("//IDPSSODescriptor") == nil {
		t.Fatal("IDPSSODescriptor missing")
	}
	if doc.FindElement("//KeyDescriptor//X509Certificate") == nil {
		t.Fatal("signing certificate missing")
	}

	var ssoBindings, sloBindings []string
	for _, e := range doc.FindElements("//SingleSignOnService") {
		ssoBindings = append(ssoBindings, e.SelectAttrValue("Binding", ""))
	}
	for _, e := range doc.FindElements("//SingleLogoutService") {
		sloBindings = append(sloBindings, e.SelectAttrValue("Binding", ""))
	}
	if !slices.Contains(ssoBindings, spec.SamlBindingHTTPRedirect) || !slices.Contains(ssoBindings, spec.SamlBindingHTTPPOST) {
		t.Errorf("SSO bindings=%v", ssoBindings)
	}
	if !slices.Contains(sloBindings, spec.SamlBindingHTTPRedirect) || !slices.Contains(sloBindings, spec.SamlBindingHTTPPOST) {
		t.Errorf("SLO bindings=%v", sloBindings)
	}
}

func TestBuildIDPMetadataOmitsSLOWhenUnset(t *testing.T) {
	cert := selfSignedCert(t)
	out, err := metadata.BuildIDPMetadata(
		"https://idp.example.com/saml",
		cert,
		metadata.Endpoints{SSOURL: "https://idp.example.com/saml/sso"},
		time.Now(),
	)
	if err != nil {
		t.Fatalf("build metadata: %v", err)
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(out); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if doc.FindElement("//SingleLogoutService") != nil {
		t.Fatal("SingleLogoutService should be omitted when SLOURL is empty")
	}
}

func TestBuildIDPMetadataRequiresEssentials(t *testing.T) {
	cert := selfSignedCert(t)
	endpoints := metadata.Endpoints{SSOURL: "https://idp.example.com/saml/sso"}
	if _, err := metadata.BuildIDPMetadata("", cert, endpoints, time.Now()); err == nil {
		t.Error("expected missing entityID to be rejected")
	}
	if _, err := metadata.BuildIDPMetadata("https://idp", nil, endpoints, time.Now()); err == nil {
		t.Error("expected missing certificate to be rejected")
	}
	if _, err := metadata.BuildIDPMetadata("https://idp", cert, metadata.Endpoints{}, time.Now()); err == nil {
		t.Error("expected missing SSO endpoint to be rejected")
	}
}
