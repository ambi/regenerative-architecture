package spec

// AuthZEN スタイルの認可ポリシー evaluate()。SCL permissions セクションを
// 「名前付きルールの集合」に分解し純粋関数で評価する。
// TS src/spec-bindings/policy/client-authorization.ts に対応。

import (
	"fmt"
	"slices"
	"time"
)

type AuthZSubject struct {
	Type       string            `json:"type"` // "Client" | "User"
	ID         string            `json:"id"`
	Properties AuthZSubjectProps `json:"properties,omitempty"`
}

type AuthZSubjectProps struct {
	ClientType    ClientType  `json:"clientType,omitempty"`
	GrantTypes    []GrantType `json:"grantTypes,omitempty"`
	Scopes        []string    `json:"scopes,omitempty"`
	RedirectURIs  []string    `json:"redirectUris,omitempty"`
	RequirePAR    bool        `json:"requirePAR,omitempty"`
	Authenticated bool        `json:"authenticated,omitempty"`
	Roles         []string    `json:"roles,omitempty"`
	TenantID      string      `json:"tenantId,omitempty"`
	DisabledAt    *time.Time  `json:"disabledAt,omitempty"`
}

type AuthZResource struct {
	Type       string             `json:"type"`
	ID         string             `json:"id,omitempty"`
	Properties AuthZResourceProps `json:"properties,omitempty"`
}

type AuthZResourceProps struct {
	IssuedToClientID    string                 `json:"issuedToClientId,omitempty"`
	RedirectURI         string                 `json:"redirectUri,omitempty"`
	CodeChallenge       string                 `json:"codeChallenge,omitempty"`
	CodeChallengeMethod string                 `json:"codeChallengeMethod,omitempty"`
	IssuedAt            time.Time              `json:"issuedAt,omitempty"`
	ExpiresAt           time.Time              `json:"expiresAt,omitempty"`
	Redeemed            bool                   `json:"redeemed,omitempty"`
	Revoked             bool                   `json:"revoked,omitempty"`
	Rotated             bool                   `json:"rotated,omitempty"`
	AbsoluteExpiresAt   time.Time              `json:"absoluteExpiresAt,omitempty"`
	SenderConstraint    *AuthZSenderConstraint `json:"senderConstraint,omitempty"`
	Scopes              []string               `json:"scopes,omitempty"`
	Approved            bool                   `json:"approved,omitempty"`
	TenantID            string                 `json:"tenantId,omitempty"`
}

type AuthZSenderConstraint struct {
	Type SenderConstraintType `json:"type"`
}

type AuthZContext struct {
	CodeVerifier      string                  `json:"codeVerifier,omitempty"`
	RedirectURI       string                  `json:"redirectUri,omitempty"`
	ProofOfPossession *AuthZProofOfPossession `json:"proofOfPossession,omitempty"`
	ParUsed           bool                    `json:"parUsed,omitempty"`
	Authenticated     bool                    `json:"authenticated,omitempty"`
	Now               time.Time               `json:"now,omitempty"`
}

type AuthZProofOfPossession struct {
	Valid   bool   `json:"valid"`
	JKT     string `json:"jkt,omitempty"`
	X5TS256 string `json:"x5t#S256,omitempty"`
}

type AuthZRequest struct {
	Subject  AuthZSubject  `json:"subject"`
	Action   string        `json:"action"`
	Resource AuthZResource `json:"resource"`
	Context  AuthZContext  `json:"context,omitempty"`
}

type AuthZResponse struct {
	Permit  bool     `json:"permit"`
	Reasons []string `json:"reasons,omitempty"`
}

const (
	ActionTokenGrantAuthorizationCode = "token:grant_authorization_code"
	ActionTokenGrantRefresh           = "token:grant_refresh"
	ActionTokenGrantClientCredentials = "token:grant_client_credentials"
	ActionTokenGrantDeviceCode        = "token:grant_device_code"
	ActionTokenIntrospect             = "token:introspect"
	ActionTokenRevoke                 = "token:revoke"
	ActionUserInfoRead                = "userinfo:read"
	ActionAuthorizeInitiate           = "authorize:initiate"
	ActionAdminUserRead               = "admin:user_read"
	ActionAdminUserCreate             = "admin:user_create"
	ActionAdminUserUpdate             = "admin:user_update"
	ActionAdminClientsManage          = "admin:clients_manage"
	ActionAdminConsentsManage         = "admin:consents_manage"
	ActionAdminTenantsManage          = "admin:tenants_manage"
	ActionAdminAuditEventsRead        = "admin:audit_events_read"
)

// PascalCase (SCL permissions のキー) → AuthZ action 名。
var actionNameMapping = map[string]string{
	"TokenGrantAuthorizationCode": ActionTokenGrantAuthorizationCode,
	"TokenGrantRefresh":           ActionTokenGrantRefresh,
	"TokenGrantClientCredentials": ActionTokenGrantClientCredentials,
	"TokenGrantDeviceCode":        ActionTokenGrantDeviceCode,
	"TokenIntrospect":             ActionTokenIntrospect,
	"TokenRevoke":                 ActionTokenRevoke,
	"UserInfoRead":                ActionUserInfoRead,
	"AuthorizeInitiate":           ActionAuthorizeInitiate,
	"AdminUserRead":               ActionAdminUserRead,
	"AdminUserCreate":             ActionAdminUserCreate,
	"AdminUserUpdate":             ActionAdminUserUpdate,
	"AdminClientsManage":          ActionAdminClientsManage,
	"AdminConsentsManage":         ActionAdminConsentsManage,
	"AdminTenantsManage":          ActionAdminTenantsManage,
	"AdminAuditEventsRead":        ActionAdminAuditEventsRead,
}

var actionRules = map[string][]string{
	ActionTokenGrantAuthorizationCode: {
		"client_must_declare_grant",
		"pkce_verification_passed",
		"redirect_uri_exact_match",
		"code_not_redeemed",
		"code_not_expired",
	},
	ActionTokenGrantRefresh: {
		"client_must_declare_grant",
		"token_active",
		"token_within_absolute_ttl",
		"sender_constraint_satisfied",
	},
	ActionTokenGrantClientCredentials: {"client_is_confidential", "client_must_declare_grant"},
	ActionTokenGrantDeviceCode:        {"device_code_approved", "device_code_not_expired"},
	ActionTokenIntrospect:             {"caller_is_authenticated_client"},
	ActionTokenRevoke:                 {"caller_owns_token"},
	ActionUserInfoRead:                {"token_has_openid_scope", "token_active"},
	ActionAuthorizeInitiate: {
		"client_registered",
		"redirect_uri_registered",
		"scope_subset_of_client_scope",
		"pkce_present",
		"par_required_if_fapi",
	},
	ActionAdminUserRead:   {"actor_is_admin", "actor_is_active", "actor_is_authenticated"},
	ActionAdminUserCreate: {"actor_is_admin", "actor_is_active", "actor_is_authenticated"},
	ActionAdminUserUpdate: {"actor_is_admin", "actor_is_active", "actor_is_authenticated"},
	ActionAdminClientsManage: {
		"actor_is_admin", "actor_is_active", "actor_is_authenticated", "actor_and_resource_share_tenant",
	},
	ActionAdminConsentsManage: {
		"actor_is_admin", "actor_is_active", "actor_is_authenticated", "actor_and_resource_share_tenant",
	},
	ActionAdminTenantsManage: {
		"actor_is_system_admin", "actor_is_control_plane_user", "actor_is_active", "actor_is_authenticated",
	},
	ActionAdminAuditEventsRead: {
		"actor_is_admin_or_system_admin", "actor_is_active", "actor_is_authenticated",
	},
}

type ruleEvaluator func(req AuthZRequest) bool

var ruleEvaluators = map[string]ruleEvaluator{
	"client_must_declare_grant": func(r AuthZRequest) bool {
		g := grantFromAction(r.Action)
		return g != "" && slices.Contains(r.Subject.Properties.GrantTypes, g)
	},
	"pkce_verification_passed": func(r AuthZRequest) bool {
		return r.Context.CodeVerifier != "" && r.Resource.Properties.CodeChallenge != ""
	},
	"redirect_uri_exact_match": func(r AuthZRequest) bool {
		return r.Context.RedirectURI != "" && r.Resource.Properties.RedirectURI != "" &&
			r.Context.RedirectURI == r.Resource.Properties.RedirectURI
	},
	"redirect_uri_registered": func(r AuthZRequest) bool {
		return slices.Contains(r.Subject.Properties.RedirectURIs, r.Context.RedirectURI)
	},
	"code_not_redeemed":         func(r AuthZRequest) bool { return !r.Resource.Properties.Redeemed },
	"code_not_expired":          func(r AuthZRequest) bool { return !isExpired(r.Resource.Properties.ExpiresAt, r.Context.Now) },
	"token_active":              func(r AuthZRequest) bool { return !r.Resource.Properties.Revoked && !r.Resource.Properties.Rotated },
	"token_within_absolute_ttl": func(r AuthZRequest) bool { return !isExpired(r.Resource.Properties.AbsoluteExpiresAt, r.Context.Now) },
	"sender_constraint_satisfied": func(r AuthZRequest) bool {
		if r.Resource.Properties.SenderConstraint == nil {
			return true
		}
		return r.Context.ProofOfPossession != nil && r.Context.ProofOfPossession.Valid
	},
	"client_is_confidential":  func(r AuthZRequest) bool { return r.Subject.Properties.ClientType == ClientConfidential },
	"device_code_approved":    func(r AuthZRequest) bool { return r.Resource.Properties.Approved },
	"device_code_not_expired": func(r AuthZRequest) bool { return !isExpired(r.Resource.Properties.ExpiresAt, r.Context.Now) },
	"caller_is_authenticated_client": func(r AuthZRequest) bool {
		return r.Subject.Type == "Client" && r.Subject.Properties.Authenticated
	},
	"caller_owns_token":      func(r AuthZRequest) bool { return r.Subject.ID == r.Resource.Properties.IssuedToClientID },
	"token_has_openid_scope": func(r AuthZRequest) bool { return slices.Contains(r.Resource.Properties.Scopes, "openid") },
	"client_registered":      func(r AuthZRequest) bool { return r.Subject.Type == "Client" && r.Subject.ID != "" },
	"scope_subset_of_client_scope": func(r AuthZRequest) bool {
		allowed := r.Subject.Properties.Scopes
		for _, s := range r.Resource.Properties.Scopes {
			if !slices.Contains(allowed, s) {
				return false
			}
		}
		return true
	},
	"pkce_present":         func(r AuthZRequest) bool { return r.Resource.Properties.CodeChallenge != "" },
	"par_required_if_fapi": func(r AuthZRequest) bool { return !r.Subject.Properties.RequirePAR || r.Context.ParUsed },
	"actor_is_admin": func(r AuthZRequest) bool {
		return r.Subject.Type == "User" && slices.Contains(r.Subject.Properties.Roles, "admin")
	},
	"actor_is_system_admin": func(r AuthZRequest) bool {
		return r.Subject.Type == "User" && slices.Contains(r.Subject.Properties.Roles, "system_admin")
	},
	"actor_is_control_plane_user": func(r AuthZRequest) bool {
		return r.Subject.Type == "User" && r.Subject.Properties.TenantID == DefaultTenantID
	},
	"actor_is_active": func(r AuthZRequest) bool {
		return r.Subject.Type == "User" && r.Subject.Properties.DisabledAt == nil
	},
	"actor_is_authenticated": func(r AuthZRequest) bool {
		return r.Subject.Type == "User" && r.Context.Authenticated
	},
	"actor_and_resource_share_tenant": func(r AuthZRequest) bool {
		return r.Subject.Type == "User" && r.Subject.Properties.TenantID != "" &&
			r.Subject.Properties.TenantID == r.Resource.Properties.TenantID
	},
	"actor_is_admin_or_system_admin": func(r AuthZRequest) bool {
		if r.Subject.Type != "User" {
			return false
		}
		return slices.Contains(r.Subject.Properties.Roles, "admin") ||
			slices.Contains(r.Subject.Properties.Roles, "system_admin")
	},
}

func grantFromAction(a string) GrantType {
	switch a {
	case ActionTokenGrantAuthorizationCode:
		return GrantAuthorizationCode
	case ActionTokenGrantRefresh:
		return GrantRefreshToken
	case ActionTokenGrantClientCredentials:
		return GrantClientCredentials
	case ActionTokenGrantDeviceCode:
		return GrantDeviceCode
	}
	return ""
}

func isExpired(expiresAt, now time.Time) bool {
	if expiresAt.IsZero() {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	return !now.Before(expiresAt)
}

func Evaluate(req AuthZRequest) AuthZResponse {
	rules, ok := actionRules[req.Action]
	if !ok {
		return AuthZResponse{Permit: false, Reasons: []string{fmt.Sprintf("未定義のアクション: %s", req.Action)}}
	}
	var failed []string
	for _, ruleID := range rules {
		ev, ok := ruleEvaluators[ruleID]
		if !ok {
			failed = append(failed, "未実装のルール: "+ruleID)
			continue
		}
		if !ev(req) {
			failed = append(failed, ruleID)
		}
	}
	if len(failed) > 0 {
		return AuthZResponse{Permit: false, Reasons: failed}
	}
	return AuthZResponse{Permit: true}
}

func AllRuleIDs() []string {
	seen := map[string]struct{}{}
	var out []string
	for _, rs := range actionRules {
		for _, r := range rs {
			if _, ok := seen[r]; !ok {
				seen[r] = struct{}{}
				out = append(out, r)
			}
		}
	}
	return out
}

func ImplementedRuleIDs() []string {
	out := make([]string, 0, len(ruleEvaluators))
	for k := range ruleEvaluators {
		out = append(out, k)
	}
	return out
}

// SCLPermissionsCoverage は SCL permissions の PascalCase 名と AuthZ action マッピングの差分を返す。
func (s *SCL) SCLPermissionsCoverage() (missing, extra []string) {
	mapped := map[string]struct{}{}
	for k := range actionNameMapping {
		mapped[k] = struct{}{}
	}
	for name := range s.Permissions {
		if _, ok := mapped[name]; !ok {
			missing = append(missing, name)
		}
	}
	for name := range mapped {
		if _, ok := s.Permissions[name]; !ok {
			extra = append(extra, name)
		}
	}
	return missing, extra
}
