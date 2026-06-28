// Package wstrust は WS-Trust 1.3 active requestor の SOAP wire adapter。
package wstrust

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/beevik/etree"
)

const (
	NSAddressing = "http://www.w3.org/2005/08/addressing"
	NSTrust13    = "http://docs.oasis-open.org/ws-sx/ws-trust/200512"
	NSPolicy     = "http://schemas.xmlsoap.org/ws/2004/09/policy"
	NSSOAP12     = "http://www.w3.org/2003/05/soap-envelope"
	NSWSU        = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"

	RequestIssue  = NSTrust13 + "/Issue"
	KeyTypeBearer = "http://docs.oasis-open.org/ws-sx/ws-trust/200512/Bearer"
	xmlDateTime   = "2006-01-02T15:04:05Z"
)

// RequestSecurityToken は Issue binding で必要な RST 要素だけを正規化した中間表現。
type RequestSecurityToken struct {
	MessageID   string
	To          string
	Action      string
	Username    string
	Password    string
	Created     time.Time
	Expires     time.Time
	AppliesTo   string
	TokenType   string
	RequestType string
	KeyType     string
}

// ParseRST は SOAP + WS-Addressing + WS-Security UsernameToken の RST を解析する。
func ParseRST(body []byte, now time.Time) (RequestSecurityToken, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(body); err != nil {
		return RequestSecurityToken{}, fmt.Errorf("wstrust: parse SOAP: %w", err)
	}
	root := doc.Root()
	if root == nil {
		return RequestSecurityToken{}, fmt.Errorf("wstrust: SOAP envelope is required")
	}

	req := RequestSecurityToken{
		MessageID: strings.TrimSpace(textAt(root, "Header", "MessageID")),
		To:        strings.TrimSpace(textAt(root, "Header", "To")),
		Action:    strings.TrimSpace(textAt(root, "Header", "Action")),
		Username:  strings.TrimSpace(textAt(root, "Header", "Security", "UsernameToken", "Username")),
		Password:  textAt(root, "Header", "Security", "UsernameToken", "Password"),
	}
	created := strings.TrimSpace(textAt(root, "Header", "Security", "Timestamp", "Created"))
	expires := strings.TrimSpace(textAt(root, "Header", "Security", "Timestamp", "Expires"))
	var err error
	if req.Created, err = parseTime(created, "Created"); err != nil {
		return RequestSecurityToken{}, err
	}
	if req.Expires, err = parseTime(expires, "Expires"); err != nil {
		return RequestSecurityToken{}, err
	}
	rst := find(root, "Body", "RequestSecurityToken")
	if rst == nil {
		return RequestSecurityToken{}, fmt.Errorf("wstrust: RequestSecurityToken is required")
	}
	req.RequestType = strings.TrimSpace(textAt(rst, "RequestType"))
	req.TokenType = strings.TrimSpace(textAt(rst, "TokenType"))
	req.KeyType = strings.TrimSpace(textAt(rst, "KeyType"))
	req.AppliesTo = strings.TrimSpace(textAt(rst, "AppliesTo", "EndpointReference", "Address"))
	if req.AppliesTo == "" {
		req.AppliesTo = strings.TrimSpace(textAt(rst, "AppliesTo", "Address"))
	}
	if err := req.Validate(now); err != nil {
		return RequestSecurityToken{}, err
	}
	return req, nil
}

// Validate は WS-Addressing / WS-Security の必須要素と Timestamp を fail-closed に検証する。
func (r RequestSecurityToken) Validate(now time.Time) error {
	switch {
	case r.MessageID == "":
		return fmt.Errorf("wstrust: MessageID is required")
	case r.To == "":
		return fmt.Errorf("wstrust: To is required")
	case r.Action == "":
		return fmt.Errorf("wstrust: Action is required")
	case r.Action != RequestIssue:
		return fmt.Errorf("wstrust: unsupported Action %q", r.Action)
	case r.Username == "":
		return fmt.Errorf("wstrust: UsernameToken Username is required")
	case r.Password == "":
		return fmt.Errorf("wstrust: UsernameToken Password is required")
	case r.AppliesTo == "":
		return fmt.Errorf("wstrust: AppliesTo is required")
	case !r.Expires.After(r.Created):
		return fmt.Errorf("wstrust: Timestamp Expires must be after Created")
	case now.Before(r.Created.Add(-5 * time.Minute)):
		return fmt.Errorf("wstrust: Timestamp Created is in the future")
	case !now.Before(r.Expires):
		return fmt.Errorf("wstrust: Timestamp is expired")
	}
	if r.RequestType != "" && r.RequestType != RequestIssue {
		return fmt.Errorf("wstrust: unsupported RequestType %q", r.RequestType)
	}
	if r.KeyType != "" && r.KeyType != KeyTypeBearer {
		return fmt.Errorf("wstrust: unsupported KeyType %q", r.KeyType)
	}
	return nil
}

func parseTime(value, field string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, fmt.Errorf("wstrust: Timestamp %s is required", field)
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("wstrust: Timestamp %s is invalid", field)
	}
	return parsed.UTC(), nil
}

func find(root *etree.Element, path ...string) *etree.Element {
	current := root
	for _, name := range path {
		current = childByLocal(current, name)
		if current == nil {
			return nil
		}
	}
	return current
}

func textAt(root *etree.Element, path ...string) string {
	el := find(root, path...)
	if el == nil {
		return ""
	}
	return el.Text()
}

func childByLocal(parent *etree.Element, local string) *etree.Element {
	for _, child := range parent.ChildElements() {
		if child.Tag == local || strings.HasSuffix(child.Tag, ":"+local) {
			return child
		}
	}
	return nil
}

// BuildRSTR は署名済み assertion を SOAP 1.2 の RSTR に包む。
func BuildRSTR(signedAssertion *etree.Element, relatesTo, appliesTo, tokenType string, created, expires time.Time) ([]byte, error) {
	if signedAssertion == nil {
		return nil, fmt.Errorf("wstrust: signed assertion is required")
	}
	if strings.TrimSpace(appliesTo) == "" {
		return nil, fmt.Errorf("wstrust: AppliesTo is required")
	}

	doc := etree.NewDocument()
	env := doc.CreateElement("s:Envelope")
	env.CreateAttr("xmlns:s", NSSOAP12)
	env.CreateAttr("xmlns:a", NSAddressing)
	env.CreateAttr("xmlns:t", NSTrust13)
	env.CreateAttr("xmlns:wsp", NSPolicy)
	env.CreateAttr("xmlns:wsu", NSWSU)

	header := env.CreateElement("s:Header")
	header.CreateElement("a:Action").SetText(NSTrust13 + "/RSTRC/IssueFinal")
	if strings.TrimSpace(relatesTo) != "" {
		header.CreateElement("a:RelatesTo").SetText(relatesTo)
	}

	body := env.CreateElement("s:Body")
	collection := body.CreateElement("t:RequestSecurityTokenResponseCollection")
	rstr := collection.CreateElement("t:RequestSecurityTokenResponse")
	lifetime := rstr.CreateElement("t:Lifetime")
	lifetime.CreateElement("wsu:Created").SetText(created.UTC().Format(xmlDateTime))
	lifetime.CreateElement("wsu:Expires").SetText(expires.UTC().Format(xmlDateTime))
	applies := rstr.CreateElement("wsp:AppliesTo")
	epr := applies.CreateElement("a:EndpointReference")
	epr.CreateElement("a:Address").SetText(appliesTo)
	requested := rstr.CreateElement("t:RequestedSecurityToken")
	requested.AddChild(signedAssertion.Copy())
	if strings.TrimSpace(tokenType) != "" {
		rstr.CreateElement("t:TokenType").SetText(tokenType)
	}
	rstr.CreateElement("t:RequestType").SetText(RequestIssue)
	rstr.CreateElement("t:KeyType").SetText(KeyTypeBearer)

	doc.Indent(2)
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("wstrust: serialize RSTR: %w", err)
	}
	return buf.Bytes(), nil
}
