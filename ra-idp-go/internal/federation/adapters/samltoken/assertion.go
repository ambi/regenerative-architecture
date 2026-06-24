// Package samltoken は Federation bounded context の SAML token アダプタ (ADR-047)。
//
// claim 発行エンジン (ADR-059) の出力を、署名済み SAML 2.0 assertion という XML ワイヤ形式に
// 変換する。XML 署名は goxmldsig に委ね、自前実装しない (ADR-060)。enveloped signature・
// exclusive C14N・RSA-SHA256 を用いる。
package samltoken

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"

	"ra-idp-go/internal/federation/domain"
)

const (
	samlAssertionNS    = "urn:oasis:names:tc:SAML:2.0:assertion"
	bearerConfirmation = "urn:oasis:names:tc:SAML:2.0:cm:bearer"
	authnContextUnspec = "urn:oasis:names:tc:SAML:2.0:ac:classes:unspecified"
	attrNameFormatURI  = "urn:oasis:names:tc:SAML:2.0:attrname-format:uri"
	xmlDateTime        = "2006-01-02T15:04:05Z"
	idAttribute        = "ID"
)

// AssertionInput は 1 つの SAML assertion を組み立てるための入力。
type AssertionInput struct {
	Issuer       string                     // IdP の entityID (issuer)。
	Audience     string                     // RP の entityID / wtrealm。
	Recipient    string                     // 任意。bearer の Recipient (ACS / wreply)。
	IssueInstant time.Time                  // 発行時刻。
	NotBefore    time.Time                  // 有効期間の開始。
	NotOnOrAfter time.Time                  // 有効期間の終了。
	AuthnInstant time.Time                  // 認証時刻。
	Result       domain.ClaimIssuanceResult // claim 発行エンジンの出力 (NameID + claims)。
}

func (in AssertionInput) validate() error {
	switch {
	case strings.TrimSpace(in.Issuer) == "":
		return fmt.Errorf("samltoken: issuer is required")
	case strings.TrimSpace(in.Audience) == "":
		return fmt.Errorf("samltoken: audience is required")
	case strings.TrimSpace(in.Result.NameIDFormat) == "":
		return fmt.Errorf("samltoken: NameID format is required")
	case strings.TrimSpace(in.Result.NameIDValue) == "":
		return fmt.Errorf("samltoken: NameID value is required")
	case in.NotOnOrAfter.Before(in.NotBefore) || in.NotOnOrAfter.Equal(in.NotBefore):
		return fmt.Errorf("samltoken: NotOnOrAfter must be after NotBefore")
	}
	return nil
}

// BuildAssertion は未署名の SAML 2.0 assertion を etree 要素として組み立て、その ID を返す。
func BuildAssertion(in AssertionInput) (*etree.Element, string, error) {
	if err := in.validate(); err != nil {
		return nil, "", err
	}

	id, err := newAssertionID()
	if err != nil {
		return nil, "", err
	}

	a := etree.NewElement("Assertion")
	a.CreateAttr("xmlns", samlAssertionNS)
	a.CreateAttr("Version", "2.0")
	a.CreateAttr(idAttribute, id)
	a.CreateAttr("IssueInstant", in.IssueInstant.UTC().Format(xmlDateTime))

	a.CreateElement("Issuer").SetText(in.Issuer)

	subject := a.CreateElement("Subject")
	nameID := subject.CreateElement("NameID")
	nameID.CreateAttr("Format", in.Result.NameIDFormat)
	nameID.SetText(in.Result.NameIDValue)
	confirmation := subject.CreateElement("SubjectConfirmation")
	confirmation.CreateAttr("Method", bearerConfirmation)
	confirmationData := confirmation.CreateElement("SubjectConfirmationData")
	confirmationData.CreateAttr("NotOnOrAfter", in.NotOnOrAfter.UTC().Format(xmlDateTime))
	if r := strings.TrimSpace(in.Recipient); r != "" {
		confirmationData.CreateAttr("Recipient", r)
	}

	conditions := a.CreateElement("Conditions")
	conditions.CreateAttr("NotBefore", in.NotBefore.UTC().Format(xmlDateTime))
	conditions.CreateAttr("NotOnOrAfter", in.NotOnOrAfter.UTC().Format(xmlDateTime))
	conditions.CreateElement("AudienceRestriction").CreateElement("Audience").SetText(in.Audience)

	authn := a.CreateElement("AuthnStatement")
	authn.CreateAttr("AuthnInstant", in.AuthnInstant.UTC().Format(xmlDateTime))
	authn.CreateElement("AuthnContext").CreateElement("AuthnContextClassRef").SetText(authnContextUnspec)

	if len(in.Result.Claims) > 0 {
		statement := a.CreateElement("AttributeStatement")
		for _, claim := range in.Result.Claims {
			attr := statement.CreateElement("Attribute")
			attr.CreateAttr("Name", claim.ClaimType)
			attr.CreateAttr("NameFormat", attrNameFormatURI)
			for _, value := range claim.Values {
				attr.CreateElement("AttributeValue").SetText(value)
			}
		}
	}

	return a, id, nil
}

// Signer は SAML assertion を enveloped 署名する (ADR-060)。
type Signer struct {
	ctx *dsig.SigningContext
}

// NewSigner は federation 署名証明書 (X.509) と RSA 秘密鍵から署名器を作る。
func NewSigner(cert *x509.Certificate, key *rsa.PrivateKey) (*Signer, error) {
	if cert == nil || key == nil {
		return nil, fmt.Errorf("samltoken: signing certificate and key are required")
	}
	ctx, err := dsig.NewSigningContext(key, [][]byte{cert.Raw})
	if err != nil {
		return nil, fmt.Errorf("samltoken: new signing context: %w", err)
	}
	ctx.Hash = crypto.SHA256
	ctx.IdAttribute = idAttribute
	ctx.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	return &Signer{ctx: ctx}, nil
}

// Sign は assertion 要素を enveloped 署名し、署名を含む新しい要素を返す。
func (s *Signer) Sign(assertion *etree.Element) (*etree.Element, error) {
	signed, err := s.ctx.SignEnveloped(assertion)
	if err != nil {
		return nil, fmt.Errorf("samltoken: sign enveloped: %w", err)
	}
	return signed, nil
}

// IssueSignedAssertion は assertion を組み立てて署名し、直列化した XML と assertion ID を返す。
func IssueSignedAssertion(in AssertionInput, s *Signer) ([]byte, string, error) {
	assertion, id, err := BuildAssertion(in)
	if err != nil {
		return nil, "", err
	}
	signed, err := s.Sign(assertion)
	if err != nil {
		return nil, "", err
	}
	doc := etree.NewDocument()
	doc.SetRoot(signed)
	out, err := doc.WriteToBytes()
	if err != nil {
		return nil, "", fmt.Errorf("samltoken: serialize assertion: %w", err)
	}
	return out, id, nil
}

// newAssertionID は SAML ID 属性に適した NCName (先頭非数字) を生成する。
func newAssertionID() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("samltoken: generate assertion id: %w", err)
	}
	return "_" + hex.EncodeToString(buf), nil
}
