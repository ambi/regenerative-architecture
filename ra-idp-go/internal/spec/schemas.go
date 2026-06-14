package spec

import (
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

type User struct {
	Sub               string     `json:"sub"`
	TenantID          string     `json:"tenant_id"`
	PreferredUsername string     `json:"preferred_username"`
	PasswordHash      string     `json:"password_hash"`
	Name              *string    `json:"name,omitempty"`
	GivenName         *string    `json:"given_name,omitempty"`
	FamilyName        *string    `json:"family_name,omitempty"`
	Email             *string    `json:"email,omitempty"`
	EmailVerified     bool       `json:"email_verified"`
	MfaEnrolled       bool       `json:"mfa_enrolled"`
	Roles             []string   `json:"roles"`
	DisabledAt        *time.Time `json:"disabled_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"`
}

func (u User) Validate() error {
	return validate(userSchema, &u)
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
