// 利用者ポータル向けの割当済みアプリ一覧と手動並び順 (wi-69, wi-70)。
// 認証済み利用者本人と所属グループに visible 割当された active アプリだけを返す。
package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	appports "ra-idp-go/internal/application/ports"
	appusecases "ra-idp-go/internal/application/usecases"
	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type myApplicationResponse struct {
	ApplicationID string               `json:"application_id"`
	Name          string               `json:"name"`
	Kind          spec.ApplicationKind `json:"kind"`
	IconURL       string               `json:"icon_url,omitempty"`
	LaunchURL     string               `json:"launch_url,omitempty"`
}

type reorderMyApplicationsRequest struct {
	ApplicationIDs []string `json:"application_ids"`
}

// errPortalUnauthorized は認証済み active 利用者が解決できなかったことを表す内部 sentinel。
var errPortalUnauthorized = errors.New("portal unauthorized")

func (d Deps) handleListMyApplications(c *echo.Context) error {
	user, err := d.resolvePortalUser(c)
	if err != nil {
		return d.writePortalAuthError(c, err)
	}
	ctx := c.Request().Context()
	subjects := d.subjectsForUser(ctx, user)
	apps, err := appusecases.ListMyApplications(ctx, d.assignmentDeps(), subjects)
	if err != nil {
		return err
	}
	order, err := appusecases.GetMyApplicationOrder(ctx, d.ApplicationOrderingRepo, user.Sub)
	if err != nil {
		return err
	}
	apps = appusecases.ApplyManualOrder(apps, order)
	out := make([]myApplicationResponse, len(apps))
	for i, app := range apps {
		out[i] = myApplicationResponse{
			ApplicationID: app.ApplicationID, Name: app.Name, Kind: app.Kind,
			IconURL: app.IconURL, LaunchURL: app.LaunchURL,
		}
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"applications": out})
}

// subjectsForUser は割当解決に使う subject 群 (本人 + 所属グループ) を組み立てる (wi-69)。
func (d Deps) subjectsForUser(ctx context.Context, user *spec.User) []appports.SubjectRef {
	subjects := []appports.SubjectRef{{Type: spec.AssignmentSubjectUser, ID: user.Sub}}
	if d.GroupRepo == nil {
		return subjects
	}
	groups, err := d.GroupRepo.ListGroupsByUser(ctx, user.TenantID, user.Sub)
	if err != nil {
		return subjects
	}
	for _, g := range groups {
		subjects = append(subjects, appports.SubjectRef{Type: spec.AssignmentSubjectGroup, ID: g.ID})
	}
	return subjects
}

// handleGetMyApplicationOrder は利用者の保存済み手動並び順を返す (wi-70)。
func (d Deps) handleGetMyApplicationOrder(c *echo.Context) error {
	user, err := d.resolvePortalUser(c)
	if err != nil {
		return d.writePortalAuthError(c, err)
	}
	order, err := appusecases.GetMyApplicationOrder(c.Request().Context(), d.ApplicationOrderingRepo, user.Sub)
	if err != nil {
		return err
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"application_ids": order})
}

// handleReorderMyApplications は利用者の手動並び順を検証して保存する (wi-70)。
func (d Deps) handleReorderMyApplications(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	user, err := d.resolvePortalUser(c)
	if err != nil {
		return d.writePortalAuthError(c, err)
	}
	var req reorderMyApplicationsRequest
	if err := core.DecodeJSON(c.Request(), &req); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	ctx := c.Request().Context()
	subjects := d.subjectsForUser(ctx, user)
	saved, err := appusecases.SaveMyApplicationOrder(ctx, d.assignmentDeps(), user.Sub, subjects, req.ApplicationIDs, time.Now().UTC())
	if err != nil {
		if errors.Is(err, appusecases.ErrUnassignedInOrder) {
			return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "割当されていないアプリは並び順に含められません")
		}
		return err
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"application_ids": saved})
}

// resolvePortalUser は認証済み (pending でない) active な利用者本人を解決する。
// 解決できなければ errPortalUnauthorized を返す。
func (d Deps) resolvePortalUser(c *echo.Context) (*spec.User, error) {
	authn, err := d.ResolveAuthentication(c)
	if err != nil {
		return nil, err
	}
	if authn == nil || authn.AuthenticationPending {
		return nil, errPortalUnauthorized
	}
	user, err := d.UserRepo.FindBySub(c.Request().Context(), authn.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil || user.TenantID != core.RequestTenantID(c) || !user.IsActive() {
		return nil, errPortalUnauthorized
	}
	return user, nil
}

func (d Deps) writePortalAuthError(c *echo.Context, err error) error {
	if errors.Is(err, errPortalUnauthorized) {
		return core.WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証済みセッションが必要です")
	}
	return err
}
