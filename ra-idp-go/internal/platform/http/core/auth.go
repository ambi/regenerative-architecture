package core

import (
	"context"
	"errors"
	"net/http"
	"slices"

	authdomain "ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

var (
	ErrAdminAuthenticationRequired = errors.New("admin authentication required")
	ErrAdminAccessDenied           = errors.New("admin access denied")
)

// ResolveAuthentication はリクエストの認証セッションを解決し、対応する有効ユーザが
// リクエスト先テナントに属する場合のみ AuthenticationContext を返す。失効/無効/
// テナント不一致のセッションは未認証 (nil) として扱う (defense-in-depth)。
func (d Deps) ResolveAuthentication(c *echo.Context) (*authdomain.AuthenticationContext, error) {
	if d.AuthnResolver == nil {
		return nil, nil
	}
	authn, err := d.AuthnResolver.Resolve(
		c.Request().Context(),
		authdomain.HTTPHeadersAdapter{H: c.Request().Header},
	)
	if err != nil || authn == nil || d.UserRepo == nil {
		return authn, err
	}
	user, err := d.UserRepo.FindBySub(c.Request().Context(), authn.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.IsActive() {
		if d.SessionManager != nil && authn.SessionID != "" {
			_ = d.SessionManager.Store.Delete(c.Request().Context(), authn.SessionID)
		}
		return nil, nil
	}
	// cookie path 分離が破られた場合に備え、リクエスト先のテナントと User の所属テナントが
	// 一致しないセッションは未認証扱い (defense-in-depth)。
	if user.TenantID != RequestTenantID(c) {
		return nil, nil
	}
	return authn, nil
}

// RequireAdmin は認証済み + 有効ロールに admin を含むユーザを要求する。
// グループ由来ロールを含めた有効ロールで判定する (ADR-038)。
func (d Deps) RequireAdmin(c *echo.Context) (*spec.User, error) {
	authn, err := d.ResolveAuthentication(c)
	if err != nil {
		return nil, err
	}
	if authn == nil || authn.AuthenticationPending {
		return nil, ErrAdminAuthenticationRequired
	}
	user, err := d.UserRepo.FindBySub(c.Request().Context(), authn.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil || user.TenantID != RequestTenantID(c) || !user.IsActive() ||
		!slices.Contains(d.EffectiveRoles(c.Request().Context(), user), "admin") {
		return nil, ErrAdminAccessDenied
	}
	return user, nil
}

func (d Deps) WriteAdminAccessError(c *echo.Context, err error) error {
	if errors.Is(err, ErrAdminAuthenticationRequired) {
		return WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証済みセッションが必要です")
	}
	if errors.Is(err, ErrAdminAccessDenied) {
		return WriteBrowserError(c, http.StatusForbidden, "access_denied", "管理者権限が必要です")
	}
	return err
}

// EffectiveRoles は User の直接ロールにグループ由来ロールを合成して返す (ADR-038)。
func (d Deps) EffectiveRoles(ctx context.Context, user *spec.User) []string {
	if d.GroupRepo == nil {
		return user.Roles
	}
	groups, err := d.GroupRepo.ListGroupsByUser(ctx, user.TenantID, user.Sub)
	if err != nil {
		return user.Roles
	}
	return spec.EffectiveRoles(user.Roles, groups)
}

// WithEffectiveRoles は Roles を有効ロールへ差し替えた User の複製を返す。
func (d Deps) WithEffectiveRoles(ctx context.Context, user *spec.User) *spec.User {
	clone := *user
	clone.Roles = d.EffectiveRoles(ctx, user)
	return &clone
}
