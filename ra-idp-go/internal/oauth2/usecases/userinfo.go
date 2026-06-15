// /userinfo (OIDC Core §5.3)
package usecases

import (
	"context"
	"slices"
	"strings"

	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

type UserInfoInput struct {
	Scopes   []string
	Sub      string
	Active   bool
	ClientID string
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
}

func UserInfo(
	ctx context.Context,
	repo ports.UserRepository,
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
	if u.DisabledAt != nil {
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
	return res, nil
}
