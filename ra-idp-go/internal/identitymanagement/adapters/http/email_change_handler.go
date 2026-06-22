// /api/account/email/* — primary email の変更と新アドレスの再検証 (self-service, wi-21)。
package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	idmusecases "ra-idp-go/internal/identitymanagement/usecases"
	"ra-idp-go/internal/platform/http/core"

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
	csrf, err := d.EnsureCSRFCookie(c)
	if err != nil {
		return err
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]string{"csrf_token": csrf})
}

func (d Deps) handleRequestEmailChange(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	// primary email の変更は高 sensitivity 操作。step-up 再認証を要求する (ADR-043)。
	sub, err := d.requireStepUpSub(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	var input emailChangeRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	err = idmusecases.RequestEmailChange(c.Request().Context(), idmusecases.RequestEmailChangeDeps{
		UserRepo: d.UserRepo, TokenStore: d.EmailChangeTokenStore,
		EmailSender: d.EmailSender, Emit: d.Emit, Issuer: core.RequestIssuer(c, d.Issuer),
	}, idmusecases.RequestEmailChangeInput{Sub: sub, NewEmail: input.NewEmail, Now: time.Now().UTC()})
	if err != nil {
		return d.writeEmailChangeError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleConfirmEmailChange(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	var input emailChangeVerifyRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if strings.TrimSpace(input.Token) == "" {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "tokenが必要です")
	}
	if _, err := idmusecases.ConfirmEmailChange(c.Request().Context(), idmusecases.ConfirmEmailChangeDeps{
		UserRepo: d.UserRepo, TokenStore: d.EmailChangeTokenStore, Emit: d.Emit,
	}, idmusecases.ConfirmEmailChangeInput{Token: input.Token, Now: time.Now().UTC()}); err != nil {
		return d.writeEmailChangeError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]string{"status": "ok"})
}

func (d Deps) writeEmailChangeError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, idmusecases.ErrInvalidEmail):
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_email", "メールアドレスの形式が正しくありません")
	case errors.Is(err, idmusecases.ErrEmailUnchanged):
		return core.WriteBrowserError(c, http.StatusBadRequest, "email_unchanged", "現在のメールアドレスと同じです")
	case errors.Is(err, idmusecases.ErrEmailTaken):
		return core.WriteBrowserError(c, http.StatusConflict, "email_taken", "このメールアドレスは既に使われています")
	case errors.Is(err, idmusecases.ErrInvalidEmailChangeToken):
		return core.WriteBrowserError(c, http.StatusGone, "invalid_email_change_token", "確認リンクが無効か期限切れです")
	default:
		return d.writeAccountError(c, err)
	}
}
