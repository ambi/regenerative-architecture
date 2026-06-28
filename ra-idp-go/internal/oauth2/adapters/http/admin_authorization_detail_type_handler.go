// authorization_details type 定義 (RFC 9396 / ADR-050) の管理 API。
// AdminAuthorizationDetailTypesManage で保護され、テナント境界に閉じる。
package http

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/shared/spec"

	"github.com/labstack/echo/v5"
)

type authorizationDetailTypeRequest struct {
	Type            string                            `json:"type"`
	Description     string                            `json:"description"`
	Schema          spec.AuthorizationDetailsSchema   `json:"schema"`
	DisplayTemplate string                            `json:"display_template"`
	State           spec.AuthorizationDetailTypeState `json:"state"`
}

func toAuthorizationDetailTypeResponse(t *spec.AuthorizationDetailType) map[string]any {
	return map[string]any{
		"tenant_id":        t.TenantID,
		"type":             t.Type,
		"description":      t.Description,
		"schema":           t.Schema,
		"display_template": t.DisplayTemplate,
		"state":            t.State,
		"created_at":       t.CreatedAt,
		"updated_at":       t.UpdatedAt,
	}
}

func (d Deps) handleListAuthorizationDetailTypes(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	types, err := d.AuthzDetailTypeRepo.ListByTenant(c.Request().Context(), support.RequestTenantID(c))
	if err != nil {
		return err
	}
	slices.SortFunc(types, func(a, b *spec.AuthorizationDetailType) int { return strings.Compare(a.Type, b.Type) })
	out := make([]map[string]any, len(types))
	for i, t := range types {
		out[i] = toAuthorizationDetailTypeResponse(t)
	}
	return support.NoStoreJSON(c, http.StatusOK, map[string]any{"types": out})
}

func (d Deps) handleGetAuthorizationDetailType(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	t, err := d.AuthzDetailTypeRepo.FindByType(c.Request().Context(), support.RequestTenantID(c), c.Param("type"))
	if err != nil {
		return err
	}
	if t == nil {
		return support.WriteBrowserError(c, http.StatusNotFound, "type_not_found", "authorization_details type が存在しません")
	}
	return support.NoStoreJSON(c, http.StatusOK, toAuthorizationDetailTypeResponse(t))
}

func (d Deps) handleCreateAuthorizationDetailType(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var req authorizationDetailTypeRequest
	if err := support.DecodeJSON(c.Request(), &req); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if strings.TrimSpace(req.Type) == "" {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "type が必要です")
	}
	tenantID := support.RequestTenantID(c)
	existing, err := d.AuthzDetailTypeRepo.FindByType(c.Request().Context(), tenantID, req.Type)
	if err != nil {
		return err
	}
	if existing != nil {
		return support.WriteBrowserError(c, http.StatusConflict, "type_exists", "同名の type が既に存在します")
	}
	now := time.Now().UTC()
	state := req.State
	if state == "" {
		state = spec.DetailTypeEnabled
	}
	t := &spec.AuthorizationDetailType{
		TenantID: tenantID, Type: req.Type, Description: req.Description,
		Schema: req.Schema, DisplayTemplate: req.DisplayTemplate, State: state,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := d.saveValidatedType(c, t); err != nil {
		return err
	}
	return support.NoStoreJSON(c, http.StatusCreated, toAuthorizationDetailTypeResponse(t))
}

func (d Deps) handleUpdateAuthorizationDetailType(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	tenantID := support.RequestTenantID(c)
	existing, err := d.AuthzDetailTypeRepo.FindByType(c.Request().Context(), tenantID, c.Param("type"))
	if err != nil {
		return err
	}
	if existing == nil {
		return support.WriteBrowserError(c, http.StatusNotFound, "type_not_found", "authorization_details type が存在しません")
	}
	var req authorizationDetailTypeRequest
	if err := support.DecodeJSON(c.Request(), &req); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	existing.Description = req.Description
	existing.Schema = req.Schema
	existing.DisplayTemplate = req.DisplayTemplate
	if req.State != "" {
		existing.State = req.State
	}
	existing.UpdatedAt = time.Now().UTC()
	if err := d.saveValidatedType(c, existing); err != nil {
		return err
	}
	return support.NoStoreJSON(c, http.StatusOK, toAuthorizationDetailTypeResponse(existing))
}

func (d Deps) handleDeleteAuthorizationDetailType(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if err := d.AuthzDetailTypeRepo.Delete(c.Request().Context(), support.RequestTenantID(c), c.Param("type")); err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

// saveValidatedType は登録 type を schema 検証してから保存する。display_template が
// 参照するフィールドが schema 規則に存在するかも軽く確認する (fail-closed 寄り)。
func (d Deps) saveValidatedType(c *echo.Context, t *spec.AuthorizationDetailType) error {
	if err := t.Validate(); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_type", err.Error())
	}
	for _, rule := range t.Schema.Rules {
		if !rule.Semantics.Valid() {
			return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_type", "未知の field semantics: "+string(rule.Semantics))
		}
	}
	if err := d.AuthzDetailTypeRepo.Save(c.Request().Context(), t); err != nil {
		return err
	}
	return nil
}
