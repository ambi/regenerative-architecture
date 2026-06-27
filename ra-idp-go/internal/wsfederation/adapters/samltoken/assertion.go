// Package samltoken は WsFederation bounded context の SAML token アダプタ (ADR-047)。
//
// claim 発行エンジン (ADR-059) の出力を、署名済み SAML assertion という XML ワイヤ形式に
// 変換する。Entra / AD FS の WS-Federation 既定である SAML 1.1 と、SAML 2.0 の双方を組み立てる
// (ADR-060, wi-61)。XML 署名は goxmldsig に委ね、自前実装しない (ADR-060)。enveloped signature・
// exclusive C14N・RSA-SHA256 を用いる。SAML 1.1 と 2.0 で ID 参照属性が異なる (AssertionID / ID)。
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

	"ra-idp-go/internal/wsfederation/domain"
)

const (
	samlAssertion20NS = "urn:oasis:names:tc:SAML:2.0:assertion"
	samlAssertion11NS = "urn:oasis:names:tc:SAML:1.0:assertion"

	bearerConfirmation20 = "urn:oasis:names:tc:SAML:2.0:cm:bearer"
	bearerConfirmation11 = "urn:oasis:names:tc:SAML:1.0:cm:bearer"

	authnContextUnspec20 = "urn:oasis:names:tc:SAML:2.0:ac:classes:unspecified"
	authnContextPassword = "urn:oasis:names:tc:SAML:2.0:ac:classes:Password"
	authnMethodUnspec11  = "urn:oasis:names:tc:SAML:1.0:am:unspecified"
	authnMethodPassword  = "urn:oasis:names:tc:SAML:1.0:am:password"

	attrNameFormatURI = "urn:oasis:names:tc:SAML:2.0:attrname-format:uri"
	xmlDateTime       = "2006-01-02T15:04:05Z"

	// ID 参照属性。SAML 2.0 は ID、SAML 1.1 は AssertionID。
	idAttribute   = "ID"
	idAttribute11 = "AssertionID"
)

// SAMLVersion は組み立てる assertion の SAML バージョン。ゼロ値は SAML 2.0。
type SAMLVersion int

const (
	// SAML20 は SAML 2.0 assertion。
	SAML20 SAMLVersion = iota
	// SAML11 は SAML 1.1 assertion (Entra / AD FS WS-Fed 既定)。
	SAML11
)

// AssertionInput は 1 つの SAML assertion を組み立てるための入力。
type AssertionInput struct {
	Version      SAMLVersion                // SAML バージョン (既定 2.0)。
	Issuer       string                     // IdP の entityID (issuer)。
	Audience     string                     // RP の entityID / wtrealm。
	Recipient    string                     // 任意。bearer の Recipient (ACS / wreply)。
	InResponseTo string                     // 任意。SP-initiated の AuthnRequest ID (SAML 2.0 のみ)。
	IssueInstant time.Time                  // 発行時刻。
	NotBefore    time.Time                  // 有効期間の開始。
	NotOnOrAfter time.Time                  // 有効期間の終了。
	AuthnInstant time.Time                  // 認証時刻。
	AuthnMethod  domain.AuthnMethodClass    // 認証方式クラス (wauth 尊重結果)。
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

// idAttributeFor は SAML バージョンに対応する署名 ID 参照属性名を返す。
func idAttributeFor(v SAMLVersion) string {
	if v == SAML11 {
		return idAttribute11
	}
	return idAttribute
}

// BuildAssertion は未署名の SAML assertion を etree 要素として組み立て、その ID を返す。
func BuildAssertion(in AssertionInput) (*etree.Element, string, error) {
	if err := in.validate(); err != nil {
		return nil, "", err
	}
	id, err := newAssertionID()
	if err != nil {
		return nil, "", err
	}
	if in.Version == SAML11 {
		return buildSAML11(in, id), id, nil
	}
	return buildSAML20(in, id), id, nil
}

// buildSAML20 は SAML 2.0 assertion を組み立てる。
func buildSAML20(in AssertionInput, id string) *etree.Element {
	a := etree.NewElement("Assertion")
	a.CreateAttr("xmlns", samlAssertion20NS)
	a.CreateAttr("Version", "2.0")
	a.CreateAttr(idAttribute, id)
	a.CreateAttr("IssueInstant", in.IssueInstant.UTC().Format(xmlDateTime))

	a.CreateElement("Issuer").SetText(in.Issuer)

	subject := a.CreateElement("Subject")
	nameID := subject.CreateElement("NameID")
	nameID.CreateAttr("Format", in.Result.NameIDFormat)
	nameID.SetText(in.Result.NameIDValue)
	confirmation := subject.CreateElement("SubjectConfirmation")
	confirmation.CreateAttr("Method", bearerConfirmation20)
	confirmationData := confirmation.CreateElement("SubjectConfirmationData")
	confirmationData.CreateAttr("NotOnOrAfter", in.NotOnOrAfter.UTC().Format(xmlDateTime))
	if r := strings.TrimSpace(in.Recipient); r != "" {
		confirmationData.CreateAttr("Recipient", r)
	}
	if irt := strings.TrimSpace(in.InResponseTo); irt != "" {
		confirmationData.CreateAttr("InResponseTo", irt)
	}

	conditions := a.CreateElement("Conditions")
	conditions.CreateAttr("NotBefore", in.NotBefore.UTC().Format(xmlDateTime))
	conditions.CreateAttr("NotOnOrAfter", in.NotOnOrAfter.UTC().Format(xmlDateTime))
	conditions.CreateElement("AudienceRestriction").CreateElement("Audience").SetText(in.Audience)

	authn := a.CreateElement("AuthnStatement")
	authn.CreateAttr("AuthnInstant", in.AuthnInstant.UTC().Format(xmlDateTime))
	classRef := authnContextUnspec20
	if in.AuthnMethod == domain.AuthnPassword {
		classRef = authnContextPassword
	}
	authn.CreateElement("AuthnContext").CreateElement("AuthnContextClassRef").SetText(classRef)

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

	return a
}

// buildSAML11 は SAML 1.1 assertion を組み立てる (Entra / AD FS WS-Fed 互換)。
func buildSAML11(in AssertionInput, id string) *etree.Element {
	a := etree.NewElement("Assertion")
	a.CreateAttr("xmlns", samlAssertion11NS)
	a.CreateAttr("MajorVersion", "1")
	a.CreateAttr("MinorVersion", "1")
	a.CreateAttr(idAttribute11, id)
	a.CreateAttr("Issuer", in.Issuer)
	a.CreateAttr("IssueInstant", in.IssueInstant.UTC().Format(xmlDateTime))

	conditions := a.CreateElement("Conditions")
	conditions.CreateAttr("NotBefore", in.NotBefore.UTC().Format(xmlDateTime))
	conditions.CreateAttr("NotOnOrAfter", in.NotOnOrAfter.UTC().Format(xmlDateTime))
	conditions.CreateElement("AudienceRestrictionCondition").CreateElement("Audience").SetText(in.Audience)

	authn := a.CreateElement("AuthenticationStatement")
	method := authnMethodUnspec11
	if in.AuthnMethod == domain.AuthnPassword {
		method = authnMethodPassword
	}
	authn.CreateAttr("AuthenticationMethod", method)
	authn.CreateAttr("AuthenticationInstant", in.AuthnInstant.UTC().Format(xmlDateTime))
	addSAML11Subject(authn, in)

	if len(in.Result.Claims) > 0 {
		statement := a.CreateElement("AttributeStatement")
		addSAML11Subject(statement, in)
		for _, claim := range in.Result.Claims {
			ns, name := splitClaimType(claim.ClaimType)
			attr := statement.CreateElement("Attribute")
			attr.CreateAttr("AttributeName", name)
			attr.CreateAttr("AttributeNamespace", ns)
			for _, value := range claim.Values {
				attr.CreateElement("AttributeValue").SetText(value)
			}
		}
	}

	return a
}

// addSAML11Subject は SAML 1.1 の Subject (NameIdentifier + bearer 確認) を親要素に追加する。
func addSAML11Subject(parent *etree.Element, in AssertionInput) {
	subject := parent.CreateElement("Subject")
	nameID := subject.CreateElement("NameIdentifier")
	nameID.CreateAttr("Format", in.Result.NameIDFormat)
	nameID.SetText(in.Result.NameIDValue)
	confirmation := subject.CreateElement("SubjectConfirmation")
	confirmation.CreateElement("ConfirmationMethod").SetText(bearerConfirmation11)
}

// splitClaimType は claim 型 URI を AD FS 流の (namespace, name) に分割する。
// 最後の '/' で分け、namespace は手前まで、name は後ろ。'/' が無ければ namespace は空。
func splitClaimType(claimType string) (namespace, name string) {
	if i := strings.LastIndex(claimType, "/"); i >= 0 {
		return claimType[:i], claimType[i+1:]
	}
	return "", claimType
}

// Signer は SAML assertion を enveloped 署名する (ADR-060)。SAML 1.1/2.0 で ID 参照属性が
// 異なるため、署名時に対象属性名を受け取り、署名コンテキストを都度構築する。
type Signer struct {
	cert *x509.Certificate
	key  *rsa.PrivateKey
}

// NewSigner は federation 署名証明書 (X.509) と RSA 秘密鍵から署名器を作る。
func NewSigner(cert *x509.Certificate, key *rsa.PrivateKey) (*Signer, error) {
	if cert == nil || key == nil {
		return nil, fmt.Errorf("samltoken: signing certificate and key are required")
	}
	return &Signer{cert: cert, key: key}, nil
}

// Certificate は federation 署名証明書の読み取り専用コピーを返す。
func (s *Signer) Certificate() *x509.Certificate {
	if s == nil || s.cert == nil {
		return nil
	}
	return s.cert
}

func (s *Signer) signingContext(idAttr string) (*dsig.SigningContext, error) {
	ctx, err := dsig.NewSigningContext(s.key, [][]byte{s.cert.Raw})
	if err != nil {
		return nil, fmt.Errorf("samltoken: new signing context: %w", err)
	}
	ctx.Hash = crypto.SHA256
	ctx.IdAttribute = idAttr
	ctx.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	return ctx, nil
}

// Sign は assertion 要素を enveloped 署名し、署名を含む新しい要素を返す。idAttribute は
// 署名対象の ID 参照属性名 (SAML 2.0 は "ID"、SAML 1.1 は "AssertionID")。
func (s *Signer) Sign(assertion *etree.Element, idAttribute string) (*etree.Element, error) {
	ctx, err := s.signingContext(idAttribute)
	if err != nil {
		return nil, err
	}
	signed, err := ctx.SignEnveloped(assertion)
	if err != nil {
		return nil, fmt.Errorf("samltoken: sign enveloped: %w", err)
	}
	return signed, nil
}

// BuildSignedAssertion は assertion を組み立てて enveloped 署名し、署名済みの要素を返す。
// RSTR への埋め込み用に直列化前の要素を返す点が IssueSignedAssertion と異なる。
func BuildSignedAssertion(in AssertionInput, s *Signer) (*etree.Element, string, error) {
	assertion, id, err := BuildAssertion(in)
	if err != nil {
		return nil, "", err
	}
	signed, err := s.Sign(assertion, idAttributeFor(in.Version))
	if err != nil {
		return nil, "", err
	}
	return signed, id, nil
}

// IssueSignedAssertion は assertion を組み立てて署名し、直列化した XML と assertion ID を返す。
func IssueSignedAssertion(in AssertionInput, s *Signer) ([]byte, string, error) {
	assertion, id, err := BuildAssertion(in)
	if err != nil {
		return nil, "", err
	}
	signed, err := s.Sign(assertion, idAttributeFor(in.Version))
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
