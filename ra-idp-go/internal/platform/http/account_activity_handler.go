// /api/account/signin_activity — エンドユーザー自身のサインイン履歴 (self-service, wi-20)。
// 既存の監査イベントストアに蓄積済みの UserAuthenticated を発生時刻の降順で返す。
// admin 向けの /api/admin/users/{sub}/signin_activity も同じ射影を返す。
package http

import (
	"net/http"
	"strconv"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/platform/http/core"

	"github.com/labstack/echo/v5"
)

type signInActivityResponse struct {
	OccurredAt time.Time `json:"occurred_at"`
	AMR        []string  `json:"amr"`
}

func toSignInActivityResponses(items []authusecases.SignInActivity) []signInActivityResponse {
	out := make([]signInActivityResponse, len(items))
	for i, item := range items {
		amr := item.AMR
		if amr == nil {
			amr = []string{}
		}
		out[i] = signInActivityResponse{OccurredAt: item.OccurredAt, AMR: amr}
	}
	return out
}

// parseLimitParam は ?limit= を読み、不正値や未指定なら fallback を返す。
// 範囲のクランプは use case 側 (ListSignInActivity) が行う。
func parseLimitParam(c *echo.Context, fallback int) int {
	if raw := c.QueryParam("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func (d Deps) handleListSignInActivity(c *echo.Context) error {
	sub, err := d.requireAuthenticatedSub(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	limit := parseLimitParam(c, authusecases.SignInActivityDefaultLimit)
	items, err := authusecases.ListSignInActivity(
		c.Request().Context(), d.AuditEventRepo, core.RequestTenantID(c), sub, limit,
	)
	if err != nil {
		return err
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"activities": toSignInActivityResponses(items)})
}

func (d Deps) handleGetUserSignInActivity(c *echo.Context) error {
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	limit := parseLimitParam(c, authusecases.SignInActivityDefaultLimit)
	items, err := authusecases.ListSignInActivity(
		c.Request().Context(), d.AuditEventRepo, actor.TenantID, c.Param("sub"), limit,
	)
	if err != nil {
		return err
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"activities": toSignInActivityResponses(items)})
}
