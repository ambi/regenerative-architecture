// /api/account/email/* — primary email の変更と新アドレスの再検証 (self-service, wi-21)。
package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"

	"github.com/labstack/echo/v5"
)

type emailChangeRequest struct {
	NewEmail string `json:"new_email"`
}

type emailChangeVerifyRequest struct {
	Token string `json:"token"`
}

// handleEmailVerifyContext は未認証で開かれうる検証ページ用に CSRF トークンを発行する
// (handlePasswordResetContext と同方針)。
func (d Deps) handleEmailVerifyContext(c *echo.Context) error {
	csrf, err := d.ensureCSRFCookie(c)
	if err != nil {
		return err
	}
	return noStoreJSON(c, http.StatusOK, map[string]string{"csrf_token": csrf})
}

func (d Deps) handleRequestEmailChange(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	// primary email の変更は高 sensitivity 操作。step-up 再認証を要求する (ADR-043)。
	sub, err := d.requireStepUpSub(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	var input emailChangeRequest
	if err := decodeJSON(c.Request(), &input); err != nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	err = authusecases.RequestEmailChange(c.Request().Context(), authusecases.RequestEmailChangeDeps{
		UserRepo: d.UserRepo, TokenStore: d.EmailChangeTokenStore,
		EmailSender: d.EmailSender, Emit: d.Emit, Issuer: requestIssuer(c, d.Issuer),
	}, authusecases.RequestEmailChangeInput{Sub: sub, NewEmail: input.NewEmail, Now: time.Now().UTC()})
	if err != nil {
		return d.writeEmailChangeError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleConfirmEmailChange(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	var input emailChangeVerifyRequest
	if err := decodeJSON(c.Request(), &input); err != nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if strings.TrimSpace(input.Token) == "" {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "tokenが必要です")
	}
	if _, err := authusecases.ConfirmEmailChange(c.Request().Context(), authusecases.ConfirmEmailChangeDeps{
		UserRepo: d.UserRepo, TokenStore: d.EmailChangeTokenStore, Emit: d.Emit,
	}, authusecases.ConfirmEmailChangeInput{Token: input.Token, Now: time.Now().UTC()}); err != nil {
		return d.writeEmailChangeError(c, err)
	}
	return noStoreJSON(c, http.StatusOK, map[string]string{"status": "ok"})
}

func (d Deps) writeEmailChangeError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, authusecases.ErrInvalidEmail):
		return writeBrowserError(c, http.StatusBadRequest, "invalid_email", "メールアドレスの形式が正しくありません")
	case errors.Is(err, authusecases.ErrEmailUnchanged):
		return writeBrowserError(c, http.StatusBadRequest, "email_unchanged", "現在のメールアドレスと同じです")
	case errors.Is(err, authusecases.ErrEmailTaken):
		return writeBrowserError(c, http.StatusConflict, "email_taken", "このメールアドレスは既に使われています")
	case errors.Is(err, authusecases.ErrInvalidEmailChangeToken):
		return writeBrowserError(c, http.StatusGone, "invalid_email_change_token", "確認リンクが無効か期限切れです")
	default:
		return d.writeAccountError(c, err)
	}
}
