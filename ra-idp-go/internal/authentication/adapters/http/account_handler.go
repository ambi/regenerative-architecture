package http

import (
	"errors"
	"net/http"

	authusecases "ra-idp-go/internal/authentication/usecases"
	idmusecases "ra-idp-go/internal/identitymanagement/usecases"
	"ra-idp-go/internal/platform/http/core"

	"github.com/labstack/echo/v5"
)

func (d Deps) accountProfileDeps() idmusecases.AccountProfileDeps {
	return idmusecases.AccountProfileDeps{
		UserRepo: d.UserRepo, AttrSchemaRepo: d.AttrSchemaRepo, Emit: d.Emit,
	}
}

// requireAuthenticatedSub は認証済み (pending でない) セッションの sub を返す。
// self-service では actor == target なので sub をそのまま操作対象に使う。
func (d Deps) requireAuthenticatedSub(c *echo.Context) (string, error) {
	authn, err := d.ResolveAuthentication(c)
	if err != nil {
		return "", err
	}
	if authn == nil || authn.AuthenticationPending {
		return "", core.ErrAdminAuthenticationRequired
	}
	return authn.Sub, nil
}

func (d Deps) writeAccountError(c *echo.Context, err error) error {
	if handled, result := writeAccountMfaError(c, err); handled {
		return result
	}
	switch {
	case errors.Is(err, core.ErrAdminAuthenticationRequired):
		return core.WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証済みセッションが必要です")
	case errors.Is(err, authusecases.ErrStepUpRequired):
		return core.WriteBrowserError(c, http.StatusForbidden, "step_up_required", "この操作には再認証が必要です")
	case errors.Is(err, idmusecases.ErrUserNotFound):
		return core.WriteBrowserError(c, http.StatusNotFound, "user_not_found", "ユーザーが存在しません")
	case errors.Is(err, authusecases.ErrSessionNotFound):
		return core.WriteBrowserError(c, http.StatusNotFound, "session_not_found", "セッションが存在しません")
	default:
		return err
	}
}
