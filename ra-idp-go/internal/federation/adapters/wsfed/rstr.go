// Package wsfed は Federation bounded context の WS-Federation passive アダプタ (ADR-047, wi-61)。
//
// 署名済み SAML assertion (samltoken アダプタ, ADR-060) を WS-Trust の RequestSecurityTokenResponse
// (RSTR) に包み、relying party へ自動 POST する passive レスポンスを組み立てる。XML 構造は etree、
// 自動 POST フォームは html/template でエスケープして出力する。
package wsfed

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// WS-Federation passive で用いる名前空間。WS-Fed 1.x passive は WS-Trust 2005/02 を用いる。
// Entra 等との相互運用上の最終確認は wi-64 (実テナント検証) で行う。
const (
	nsTrust      = "http://schemas.xmlsoap.org/ws/2005/02/trust"
	nsPolicy     = "http://schemas.xmlsoap.org/ws/2004/09/policy"
	nsAddressing = "http://www.w3.org/2005/08/addressing"
	nsWSU        = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"

	tokenTypeSAML11 = "urn:oasis:names:tc:SAML:1.0:assertion"
	requestIssue    = "http://schemas.xmlsoap.org/ws/2005/02/trust/Issue"
	keyTypeBearer   = "http://schemas.xmlsoap.org/ws/2005/05/identity/NoProofKey"

	xmlDateTime = "2006-01-02T15:04:05Z"
)

// BuildRSTR は署名済み assertion を RSTR 要素に包む。appliesTo は RP の wtrealm、
// tokenType は assertion の SAML バージョンを表す URN (空なら SAML 1.1 を既定とする)。
func BuildRSTR(signedAssertion *etree.Element, appliesTo, tokenType string, created, expires time.Time) (*etree.Element, error) {
	if signedAssertion == nil {
		return nil, fmt.Errorf("wsfed: signed assertion is required")
	}
	if strings.TrimSpace(appliesTo) == "" {
		return nil, fmt.Errorf("wsfed: appliesTo is required")
	}
	if strings.TrimSpace(tokenType) == "" {
		tokenType = tokenTypeSAML11
	}

	rstr := etree.NewElement("t:RequestSecurityTokenResponse")
	rstr.CreateAttr("xmlns:t", nsTrust)

	lifetime := rstr.CreateElement("t:Lifetime")
	createdEl := lifetime.CreateElement("wsu:Created")
	createdEl.CreateAttr("xmlns:wsu", nsWSU)
	createdEl.SetText(created.UTC().Format(xmlDateTime))
	expiresEl := lifetime.CreateElement("wsu:Expires")
	expiresEl.CreateAttr("xmlns:wsu", nsWSU)
	expiresEl.SetText(expires.UTC().Format(xmlDateTime))

	appliesToEl := rstr.CreateElement("wsp:AppliesTo")
	appliesToEl.CreateAttr("xmlns:wsp", nsPolicy)
	epr := appliesToEl.CreateElement("wsa:EndpointReference")
	epr.CreateAttr("xmlns:wsa", nsAddressing)
	epr.CreateElement("wsa:Address").SetText(appliesTo)

	requested := rstr.CreateElement("t:RequestedSecurityToken")
	requested.AddChild(signedAssertion.Copy())

	rstr.CreateElement("t:TokenType").SetText(tokenType)
	rstr.CreateElement("t:RequestType").SetText(requestIssue)
	rstr.CreateElement("t:KeyType").SetText(keyTypeBearer)

	return rstr, nil
}

// SerializeRSTR は RSTR 要素を XML 文字列に直列化する (wresult 値になる)。
func SerializeRSTR(rstr *etree.Element) (string, error) {
	doc := etree.NewDocument()
	doc.SetRoot(rstr.Copy())
	out, err := doc.WriteToString()
	if err != nil {
		return "", fmt.Errorf("wsfed: serialize RSTR: %w", err)
	}
	return out, nil
}

// passiveForm は relying party へ wresult を自動 POST する HTML フォーム。
// 値はすべて html/template により属性コンテキストでエスケープされる。
var passiveForm = template.Must(template.New("wsfed-passive").Parse(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Sign in</title></head>
<body onload="document.forms[0].submit()">
<form method="POST" action="{{.ReplyURL}}">
<input type="hidden" name="wa" value="wsignin1.0">
<input type="hidden" name="wresult" value="{{.Wresult}}">
{{if .Wctx}}<input type="hidden" name="wctx" value="{{.Wctx}}">
{{end}}<noscript><input type="submit" value="Continue"></noscript>
</form>
</body>
</html>
`))

// RenderPassiveForm は自動 POST フォームの HTML を生成する。replyURL は呼び出し側で
// 許可集合に対して検証済みであること (ValidateSignIn)。
func RenderPassiveForm(replyURL, wresult, wctx string) ([]byte, error) {
	if strings.TrimSpace(replyURL) == "" {
		return nil, fmt.Errorf("wsfed: reply URL is required")
	}
	var buf bytes.Buffer
	data := struct {
		ReplyURL template.URL
		Wresult  string
		Wctx     string
	}{
		ReplyURL: template.URL(replyURL), //nolint:gosec // 呼び出し側が許可集合に対して検証済み (ValidateSignIn)。
		Wresult:  wresult,
		Wctx:     wctx,
	}
	if err := passiveForm.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("wsfed: render passive form: %w", err)
	}
	return buf.Bytes(), nil
}
