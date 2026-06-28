package spec

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	z "github.com/Oudwins/zog"
)

var tenantIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// attrKeyPattern は ADR-040 の属性キー命名規則: snake_case、英字始まり。
var attrKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

var tenantSchema = z.Struct(z.Shape{
	"ID": z.String().Min(1).Max(63).TestFunc(
		func(value *string, _ z.Ctx) bool {
			return value != nil && tenantIDPattern.MatchString(*value) && *value != "admin"
		},
		z.Message("tenant id must be a URL-safe slug and must not be admin"),
	).Required(),
	"DisplayName": z.String().Min(1).Max(200).Required(),
	"Status": z.StringLike[TenantStatus]().TestFunc(
		func(value *TenantStatus, _ z.Ctx) bool { return value.Valid() },
		z.Message("tenant status is not in enum"),
	).Required(),
	"CreatedAt": z.Time().Required(),
})

var oauth2ClientSchema = z.Struct(z.Shape{
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
	),
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
	client, ok := value.(*OAuth2Client)
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
}, z.Message("client authentication method requires matching credentials")).
	TestFunc(func(value any, _ z.Ctx) bool {
		client, ok := value.(*OAuth2Client)
		if !ok {
			return false
		}
		// redirect 系グラント (authorization_code) は redirect_uri を必須とする
		// (RFC 6749 §3.1.2)。client_credentials のみの M2M クライアントは redirect を持たない。
		if clientUsesRedirect(client) {
			return len(client.RedirectURIs) > 0
		}
		return true
	}, z.Message("redirect_uris is required for redirect-based grants"))

// clientUsesRedirect は client が redirect 系グラント (authorization_code) または
// code response_type を使うかを返す。これらは redirect_uri を必要とする。
func clientUsesRedirect(client *OAuth2Client) bool {
	return slices.Contains(client.GrantTypes, GrantAuthorizationCode) ||
		slices.Contains(client.ResponseTypes, ResponseTypeCode)
}

var userSchema = z.Struct(z.Shape{
	"Sub":               z.String().Required(),
	"PreferredUsername": z.String().Min(1).Max(100).Required(),
	"PasswordHash":      z.String().Required(),
	"Name":              z.Ptr(z.String().Max(200)),
	"GivenName":         z.Ptr(z.String().Max(100)),
	"FamilyName":        z.Ptr(z.String().Max(100)),
	"Email":             z.Ptr(z.String().Email()),
	"Roles":             z.Slice(z.String().Min(1)),
	"CreatedAt":         z.Time().Required(),
	"UpdatedAt":         z.Time().Required(),
})

var userAttributeDefSchema = z.Struct(z.Shape{
	"Key": z.String().TestFunc(
		func(value *string, _ z.Ctx) bool {
			return value != nil && attrKeyPattern.MatchString(*value)
		},
		z.Message("attribute key must be snake_case starting with a letter"),
	).Required(),
	"Type": z.StringLike[AttributeType]().TestFunc(
		func(value *AttributeType, _ z.Ctx) bool { return value.Valid() },
		z.Message("attribute type is not in enum"),
	).Required(),
	"Label":     z.String().Max(100),
	"ClaimName": z.Ptr(z.String().Min(1).Max(100)),
	"OIDCScope": z.Ptr(z.String().Min(1).Max(60)),
	"Visibility": z.StringLike[AttrVisibility]().TestFunc(
		func(value *AttrVisibility, _ z.Ctx) bool { return value.Valid() },
		z.Message("attribute visibility is not in enum"),
	).Required(),
})

var groupSchema = z.Struct(z.Shape{
	"ID":          z.String().Min(1).Max(64).Required(),
	"TenantID":    z.String().Min(1).Required(),
	"Name":        z.String().Min(1).Max(100).Required(),
	"Description": z.Ptr(z.String().Max(500)),
	"Roles":       z.Slice(z.String().Min(1)),
	"CreatedAt":   z.Time().Required(),
})

var groupMemberSchema = z.Struct(z.Shape{
	"GroupID": z.String().Min(1).Required(),
	"UserSub": z.String().Min(1).Required(),
	"AddedAt": z.Time().Required(),
})

var agentSchema = z.Struct(z.Shape{
	"ID":          z.String().Min(1).Max(64).Required(),
	"TenantID":    z.String().Min(1).Required(),
	"Name":        z.String().Min(1).Max(100).Required(),
	"Description": z.Ptr(z.String().Max(500)),
	"Kind": z.StringLike[AgentKind]().TestFunc(
		func(value *AgentKind, _ z.Ctx) bool { return value.Valid() },
		z.Message("agent kind is not in enum"),
	).Required(),
	"OwnerSub": z.String().Min(1).Required(),
	"Status": z.StringLike[AgentStatus]().TestFunc(
		func(value *AgentStatus, _ z.Ctx) bool { return value.Valid() },
		z.Message("agent status is not in enum"),
	).Required(),
	"Roles":     z.Slice(z.String().Min(1)),
	"CreatedAt": z.Time().Required(),
})

var agentCredentialBindingSchema = z.Struct(z.Shape{
	"AgentID":   z.String().Min(1).Required(),
	"ClientID":  z.String().Min(1).Required(),
	"TenantID":  z.String().Min(1).Required(),
	"CreatedAt": z.Time().Required(),
})

var authorizationDetailTypeSchema = z.Struct(z.Shape{
	"TenantID":        z.String().Min(1).Required(),
	"Type":            z.String().Min(1).Required(),
	"DisplayTemplate": z.String().Min(1).Required(),
	"State": z.StringLike[AuthorizationDetailTypeState]().TestFunc(
		func(value *AuthorizationDetailTypeState, _ z.Ctx) bool { return value.Valid() },
		z.Message("authorization detail type state is not in enum"),
	).Required(),
	"Schema": z.Struct(z.Shape{
		"Rules": z.Slice(z.Struct(z.Shape{
			"Name": z.String().Min(1).Required(),
			"Semantics": z.StringLike[AuthorizationDetailFieldSemantics]().TestFunc(
				func(value *AuthorizationDetailFieldSemantics, _ z.Ctx) bool { return value.Valid() },
				z.Message("authorization detail field semantics is not in enum"),
			).Required(),
		})).Min(1).Required(),
	}),
	"CreatedAt": z.Time().Required(),
	"UpdatedAt": z.Time().Required(),
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
	return zogError(schema.Validate(value))
}

func zogError(issues z.ZogIssueList) error {
	if len(issues) == 0 {
		return nil
	}

	messages := make([]string, 0, len(issues))
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		message := issue.Message
		if message == "" && issue.Err != nil {
			message = issue.Err.Error()
		}
		if message == "" {
			message = issue.Code
		}
		if path := issue.PathString(); path != "" {
			message = fmt.Sprintf("%s: %s", path, message)
		}
		messages = append(messages, message)
	}
	if len(messages) == 0 {
		return errors.New("validation failed")
	}
	sort.Strings(messages)
	return errors.New(strings.Join(messages, "; "))
}
