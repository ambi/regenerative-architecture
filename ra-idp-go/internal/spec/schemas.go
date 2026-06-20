package spec

import (
	"fmt"
	"time"
)

const DefaultTenantID = "default"

type Tenant struct {
	ID                     string                  `json:"id"`
	DisplayName            string                  `json:"display_name"`
	Status                 TenantStatus            `json:"status"`
	PasswordPolicyOverride *PasswordPolicyOverride `json:"password_policy_override,omitempty"`
	CreatedAt              time.Time               `json:"created_at"`
	UpdatedAt              *time.Time              `json:"updated_at,omitempty"`
	DisabledAt             *time.Time              `json:"disabled_at,omitempty"`
}

func (t Tenant) Validate() error {
	return validate(tenantSchema, &t)
}

// PasswordPolicyOverride はテナント固有の objectives.PasswordPolicy 上書き値。
// SCL `PasswordPolicyOverride` の双子定義。省略フィールドは global default を継承する。
type PasswordPolicyOverride struct {
	MinLength    *int `json:"min_length,omitempty"`
	MaxLength    *int `json:"max_length,omitempty"`
	HistoryDepth *int `json:"history_depth,omitempty"`
}

// ===============================================================
// クライアント
// ===============================================================

type Client struct {
	TenantID                           string                  `json:"tenant_id"`
	ClientID                           string                  `json:"client_id"`
	ClientSecretHash                   *string                 `json:"client_secret_hash,omitempty"`
	ClientName                         *string                 `json:"client_name,omitempty"`
	ClientType                         ClientType              `json:"client_type"`
	RedirectURIs                       []string                `json:"redirect_uris"`
	GrantTypes                         []GrantType             `json:"grant_types"`
	ResponseTypes                      []ResponseType          `json:"response_types"`
	TokenEndpointAuthMethod            TokenEndpointAuthMethod `json:"token_endpoint_auth_method"`
	Scope                              string                  `json:"scope"`
	JWKS                               map[string]any          `json:"jwks,omitempty"`
	JwksURI                            *string                 `json:"jwks_uri,omitempty"`
	TlsClientAuthSubjectDN             *string                 `json:"tls_client_auth_subject_dn,omitempty"`
	IDTokenSignedResponseAlg           SignatureAlgorithm      `json:"id_token_signed_response_alg"`
	RequirePushedAuthorizationRequests bool                    `json:"require_pushed_authorization_requests"`
	DpopBoundAccessTokens              bool                    `json:"dpop_bound_access_tokens"`
	FapiProfile                        FapiProfile             `json:"fapi_profile"`
	CreatedAt                          time.Time               `json:"created_at"`
}

func (c Client) Validate() error {
	return validate(clientSchema, &c)
}

func hasJWKs(jwks map[string]any) bool {
	switch keys := jwks["keys"].(type) {
	case []any:
		return len(keys) > 0
	case []map[string]any:
		return len(keys) > 0
	default:
		return false
	}
}

// ===============================================================
// ユーザー
// ===============================================================

// User は IdP の最小コア。識別・認証・表示名・RBAC・ライフサイクルだけを型付きで
// 持ち、それ以外のプロフィール属性 (OIDC §5.1 optional claim / 組織属性 / tenant
// 定義 custom) は Attributes に sparse に格納する (ADR-039)。テナントは実際に使う
// 属性しか持たないため、固定カラム/フィールドの肥大を避けられる。
type User struct {
	Sub               string                    `json:"sub"`
	TenantID          string                    `json:"tenant_id"`
	PreferredUsername string                    `json:"preferred_username"`
	PasswordHash      string                    `json:"password_hash"`
	Name              *string                   `json:"name,omitempty"`
	GivenName         *string                   `json:"given_name,omitempty"`
	FamilyName        *string                   `json:"family_name,omitempty"`
	Email             *string                   `json:"email,omitempty"`
	EmailVerified     bool                      `json:"email_verified"`
	MfaEnrolled       bool                      `json:"mfa_enrolled"`
	Roles             []string                  `json:"roles"`
	Lifecycle         UserLifecycle             `json:"lifecycle"`
	Attributes        map[string]AttributeValue `json:"attributes,omitempty"`
	CreatedAt         time.Time                 `json:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at"`
}

func (u User) Validate() error {
	if err := validate(userSchema, &u); err != nil {
		return err
	}
	if err := u.Lifecycle.Validate(); err != nil {
		return err
	}
	for key, v := range u.Attributes {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("attribute %q: %w", key, err)
		}
	}
	return nil
}

// IsDeleted は User が ADR-036 の tombstone 状態 (status == Deleted) かを返す。
func (u User) IsDeleted() bool { return u.Lifecycle.EffectiveStatus() == UserStatusDeleted }

// IsActive は認証を許可してよい状態 (status == Active) かを返す。
// Disabled / Locked / Staged / Suspended / Deleted はすべて非アクティブ。
func (u User) IsActive() bool { return u.Lifecycle.EffectiveStatus() == UserStatusActive }

// ===============================================================
// ユーザーライフサイクル / 属性 (wi-19 / ADR-039 / ADR-040)
// ===============================================================

// UserLifecycle は User の運用ライフサイクル属性。status は状態機械 UserLifecycle
// (states セクション) と一致する唯一の真実。status_changed_at が現在の状態に
// なった時刻 (旧 disabled_at / deleted_at を統合)。
type UserLifecycle struct {
	Status            UserStatus       `json:"status"`
	StatusChangedAt   *time.Time       `json:"status_changed_at,omitempty"`
	LastLoginAt       *time.Time       `json:"last_login_at,omitempty"`
	PasswordChangedAt *time.Time       `json:"password_changed_at,omitempty"`
	RequiredActions   []RequiredAction `json:"required_actions,omitempty"`
}

// EffectiveStatus は未設定 (zero-value) を既定の Active として解決する。
// SCL の `status: { default: Active }` と整合し、新規構築 User を Active 扱いにする。
func (l UserLifecycle) EffectiveStatus() UserStatus {
	if l.Status == "" {
		return UserStatusActive
	}
	return l.Status
}

func (l UserLifecycle) Validate() error {
	if l.Status != "" && !l.Status.Valid() {
		return fmt.Errorf("user status %q is not in enum", l.Status)
	}
	for _, a := range l.RequiredActions {
		if !a.Valid() {
			return fmt.Errorf("required action %q is not in enum", a)
		}
	}
	return nil
}

// AttributeValue は属性 1 件の値 (sum type)。Type が示すフィールドだけが設定される。
// OIDC 標準クレームの組み込み属性と tenant 定義 custom 属性で共通 (ADR-040)。
type AttributeValue struct {
	Type        AttributeType `json:"type"`
	String      *string       `json:"string,omitempty"`
	Number      *float64      `json:"number,omitempty"`
	Boolean     *bool         `json:"boolean,omitempty"`
	Date        *string       `json:"date,omitempty"` // ISO 8601 date
	StringArray []string      `json:"string_array,omitempty"`
}

func (v AttributeValue) Validate() error {
	if !v.Type.Valid() {
		return fmt.Errorf("attribute type %q is not in enum", v.Type)
	}
	set := 0
	matches := false
	check := func(present bool, t AttributeType) {
		if present {
			set++
			if v.Type == t {
				matches = true
			}
		}
	}
	check(v.String != nil, AttributeTypeString)
	check(v.Number != nil, AttributeTypeNumber)
	check(v.Boolean != nil, AttributeTypeBoolean)
	check(v.Date != nil, AttributeTypeDate)
	check(v.StringArray != nil, AttributeTypeStringArray)
	if set != 1 || !matches {
		return fmt.Errorf("attribute value must set exactly the one field matching type %q", v.Type)
	}
	return nil
}

// JSONValue は属性値を OIDC claim へ載せる JSON ネイティブ値に変換する。型と
// 中身が食い違う / 値が無い場合は nil を返し、呼び出し側が claim を省略できる。
func (v AttributeValue) JSONValue() any {
	switch v.Type {
	case AttributeTypeString:
		if v.String != nil {
			return *v.String
		}
	case AttributeTypeNumber:
		if v.Number != nil {
			return *v.Number
		}
	case AttributeTypeBoolean:
		if v.Boolean != nil {
			return *v.Boolean
		}
	case AttributeTypeDate:
		if v.Date != nil {
			return *v.Date
		}
	case AttributeTypeStringArray:
		if v.StringArray != nil {
			return v.StringArray
		}
	}
	return nil
}

// UserAttributeDef は属性 1 件の定義 (ADR-040)。OIDC 組み込みカタログ
// (BuiltinUserAttributeDefs) と tenant 定義 (TenantUserAttributeSchema) の両方で使う。
type UserAttributeDef struct {
	Key            string         `json:"key"`
	Type           AttributeType  `json:"type"`
	MultiValued    bool           `json:"multi_valued"`
	Required       bool           `json:"required"`
	EditableByUser bool           `json:"editable_by_user"`
	ClaimName      *string        `json:"claim_name,omitempty"` // OIDC claim 名 (露出時)
	OIDCScope      *string        `json:"oidc_scope,omitempty"` // 露出を解禁する OIDC scope
	Visibility     AttrVisibility `json:"visibility"`
	PII            bool           `json:"pii"` // 省略時は PII 扱い (hash 化) が安全側 default
}

func (d UserAttributeDef) Validate() error { return validate(userAttributeDefSchema, &d) }

// TenantUserAttributeSchema は tenant 単位の custom 属性定義集合 (ADR-040)。
// 組み込み属性は BuiltinUserAttributeDefs() がコードで持ち、本集合は tenant 固有分のみ。
// tenant 削除時に cascade する。
type TenantUserAttributeSchema struct {
	TenantID   string             `json:"tenant_id"`
	Attributes []UserAttributeDef `json:"attributes"`
	UpdatedAt  time.Time          `json:"updated_at"`
}

func (s TenantUserAttributeSchema) Validate() error {
	builtin := map[string]bool{}
	for _, d := range BuiltinUserAttributeDefs() {
		builtin[d.Key] = true
	}
	seen := map[string]bool{}
	for _, d := range s.Attributes {
		if err := d.Validate(); err != nil {
			return err
		}
		if builtin[d.Key] {
			return fmt.Errorf("custom attribute %q collides with a builtin attribute", d.Key)
		}
		if seen[d.Key] {
			return fmt.Errorf("duplicate custom attribute key %q", d.Key)
		}
		seen[d.Key] = true
	}
	return nil
}

// EffectiveDefs は組み込み属性 + tenant custom 属性を結合した実効定義を返す。
func (s TenantUserAttributeSchema) EffectiveDefs() []UserAttributeDef {
	defs := BuiltinUserAttributeDefs()
	return append(defs, s.Attributes...)
}

// ValidateAttributes は User.Attributes を実効属性定義に対して検証する。
// 未定義 key の拒否、型の一致、multi_valued の整合、required の充足を見る。
func ValidateAttributes(values map[string]AttributeValue, defs []UserAttributeDef) error {
	byKey := make(map[string]UserAttributeDef, len(defs))
	for _, d := range defs {
		byKey[d.Key] = d
	}
	for key, v := range values {
		def, ok := byKey[key]
		if !ok {
			return fmt.Errorf("attribute %q is not defined", key)
		}
		if err := v.Validate(); err != nil {
			return fmt.Errorf("attribute %q: %w", key, err)
		}
		if v.Type != def.Type {
			return fmt.Errorf("attribute %q expects type %q, got %q", key, def.Type, v.Type)
		}
		if def.MultiValued != (def.Type == AttributeTypeStringArray) {
			return fmt.Errorf("attribute %q multi_valued/type mismatch", key)
		}
	}
	for _, def := range defs {
		if def.Required {
			if _, ok := values[def.Key]; !ok {
				return fmt.Errorf("required attribute %q is missing", def.Key)
			}
		}
	}
	return nil
}

type MfaFactor struct {
	Sub        string        `json:"sub"`
	Type       MfaFactorType `json:"type"`
	Secret     *string       `json:"secret,omitempty"`
	Label      *string       `json:"label,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
	LastUsedAt *time.Time    `json:"last_used_at,omitempty"`
}

func (m MfaFactor) Validate() error {
	return validate(mfaFactorSchema, &m)
}

// ===============================================================
// コンセント
// ===============================================================

type Consent struct {
	TenantID  string       `json:"tenant_id"`
	Sub       string       `json:"sub"`
	ClientID  string       `json:"client_id"`
	Scopes    []string     `json:"scopes"`
	State     ConsentState `json:"state"`
	GrantedAt time.Time    `json:"granted_at"`
	ExpiresAt time.Time    `json:"expires_at"`
	RevokedAt *time.Time   `json:"revoked_at,omitempty"`
}

func (c Consent) Validate() error {
	return validate(consentSchema, &c)
}

// ===============================================================
// 認可リクエスト
// ===============================================================

type AuthorizationRequest struct {
	ID                  string                     `json:"id"`
	TenantID            string                     `json:"tenant_id"`
	State               AuthorizationCodeFlowState `json:"state"`
	ClientID            string                     `json:"client_id"`
	RedirectURI         string                     `json:"redirect_uri"`
	ResponseType        ResponseType               `json:"response_type"`
	Scope               string                     `json:"scope"`
	StateParam          *string                    `json:"state_param,omitempty"`
	Nonce               *string                    `json:"nonce,omitempty"`
	CodeChallenge       string                     `json:"code_challenge"`
	CodeChallengeMethod CodeChallengeMethod        `json:"code_challenge_method"`
	Prompt              *string                    `json:"prompt,omitempty"`
	MaxAge              *int                       `json:"max_age,omitempty"`
	ParRequestURI       *string                    `json:"par_request_uri,omitempty"`
	Sub                 *string                    `json:"sub,omitempty"`
	AuthTime            *int64                     `json:"auth_time,omitempty"`
	AMR                 []string                   `json:"amr,omitempty"`
	ACR                 *string                    `json:"acr,omitempty"`
	ACRValues           *string                    `json:"acr_values,omitempty"`
	CreatedAt           time.Time                  `json:"created_at"`
	ExpiresAt           time.Time                  `json:"expires_at"`
}

func (a AuthorizationRequest) Validate() error {
	return validate(authorizationRequestSchema, &a)
}

// ===============================================================
// 認可コードレコード
// ===============================================================

type AuthorizationCodeRecord struct {
	Code                   string                       `json:"code"`
	TenantID               string                       `json:"tenant_id"`
	AuthorizationRequestID string                       `json:"authorization_request_id"`
	ClientID               string                       `json:"client_id"`
	Sub                    string                       `json:"sub"`
	Scopes                 []string                     `json:"scopes"`
	RedirectURI            string                       `json:"redirect_uri"`
	CodeChallenge          string                       `json:"code_challenge"`
	CodeChallengeMethod    CodeChallengeMethod          `json:"code_challenge_method"`
	Nonce                  *string                      `json:"nonce,omitempty"`
	AuthTime               int64                        `json:"auth_time"`
	AMR                    []string                     `json:"amr,omitempty"`
	ACR                    *string                      `json:"acr,omitempty"`
	State                  AuthorizationCodeRecordState `json:"state"`
	IssuedAt               time.Time                    `json:"issued_at"`
	ExpiresAt              time.Time                    `json:"expires_at"`
	RedeemedAt             *time.Time                   `json:"redeemed_at,omitempty"`
	IssuedFamilyID         *string                      `json:"issued_family_id,omitempty"`
}

func (a AuthorizationCodeRecord) Validate() error {
	return validate(authorizationCodeRecordSchema, &a)
}

// ===============================================================
// ログインセッション / ログインリクエスト
// ===============================================================

type LoginSession struct {
	ID                    string    `json:"id"`
	TenantID              string    `json:"tenant_id"`
	Sub                   string    `json:"sub"`
	AuthTime              int64     `json:"auth_time"`
	AMR                   []string  `json:"amr"`
	ACR                   string    `json:"acr"`
	AuthenticationPending bool      `json:"authentication_pending"`
	ExpiresAt             time.Time `json:"expires_at"`
}

func (s LoginSession) Validate() error {
	return validate(loginSessionSchema, &s)
}

type LoginRequest struct {
	RequestID string `json:"request_id"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Csrf      string `json:"csrf"`
}

func (r LoginRequest) Validate() error {
	return validate(loginRequestSchema, &r)
}

// ===============================================================
// SenderConstraint (DPoP / mTLS)
// ===============================================================

type SenderConstraint struct {
	Type    SenderConstraintType `json:"type"`
	JKT     string               `json:"jkt,omitempty"`
	X5TS256 string               `json:"x5t#S256,omitempty"`
}

// ===============================================================
// リフレッシュトークン (ストアレコード)
// ===============================================================

type RefreshTokenRecord struct {
	ID                string            `json:"id"`
	TenantID          string            `json:"tenant_id"`
	Hash              string            `json:"hash"`
	FamilyID          string            `json:"family_id"`
	ParentID          *string           `json:"parent_id,omitempty"`
	ClientID          string            `json:"client_id"`
	Sub               string            `json:"sub"`
	Scopes            []string          `json:"scopes"`
	IssuedAt          time.Time         `json:"issued_at"`
	ExpiresAt         time.Time         `json:"expires_at"`
	AbsoluteExpiresAt time.Time         `json:"absolute_expires_at"`
	Revoked           bool              `json:"revoked"`
	Rotated           bool              `json:"rotated"`
	SenderConstraint  *SenderConstraint `json:"sender_constraint,omitempty"`
}

func (r RefreshTokenRecord) Validate() error {
	return validate(refreshTokenRecordSchema, &r)
}

// ===============================================================
// PAR (Pushed Authorization Request) レコード
// ===============================================================

type PARRecord struct {
	RequestURI string            `json:"request_uri"`
	TenantID   string            `json:"tenant_id"`
	ClientID   string            `json:"client_id"`
	Parameters map[string]string `json:"parameters"`
	IssuedAt   time.Time         `json:"issued_at"`
	ExpiresAt  time.Time         `json:"expires_at"`
	Used       bool              `json:"used"`
}

func (p PARRecord) Validate() error {
	return validate(parRecordSchema, &p)
}

// ===============================================================
// DeviceAuthorization (RFC 8628 ストアレコード)
// ===============================================================

type DeviceAuthorization struct {
	DeviceCodeHash  string              `json:"device_code_hash"`
	TenantID        string              `json:"tenant_id"`
	UserCode        string              `json:"user_code"`
	ClientID        string              `json:"client_id"`
	Scopes          []string            `json:"scopes"`
	State           DeviceCodeFlowState `json:"state"`
	Sub             *string             `json:"sub,omitempty"`
	AuthTime        *int64              `json:"auth_time,omitempty"`
	IntervalSeconds int                 `json:"interval_seconds"`
	LastPolledAt    *time.Time          `json:"last_polled_at,omitempty"`
	IssuedFamilyID  *string             `json:"issued_family_id,omitempty"`
	IssuedAt        time.Time           `json:"issued_at"`
	ExpiresAt       time.Time           `json:"expires_at"`
}

func (d DeviceAuthorization) Validate() error {
	return validate(deviceAuthorizationSchema, &d)
}

// ===============================================================
// アクセストークン / ID トークン クレーム
// ===============================================================

type AccessTokenClaims struct {
	Issuer   string            `json:"iss"`
	Subject  string            `json:"sub"`
	Audience any               `json:"aud"`
	ClientID string            `json:"client_id"`
	Scope    string            `json:"scope"`
	Exp      int64             `json:"exp"`
	Iat      int64             `json:"iat"`
	Nbf      int64             `json:"nbf,omitempty"`
	JTI      string            `json:"jti"`
	AuthTime int64             `json:"auth_time,omitempty"`
	ACR      string            `json:"acr,omitempty"`
	AMR      []string          `json:"amr,omitempty"`
	CNF      map[string]string `json:"cnf,omitempty"`
}

type IDTokenClaims struct {
	Issuer            string   `json:"iss"`
	Subject           string   `json:"sub"`
	Audience          any      `json:"aud"`
	Exp               int64    `json:"exp"`
	Iat               int64    `json:"iat"`
	AuthTime          int64    `json:"auth_time"`
	Nonce             string   `json:"nonce,omitempty"`
	ACR               string   `json:"acr,omitempty"`
	AMR               []string `json:"amr,omitempty"`
	AZP               string   `json:"azp,omitempty"`
	AtHash            string   `json:"at_hash,omitempty"`
	Name              string   `json:"name,omitempty"`
	PreferredUsername string   `json:"preferred_username,omitempty"`
	Email             string   `json:"email,omitempty"`
	EmailVerified     bool     `json:"email_verified,omitempty"`
}
