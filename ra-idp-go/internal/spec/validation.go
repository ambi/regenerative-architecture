package spec

import (
	"ra-idp-go/internal/validation"

	z "github.com/Oudwins/zog"
)

var clientSchema = z.Struct(z.Shape{
	"ClientID": z.String().Min(1).Max(128).Required(),
	"ClientName": z.Ptr(
		z.String().Min(1).Max(200),
	),
	"ClientType": z.StringLike[ClientType]().TestFunc(
		func(value *ClientType, _ z.Ctx) bool { return value.Valid() },
		z.Message("client_type is not in enum"),
	).Required(),
	"RedirectURIs": z.Slice(
		z.String().URL(),
	).Min(1).Required(),
	"GrantTypes": z.Slice(
		z.StringLike[GrantType]().TestFunc(
			func(value *GrantType, _ z.Ctx) bool { return value.Valid() },
			z.Message("grant_type is not in enum"),
		),
	).Min(1).Required(),
	"ResponseTypes": z.Slice(
		z.StringLike[ResponseType]().TestFunc(
			func(value *ResponseType, _ z.Ctx) bool { return value.Valid() },
			z.Message("response_type is not in enum"),
		),
	),
	"TokenEndpointAuthMethod": z.StringLike[TokenEndpointAuthMethod]().TestFunc(
		func(value *TokenEndpointAuthMethod, _ z.Ctx) bool { return value.Valid() },
		z.Message("token_endpoint_auth_method is not in enum"),
	).Required(),
	"IDTokenSignedResponseAlg": z.StringLike[SignatureAlgorithm]().TestFunc(
		func(value *SignatureAlgorithm, _ z.Ctx) bool { return value.Valid() },
		z.Message("id_token_signed_response_alg is not in enum"),
	).Required(),
	"FapiProfile": z.StringLike[FapiProfile]().TestFunc(
		func(value *FapiProfile, _ z.Ctx) bool { return value.Valid() },
		z.Message("fapi_profile is not in enum"),
	).Required(),
	"CreatedAt": z.Time().Required(),
}).TestFunc(func(value any, _ z.Ctx) bool {
	client, ok := value.(*Client)
	if !ok {
		return false
	}
	switch client.TokenEndpointAuthMethod {
	case AuthMethodPrivateKeyJwt:
		return hasJWKs(client.JWKS) || client.JwksURI != nil && *client.JwksURI != ""
	case AuthMethodTlsClientAuth:
		return client.TlsClientAuthSubjectDN != nil && *client.TlsClientAuthSubjectDN != ""
	default:
		return true
	}
}, z.Message("client authentication method requires matching credentials"))

var userSchema = z.Struct(z.Shape{
	"Sub":               z.String().Required(),
	"PreferredUsername": z.String().Min(1).Max(100).Required(),
	"PasswordHash":      z.String().Required(),
	"Email":             z.Ptr(z.String().Email()),
	"Roles":             z.Slice(z.String().Min(1)),
	"CreatedAt":         z.Time().Required(),
	"UpdatedAt":         z.Time().Required(),
})

var mfaFactorSchema = z.Struct(z.Shape{
	"Sub": z.String().Required(),
	"Type": z.StringLike[MfaFactorType]().TestFunc(
		func(value *MfaFactorType, _ z.Ctx) bool { return value.Valid() },
		z.Message("mfa factor type is not in enum"),
	).Required(),
	"CreatedAt": z.Time().Required(),
}).TestFunc(func(value any, _ z.Ctx) bool {
	factor, ok := value.(*MfaFactor)
	return ok && (factor.Type != MfaFactorTOTP ||
		factor.Secret != nil && *factor.Secret != "")
}, z.Message("totp factor requires secret"))

var consentSchema = z.Struct(z.Shape{
	"Sub":      z.String().Required(),
	"ClientID": z.String().Required(),
	"Scopes":   z.Slice(z.String()).Min(1).Required(),
	"State": z.StringLike[ConsentState]().TestFunc(
		func(value *ConsentState, _ z.Ctx) bool { return value.Valid() },
		z.Message("state is not in enum"),
	).Required(),
	"GrantedAt": z.Time().Required(),
	"ExpiresAt": z.Time().Required(),
})

var authorizationRequestSchema = z.Struct(z.Shape{
	"ID": z.String().UUID().Required(),
	"State": z.StringLike[AuthorizationCodeFlowState]().TestFunc(
		func(value *AuthorizationCodeFlowState, _ z.Ctx) bool { return value.Valid() },
		z.Message("state is not in enum"),
	).Required(),
	"ClientID":    z.String().Required(),
	"RedirectURI": z.String().URL().Required(),
	"ResponseType": z.StringLike[ResponseType]().OneOf(
		[]ResponseType{ResponseTypeCode},
		z.Message("response_type must be code"),
	).Required(),
	"CodeChallenge": z.String().Required(),
	"CodeChallengeMethod": z.StringLike[CodeChallengeMethod]().OneOf(
		[]CodeChallengeMethod{CodeChallengeMethodS256},
		z.Message("code_challenge_method must be S256"),
	).Required(),
	"MaxAge":    z.Ptr(z.Int().GTE(0)),
	"CreatedAt": z.Time().Required(),
	"ExpiresAt": z.Time().Required(),
})

var authorizationCodeRecordSchema = z.Struct(z.Shape{
	"Code":                   z.String().Required(),
	"AuthorizationRequestID": z.String().UUID().Required(),
	"ClientID":               z.String().Required(),
	"Sub":                    z.String().Required(),
	"RedirectURI":            z.String().URL().Required(),
	"CodeChallenge":          z.String().Required(),
	"CodeChallengeMethod": z.StringLike[CodeChallengeMethod]().OneOf(
		[]CodeChallengeMethod{CodeChallengeMethodS256},
		z.Message("code_challenge_method must be S256"),
	).Required(),
	"State": z.StringLike[AuthorizationCodeRecordState]().TestFunc(
		func(value *AuthorizationCodeRecordState, _ z.Ctx) bool { return value.Valid() },
		z.Message("state is not in enum"),
	).Required(),
	"IssuedAt":  z.Time().Required(),
	"ExpiresAt": z.Time().Required(),
})

var loginSessionSchema = z.Struct(z.Shape{
	"ID":        z.String().UUID().Required(),
	"Sub":       z.String().Required(),
	"AMR":       z.Slice(z.String()).Min(1).Required(),
	"ACR":       z.String().Required(),
	"ExpiresAt": z.Time().Required(),
})

var loginRequestSchema = z.Struct(z.Shape{
	"RequestID": z.String().UUID().Required(),
	"Username":  z.String().Required(),
	"Password":  z.String().Required(),
})

var refreshTokenRecordSchema = z.Struct(z.Shape{
	"ID":                z.String().UUID().Required(),
	"Hash":              z.String().Required(),
	"FamilyID":          z.String().UUID().Required(),
	"ClientID":          z.String().Required(),
	"Sub":               z.String().Required(),
	"IssuedAt":          z.Time().Required(),
	"ExpiresAt":         z.Time().Required(),
	"AbsoluteExpiresAt": z.Time().Required(),
})

var parRecordSchema = z.Struct(z.Shape{
	"RequestURI": z.String().Required(),
	"ClientID":   z.String().Required(),
	"IssuedAt":   z.Time().Required(),
	"ExpiresAt":  z.Time().Required(),
})

var deviceAuthorizationSchema = z.Struct(z.Shape{
	"DeviceCodeHash": z.String().Required(),
	"UserCode":       z.String().Required(),
	"ClientID":       z.String().Required(),
	"State": z.StringLike[DeviceCodeFlowState]().TestFunc(
		func(value *DeviceCodeFlowState, _ z.Ctx) bool { return value.Valid() },
		z.Message("state is not in enum"),
	).Required(),
	"IntervalSeconds": z.Int().GT(0).Required(),
	"IssuedAt":        z.Time().Required(),
	"ExpiresAt":       z.Time().Required(),
})

func validate(schema *z.StructSchema, value any) error {
	return validation.Error(schema.Validate(value))
}
