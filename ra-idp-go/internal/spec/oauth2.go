package spec

// OAuth2 bounded context の双子定義。client / consent / authorization request /
// code / refresh / PAR / device / token claims と Rich Authorization Requests。

import "time"

type OAuth2Client struct {
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
	// FirstParty は IdP 自身が所有する信頼済みクライアント (管理コンソール /
	// アカウントポータル) を表す。resource owner が IdP 利用者自身であるため、
	// authorization_code フローで consent 画面をスキップする (ADR-061)。
	FirstParty bool      `json:"first_party"`
	CreatedAt  time.Time `json:"created_at"`
}

func (c OAuth2Client) Validate() error {
	return validate(oauth2ClientSchema, &c)
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
// コンセント
// ===============================================================

type Consent struct {
	TenantID             string                `json:"tenant_id"`
	Sub                  string                `json:"sub"`
	ClientID             string                `json:"client_id"`
	Scopes               []string              `json:"scopes"`
	State                ConsentState          `json:"state"`
	GrantedAt            time.Time             `json:"granted_at"`
	ExpiresAt            time.Time             `json:"expires_at"`
	RevokedAt            *time.Time            `json:"revoked_at,omitempty"`
	AuthorizationDetails []AuthorizationDetail `json:"authorization_details,omitempty"`
}

func (c Consent) Validate() error {
	return validate(consentSchema, &c)
}

// ===============================================================
// Rich Authorization Requests (RFC 9396 / ADR-050)
// ===============================================================

// AuthorizationDetail は RFC 9396 authorization_details の 1 要素。type で識別される
// 構造化された細粒度権限を表し、登録済み AuthorizationDetailType に対し fail-closed に検証する。
type AuthorizationDetail struct {
	Type       string         `json:"type"`
	Locations  []string       `json:"locations,omitempty"`
	Actions    []string       `json:"actions,omitempty"`
	Datatypes  []string       `json:"datatypes,omitempty"`
	Identifier string         `json:"identifier,omitempty"`
	Privileges []string       `json:"privileges,omitempty"`
	Fields     map[string]any `json:"fields,omitempty"`
}

// AuthorizationDetailFieldRule は登録スキーマの 1 フィールド規則。ダウンスコープ半順序と
// 許可値を定義する。
type AuthorizationDetailFieldRule struct {
	Name      string                            `json:"name"`
	Semantics AuthorizationDetailFieldSemantics `json:"semantics"`
	Required  bool                              `json:"required"`
	Allowed   []string                          `json:"allowed,omitempty"`
}

// AuthorizationDetailsSchema はある type の構造的スキーマ。受理するフィールドと
// 各フィールドの半順序を列挙する。
type AuthorizationDetailsSchema struct {
	Rules []AuthorizationDetailFieldRule `json:"rules"`
}

// AuthorizationDetailType はテナントが登録する authorization_details の type 定義 (ADR-050)。
type AuthorizationDetailType struct {
	TenantID        string                       `json:"tenant_id"`
	Type            string                       `json:"type"`
	Description     string                       `json:"description,omitempty"`
	Schema          AuthorizationDetailsSchema   `json:"schema"`
	DisplayTemplate string                       `json:"display_template"`
	State           AuthorizationDetailTypeState `json:"state"`
	CreatedAt       time.Time                    `json:"created_at"`
	UpdatedAt       time.Time                    `json:"updated_at"`
}

func (t AuthorizationDetailType) Validate() error {
	return validate(authorizationDetailTypeSchema, &t)
}

// ===============================================================
// 認可リクエスト
// ===============================================================

type AuthorizationRequest struct {
	ID                   string                     `json:"id"`
	TenantID             string                     `json:"tenant_id"`
	State                AuthorizationCodeFlowState `json:"state"`
	ClientID             string                     `json:"client_id"`
	RedirectURI          string                     `json:"redirect_uri"`
	ResponseType         ResponseType               `json:"response_type"`
	Scope                string                     `json:"scope"`
	StateParam           *string                    `json:"state_param,omitempty"`
	Nonce                *string                    `json:"nonce,omitempty"`
	CodeChallenge        string                     `json:"code_challenge"`
	CodeChallengeMethod  CodeChallengeMethod        `json:"code_challenge_method"`
	Prompt               *string                    `json:"prompt,omitempty"`
	MaxAge               *int                       `json:"max_age,omitempty"`
	ParRequestURI        *string                    `json:"par_request_uri,omitempty"`
	Sub                  *string                    `json:"sub,omitempty"`
	AuthTime             *int64                     `json:"auth_time,omitempty"`
	AMR                  []string                   `json:"amr,omitempty"`
	ACR                  *string                    `json:"acr,omitempty"`
	ACRValues            *string                    `json:"acr_values,omitempty"`
	AuthorizationDetails []AuthorizationDetail      `json:"authorization_details,omitempty"`
	CreatedAt            time.Time                  `json:"created_at"`
	ExpiresAt            time.Time                  `json:"expires_at"`
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
	AuthorizationDetails   []AuthorizationDetail        `json:"authorization_details,omitempty"`
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
	Issuer               string                `json:"iss"`
	Subject              string                `json:"sub"`
	Audience             any                   `json:"aud"`
	ClientID             string                `json:"client_id"`
	Scope                string                `json:"scope"`
	Exp                  int64                 `json:"exp"`
	Iat                  int64                 `json:"iat"`
	Nbf                  int64                 `json:"nbf,omitempty"`
	JTI                  string                `json:"jti"`
	AuthTime             int64                 `json:"auth_time,omitempty"`
	ACR                  string                `json:"acr,omitempty"`
	AMR                  []string              `json:"amr,omitempty"`
	CNF                  map[string]string     `json:"cnf,omitempty"`
	AuthorizationDetails []AuthorizationDetail `json:"authorization_details,omitempty"`
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
