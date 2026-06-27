// Package metadata は SAML 2.0 IdP の metadata XML を組み立てる (wi-29, ADR-067)。
//
// realm 単位の IdP entityID・署名証明書・SSO/SLO endpoint から、SP が取り込める
// <EntityDescriptor><IDPSSODescriptor> を導出する。署名証明書は KeyDescriptor として広告し、
// 鍵ローテーション・metadata 署名は後続の KMS/key lifecycle WI に委ねる。
package metadata

import (
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/beevik/etree"

	"ra-idp-go/internal/spec"
)

const (
	nsMD = "urn:oasis:names:tc:SAML:2.0:metadata"
	nsDS = "http://www.w3.org/2000/09/xmldsig#"
)

// Endpoints は metadata に広告する IdP の SSO / SLO endpoint。
type Endpoints struct {
	SSOURL string // SingleSignOnService (HTTP-Redirect / HTTP-POST)。
	SLOURL string // 任意。SingleLogoutService。
}

// nameIDFormats は IdP が広告する NameID format。
var nameIDFormats = []string{
	spec.SamlNameIDFormatPersistent,
	spec.SamlNameIDFormatEmailAddress,
	spec.SamlNameIDFormatTransient,
	spec.SamlNameIDFormatUnspecified,
}

// BuildIDPMetadata は realm 単位の IdP metadata XML を生成する。
func BuildIDPMetadata(entityID string, cert *x509.Certificate, endpoints Endpoints, now time.Time) ([]byte, error) {
	if strings.TrimSpace(entityID) == "" {
		return nil, fmt.Errorf("metadata: entityID is required")
	}
	if cert == nil {
		return nil, fmt.Errorf("metadata: signing certificate is required")
	}
	if strings.TrimSpace(endpoints.SSOURL) == "" {
		return nil, fmt.Errorf("metadata: SSO endpoint is required")
	}

	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="utf-8"`)
	root := doc.CreateElement("md:EntityDescriptor")
	root.CreateAttr("xmlns:md", nsMD)
	root.CreateAttr("xmlns:ds", nsDS)
	root.CreateAttr("entityID", entityID)
	root.CreateAttr("validUntil", now.UTC().Add(24*time.Hour).Format(time.RFC3339))

	idp := root.CreateElement("md:IDPSSODescriptor")
	idp.CreateAttr("protocolSupportEnumeration", "urn:oasis:names:tc:SAML:2.0:protocol")
	idp.CreateAttr("WantAuthnRequestsSigned", "false")

	addKeyDescriptor(idp, cert)

	if slo := strings.TrimSpace(endpoints.SLOURL); slo != "" {
		addEndpoint(idp, "md:SingleLogoutService", spec.SamlBindingHTTPRedirect, slo)
		addEndpoint(idp, "md:SingleLogoutService", spec.SamlBindingHTTPPOST, slo)
	}

	for _, format := range nameIDFormats {
		idp.CreateElement("md:NameIDFormat").SetText(format)
	}

	addEndpoint(idp, "md:SingleSignOnService", spec.SamlBindingHTTPRedirect, endpoints.SSOURL)
	addEndpoint(idp, "md:SingleSignOnService", spec.SamlBindingHTTPPOST, endpoints.SSOURL)

	doc.Indent(2)
	out, err := doc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("metadata: serialize: %w", err)
	}
	return out, nil
}

func addKeyDescriptor(parent *etree.Element, cert *x509.Certificate) {
	key := parent.CreateElement("md:KeyDescriptor")
	key.CreateAttr("use", "signing")
	x509Data := key.CreateElement("ds:KeyInfo").CreateElement("ds:X509Data")
	x509Data.CreateElement("ds:X509Certificate").SetText(base64.StdEncoding.EncodeToString(cert.Raw))
}

func addEndpoint(parent *etree.Element, tag, binding, location string) {
	endpoint := parent.CreateElement(tag)
	endpoint.CreateAttr("Binding", binding)
	endpoint.CreateAttr("Location", location)
}
