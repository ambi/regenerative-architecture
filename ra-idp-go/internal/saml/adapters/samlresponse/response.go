// Package samlresponse は Saml bounded context の SAMLResponse アダプタ (wi-29, ADR-067)。
//
// 署名済み <saml:Assertion> (samltoken, ADR-060) を SAML 2.0 Web Browser SSO の
// <samlp:Response> に包み、必要なら Response 全体も enveloped 署名し、HTTP-POST binding の
// 自動 POST フォームに直列化する。XML 署名・canonicalization は goxmldsig に委ね、自前実装しない。
package samlresponse

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/beevik/etree"

	"ra-idp-go/internal/wsfederation/adapters/samltoken"
)

const (
	samlProtocolNS  = "urn:oasis:names:tc:SAML:2.0:protocol"
	samlAssertionNS = "urn:oasis:names:tc:SAML:2.0:assertion"
	statusSuccess   = "urn:oasis:names:tc:SAML:2.0:status:Success"
	xmlDateTime     = "2006-01-02T15:04:05Z"
)

// ResponseInput は 1 つの <samlp:Response> を組み立てるための入力。
type ResponseInput struct {
	Issuer       string         // IdP の entityID。
	Destination  string         // ACS URL (Response/@Destination)。
	InResponseTo string         // 任意。SP-initiated の AuthnRequest ID。
	IssueInstant time.Time      // 発行時刻。
	Assertion    *etree.Element // 埋め込む <saml:Assertion> (署名済み・未署名いずれも可)。
	SignResponse bool           // <Response> 全体を enveloped 署名するか。
}

// BuildResponse は <samlp:Response> を組み立て、必要なら Response 全体を署名し、直列化した
// XML を返す。signer は SignResponse が true のときのみ用いる。
func BuildResponse(in ResponseInput, signer *samltoken.Signer) ([]byte, error) {
	if strings.TrimSpace(in.Issuer) == "" {
		return nil, fmt.Errorf("samlresponse: issuer is required")
	}
	if strings.TrimSpace(in.Destination) == "" {
		return nil, fmt.Errorf("samlresponse: destination is required")
	}
	if in.Assertion == nil {
		return nil, fmt.Errorf("samlresponse: assertion is required")
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}

	resp := etree.NewElement("samlp:Response")
	resp.CreateAttr("xmlns:samlp", samlProtocolNS)
	resp.CreateAttr("xmlns:saml", samlAssertionNS)
	resp.CreateAttr("ID", id)
	resp.CreateAttr("Version", "2.0")
	resp.CreateAttr("IssueInstant", in.IssueInstant.UTC().Format(xmlDateTime))
	resp.CreateAttr("Destination", in.Destination)
	if irt := strings.TrimSpace(in.InResponseTo); irt != "" {
		resp.CreateAttr("InResponseTo", irt)
	}

	issuer := resp.CreateElement("saml:Issuer")
	issuer.SetText(in.Issuer)

	status := resp.CreateElement("samlp:Status")
	status.CreateElement("samlp:StatusCode").CreateAttr("Value", statusSuccess)

	resp.AddChild(in.Assertion.Copy())

	root := resp
	if in.SignResponse {
		if signer == nil {
			return nil, fmt.Errorf("samlresponse: signer is required to sign response")
		}
		// goxmldsig は enveloped 署名を末尾に付与する。enveloped transform は署名の位置に
		// 依存せず検証されるため、署名要素はその位置のまま残す (assertion 署名と同じ扱い)。
		// 署名後に要素を再配置すると名前空間の再描画で digest が変わり検証不能になる。
		signed, err := signer.Sign(resp, "ID")
		if err != nil {
			return nil, fmt.Errorf("samlresponse: sign response: %w", err)
		}
		root = signed
	}

	doc := etree.NewDocument()
	doc.SetRoot(root)
	out, err := doc.WriteToBytes()
	if err != nil {
		return nil, fmt.Errorf("samlresponse: serialize response: %w", err)
	}
	return out, nil
}

// EncodePostForm は SAMLResponse を base64 化し、ACS への自動 POST フォーム HTML を返す。
func EncodePostForm(responseXML []byte, acsURL, relayState string) ([]byte, error) {
	if strings.TrimSpace(acsURL) == "" {
		return nil, fmt.Errorf("samlresponse: ACS URL is required")
	}
	encoded := base64.StdEncoding.EncodeToString(responseXML)
	var buf bytes.Buffer
	data := struct {
		ACSURL       template.URL
		SAMLResponse string
		RelayState   string
	}{
		ACSURL:       template.URL(acsURL), //nolint:gosec // 呼び出し側が許可集合に対して検証済み (ValidateSignIn)。
		SAMLResponse: encoded,
		RelayState:   relayState,
	}
	if err := postForm.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("samlresponse: render post form: %w", err)
	}
	return buf.Bytes(), nil
}

// newID は SAML ID 属性に適した NCName (先頭非数字) を生成する。
func newID() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("samlresponse: generate response id: %w", err)
	}
	return "_" + hex.EncodeToString(buf), nil
}

var postForm = template.Must(template.New("saml-post").Parse(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Sign in</title></head>
<body onload="document.forms[0].submit()">
<form method="POST" action="{{.ACSURL}}">
<input type="hidden" name="SAMLResponse" value="{{.SAMLResponse}}">
{{if .RelayState}}<input type="hidden" name="RelayState" value="{{.RelayState}}">
{{end}}<noscript><input type="submit" value="Continue"></noscript>
</form>
</body>
</html>
`))
