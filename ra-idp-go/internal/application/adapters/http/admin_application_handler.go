// Application カタログの管理 API (wi-69)。RequireAdmin で保護し、テナント境界に閉じる。
package http

import (
	"errors"
	"io"
	"net/http"
	"time"

	appusecases "ra-idp-go/internal/application/usecases"
	"ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/shared/spec"

	"github.com/labstack/echo/v5"
)

type protocolBindingResponse struct {
	Type     spec.ProtocolBindingType `json:"type"`
	ClientID string                   `json:"client_id,omitempty"`
	Wtrealm  string                   `json:"wtrealm,omitempty"`
}

type applicationResponse struct {
	ApplicationID string                    `json:"application_id"`
	Name          string                    `json:"name"`
	Kind          spec.ApplicationKind      `json:"kind"`
	Status        spec.ApplicationStatus    `json:"status"`
	IconURL       string                    `json:"icon_url,omitempty"`
	IconObjectKey string                    `json:"icon_object_key,omitempty"`
	LaunchURL     string                    `json:"launch_url,omitempty"`
	Bindings      []protocolBindingResponse `json:"bindings"`
	CategoryIDs   []string                  `json:"category_ids"`
	CreatedAt     time.Time                 `json:"created_at"`
	UpdatedAt     time.Time                 `json:"updated_at"`
}

type applicationUpdateRequest struct {
	Name      *string                 `json:"name"`
	Status    *spec.ApplicationStatus `json:"status"`
	LaunchURL *string                 `json:"launch_url"`
}

type protocolBindingRequest struct {
	Type     spec.ProtocolBindingType `json:"type"`
	ClientID string                   `json:"client_id"`
	Wtrealm  string                   `json:"wtrealm"`
}

type assignmentRequest struct {
	SubjectType spec.AssignmentSubjectType `json:"subject_type"`
	SubjectID   string                     `json:"subject_id"`
	Visibility  spec.AssignmentVisibility  `json:"visibility"`
}

type assignmentResponse struct {
	SubjectType spec.AssignmentSubjectType `json:"subject_type"`
	SubjectID   string                     `json:"subject_id"`
	Visibility  spec.AssignmentVisibility  `json:"visibility"`
	CreatedAt   time.Time                  `json:"created_at"`
}

func (d Deps) handleListApplications(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	apps, err := d.ApplicationRepo.ListByTenant(c.Request().Context(), support.RequestTenantID(c))
	if err != nil {
		return err
	}
	out := make([]applicationResponse, len(apps))
	for i, app := range apps {
		out[i] = toApplicationResponse(app)
	}
	return support.NoStoreJSON(c, http.StatusOK, map[string]any{"applications": out})
}

func (d Deps) handleGetApplication(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	app, err := d.ApplicationRepo.FindByID(c.Request().Context(), support.RequestTenantID(c), c.Param("application_id"))
	if err != nil {
		return err
	}
	if app == nil {
		return d.writeApplicationError(c, appusecases.ErrApplicationNotFound)
	}
	oidc, wsfed, saml := d.resolveProtocolConfig(c, app)
	return support.NoStoreJSON(c, http.StatusOK, map[string]any{
		"application": toApplicationResponse(app), "oidc": oidc, "wsfed": wsfed, "saml": saml,
	})
}

func (d Deps) handleUpdateApplication(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var req applicationUpdateRequest
	if err := support.DecodeJSON(c.Request(), &req); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	app, err := appusecases.UpdateApplication(c.Request().Context(), d.applicationDeps(), appusecases.UpdateApplicationInput{
		ActorSub: actor.Sub, ApplicationID: c.Param("application_id"),
		Name: req.Name, Status: req.Status, LaunchURL: req.LaunchURL, Now: time.Now().UTC(),
	})
	if err != nil {
		return d.writeApplicationError(c, err)
	}
	return support.NoStoreJSON(c, http.StatusOK, toApplicationResponse(app))
}

func (d Deps) handleUploadApplicationIcon(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	file, err := c.FormFile("file")
	if err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "アイコン画像ファイルを指定してください")
	}
	src, err := file.Open()
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(src, appusecases.MaxApplicationIconBytes+1))
	if closeErr := src.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	objectKey, err := spec.NewUUIDv4()
	if err != nil {
		return err
	}
	iconURL := support.TenantRoute(c, "/application-icons/"+c.Param("application_id")+"/"+objectKey)
	app, err := appusecases.UploadApplicationIcon(c.Request().Context(), d.applicationDeps(), appusecases.UploadApplicationIconInput{
		ActorSub: actor.Sub, ApplicationID: c.Param("application_id"), ObjectKey: objectKey,
		Data: data, IconURL: iconURL, Now: time.Now().UTC(),
	})
	if err != nil {
		return d.writeApplicationError(c, err)
	}
	return support.NoStoreJSON(c, http.StatusOK, map[string]any{"application": toApplicationResponse(app)})
}

func (d Deps) handleDeleteApplicationIcon(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	app, err := appusecases.DeleteApplicationIcon(
		c.Request().Context(), d.applicationDeps(), actor.Sub, c.Param("application_id"), time.Now().UTC(),
	)
	if err != nil {
		return d.writeApplicationError(c, err)
	}
	return support.NoStoreJSON(c, http.StatusOK, map[string]any{"application": toApplicationResponse(app)})
}

func (d Deps) handleGetApplicationIcon(c *echo.Context) error {
	if d.ApplicationIconStore == nil {
		return support.WriteBrowserError(c, http.StatusNotFound, "not_found", "アイコン画像が存在しません")
	}
	icon, err := d.ApplicationIconStore.Find(
		c.Request().Context(), support.RequestTenantID(c), c.Param("application_id"), c.Param("object_key"),
	)
	if err != nil {
		return err
	}
	if icon == nil {
		return support.WriteBrowserError(c, http.StatusNotFound, "not_found", "アイコン画像が存在しません")
	}
	c.Response().Header().Set("Content-Type", icon.ContentType)
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	c.Response().Header().Set("Cache-Control", "private, max-age=3600")
	return c.Blob(http.StatusOK, icon.ContentType, icon.Data)
}

func (d Deps) handleDeleteApplication(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if err := appusecases.DeleteApplication(
		c.Request().Context(), d.applicationDeps(), actor.Sub, c.Param("application_id"), time.Now().UTC(),
	); err != nil {
		return d.writeApplicationError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleAttachBinding(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var req protocolBindingRequest
	if err := support.DecodeJSON(c.Request(), &req); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	app, err := appusecases.AttachBinding(c.Request().Context(), d.applicationDeps(), appusecases.AttachBindingInput{
		ActorSub: actor.Sub, ApplicationID: c.Param("application_id"),
		Binding: spec.ProtocolBinding{Type: req.Type, ClientID: req.ClientID, Wtrealm: req.Wtrealm}, Now: time.Now().UTC(),
	})
	if err != nil {
		return d.writeApplicationError(c, err)
	}
	return support.NoStoreJSON(c, http.StatusCreated, toApplicationResponse(app))
}

func (d Deps) handleDetachBinding(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if err := appusecases.DetachBinding(
		c.Request().Context(), d.applicationDeps(), actor.Sub, c.Param("application_id"),
		spec.ProtocolBindingType(c.Param("binding_type")), time.Now().UTC(),
	); err != nil {
		return d.writeApplicationError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleListAssignments(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	assignments, err := appusecases.ListAssignments(c.Request().Context(), d.assignmentDeps(), c.Param("application_id"))
	if err != nil {
		return d.writeApplicationError(c, err)
	}
	out := make([]assignmentResponse, len(assignments))
	for i, a := range assignments {
		out[i] = toAssignmentResponse(a)
	}
	return support.NoStoreJSON(c, http.StatusOK, map[string]any{"assignments": out})
}

func (d Deps) handleAssignApplication(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var req assignmentRequest
	if err := support.DecodeJSON(c.Request(), &req); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	assignment, err := appusecases.AssignApplication(c.Request().Context(), d.assignmentDeps(), appusecases.AssignApplicationInput{
		ActorSub: actor.Sub, ApplicationID: c.Param("application_id"),
		SubjectType: req.SubjectType, SubjectID: req.SubjectID, Visibility: req.Visibility, Now: time.Now().UTC(),
	})
	if err != nil {
		return d.writeApplicationError(c, err)
	}
	return support.NoStoreJSON(c, http.StatusCreated, toAssignmentResponse(assignment))
}

func (d Deps) handleUnassignApplication(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if err := appusecases.UnassignApplication(
		c.Request().Context(), d.assignmentDeps(), actor.Sub, c.Param("application_id"),
		spec.AssignmentSubjectType(c.Param("subject_type")), c.Param("subject_id"), time.Now().UTC(),
	); err != nil {
		return d.writeApplicationError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) applicationDeps() appusecases.ApplicationDeps {
	return appusecases.ApplicationDeps{
		Repo: d.ApplicationRepo, IconStore: d.ApplicationIconStore,
		AssignmentRepo: d.ApplicationAssignmentRepo, Emit: d.Emit,
	}
}

func (d Deps) assignmentDeps() appusecases.AssignmentDeps {
	return appusecases.AssignmentDeps{
		Repo: d.ApplicationRepo, AssignmentRepo: d.ApplicationAssignmentRepo,
		OrderingRepo: d.ApplicationOrderingRepo, Emit: d.Emit,
	}
}

func (d Deps) writeApplicationError(c *echo.Context, err error) error {
	if errors.Is(err, appusecases.ErrApplicationNotFound) {
		return support.WriteBrowserError(c, http.StatusNotFound, "application_not_found", "アプリケーションが存在しません")
	}
	if errors.Is(err, appusecases.ErrApplicationIconRequired) ||
		errors.Is(err, appusecases.ErrApplicationIconTooLarge) ||
		errors.Is(err, appusecases.ErrApplicationIconFormat) {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_icon", err.Error())
	}
	return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", err.Error())
}

func toApplicationResponse(app *spec.Application) applicationResponse {
	bindings := make([]protocolBindingResponse, len(app.Bindings))
	for i, b := range app.Bindings {
		bindings[i] = protocolBindingResponse{Type: b.Type, ClientID: b.ClientID, Wtrealm: b.Wtrealm}
	}
	categoryIDs := app.CategoryIDs
	if categoryIDs == nil {
		categoryIDs = []string{}
	}
	return applicationResponse{
		ApplicationID: app.ApplicationID, Name: app.Name, Kind: app.Kind, Status: app.Status,
		IconURL: app.IconURL, IconObjectKey: app.IconObjectKey, LaunchURL: app.LaunchURL, Bindings: bindings, CategoryIDs: categoryIDs,
		CreatedAt: app.CreatedAt, UpdatedAt: app.UpdatedAt,
	}
}

func toAssignmentResponse(a *spec.ApplicationAssignment) assignmentResponse {
	return assignmentResponse{
		SubjectType: a.SubjectType, SubjectID: a.SubjectID, Visibility: a.Visibility, CreatedAt: a.CreatedAt,
	}
}
