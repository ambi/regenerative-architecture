package spec

import "slices"

// Grant matrix: 標準 OAuth 2.0 / OIDC RFC からの写し。
// TS src/spec-bindings/grants/grant-types.ts に対応。

type GrantSpecEntry struct {
	AllowedClientTypes []ClientType
	RequiresPKCE       bool
	ResponseTypes      []ResponseType
	Issues             []string // "access_token", "refresh_token", "id_token"
}

var grantSpec = map[GrantType]GrantSpecEntry{
	GrantAuthorizationCode: {
		AllowedClientTypes: []ClientType{ClientPublic, ClientConfidential},
		RequiresPKCE:       true,
		ResponseTypes:      []ResponseType{ResponseTypeCode},
		Issues:             []string{"access_token", "refresh_token", "id_token"},
	},
	GrantRefreshToken: {
		AllowedClientTypes: []ClientType{ClientPublic, ClientConfidential},
		RequiresPKCE:       false,
		Issues:             []string{"access_token", "refresh_token"},
	},
	GrantClientCredentials: {
		AllowedClientTypes: []ClientType{ClientConfidential},
		RequiresPKCE:       false,
		Issues:             []string{"access_token"},
	},
	GrantDeviceCode: {
		AllowedClientTypes: []ClientType{ClientPublic, ClientConfidential},
		RequiresPKCE:       false,
		Issues:             []string{"access_token", "refresh_token", "id_token"},
	},
}

func GetGrantSpec(g GrantType) (GrantSpecEntry, bool) {
	s, ok := grantSpec[g]
	return s, ok
}

func GrantAllowsClientType(g GrantType, ct ClientType) bool {
	s, ok := grantSpec[g]
	return ok && slices.Contains(s.AllowedClientTypes, ct)
}

func GrantRequiresPKCE(g GrantType) bool {
	s, ok := grantSpec[g]
	return ok && s.RequiresPKCE
}

func GrantIssues(g GrantType, what string) bool {
	s, ok := grantSpec[g]
	return ok && slices.Contains(s.Issues, what)
}
