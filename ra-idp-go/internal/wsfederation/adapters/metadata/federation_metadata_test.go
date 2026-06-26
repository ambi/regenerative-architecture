package metadata

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
	"time"
)

func testCert(t *testing.T) *x509.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "metadata test"},
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
		t.Fatalf("parse: %v", err)
	}
	return cert
}

func TestBuildFederationMetadata_AdvertisesCertificateAndEndpoints(t *testing.T) {
	cert := testCert(t)
	xml, err := BuildFederationMetadata("https://idp.example", cert, EndpointSet{
		PassiveURL:        "https://idp.example/wsfed",
		ActiveURL:         "https://idp.example/trust/usernamemixed",
		MEXURL:            "https://idp.example/trust/mex",
		FederationMetaURL: "https://idp.example/federationmetadata/2007-06/federationmetadata.xml",
	}, time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatalf("BuildFederationMetadata: %v", err)
	}
	body := string(xml)
	for _, want := range []string{
		`entityID="https://idp.example"`,
		`fed:SecurityTokenServiceType`,
		`fed:ApplicationServiceType`,
		`fed:PassiveRequestorEndpoint`,
		`fed:SecurityTokenServiceEndpoint`,
		"https://idp.example/wsfed",
		"https://idp.example/trust/usernamemixed",
		base64.StdEncoding.EncodeToString(cert.Raw),
		"http://schemas.xmlsoap.org/claims/UPN",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metadata missing %q:\n%s", want, body)
		}
	}
}

func TestBuildMEX_AdvertisesUserNameMixedEndpoint(t *testing.T) {
	xml, err := BuildMEX(EndpointSet{
		ActiveURL:         "https://idp.example/trust/usernamemixed",
		MEXURL:            "https://idp.example/trust/mex",
		FederationMetaURL: "https://idp.example/federationmetadata/2007-06/federationmetadata.xml",
	})
	if err != nil {
		t.Fatalf("BuildMEX: %v", err)
	}
	body := string(xml)
	for _, want := range []string{
		"mex:Metadata",
		"UserNameWSTrustBinding_IWSTrust13Sync",
		"https://idp.example/trust/usernamemixed",
		"https://idp.example/federationmetadata/2007-06/federationmetadata.xml",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("MEX missing %q:\n%s", want, body)
		}
	}
}
