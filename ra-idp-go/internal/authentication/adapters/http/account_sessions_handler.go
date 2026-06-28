// /api/account/sessions — エンドユーザー自身の有効なセッションの一覧と失効 (self-service,
// wi-20 スライス 2)。actor.sub に固定し、本人のセッションのみ参照・失効できる。失効は
// LoginSession を物理削除して SSO セッションを終了する。
package http

import (
	"net/http"
	"time"

	authnports "ra-idp-go/internal/authentication/ports"
	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/infrastructure/http/core"

	"github.com/labstack/echo/v5"
)

type accountSessionResponse struct {
	ID        string    `json:"id"`
	Current   bool      `json:"current"`
	AMR       []string  `json:"amr"`
	ACR       string    `json:"acr"`
	StartedAt time.Time `json:"started_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func toAccountSessionResponse(view authusecases.SessionView) accountSessionResponse {
	amr := view.AMR
	if amr == nil {
		amr = []string{}
	}
	return accountSessionResponse{
		ID: view.ID, Current: view.Current, AMR: amr, ACR: view.ACR,
		StartedAt: view.StartedAt, ExpiresAt: view.ExpiresAt,
	}
}

func (d Deps) sessionStore() authnports.SessionStore {
	if d.SessionManager == nil {
		return nil
	}
	return d.SessionManager.Store
}

func (d Deps) accountSessionDeps() authusecases.SessionDeps {
	return authusecases.SessionDeps{Store: d.sessionStore(), Emit: d.Emit}
}

// requireAuthenticatedSession は認証済み (pending でない) セッションの sub と sessionID を
// 返す。sessionID は "現在のセッション" の判定と revoke_others の除外に使う。
func (d Deps) requireAuthenticatedSession(c *echo.Context) (sub, sessionID string, err error) {
	authn, err := d.ResolveAuthentication(c)
	if err != nil {
		return "", "", err
	}
	if authn == nil || authn.AuthenticationPending {
		return "", "", core.ErrAdminAuthenticationRequired
	}
	return authn.Sub, authn.SessionID, nil
}

func (d Deps) handleListAccountSessions(c *echo.Context) error {
	sub, sessionID, err := d.requireAuthenticatedSession(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	views, err := authusecases.ListSessions(c.Request().Context(), d.sessionStore(), sub, sessionID)
	if err != nil {
		return err
	}
	sessions := make([]accountSessionResponse, len(views))
	for i, view := range views {
		sessions[i] = toAccountSessionResponse(view)
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"sessions": sessions})
}

func (d Deps) handleRevokeAccountSession(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	sub, _, err := d.requireAuthenticatedSession(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	if err := authusecases.RevokeOwnSession(
		c.Request().Context(), d.accountSessionDeps(), sub, c.Param("id"), time.Now().UTC(),
	); err != nil {
		return d.writeAccountError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleRevokeOtherAccountSessions(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	// 他の全セッションの失効は高 sensitivity 操作。step-up 再認証を要求する (ADR-043)。
	sub, sessionID, err := d.requireStepUpSession(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	if err := authusecases.RevokeOtherSessions(
		c.Request().Context(), d.accountSessionDeps(), sub, sessionID, time.Now().UTC(),
	); err != nil {
		return d.writeAccountError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}
