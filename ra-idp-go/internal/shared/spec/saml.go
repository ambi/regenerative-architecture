package spec

import (
	"slices"
	"time"
)

// SAML 2.0 IdP の双子定義 (wi-29, ADR-067)。
//
// SamlServiceProvider は SAML 2.0 Web Browser SSO Profile の relying party (SP) 登録を表す。
// entityID で識別し、許可 AssertionConsumerService の閉集合・audience・NameID format・
// 署名方針・claim policy を束ねる。claim mapping (ADR-059) と SAML assertion 直列化
// (ADR-060) は WS-Federation / WS-Trust と共有する protocol-agnostic な仕組みを再利用する。

// SAML 2.0 binding URI。SP-initiated SSO / SLO で用いる。
const (
	// SamlBindingHTTPRedirect は HTTP-Redirect binding (GET, deflate+base64)。
	SamlBindingHTTPRedirect = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"
	// SamlBindingHTTPPOST は HTTP-POST binding (POST, base64, 自動 POST フォーム)。
	SamlBindingHTTPPOST = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
)

// SAML 2.0 NameID format。SP metadata と AuthnRequest の NameIDPolicy で用いる。
const (
	SamlNameIDFormatUnspecified  = "urn:oasis:names:tc:SAML:2.0:nameid-format:unspecified"
	SamlNameIDFormatEmailAddress = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
	SamlNameIDFormatPersistent   = "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"
	SamlNameIDFormatTransient    = "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"
)

// ValidSamlNameIDFormat は対応 NameID format かを返す。
func ValidSamlNameIDFormat(format string) bool {
	switch format {
	case SamlNameIDFormatUnspecified, SamlNameIDFormatEmailAddress, SamlNameIDFormatPersistent, SamlNameIDFormatTransient:
		return true
	}
	return false
}

// SamlServiceProvider は SAML 2.0 Web Browser SSO の SP 登録 (wi-29)。
// entityID で識別し、許可 ACS の閉集合・audience・NameID format・署名方針・claim policy を束ねる。
type SamlServiceProvider struct {
	TenantID    string             `json:"tenant_id"`
	EntityID    string             `json:"entity_id"`
	DisplayName string             `json:"display_name,omitempty"`
	ACSURLs     []string           `json:"acs_urls"`
	SLOURL      string             `json:"slo_url,omitempty"`
	Audience    string             `json:"audience,omitempty"`
	ClaimPolicy ClaimMappingPolicy `json:"claim_policy"`
	// SignAssertion は発行する <Assertion> を enveloped 署名する (既定 true)。
	SignAssertion bool `json:"sign_assertion"`
	// SignResponse は <Response> 全体を enveloped 署名する (Okta / Entra の "Sign Response")。
	SignResponse bool `json:"sign_response"`
	// WantAuthnRequestsSigned は将来の SP ごとの AuthnRequest 署名検証 policy 用予約フィールド。
	// true の場合は AuthnRequestSigningCertificatePEM で AuthnRequest / LogoutRequest を検証する。
	WantAuthnRequestsSigned           bool       `json:"want_authn_requests_signed,omitempty"`
	AuthnRequestSigningCertificatePEM string     `json:"authn_request_signing_certificate_pem,omitempty"`
	CreatedAt                         time.Time  `json:"created_at"`
	UpdatedAt                         *time.Time `json:"updated_at,omitempty"`
}

// EffectiveAudience は assertion の AudienceRestriction に用いる値を返す。未設定なら entityID。
func (sp SamlServiceProvider) EffectiveAudience() string {
	if sp.Audience != "" {
		return sp.Audience
	}
	return sp.EntityID
}

// DefaultACSURL は AuthnRequest が ACS を指定しないときの既定の返信先を返す。
func (sp SamlServiceProvider) DefaultACSURL() string {
	if len(sp.ACSURLs) == 0 {
		return ""
	}
	return sp.ACSURLs[0]
}

// AllowsACSURL は要求された ACS URL が登録済みの許可集合に含まれるかを返す (open redirect 防止)。
func (sp SamlServiceProvider) AllowsACSURL(acsURL string) bool {
	return slices.Contains(sp.ACSURLs, acsURL)
}

// SamlSignInIssued は SAML SSO で SAMLResponse を発行した event (wi-29)。
type SamlSignInIssued struct {
	At       time.Time `json:"-"`
	TenantID string    `json:"tenantId"`
	EntityID string    `json:"entityId"`
	Sub      string    `json:"sub"`
}

func (e *SamlSignInIssued) EventType() string     { return "SamlSignInIssued" }
func (e *SamlSignInIssued) OccurredAt() time.Time { return e.At }

// SamlSignInRejected は SAML SSO 要求を拒否した event (wi-29)。
type SamlSignInRejected struct {
	At       time.Time `json:"-"`
	TenantID string    `json:"tenantId"`
	EntityID string    `json:"entityId,omitempty"`
	Reason   string    `json:"reason"`
}

func (e *SamlSignInRejected) EventType() string     { return "SamlSignInRejected" }
func (e *SamlSignInRejected) OccurredAt() time.Time { return e.At }

// SamlLogout は SAML Single Logout でローカルセッションを破棄した event (wi-29)。
type SamlLogout struct {
	At       time.Time `json:"-"`
	TenantID string    `json:"tenantId"`
	EntityID string    `json:"entityId,omitempty"`
}

func (e *SamlLogout) EventType() string     { return "SamlLogout" }
func (e *SamlLogout) OccurredAt() time.Time { return e.At }
