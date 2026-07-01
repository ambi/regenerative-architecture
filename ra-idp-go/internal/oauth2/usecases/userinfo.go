// /userinfo (OIDC Core §5.3)
package usecases

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	idmports "ra-idp-go/internal/identitymanagement/ports"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/shared/spec"
)

type UserInfoInput struct {
	Scopes   []string
	Sub      string
	Active   bool
	ClientID string
	// ResolveAttributeDefs はユーザのテナントに有効な属性定義 (builtin + custom) を
	// 返す。nil の場合は属性ベースの claim 生成をスキップする。tenant_id は対象
	// ユーザを読み込んだ後に確定するため、関数として遅延解決する。
	ResolveAttributeDefs func(ctx context.Context, tenantID string) ([]spec.UserAttributeDef, error)
}

type UserInfoResponse struct {
	Sub               string `json:"sub"`
	Name              string `json:"name,omitempty"`
	GivenName         string `json:"given_name,omitempty"`
	FamilyName        string `json:"family_name,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
	Email             string `json:"email,omitempty"`
	EmailVerified     bool   `json:"email_verified,omitempty"`
	UpdatedAt         int64  `json:"updated_at,omitempty"`
	// Extra は scope に応じて開示する属性ベースの追加 claim (OIDC §5.4 / wi-19)。
	// 標準 claim とキーが衝突した場合は標準 claim を優先する。
	Extra map[string]any `json:"-"`
}

func (r UserInfoResponse) MarshalJSON() ([]byte, error) {
	type alias UserInfoResponse
	raw, err := json.Marshal(alias(r))
	if err != nil {
		return nil, err
	}
	if len(r.Extra) == 0 {
		return raw, nil
	}
	var merged map[string]any
	if err := json.Unmarshal(raw, &merged); err != nil {
		return nil, err
	}
	for key, value := range r.Extra {
		if _, exists := merged[key]; !exists {
			merged[key] = value
		}
	}
	return json.Marshal(merged)
}

func UserInfo(
	ctx context.Context,
	repo idmports.UserRepository,
	authorizer ports.Authorizer,
	in UserInfoInput,
) (*UserInfoResponse, error) {
	req := spec.AuthZRequest{
		Subject:  spec.AuthZSubject{Type: "Client", ID: in.ClientID},
		Action:   spec.ActionUserInfoRead,
		Resource: spec.AuthZResource{Type: "UserInfo", Properties: spec.AuthZResourceProps{Scopes: in.Scopes, Revoked: !in.Active}},
	}
	d := spec.Evaluate(req)
	if authorizer != nil {
		var err error
		d, err = authorizer.Authorize(ctx, req)
		if err != nil {
			return nil, err
		}
	}
	if !d.Permit {
		if slices.Contains(d.Reasons, "token_has_openid_scope") {
			return nil, NewOAuthError("insufficient_scope", "openid スコープが必要")
		}
		return nil, NewOAuthError("invalid_request", "userinfo 拒否: "+strings.Join(d.Reasons, ", "))
	}
	u, err := repo.FindBySubIncludingDeleted(ctx, in.Sub)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, NewOAuthError("invalid_request", "ユーザーが存在しません")
	}
	if u.IsDeleted() {
		return nil, NewOAuthError("invalid_token", "ユーザーは利用できません")
	}
	if !u.IsActive() {
		return nil, NewOAuthError("invalid_token", "ユーザーは無効化されています")
	}
	res := &UserInfoResponse{Sub: u.Sub}
	if slices.Contains(in.Scopes, "profile") {
		if u.Name != nil {
			res.Name = *u.Name
		}
		if u.GivenName != nil {
			res.GivenName = *u.GivenName
		}
		if u.FamilyName != nil {
			res.FamilyName = *u.FamilyName
		}
		res.PreferredUsername = u.PreferredUsername
		res.UpdatedAt = u.UpdatedAt.Unix()
	}
	if slices.Contains(in.Scopes, "email") && u.Email != nil {
		res.Email = *u.Email
		res.EmailVerified = u.EmailVerified
	}
	if in.ResolveAttributeDefs != nil {
		defs, err := in.ResolveAttributeDefs(ctx, u.TenantID)
		if err != nil {
			return nil, err
		}
		res.Extra = spec.ClaimsForScopes(*u, defs, in.Scopes)
	}
	return res, nil
}
