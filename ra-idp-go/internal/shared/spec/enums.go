// Package spec: SCL → Go バインディング。
//
// 仕様本体（language-agnostic）は spec/scl.yaml。
// 本ファイルはランタイム検証のための Go バインディング。SCL を変更したら本ファイル
// も合わせて更新する。乖離は coherence test で検出する。
package spec

// ===============================================================
// SCL `models` セクションの enum を Go の typed string で表す。
// ワイヤ形式（snake_case）は vocabulary[].aliases[0] と同じ。
// ===============================================================

type ClientType string

const (
	ClientPublic       ClientType = "public"
	ClientConfidential ClientType = "confidential"
)

func (c ClientType) Valid() bool { return c == ClientPublic || c == ClientConfidential }

type GrantType string

const (
	GrantAuthorizationCode GrantType = "authorization_code"
	GrantRefreshToken      GrantType = "refresh_token"
	GrantClientCredentials GrantType = "client_credentials"
	GrantDeviceCode        GrantType = "urn:ietf:params:oauth:grant-type:device_code"
	GrantTokenExchange     GrantType = "urn:ietf:params:oauth:grant-type:token-exchange"
)

func (g GrantType) Valid() bool {
	switch g {
	case GrantAuthorizationCode, GrantRefreshToken, GrantClientCredentials, GrantDeviceCode, GrantTokenExchange:
		return true
	}
	return false
}

type ResponseType string

const ResponseTypeCode ResponseType = "code"

func (r ResponseType) Valid() bool { return r == ResponseTypeCode }

type TokenEndpointAuthMethod string

const (
	AuthMethodClientSecretBasic TokenEndpointAuthMethod = "client_secret_basic"
	AuthMethodClientSecretPost  TokenEndpointAuthMethod = "client_secret_post"
	AuthMethodPrivateKeyJwt     TokenEndpointAuthMethod = "private_key_jwt"
	AuthMethodTlsClientAuth     TokenEndpointAuthMethod = "tls_client_auth"
	AuthMethodNone              TokenEndpointAuthMethod = "none"
)

func (m TokenEndpointAuthMethod) Valid() bool {
	switch m {
	case AuthMethodClientSecretBasic, AuthMethodClientSecretPost,
		AuthMethodPrivateKeyJwt, AuthMethodTlsClientAuth, AuthMethodNone:
		return true
	}
	return false
}

type SignatureAlgorithm string

const (
	SigAlgPS256 SignatureAlgorithm = "PS256"
	SigAlgES256 SignatureAlgorithm = "ES256"
)

func (s SignatureAlgorithm) Valid() bool { return s == SigAlgPS256 || s == SigAlgES256 }

type FapiProfile string

const (
	FapiNone              FapiProfile = "none"
	FapiSecurityProfileV2 FapiProfile = "fapi_2_security_profile"
)

func (f FapiProfile) Valid() bool { return f == FapiNone || f == FapiSecurityProfileV2 }

type CodeChallengeMethod string

const CodeChallengeMethodS256 CodeChallengeMethod = "S256"

func (c CodeChallengeMethod) Valid() bool { return c == CodeChallengeMethodS256 }

type MfaFactorType string

const (
	MfaFactorTOTP     MfaFactorType = "totp"
	MfaFactorWebAuthn MfaFactorType = "webauthn"
	MfaFactorHWK      MfaFactorType = "hwk"
	MfaFactorSWK      MfaFactorType = "swk"
)

func (m MfaFactorType) Valid() bool {
	switch m {
	case MfaFactorTOTP, MfaFactorWebAuthn, MfaFactorHWK, MfaFactorSWK:
		return true
	}
	return false
}

// ===============================================================
// 状態機械 (SCL state_machines)
// ===============================================================

type AuthorizationCodeFlowState string

const (
	AuthFlowReceived              AuthorizationCodeFlowState = "received"
	AuthFlowAuthenticationPending AuthorizationCodeFlowState = "authentication_pending"
	AuthFlowAuthenticated         AuthorizationCodeFlowState = "authenticated"
	AuthFlowConsentPending        AuthorizationCodeFlowState = "consent_pending"
	AuthFlowConsented             AuthorizationCodeFlowState = "consented"
	AuthFlowCodeIssued            AuthorizationCodeFlowState = "code_issued"
	AuthFlowExchanged             AuthorizationCodeFlowState = "exchanged"
	AuthFlowRejected              AuthorizationCodeFlowState = "rejected"
	AuthFlowExpired               AuthorizationCodeFlowState = "expired"
)

func (s AuthorizationCodeFlowState) Valid() bool {
	switch s {
	case AuthFlowReceived, AuthFlowAuthenticationPending, AuthFlowAuthenticated,
		AuthFlowConsentPending, AuthFlowConsented, AuthFlowCodeIssued,
		AuthFlowExchanged, AuthFlowRejected, AuthFlowExpired:
		return true
	}
	return false
}

type AuthorizationCodeRecordState string

const (
	AuthCodeRecordIssued   AuthorizationCodeRecordState = "issued"
	AuthCodeRecordRedeemed AuthorizationCodeRecordState = "redeemed"
	AuthCodeRecordExpired  AuthorizationCodeRecordState = "expired"
)

func (s AuthorizationCodeRecordState) Valid() bool {
	switch s {
	case AuthCodeRecordIssued, AuthCodeRecordRedeemed, AuthCodeRecordExpired:
		return true
	}
	return false
}

type ConsentState string

const (
	ConsentGranted ConsentState = "granted"
	ConsentRevoked ConsentState = "revoked"
	ConsentExpired ConsentState = "expired"
)

func (s ConsentState) Valid() bool {
	switch s {
	case ConsentGranted, ConsentRevoked, ConsentExpired:
		return true
	}
	return false
}

// AuthorizationDetailFieldSemantics は authorization_details の登録スキーマで
// 各フィールドが担うダウンスコープ半順序を表す (RFC 9396 / ADR-050)。
type AuthorizationDetailFieldSemantics string

const (
	DetailFieldSet    AuthorizationDetailFieldSemantics = "set"     // 集合包含: 要求 ⊆ 同意/元
	DetailFieldAtMost AuthorizationDetailFieldSemantics = "at_most" // 単調減少: 要求 ≤ 同意/元
	DetailFieldEnum   AuthorizationDetailFieldSemantics = "enum"    // 許可列挙からの完全一致
	DetailFieldExact  AuthorizationDetailFieldSemantics = "exact"   // 不透明値の完全一致
)

func (s AuthorizationDetailFieldSemantics) Valid() bool {
	switch s {
	case DetailFieldSet, DetailFieldAtMost, DetailFieldEnum, DetailFieldExact:
		return true
	}
	return false
}

// AuthorizationDetailTypeState は登録 type の運用状態を表す (ADR-050)。
type AuthorizationDetailTypeState string

const (
	DetailTypeEnabled  AuthorizationDetailTypeState = "Enabled"
	DetailTypeDisabled AuthorizationDetailTypeState = "Disabled"
)

func (s AuthorizationDetailTypeState) Valid() bool {
	switch s {
	case DetailTypeEnabled, DetailTypeDisabled:
		return true
	}
	return false
}

// SessionEndReason は LoginSession 終了の理由 (wi-20)。self_revoke / admin_revoke は
// 明示的なセッション失効、それ以外は自動失効・ライフサイクル起因。
type SessionEndReason string

const (
	SessionEndLogout         SessionEndReason = "logout"
	SessionEndIdle           SessionEndReason = "idle"
	SessionEndAbsolute       SessionEndReason = "absolute"
	SessionEndSelfRevoke     SessionEndReason = "self_revoke"
	SessionEndAdminRevoke    SessionEndReason = "admin_revoke"
	SessionEndPasswordChange SessionEndReason = "password_change"
	SessionEndMfaChange      SessionEndReason = "mfa_change"
	SessionEndOther          SessionEndReason = "other"
)

func (r SessionEndReason) Valid() bool {
	switch r {
	case SessionEndLogout, SessionEndIdle, SessionEndAbsolute, SessionEndSelfRevoke,
		SessionEndAdminRevoke, SessionEndPasswordChange, SessionEndMfaChange, SessionEndOther:
		return true
	}
	return false
}

type DeviceCodeFlowState string

const (
	DeviceFlowIssued          DeviceCodeFlowState = "issued"
	DeviceFlowUserCodeEntered DeviceCodeFlowState = "user_code_entered"
	DeviceFlowApproved        DeviceCodeFlowState = "approved"
	DeviceFlowDenied          DeviceCodeFlowState = "denied"
	DeviceFlowExchanged       DeviceCodeFlowState = "exchanged"
	DeviceFlowExpired         DeviceCodeFlowState = "expired"
)

func (s DeviceCodeFlowState) Valid() bool {
	switch s {
	case DeviceFlowIssued, DeviceFlowUserCodeEntered,
		DeviceFlowApproved, DeviceFlowDenied, DeviceFlowExchanged, DeviceFlowExpired:
		return true
	}
	return false
}

// レスポンスモード（authorize エンドポイントから redirect_uri に code を運ぶ方式）
type ResponseMode string

const (
	ResponseModeQuery    ResponseMode = "query"
	ResponseModeFormPost ResponseMode = "form_post"
)

func (r ResponseMode) Valid() bool { return r == ResponseModeQuery || r == ResponseModeFormPost }

// SenderConstraint は DPoP / mTLS による proof-of-possession トークン拘束。
type SenderConstraintType string

const (
	SenderConstraintDPoP SenderConstraintType = "dpop"
	SenderConstraintMTLS SenderConstraintType = "mtls"
)

type TenantStatus string

const (
	TenantStatusActive   TenantStatus = "active"
	TenantStatusDisabled TenantStatus = "disabled"
)

func (s TenantStatus) Valid() bool {
	return s == TenantStatusActive || s == TenantStatusDisabled
}

// AgentStatus は Agent の運用状態 (ADR-048)。Active 通常稼働 / Disabled 可逆な運用停止 /
// Killed 緊急停止 (一方向終端)。Active 以外は新規トークンを発行しない (fail-closed)。
type AgentStatus string

const (
	AgentStatusActive   AgentStatus = "active"
	AgentStatusDisabled AgentStatus = "disabled"
	AgentStatusKilled   AgentStatus = "killed"
)

func (s AgentStatus) Valid() bool {
	switch s {
	case AgentStatusActive, AgentStatusDisabled, AgentStatusKilled:
		return true
	}
	return false
}

// AgentKind は Agent の自律性区分 (ADR-048)。Autonomous 自律実行 / Supervised 監督下実行。
type AgentKind string

const (
	AgentKindAutonomous AgentKind = "autonomous"
	AgentKindSupervised AgentKind = "supervised"
)

func (k AgentKind) Valid() bool {
	return k == AgentKindAutonomous || k == AgentKindSupervised
}

// ===============================================================
// ユーザー属性拡張 (wi-19 / ADR-039 / ADR-040)
// ===============================================================

// UserStatus は UserLifecycle.status。User の運用状態の **唯一の真実** で、
// 状態機械 UserLifecycle (states セクション) と一致する。Active / Disabled /
// Deleted が状態機械の状態、Locked / Staged / Suspended は Okta lifecycle_state /
// Keycloak 相当の追加状態。Active 以外は認証不可。Deleted は終端 (tombstone)。
// 「いつ遷移したか」は監査イベント (UserDisabled / UserDeleted 等) と
// UserLifecycle.status_changed_at が持つので、専用の disabled_at / deleted_at は持たない。
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusDisabled  UserStatus = "disabled"
	UserStatusDeleted   UserStatus = "deleted"
	UserStatusLocked    UserStatus = "locked"
	UserStatusStaged    UserStatus = "staged"
	UserStatusSuspended UserStatus = "suspended"
)

func (s UserStatus) Valid() bool {
	switch s {
	case UserStatusActive, UserStatusDisabled, UserStatusDeleted,
		UserStatusLocked, UserStatusStaged, UserStatusSuspended:
		return true
	}
	return false
}

// RequiredAction は次回ログイン時にユーザへ強制するアクション (Keycloak Required Actions 相当)。
type RequiredAction string

const (
	RequiredActionUpdatePassword     RequiredAction = "update_password"
	RequiredActionVerifyEmail        RequiredAction = "verify_email"
	RequiredActionConfigureTOTP      RequiredAction = "configure_totp"
	RequiredActionUpdateProfile      RequiredAction = "update_profile"
	RequiredActionTermsAndConditions RequiredAction = "terms_and_conditions"
)

func (a RequiredAction) Valid() bool {
	switch a {
	case RequiredActionUpdatePassword, RequiredActionVerifyEmail,
		RequiredActionConfigureTOTP, RequiredActionUpdateProfile,
		RequiredActionTermsAndConditions:
		return true
	}
	return false
}

// AttributeType は属性値の sum type discriminator (ADR-040)。OIDC 標準クレームの
// 組み込み属性と tenant 定義カスタム属性の両方で共通に使う。
type AttributeType string

const (
	AttributeTypeString      AttributeType = "string"
	AttributeTypeNumber      AttributeType = "number"
	AttributeTypeBoolean     AttributeType = "boolean"
	AttributeTypeDate        AttributeType = "date"
	AttributeTypeStringArray AttributeType = "string_array"
)

func (t AttributeType) Valid() bool {
	switch t {
	case AttributeTypeString, AttributeTypeNumber, AttributeTypeBoolean,
		AttributeTypeDate, AttributeTypeStringArray:
		return true
	}
	return false
}

// AttrVisibility は属性の開示範囲 (ADR-040)。claim_exposed のみ UserInfo / ID Token に出せる。
type AttrVisibility string

const (
	AttrVisibilityPrivate       AttrVisibility = "private"
	AttrVisibilitySelfReadable  AttrVisibility = "self_readable"
	AttrVisibilityAdminReadable AttrVisibility = "admin_readable"
	AttrVisibilityClaimExposed  AttrVisibility = "claim_exposed"
)

func (v AttrVisibility) Valid() bool {
	switch v {
	case AttrVisibilityPrivate, AttrVisibilitySelfReadable,
		AttrVisibilityAdminReadable, AttrVisibilityClaimExposed:
		return true
	}
	return false
}
