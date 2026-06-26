package core

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

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
	authn, err := d.resolveAuthnContext(c)
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

// resolveAuthnContext は AuthenticationContext を解決する。OIDC RP 化した portal が
// 提示する Bearer access token を優先し ([[ADR-061]])、無ければ first-party セッション
// cookie で解決する (dual-mode)。Bearer は緊急セッションログイン経路と併存する。
func (d Deps) resolveAuthnContext(c *echo.Context) (*authdomain.AuthenticationContext, error) {
	if token := bearerToken(c); token != "" {
		if d.TokenIntrospector == nil {
			return nil, nil
		}
		res, err := d.TokenIntrospector.IntrospectAccessToken(c.Request().Context(), token)
		if err != nil {
			return nil, err
		}
		if res == nil || !res.Active || res.Sub == "" {
			return nil, nil
		}
		// access token 経由は session を持たないので SessionID は空。完全発行された
		// access token は認証完了を含意するため AuthenticationPending は false。
		return &authdomain.AuthenticationContext{Sub: res.Sub, AuthTime: res.Iat}, nil
	}
	if d.AuthnResolver == nil {
		return nil, nil
	}
	return d.AuthnResolver.Resolve(
		c.Request().Context(),
		authdomain.HTTPHeadersAdapter{H: c.Request().Header},
	)
}

// bearerToken は Authorization: Bearer <token> を抽出する。無ければ空文字を返す。
func bearerToken(c *echo.Context) string {
	const prefix = "bearer "
	h := c.Request().Header.Get("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
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

// ResolveAdminActor は認証済みかつ有効なユーザを、グループ由来ロールを合成した
// 形で返す。ロール別の細かな認可判定 (key reader / settings admin など) を呼び出し側に
// 委ねる管理系ハンドラが、actor の解決だけを共有するために使う。
func (d Deps) ResolveAdminActor(c *echo.Context) (*spec.User, error) {
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
	if user == nil || !user.IsActive() {
		return nil, ErrAdminAccessDenied
	}
	return d.WithEffectiveRoles(c.Request().Context(), user), nil
}

// RequireAuditReader は admin または system_admin ロールを持つ認証済みユーザを要求する。
// 監査イベントの閲覧と、そこから派生する認証イベントバケット閲覧が共有する。
func (d Deps) RequireAuditReader(c *echo.Context) (*spec.User, error) {
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
	if user == nil || !user.IsActive() {
		return nil, ErrAdminAccessDenied
	}
	actor := d.WithEffectiveRoles(c.Request().Context(), user)
	if !slices.Contains(actor.Roles, "admin") && !slices.Contains(actor.Roles, "system_admin") {
		return nil, ErrAdminAccessDenied
	}
	return actor, nil
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
