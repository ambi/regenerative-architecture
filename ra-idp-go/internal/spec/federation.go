package spec

import "time"

// Federation bounded context の双子定義 (ADR-059)。
//
// WS-Federation / WS-Trust / SAML が共有する宣言的 claim mapping と、claim 発行
// エンジンの出力を表す。XML 署名・assertion 直列化は本ファイルの範囲外で、これらは
// プロトコル非依存の構造化中間表現に留まる。

// ClaimMappingSource は claim 値の供給元を表す (ADR-059)。
type ClaimMappingSource string

const (
	// ClaimSourceUserAttribute は identity principal の属性を供給元とする。
	ClaimSourceUserAttribute ClaimMappingSource = "user_attribute"
	// ClaimSourceFixed は静的な固定値を供給元とする。
	ClaimSourceFixed ClaimMappingSource = "fixed"
	// ClaimSourceNameID は解決済み NameID 値を claim にも反映する。
	ClaimSourceNameID ClaimMappingSource = "nameid"
)

func (s ClaimMappingSource) Valid() bool {
	switch s {
	case ClaimSourceUserAttribute, ClaimSourceFixed, ClaimSourceNameID:
		return true
	}
	return false
}

// ClaimMappingRule は 1 つの出力 claim の宣言的 mapping (ADR-059)。
type ClaimMappingRule struct {
	ClaimType  string             `json:"claim_type"`
	Source     ClaimMappingSource `json:"source"`
	SourceKey  string             `json:"source_key,omitempty"`
	FixedValue string             `json:"fixed_value,omitempty"`
	Required   bool               `json:"required,omitempty"`
}

// NameIdConfiguration は発行 assertion の NameID の format と供給元 (ADR-059)。
type NameIdConfiguration struct {
	Format          string `json:"format"`
	SourceAttribute string `json:"source_attribute"`
}

// ClaimMappingPolicy は RP/SP trust ごとの claim 発行規則一式 (ADR-059)。
type ClaimMappingPolicy struct {
	NameID NameIdConfiguration `json:"name_id"`
	Rules  []ClaimMappingRule  `json:"rules,omitempty"`
}

// IssuedClaim は claim 発行エンジンの出力。1 つの claim 型と値群 (ADR-059)。
type IssuedClaim struct {
	ClaimType string   `json:"claim_type"`
	Values    []string `json:"values"`
}

// WsFedTokenType は発行 assertion の SAML バージョン (wi-61)。RSTR の TokenType にもなる。
type WsFedTokenType string

const (
	// TokenTypeSAML11 は SAML 1.1 assertion。Entra / AD FS の WS-Federation 既定。
	TokenTypeSAML11 WsFedTokenType = "urn:oasis:names:tc:SAML:1.0:assertion"
	// TokenTypeSAML20 は SAML 2.0 assertion。
	TokenTypeSAML20 WsFedTokenType = "urn:oasis:names:tc:SAML:2.0:assertion"
)

func (t WsFedTokenType) Valid() bool {
	switch t {
	case TokenTypeSAML11, TokenTypeSAML20:
		return true
	}
	return false
}

// WsFedRelyingParty は WS-Federation passive の relying party 登録 (ADR-059)。
// wtrealm で識別し、許可 wreply の閉集合・audience・token type・claim policy を束ねる。
type WsFedRelyingParty struct {
	TenantID    string             `json:"tenant_id"`
	Wtrealm     string             `json:"wtrealm"`
	DisplayName string             `json:"display_name,omitempty"`
	ReplyURLs   []string           `json:"reply_urls"`
	Audience    string             `json:"audience,omitempty"`
	TokenType   WsFedTokenType     `json:"token_type,omitempty"`
	ClaimPolicy ClaimMappingPolicy `json:"claim_policy"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   *time.Time         `json:"updated_at,omitempty"`
}

// EffectiveAudience は assertion に用いる audience を返す。未設定なら wtrealm。
func (rp WsFedRelyingParty) EffectiveAudience() string {
	if rp.Audience != "" {
		return rp.Audience
	}
	return rp.Wtrealm
}

// EffectiveTokenType は発行する assertion の SAML バージョンを返す。未設定は SAML 1.1
// (Entra WS-Fed 互換の既定)。
func (rp WsFedRelyingParty) EffectiveTokenType() WsFedTokenType {
	if rp.TokenType == TokenTypeSAML20 {
		return TokenTypeSAML20
	}
	return TokenTypeSAML11
}

// WsFedSignInIssued は WS-Federation passive sign-in で assertion を発行した event (wi-61)。
type WsFedSignInIssued struct {
	At       time.Time `json:"-"`
	TenantID string    `json:"tenantId"`
	Wtrealm  string    `json:"wtrealm"`
	Sub      string    `json:"sub"`
}

func (e *WsFedSignInIssued) EventType() string     { return "WsFedSignInIssued" }
func (e *WsFedSignInIssued) OccurredAt() time.Time { return e.At }

// WsFedSignInRejected は WS-Federation passive sign-in を拒否した event (wi-61)。
type WsFedSignInRejected struct {
	At       time.Time `json:"-"`
	TenantID string    `json:"tenantId"`
	Wtrealm  string    `json:"wtrealm,omitempty"`
	Reason   string    `json:"reason"`
}

func (e *WsFedSignInRejected) EventType() string     { return "WsFedSignInRejected" }
func (e *WsFedSignInRejected) OccurredAt() time.Time { return e.At }

// WsFedSignOut は WS-Federation sign-out でローカルセッションを破棄した event (wi-61)。
type WsFedSignOut struct {
	At       time.Time `json:"-"`
	TenantID string    `json:"tenantId"`
	Wtrealm  string    `json:"wtrealm,omitempty"`
}

func (e *WsFedSignOut) EventType() string     { return "WsFedSignOut" }
func (e *WsFedSignOut) OccurredAt() time.Time { return e.At }
