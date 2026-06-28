// Package domain は SAML 2.0 Web Browser SSO Profile の純粋なドメイン判定を所有する (wi-29)。
//
// 本ファイルは SP-initiated の <samlp:AuthnRequest> の復号・解析・検証を担う。HTTP や XML 署名・
// assertion 直列化には依存しない:
//
//   - DecodeRedirect / DecodePost: HTTP-Redirect (deflate+base64) / HTTP-POST (base64) を復号する。
//   - ParseAuthnRequest: ID / Issuer / AssertionConsumerServiceURL / Destination / NameIDPolicy を取り出す。
//   - ValidateSignIn: 要求を登録済み SP に解決し、ACS URL を許可集合に限定する (open redirect 防止, fail-closed)。
//
// 判定不能・不一致はすべて拒否側へ倒す。
package domain

import (
	"bytes"
	"compress/flate"
	"crypto"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // SAML legacy interop: RSA-SHA1 is accepted only for verification of configured SP signatures.
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"

	"ra-idp-go/internal/shared/spec"
)

const (
	// maxAuthnRequestBytes は復号後 AuthnRequest XML の上限 (deflate 爆弾・巨大 POST の防御)。
	maxAuthnRequestBytes = 256 * 1024
	// freshAuthGrace は ForceAuthn=true のログイン往復直後に再リダイレクトループを避ける猶予。
	freshAuthGrace = 30 * time.Second
)

// AuthnRequest は SP-initiated SSO の要求から取り出した検証対象の値。
type AuthnRequest struct {
	ID           string // 要求 ID。SAMLResponse の InResponseTo に往復させる。
	Issuer       string // SP の entityID。
	ACSURL       string // 任意。AssertionConsumerServiceURL (許可集合に対して検証する)。
	Destination  string // 任意。要求が宛てた IdP endpoint URL。
	NameIDFormat string // 任意。NameIDPolicy/@Format。
	ForceAuthn   bool   // 任意。再認証の要求。
}

type LogoutRequest struct {
	ID          string
	Issuer      string
	Destination string
	NameID      string
}

type Binding string

const (
	BindingRedirect Binding = "redirect"
	BindingPOST     Binding = "post"
)

// DecodeRedirect は HTTP-Redirect binding の SAMLRequest を復号する。
// base64 デコード後、raw DEFLATE で展開する (SAML deflate encoding)。
func DecodeRedirect(samlRequest string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(samlRequest))
	if err != nil {
		return nil, fmt.Errorf("saml: decode redirect SAMLRequest base64: %w", err)
	}
	reader := flate.NewReader(bytes.NewReader(raw))
	defer func() { _ = reader.Close() }()
	out, err := io.ReadAll(io.LimitReader(reader, maxAuthnRequestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("saml: inflate redirect SAMLRequest: %w", err)
	}
	if len(out) > maxAuthnRequestBytes {
		return nil, fmt.Errorf("saml: AuthnRequest exceeds %d bytes", maxAuthnRequestBytes)
	}
	return out, nil
}

// EncodeRedirect は AuthnRequest XML を HTTP-Redirect binding 形式 (raw DEFLATE + base64) に符号化する。
// 未認証時にログイン往復をまたいで SP-initiated 要求を保つための resume URL 構築に使う。
func EncodeRedirect(xml []byte) (string, error) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return "", fmt.Errorf("saml: new deflate writer: %w", err)
	}
	if _, err := w.Write(xml); err != nil {
		return "", fmt.Errorf("saml: deflate AuthnRequest: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("saml: close deflate writer: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// DecodePost は HTTP-POST binding の SAMLRequest を復号する (base64 のみ)。
func DecodePost(samlRequest string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(samlRequest))
	if err != nil {
		return nil, fmt.Errorf("saml: decode POST SAMLRequest base64: %w", err)
	}
	if len(raw) > maxAuthnRequestBytes {
		return nil, fmt.Errorf("saml: AuthnRequest exceeds %d bytes", maxAuthnRequestBytes)
	}
	return raw, nil
}

// ParseAuthnRequest は復号済み XML から AuthnRequest の検証対象値を取り出す。
func ParseAuthnRequest(xml []byte) (AuthnRequest, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xml); err != nil {
		return AuthnRequest{}, fmt.Errorf("saml: parse AuthnRequest XML: %w", err)
	}
	root := doc.Root()
	if root == nil || root.Tag != "AuthnRequest" {
		return AuthnRequest{}, fmt.Errorf("saml: root element is not AuthnRequest")
	}

	req := AuthnRequest{
		ID:          strings.TrimSpace(root.SelectAttrValue("ID", "")),
		ACSURL:      strings.TrimSpace(root.SelectAttrValue("AssertionConsumerServiceURL", "")),
		Destination: strings.TrimSpace(root.SelectAttrValue("Destination", "")),
		ForceAuthn:  strings.EqualFold(root.SelectAttrValue("ForceAuthn", ""), "true"),
	}
	if issuer := childByTag(root, "Issuer"); issuer != nil {
		req.Issuer = strings.TrimSpace(issuer.Text())
	}
	if policy := childByTag(root, "NameIDPolicy"); policy != nil {
		req.NameIDFormat = strings.TrimSpace(policy.SelectAttrValue("Format", ""))
	}
	if req.ID == "" {
		return AuthnRequest{}, fmt.Errorf("saml: AuthnRequest is missing ID")
	}
	if req.Issuer == "" {
		return AuthnRequest{}, fmt.Errorf("saml: AuthnRequest is missing Issuer")
	}
	return req, nil
}

func ParseLogoutRequest(xml []byte) (LogoutRequest, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xml); err != nil {
		return LogoutRequest{}, fmt.Errorf("saml: parse LogoutRequest XML: %w", err)
	}
	root := doc.Root()
	if root == nil || root.Tag != "LogoutRequest" {
		return LogoutRequest{}, fmt.Errorf("saml: root element is not LogoutRequest")
	}
	req := LogoutRequest{
		ID:          strings.TrimSpace(root.SelectAttrValue("ID", "")),
		Destination: strings.TrimSpace(root.SelectAttrValue("Destination", "")),
	}
	if issuer := childByTag(root, "Issuer"); issuer != nil {
		req.Issuer = strings.TrimSpace(issuer.Text())
	}
	if nameID := childByTag(root, "NameID"); nameID != nil {
		req.NameID = strings.TrimSpace(nameID.Text())
	}
	if req.ID == "" {
		return LogoutRequest{}, fmt.Errorf("saml: LogoutRequest is missing ID")
	}
	if req.Issuer == "" {
		return LogoutRequest{}, fmt.Errorf("saml: LogoutRequest is missing Issuer")
	}
	return req, nil
}

// childByTag は名前空間接頭辞を無視して指定ローカル名の最初の子要素を返す。
func childByTag(parent *etree.Element, tag string) *etree.Element {
	for _, child := range parent.ChildElements() {
		if child.Tag == tag {
			return child
		}
	}
	return nil
}

// ValidatedSignIn は検証を通った SP-initiated SSO 要求の確定結果。
type ValidatedSignIn struct {
	ServiceProvider spec.SamlServiceProvider
	ACSURL          string // 実際に POST する ACS (許可集合内に確定済み)。
	InResponseTo    string // SAMLResponse に往復させる AuthnRequest ID。
	NameIDFormat    string // 発行 assertion の NameID format (要求 > SP 既定)。
}

// ValidateSignIn は要求を SP に解決し、ACS URL と Destination を許可集合に限定する (fail-closed)。
//
//   - Issuer は sp.EntityID と完全一致しなければならない。
//   - Destination 指定時は現在 realm の SSO endpoint と完全一致しなければならない。
//   - AssertionConsumerServiceURL 指定時は sp.ACSURLs の完全一致のみ受理する (open redirect 防止)。
//   - 省略時は sp.ACSURLs の先頭を既定の ACS とする。
//   - NameID format は要求の NameIDPolicy を尊重し、未指定なら SP の claim policy の format を用いる。
func ValidateSignIn(req AuthnRequest, sp spec.SamlServiceProvider, expectedDestination string) (ValidatedSignIn, error) {
	if req.Issuer != sp.EntityID {
		return ValidatedSignIn{}, fmt.Errorf("saml: issuer %q does not match service provider", req.Issuer)
	}
	if req.Destination != "" && req.Destination != expectedDestination {
		return ValidatedSignIn{}, fmt.Errorf("saml: destination %q does not match SSO endpoint", req.Destination)
	}
	if len(sp.ACSURLs) == 0 {
		return ValidatedSignIn{}, fmt.Errorf("saml: service provider %q has no assertion consumer service URL", sp.EntityID)
	}

	acsURL := sp.DefaultACSURL()
	if req.ACSURL != "" {
		if !sp.AllowsACSURL(req.ACSURL) {
			return ValidatedSignIn{}, fmt.Errorf("saml: assertion consumer service URL %q is not allowed", req.ACSURL)
		}
		acsURL = req.ACSURL
	}

	nameIDFormat := sp.ClaimPolicy.NameID.Format
	if req.NameIDFormat != "" && req.NameIDFormat != spec.SamlNameIDFormatUnspecified {
		nameIDFormat = req.NameIDFormat
	}

	return ValidatedSignIn{
		ServiceProvider: sp,
		ACSURL:          acsURL,
		InResponseTo:    req.ID,
		NameIDFormat:    nameIDFormat,
	}, nil
}

// RequiresFreshAuth は ForceAuthn=true の要求に対して、既存認証を使ってよいかを判定する。
func RequiresFreshAuth(forceAuthn bool, authTime, now time.Time) bool {
	if !forceAuthn {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return now.Sub(authTime) > freshAuthGrace
}

func ParseCertificatePEM(pemText string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(pemText)))
	if block == nil {
		return nil, fmt.Errorf("saml: signing certificate PEM is required")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("saml: parse signing certificate: %w", err)
	}
	return cert, nil
}

func ValidateRequestSignature(binding Binding, xml []byte, rawQuery string, sp spec.SamlServiceProvider) error {
	if !sp.WantAuthnRequestsSigned {
		return nil
	}
	cert, err := ParseCertificatePEM(sp.AuthnRequestSigningCertificatePEM)
	if err != nil {
		return err
	}
	switch binding {
	case BindingRedirect:
		return validateRedirectSignature(rawQuery, cert)
	case BindingPOST:
		return validateXMLSignature(xml, cert)
	default:
		return fmt.Errorf("saml: unsupported binding for signature verification")
	}
}

func validateRedirectSignature(rawQuery string, cert *x509.Certificate) error {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return fmt.Errorf("saml: parse redirect query: %w", err)
	}
	sigAlg := values.Get("SigAlg")
	signature := values.Get("Signature")
	if values.Get("SAMLRequest") == "" || sigAlg == "" || signature == "" {
		return fmt.Errorf("saml: signed Redirect binding requires SAMLRequest, SigAlg, and Signature")
	}
	signed := "SAMLRequest=" + url.QueryEscape(values.Get("SAMLRequest"))
	if relay := values.Get("RelayState"); relay != "" {
		signed += "&RelayState=" + url.QueryEscape(relay)
	}
	signed += "&SigAlg=" + url.QueryEscape(sigAlg)
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("saml: decode redirect Signature: %w", err)
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("saml: signing certificate must contain an RSA public key")
	}
	switch sigAlg {
	case "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256":
		sum := sha256.Sum256([]byte(signed))
		if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); err != nil {
			return fmt.Errorf("saml: invalid redirect signature: %w", err)
		}
	case "http://www.w3.org/2000/09/xmldsig#rsa-sha1":
		sum := sha1.Sum([]byte(signed)) //nolint:gosec // SAML legacy interop: verify-only for configured SP signatures.
		if err := rsa.VerifyPKCS1v15(pub, crypto.SHA1, sum[:], sig); err != nil {
			return fmt.Errorf("saml: invalid redirect signature: %w", err)
		}
	default:
		return fmt.Errorf("saml: unsupported SigAlg %q", sigAlg)
	}
	return nil
}

func validateXMLSignature(xml []byte, cert *x509.Certificate) error {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xml); err != nil {
		return fmt.Errorf("saml: parse signed XML: %w", err)
	}
	if doc.FindElement("//Signature") == nil {
		return fmt.Errorf("saml: XML signature is required")
	}
	store := &dsig.MemoryX509CertificateStore{Roots: []*x509.Certificate{cert}}
	vctx := dsig.NewDefaultValidationContext(store)
	vctx.IdAttribute = "ID"
	if _, err := vctx.Validate(doc.Root()); err != nil {
		return fmt.Errorf("saml: invalid XML signature: %w", err)
	}
	return nil
}
