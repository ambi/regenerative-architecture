// Package metadata は WS-* federation metadata の XML ワイヤ形式を組み立てる。
//
// WsFederation bounded context の claim policy / endpoint 情報から AD FS 互換の
// federationmetadata.xml を導出する。署名証明書は KeyDescriptor として広告し、文書署名や
// 鍵ローテーションの永続化は後続の KMS/key lifecycle WI に委ねる。
package metadata

import (
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/beevik/etree"
)

const (
	nsMD      = "urn:oasis:names:tc:SAML:2.0:metadata"
	nsDS      = "http://www.w3.org/2000/09/xmldsig#"
	nsFed     = "http://docs.oasis-open.org/wsfed/federation/200706"
	nsPolicy  = "http://schemas.xmlsoap.org/ws/2004/09/policy"
	nsAddress = "http://www.w3.org/2005/08/addressing"
	nsTrust13 = "http://docs.oasis-open.org/ws-sx/ws-trust/200512"
	nsSoap12  = "http://schemas.xmlsoap.org/wsdl/soap12/"
)

// EndpointSet は metadata に広告する WS-Federation / WS-Trust endpoint 群。
type EndpointSet struct {
	PassiveURL        string
	ActiveURL         string
	MEXURL            string
	FederationMetaURL string
}

// BuildFederationMetadata は realm 単位の federationmetadata.xml を生成する。
func BuildFederationMetadata(entityID string, cert *x509.Certificate, endpoints EndpointSet, now time.Time) ([]byte, error) {
	if strings.TrimSpace(entityID) == "" {
		return nil, fmt.Errorf("metadata: entityID is required")
	}
	if cert == nil {
		return nil, fmt.Errorf("metadata: signing certificate is required")
	}
	if strings.TrimSpace(endpoints.PassiveURL) == "" {
		return nil, fmt.Errorf("metadata: passive endpoint is required")
	}

	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="utf-8"`)
	root := doc.CreateElement("EntityDescriptor")
	root.CreateAttr("xmlns", nsMD)
	root.CreateAttr("xmlns:ds", nsDS)
	root.CreateAttr("xmlns:fed", nsFed)
	root.CreateAttr("xmlns:wsp", nsPolicy)
	root.CreateAttr("xmlns:wsa", nsAddress)
	root.CreateAttr("xmlns:t", nsTrust13)
	root.CreateAttr("entityID", entityID)
	root.CreateAttr("ID", "FederationMetadata")
	root.CreateAttr("validUntil", now.UTC().Add(24*time.Hour).Format(time.RFC3339))

	sts := root.CreateElement("RoleDescriptor")
	sts.CreateAttr("xsi:type", "fed:SecurityTokenServiceType")
	sts.CreateAttr("xmlns:xsi", "http://www.w3.org/2001/XMLSchema-instance")
	sts.CreateAttr("protocolSupportEnumeration", nsFed)
	addKeyDescriptor(sts, cert)
	addClaimTypes(sts)
	addPassiveEndpoint(sts, endpoints.PassiveURL)
	if strings.TrimSpace(endpoints.ActiveURL) != "" {
		addActiveEndpoint(sts, endpoints.ActiveURL)
	}
	if strings.TrimSpace(endpoints.MEXURL) != "" {
		addMetadataEndpoint(sts, endpoints.MEXURL)
	}

	app := root.CreateElement("RoleDescriptor")
	app.CreateAttr("xsi:type", "fed:ApplicationServiceType")
	app.CreateAttr("xmlns:xsi", "http://www.w3.org/2001/XMLSchema-instance")
	app.CreateAttr("protocolSupportEnumeration", nsFed)
	addKeyDescriptor(app, cert)
	addPassiveEndpoint(app, endpoints.PassiveURL)

	doc.Indent(2)
	out, err := doc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("metadata: serialize: %w", err)
	}
	return out, nil
}

func addKeyDescriptor(parent *etree.Element, cert *x509.Certificate) {
	key := parent.CreateElement("KeyDescriptor")
	key.CreateAttr("use", "signing")
	keyInfo := key.CreateElement("ds:KeyInfo")
	x509Data := keyInfo.CreateElement("ds:X509Data")
	x509Data.CreateElement("ds:X509Certificate").SetText(base64.StdEncoding.EncodeToString(cert.Raw))
}

func addClaimTypes(parent *etree.Element) {
	offered := parent.CreateElement("fed:ClaimTypesOffered")
	for _, claimType := range []string{
		"http://schemas.xmlsoap.org/claims/UPN",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/nameidentifier",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
	} {
		claim := offered.CreateElement("auth:ClaimType")
		claim.CreateAttr("xmlns:auth", "http://docs.oasis-open.org/wsfed/authorization/200706")
		claim.CreateAttr("Uri", claimType)
	}
}

func addPassiveEndpoint(parent *etree.Element, endpoint string) {
	passive := parent.CreateElement("fed:PassiveRequestorEndpoint")
	addEndpointReference(passive, endpoint)
}

func addActiveEndpoint(parent *etree.Element, endpoint string) {
	active := parent.CreateElement("fed:SecurityTokenServiceEndpoint")
	active.CreateElement("wsp:AppliesTo").CreateElement("wsa:EndpointReference").CreateElement("wsa:Address").SetText(endpoint)
	addEndpointReference(active, endpoint)
	policy := active.CreateElement("wsp:Policy")
	binding := policy.CreateElement("sp:TransportBinding")
	binding.CreateAttr("xmlns:sp", "http://docs.oasis-open.org/ws-sx/ws-securitypolicy/200702")
	supportingTokens := policy.CreateElement("sp:SignedSupportingTokens")
	supportingTokens.CreateAttr("xmlns:sp", "http://docs.oasis-open.org/ws-sx/ws-securitypolicy/200702")
	policy.CreateElement("t:RequestType").SetText(nsTrust13 + "/Issue")
}

func addMetadataEndpoint(parent *etree.Element, endpoint string) {
	mex := parent.CreateElement("fed:MetadataEndpoint")
	addEndpointReference(mex, endpoint)
}

func addEndpointReference(parent *etree.Element, endpoint string) {
	ref := parent.CreateElement("wsa:EndpointReference")
	ref.CreateElement("wsa:Address").SetText(endpoint)
}

// BuildMEX は WS-Trust active endpoint と federation metadata URL を広告する簡易 MEX を生成する。
func BuildMEX(endpoints EndpointSet) ([]byte, error) {
	if strings.TrimSpace(endpoints.ActiveURL) == "" {
		return nil, fmt.Errorf("metadata: active endpoint is required")
	}
	if strings.TrimSpace(endpoints.MEXURL) == "" {
		return nil, fmt.Errorf("metadata: MEX endpoint is required")
	}
	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="utf-8"`)
	root := doc.CreateElement("mex:Metadata")
	root.CreateAttr("xmlns:mex", "http://schemas.xmlsoap.org/ws/2004/09/mex")
	root.CreateAttr("xmlns:wsa", nsAddress)
	root.CreateAttr("xmlns:wsp", nsPolicy)
	root.CreateAttr("xmlns:wsdl", "http://schemas.xmlsoap.org/wsdl/")
	root.CreateAttr("xmlns:soap12", nsSoap12)
	root.CreateAttr("xmlns:t", nsTrust13)

	section := root.CreateElement("mex:MetadataSection")
	section.CreateAttr("Dialect", "http://schemas.xmlsoap.org/wsdl/")
	wsdl := section.CreateElement("wsdl:definitions")
	wsdl.CreateAttr("targetNamespace", nsTrust13)
	service := wsdl.CreateElement("wsdl:service")
	service.CreateAttr("name", "SecurityTokenService")
	port := service.CreateElement("wsdl:port")
	port.CreateAttr("name", "UserNameWSTrustBinding_IWSTrust13Sync")
	port.CreateAttr("binding", "t:UserNameWSTrustBinding_IWSTrust13Sync")
	address := port.CreateElement("soap12:address")
	address.CreateAttr("location", endpoints.ActiveURL)

	policySection := root.CreateElement("mex:MetadataSection")
	policySection.CreateAttr("Dialect", nsPolicy)
	policy := policySection.CreateElement("wsp:Policy")
	policy.CreateAttr("wsu:Id", "UserNameWSTrustBinding")
	policy.CreateAttr("xmlns:wsu", "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd")
	supportingTokens := policy.CreateElement("sp:SignedSupportingTokens")
	supportingTokens.CreateAttr("xmlns:sp", "http://docs.oasis-open.org/ws-sx/ws-securitypolicy/200702")
	policy.CreateElement("t:RequestType").SetText(nsTrust13 + "/Issue")

	if strings.TrimSpace(endpoints.FederationMetaURL) != "" {
		metaSection := root.CreateElement("mex:MetadataSection")
		metaSection.CreateAttr("Dialect", nsMD)
		ref := metaSection.CreateElement("mex:MetadataReference")
		addEndpointReference(ref, endpoints.FederationMetaURL)
	}

	doc.Indent(2)
	out, err := doc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("metadata: serialize MEX: %w", err)
	}
	return out, nil
}
