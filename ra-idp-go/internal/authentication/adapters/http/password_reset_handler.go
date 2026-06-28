package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"

	"github.com/labstack/echo/v5"
)

// resolvePasswordPolicy は global SCL + tenant override を合成した snapshot を返す。
// テナント解決失敗時はサイレントに global default にフォールバックする
// (パスワードポリシーで認証経路を落とすのは過剰)。
func (d Deps) resolvePasswordPolicy(ctx context.Context) authusecases.PasswordPolicySnapshot {
	defaults := spec.PasswordPolicySnapshot{
		MinLength:    authusecases.PasswordPolicyMinLength,
		MaxLength:    authusecases.PasswordPolicyMaxLength,
		HistoryDepth: authusecases.PasswordPolicyHistoryDepth,
	}
	var tenant *spec.Tenant
	if d.TenantRepo != nil {
		if id := tenancy.TenantID(ctx); id != "" {
			if found, err := d.TenantRepo.FindByID(ctx, id); err == nil {
				tenant = found
			}
		}
	}
	if d.SCL == nil {
		return authusecases.PasswordPolicySnapshot{
			MinLength:    defaults.MinLength,
			MaxLength:    defaults.MaxLength,
			HistoryDepth: defaults.HistoryDepth,
		}
	}
	resolved := d.SCL.ResolvePasswordPolicy(tenant, defaults)
	return authusecases.PasswordPolicySnapshot{
		MinLength:    resolved.MinLength,
		MaxLength:    resolved.MaxLength,
		HistoryDepth: resolved.HistoryDepth,
	}
}

type forgotPasswordAPIRequest struct {
	Email string `json:"email"`
}

type resetPasswordAPIRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (d Deps) handlePasswordResetContext(c *echo.Context) error {
	csrf, err := d.EnsureCSRFCookie(c)
	if err != nil {
		return err
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]string{"csrf_token": csrf})
}

func (d Deps) handleForgotPasswordAPI(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	var input forgotPasswordAPIRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	ttl := time.Duration(authusecases.PasswordResetTokenTTLSeconds) * time.Second
	if d.SCL != nil {
		if configured, ok := d.SCL.ObjectiveLifetime("PasswordResetTokenLifetime"); ok && configured > 0 {
			ttl = configured
		}
	}
	if err := authusecases.RequestPasswordReset(
		c.Request().Context(),
		authusecases.RequestPasswordResetDeps{
			UserRepo: d.UserRepo, TokenStore: d.PasswordResetTokenStore,
			EmailSender: d.EmailSender, Emit: d.Emit,
			Issuer: core.RequestIssuer(c, d.Issuer), TokenTTL: ttl,
		},
		authusecases.RequestPasswordResetInput{Email: input.Email, Now: time.Now().UTC()},
	); err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleResetPasswordAPI(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	var input resetPasswordAPIRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if strings.TrimSpace(input.Token) == "" || input.NewPassword == "" {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "tokenと新しいパスワードが必要です")
	}
	snap := d.resolvePasswordPolicy(c.Request().Context())
	_, err := authusecases.ResetPasswordWithToken(
		c.Request().Context(),
		authusecases.ResetPasswordWithTokenDeps{
			UserRepo: d.UserRepo, TokenStore: d.PasswordResetTokenStore,
			PasswordHasher: d.PasswordHasher, PasswordHistoryRepo: d.PasswordHistoryRepo,
			BreachedPasswordChecker: d.BreachedPasswordChecker,
			Emit:                    d.Emit, Policy: snap,
		},
		authusecases.ResetPasswordWithTokenInput{
			Token: input.Token, NewPassword: input.NewPassword, Now: time.Now().UTC(),
		},
	)
	switch {
	case err == nil:
		return core.NoStoreJSON(c, http.StatusOK, map[string]string{"status": "ok"})
	case errors.Is(err, authusecases.ErrInvalidResetToken):
		return core.WriteBrowserError(c, http.StatusGone, "invalid_reset_token", "リセットリンクが無効か期限切れです")
	case errors.Is(err, authusecases.ErrPasswordReused):
		return core.WriteBrowserError(c, http.StatusBadRequest, "password_reuse", "直近に使ったパスワードは再利用できません")
	default:
		var policyErr *authusecases.PasswordPolicyError
		if errors.As(err, &policyErr) {
			violations := make([]string, len(policyErr.Violations))
			for i, violation := range policyErr.Violations {
				violations[i] = string(violation)
			}
			return core.NoStoreJSON(c, http.StatusBadRequest, map[string]any{
				"error": "password_policy", "message": "パスワードがセキュリティ要件を満たしていません。",
				"violations": violations,
			})
		}
		return err
	}
}
