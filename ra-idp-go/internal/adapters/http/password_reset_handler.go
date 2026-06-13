package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"

	"github.com/labstack/echo/v5"
)

type forgotPasswordAPIRequest struct {
	Email string `json:"email"`
}

type resetPasswordAPIRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (d Deps) handlePasswordResetContext(c *echo.Context) error {
	csrf, err := d.ensureCSRFCookie(c)
	if err != nil {
		return err
	}
	return noStoreJSON(c, http.StatusOK, map[string]string{"csrf_token": csrf})
}

func (d Deps) handleForgotPasswordAPI(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	var input forgotPasswordAPIRequest
	if err := decodeJSON(c.Request(), &input); err != nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	ttl := time.Duration(authusecases.PasswordResetTokenTTLSeconds) * time.Second
	if d.SCL != nil && d.SCL.Annotations.PasswordResetPolicy.TokenTTLSeconds > 0 {
		ttl = time.Duration(d.SCL.Annotations.PasswordResetPolicy.TokenTTLSeconds) * time.Second
	}
	if err := authusecases.RequestPasswordReset(
		c.Request().Context(),
		authusecases.RequestPasswordResetDeps{
			UserRepo: d.UserRepo, TokenStore: d.PasswordResetTokenStore,
			EmailSender: d.EmailSender, Emit: d.Emit, Issuer: d.Issuer, TokenTTL: ttl,
		},
		authusecases.RequestPasswordResetInput{Email: input.Email, Now: time.Now().UTC()},
	); err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleResetPasswordAPI(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	var input resetPasswordAPIRequest
	if err := decodeJSON(c.Request(), &input); err != nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if strings.TrimSpace(input.Token) == "" || input.NewPassword == "" {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "tokenと新しいパスワードが必要です")
	}
	historyDepth := authusecases.PasswordPolicyHistoryDepth
	if d.SCL != nil && d.SCL.Annotations.PasswordPolicy.HistoryDepth > 0 {
		historyDepth = d.SCL.Annotations.PasswordPolicy.HistoryDepth
	}
	_, err := authusecases.ResetPasswordWithToken(
		c.Request().Context(),
		authusecases.ResetPasswordWithTokenDeps{
			UserRepo: d.UserRepo, TokenStore: d.PasswordResetTokenStore,
			PasswordHasher: d.PasswordHasher, PasswordHistoryRepo: d.PasswordHistoryRepo,
			BreachedPasswordChecker: d.BreachedPasswordChecker,
			Emit:                    d.Emit, HistoryDepth: historyDepth,
		},
		authusecases.ResetPasswordWithTokenInput{
			Token: input.Token, NewPassword: input.NewPassword, Now: time.Now().UTC(),
		},
	)
	switch {
	case err == nil:
		return noStoreJSON(c, http.StatusOK, map[string]string{"status": "ok"})
	case errors.Is(err, authusecases.ErrInvalidResetToken):
		return writeBrowserError(c, http.StatusGone, "invalid_reset_token", "リセットリンクが無効か期限切れです")
	case errors.Is(err, authusecases.ErrPasswordReused):
		return writeBrowserError(c, http.StatusBadRequest, "password_reuse", "直近に使ったパスワードは再利用できません")
	default:
		var policyErr *authusecases.PasswordPolicyError
		if errors.As(err, &policyErr) {
			violations := make([]string, len(policyErr.Violations))
			for i, violation := range policyErr.Violations {
				violations[i] = string(violation)
			}
			return noStoreJSON(c, http.StatusBadRequest, map[string]any{
				"error": "password_policy", "message": "パスワードがセキュリティ要件を満たしていません。",
				"violations": violations,
			})
		}
		return err
	}
}
