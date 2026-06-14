// Package spec: SCL → Go バインディング。
//
// 仕様本体（language-agnostic）は spec/scl.yaml（symlink で TS 実装と共有）。
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
)

func (g GrantType) Valid() bool {
	switch g {
	case GrantAuthorizationCode, GrantRefreshToken, GrantClientCredentials, GrantDeviceCode:
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
