// /api/auth/account + /api/auth/change_password: 認証済みセッション向けの
// アカウントコンテキスト取得とパスワード変更。authentication コンテキスト所有。
package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/shared/adapters/http/support"
)

type accountContextResponse struct {
	CSRFToken         string   `json:"csrf_token"`
	Sub               string   `json:"sub"`
	PreferredUsername string   `json:"preferred_username,omitempty"`
	TenantID          string   `json:"tenant_id,omitempty"`
	Roles             []string `json:"roles,omitempty"`
}

type changePasswordAPIRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (d Deps) handleAccountContext(c *echo.Context) error {
	authn, err := d.ResolveAuthentication(c)
	if err != nil {
		return err
	}
	if authn == nil || authn.AuthenticationPending {
		return support.WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証済みセッションが必要です")
	}
	csrf, err := d.EnsureCSRFCookie(c)
	if err != nil {
		return err
	}
	resp := accountContextResponse{CSRFToken: csrf, Sub: authn.Sub}
	if d.UserRepo != nil {
		if user, _ := d.UserRepo.FindBySub(c.Request().Context(), authn.Sub); user != nil {
			resp.PreferredUsername = user.PreferredUsername
			resp.TenantID = user.TenantID
			// グループ由来ロールを含む有効ロールを返す (ADR-038)。
			resp.Roles = d.EffectiveRoles(c.Request().Context(), user)
		}
	}
	return support.NoStoreJSON(c, http.StatusOK, resp)
}

func (d Deps) handleChangePasswordAPI(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	authn, err := d.ResolveAuthentication(c)
	if err != nil {
		return err
	}
	if authn == nil || authn.AuthenticationPending {
		return support.WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証済みセッションが必要です")
	}
	// パスワード変更は高 sensitivity 操作。step-up 再認証を要求する (ADR-043)。
	if !authusecases.StepUpSatisfied(authn, time.Now().UTC()) {
		return support.WriteBrowserError(c, http.StatusForbidden, "step_up_required", "この操作には再認証が必要です")
	}
	var input changePasswordAPIRequest
	if err := support.DecodeJSON(c.Request(), &input); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if input.CurrentPassword == "" || input.NewPassword == "" {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "現在と新しいパスワードが必要です")
	}

	snap := d.resolvePasswordPolicy(c.Request().Context())
	_, err = authusecases.ChangePassword(c.Request().Context(), authusecases.ChangePasswordDeps{
		UserRepo:            d.UserRepo,
		PasswordHasher:      d.PasswordHasher,
		PasswordHistoryRepo: d.PasswordHistoryRepo,
		Emit:                d.Emit,
		Policy:              snap,
	}, authusecases.ChangePasswordInput{
		Sub:             authn.Sub,
		CurrentPassword: input.CurrentPassword,
		NewPassword:     input.NewPassword,
		Now:             time.Now().UTC(),
	})
	switch {
	case err == nil:
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.NoContent(http.StatusNoContent)
	case errors.Is(err, authusecases.ErrUserNotFound):
		return support.WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証済みセッションが無効です")
	case errors.Is(err, authusecases.ErrCurrentPasswordMismatch):
		return support.WriteBrowserError(c, http.StatusForbidden, "access_denied", "現在のパスワードが一致しません")
	case errors.Is(err, authusecases.ErrPasswordReused):
		return support.WriteBrowserError(c, http.StatusBadRequest, "password_reuse", "新しいパスワードは最近使用したものを再利用できません")
	default:
		var policyErr *authusecases.PasswordPolicyError
		if errors.As(err, &policyErr) {
			violations := make([]string, len(policyErr.Violations))
			for i, v := range policyErr.Violations {
				violations[i] = string(v)
			}
			return support.NoStoreJSON(c, http.StatusBadRequest, map[string]any{
				"error":      "password_policy",
				"message":    "パスワードがセキュリティ要件を満たしていません。",
				"violations": violations,
			})
		}
		return err
	}
}
