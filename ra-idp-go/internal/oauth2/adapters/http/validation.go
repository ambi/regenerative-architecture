package http

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	z "github.com/Oudwins/zog"
)

type authorizeRequest struct {
	ClientID            string `zog:"client_id"`
	RedirectURI         string `zog:"redirect_uri"`
	ResponseType        string `zog:"response_type"`
	Scope               string `zog:"scope"`
	StateParam          string `zog:"state"`
	Nonce               string `zog:"nonce"`
	CodeChallenge       string `zog:"code_challenge"`
	CodeChallengeMethod string `zog:"code_challenge_method"`
	Prompt              string `zog:"prompt"`
	MaxAge              *int   `zog:"max_age"`
	AcrValues           string `zog:"acr_values"`
}

var authorizeRequestSchema = z.Struct(z.Shape{
	"clientID":            z.String().Required(),
	"redirectURI":         z.String().URL().Required(),
	"responseType":        z.String().Required(),
	"scope":               z.String(),
	"stateParam":          z.String(),
	"nonce":               z.String(),
	"codeChallenge":       z.String().Required(),
	"codeChallengeMethod": z.String().Required(),
	"prompt":              z.String(),
	"maxAge":              z.Ptr(z.Int().GTE(0)),
	"AcrValues":           z.String(),
})

func parseAuthorizeRequest(values url.Values) (authorizeRequest, error) {
	data := make(map[string]string, len(values))
	for key := range values {
		data[key] = values.Get(key)
	}
	var request authorizeRequest
	err := zogError(authorizeRequestSchema.Parse(data, &request))
	return request, err
}

type registerClientRequest struct {
	ClientName              string         `json:"client_name"`
	ClientType              string         `json:"client_type"`
	RedirectURIs            []string       `json:"redirect_uris"`
	GrantTypes              []string       `json:"grant_types"`
	ResponseTypes           []string       `json:"response_types"`
	TokenEndpointAuthMethod string         `json:"token_endpoint_auth_method"`
	Scope                   string         `json:"scope"`
	JWKS                    map[string]any `json:"jwks"`
	JwksURI                 *string        `json:"jwks_uri"`
	TlsClientAuthSubjectDN  *string        `json:"tls_client_auth_subject_dn"`
	RequirePAR              bool           `json:"require_pushed_authorization_requests"`
	DpopBoundAccessTokens   bool           `json:"dpop_bound_access_tokens"`
	FapiProfile             string         `json:"fapi_profile"`
}

var registerClientRequestSchema = z.Struct(z.Shape{
	"ClientName":   z.String().Max(200),
	"ClientType":   z.String().OneOf([]string{"public", "confidential"}),
	"RedirectURIs": z.Slice(z.String().URL()).Min(1),
	"GrantTypes": z.Slice(z.String().OneOf([]string{
		"authorization_code",
		"refresh_token",
		"client_credentials",
		"urn:ietf:params:oauth:grant-type:device_code",
	})),
	"ResponseTypes": z.Slice(z.String().OneOf([]string{"code"})),
	"TokenEndpointAuthMethod": z.String().OneOf([]string{
		"client_secret_basic",
		"client_secret_post",
		"private_key_jwt",
		"tls_client_auth",
		"none",
	}),
	"JwksURI":                z.Ptr(jwksURI()),
	"TlsClientAuthSubjectDN": z.Ptr(z.String().Min(1)),
	"FapiProfile":            z.String().OneOf([]string{"none", "fapi_2_security_profile"}),
}).TestFunc(func(value any, _ z.Ctx) bool {
	request, ok := value.(*registerClientRequest)
	return ok && (request.TokenEndpointAuthMethod != "tls_client_auth" ||
		request.TlsClientAuthSubjectDN != nil && *request.TlsClientAuthSubjectDN != "")
}, z.Message("tls_client_auth requires tls_client_auth_subject_dn"))

func validateRegisterClientRequest(request *registerClientRequest) error {
	return zogError(registerClientRequestSchema.Validate(request))
}

func jwksURI() *z.StringSchema[string] {
	return z.String().URL().TestFunc(
		func(value *string, _ z.Ctx) bool {
			parsed, err := url.Parse(*value)
			if err != nil {
				return false
			}
			return parsed.Scheme == "https" && parsed.User == nil && parsed.Fragment == ""
		},
		z.Message("jwks_uri must be https and must not contain userinfo or fragment"),
	)
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
