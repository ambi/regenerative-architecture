// 利用者ポータル向けの割当済みアプリ一覧 (wi-69)。
// 認証済み利用者本人と所属グループに visible 割当された active アプリだけを返す。
package http

import (
	"context"
	"net/http"

	appports "ra-idp-go/internal/application/ports"
	appusecases "ra-idp-go/internal/application/usecases"
	"ra-idp-go/internal/platform/http/core"
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

func (d Deps) handleListMyApplications(c *echo.Context) error {
	authn, err := d.ResolveAuthentication(c)
	if err != nil {
		return err
	}
	if authn == nil || authn.AuthenticationPending {
		return core.WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証済みセッションが必要です")
	}
	ctx := c.Request().Context()
	user, err := d.UserRepo.FindBySub(ctx, authn.Sub)
	if err != nil {
		return err
	}
	if user == nil || user.TenantID != core.RequestTenantID(c) || !user.IsActive() {
		return core.WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証済みセッションが必要です")
	}

	subjects := d.subjectsForUser(ctx, user)
	apps, err := appusecases.ListMyApplications(ctx, d.assignmentDeps(), subjects)
	if err != nil {
		return err
	}
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
