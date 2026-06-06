package spec

import (
	"fmt"
	"slices"
)

// Discovery 文書 (OIDC Discovery 1.0 / RFC 8414) を SCL から派生する。
// TS src/spec-bindings/discovery.ts に対応。

type discoveryEndpoint struct {
	Field         string
	InterfaceName string
}

var discoveryEndpoints = []discoveryEndpoint{
	{"authorization_endpoint", "Authorize"},
	{"token_endpoint", "Token"},
	{"userinfo_endpoint", "UserInfo"},
	{"jwks_uri", "GetJwks"},
	{"introspection_endpoint", "Introspect"},
	{"revocation_endpoint", "Revoke"},
	{"pushed_authorization_request_endpoint", "PushAuthorizationRequest"},
	{"device_authorization_endpoint", "DeviceAuthorization"},
	{"registration_endpoint", "RegisterClient"},
	{"end_session_endpoint", "EndSession"},
}

func (s *SCL) BuildDiscoveryDocument(issuer string) (map[string]any, error) {
	doc := map[string]any{"issuer": issuer}
	for _, e := range discoveryEndpoints {
		iface, ok := s.Interfaces[e.InterfaceName]
		if !ok {
			return nil, fmt.Errorf("interface %s not found", e.InterfaceName)
		}
		b, ok := s.HTTPBinding(iface)
		if !ok {
			return nil, fmt.Errorf("interface %s has no http binding", e.InterfaceName)
		}
		path := b.String("path")
		if path == "" {
			return nil, fmt.Errorf("interface %s http binding has no path", e.InterfaceName)
		}
		doc[e.Field] = issuer + path
	}

	tpl := s.Annotations.DiscoveryTemplate
	for _, e := range []struct {
		field string
		model string
	}{
		{"response_types_supported", "ResponseType"},
		{"response_modes_supported", "ResponseMode"},
		{"grant_types_supported", "GrantType"},
		{"id_token_signing_alg_values_supported", "SignatureAlgorithm"},
		{"token_endpoint_auth_signing_alg_values_supported", "SignatureAlgorithm"},
		{"code_challenge_methods_supported", "CodeChallengeMethod"},
		{"dpop_signing_alg_values_supported", "SignatureAlgorithm"},
	} {
		v, err := s.EnumWireValues(e.model)
		if err != nil {
			return nil, err
		}
		doc[e.field] = v
	}

	authMethods, err := s.EnumWireValues("TokenEndpointAuthMethod")
	if err != nil {
		return nil, err
	}
	doc["token_endpoint_auth_methods_supported"] = slices.DeleteFunc(slices.Clone(authMethods), func(m string) bool { return m == "none" })

	doc["scopes_supported"] = tpl.ScopesSupported
	doc["subject_types_supported"] = defaultStrings(tpl.SubjectTypesSupported, []string{"public"})
	doc["introspection_endpoint_auth_methods_supported"] = s.ToWireAll(tpl.IntrospectionEndpointAuthMethods)
	doc["revocation_endpoint_auth_methods_supported"] = s.ToWireAll(tpl.RevocationEndpointAuthMethods)
	doc["require_pushed_authorization_requests"] = false
	doc["require_pkce"] = true
	doc["tls_client_certificate_bound_access_tokens"] = true
	doc["claims_supported"] = tpl.ClaimsSupported
	doc["service_documentation"] = issuer + "/docs"
	doc["ui_locales_supported"] = defaultStrings(tpl.UILocalesSupported, []string{"en", "ja"})
	return doc, nil
}

func defaultStrings(v, fallback []string) []string {
	if len(v) == 0 {
		return fallback
	}
	return v
}
