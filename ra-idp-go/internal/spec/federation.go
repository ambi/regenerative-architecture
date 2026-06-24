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

// WsFedRelyingParty は WS-Federation passive の relying party 登録 (ADR-059)。
// wtrealm で識別し、許可 wreply の閉集合・audience・claim policy を束ねる。
type WsFedRelyingParty struct {
	TenantID    string             `json:"tenant_id"`
	Wtrealm     string             `json:"wtrealm"`
	DisplayName string             `json:"display_name,omitempty"`
	ReplyURLs   []string           `json:"reply_urls"`
	Audience    string             `json:"audience,omitempty"`
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
