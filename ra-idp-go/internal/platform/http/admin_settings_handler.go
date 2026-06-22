package http

import (
	"net/http"
	"slices"
	"time"

	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/spec"
	tenantusecases "ra-idp-go/internal/tenancy/usecases"

	"github.com/labstack/echo/v5"
)

// requireTenantAdmin は actor.tenant_id を権限境界として、admin / system_admin の
// いずれかが actor.tenant_id に居る場合にだけ通す。AdminSettings* permissions の
// allow_when と一致する。
func (d Deps) requireTenantAdmin(c *echo.Context) (*spec.User, error) {
	actor, err := d.resolveAdminActor(c)
	if err != nil {
		return nil, err
	}
	if actor.TenantID != core.RequestTenantID(c) {
		return nil, core.ErrAdminAccessDenied
	}
	if !slices.Contains(actor.Roles, "admin") && !slices.Contains(actor.Roles, "system_admin") {
		return nil, core.ErrAdminAccessDenied
	}
	return actor, nil
}

type adminSettingsResponse struct {
	TenantID               string                       `json:"tenant_id"`
	DisplayName            string                       `json:"display_name"`
	PasswordPolicyOverride *spec.PasswordPolicyOverride `json:"password_policy_override,omitempty"`
	PasswordPolicyDefaults passwordPolicyDefaults       `json:"password_policy_defaults"`
}

type passwordPolicyDefaults struct {
	MinLength    int `json:"min_length"`
	MaxLength    int `json:"max_length"`
	HistoryDepth int `json:"history_depth"`
}

type adminSettingsUpdateRequest struct {
	DisplayName            *string                      `json:"display_name,omitempty"`
	PasswordPolicyOverride *spec.PasswordPolicyOverride `json:"password_policy_override,omitempty"`
}

func (d Deps) handleGetAdminSettings(c *echo.Context) error {
	actor, err := d.requireTenantAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	tenant, err := d.TenantRepo.FindByID(c.Request().Context(), actor.TenantID)
	if err != nil {
		return err
	}
	if tenant == nil {
		return core.WriteBrowserError(c, http.StatusNotFound, "tenant_not_found", "テナントが存在しません")
	}
	return core.NoStoreJSON(c, http.StatusOK, d.toAdminSettingsResponse(tenant))
}

func (d Deps) handleUpdateAdminSettings(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.requireTenantAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var input adminSettingsUpdateRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	now := time.Now().UTC()
	tenant, err := tenantusecases.Update(
		c.Request().Context(), d.TenantRepo, actor.TenantID,
		tenantusecases.UpdateInput{
			DisplayName:            input.DisplayName,
			PasswordPolicyOverride: input.PasswordPolicyOverride,
		},
		d.tenantPolicyFloor(),
		now,
	)
	if err != nil {
		return d.writeTenantError(c, err)
	}
	if d.Emit != nil {
		d.Emit(&spec.TenantUpdated{
			At: now, ActorSub: actor.Sub, TenantID: tenant.ID,
			ChangedFields: adminSettingsChangedFields(input),
		})
	}
	return core.NoStoreJSON(c, http.StatusOK, d.toAdminSettingsResponse(tenant))
}

func (d Deps) toAdminSettingsResponse(t *spec.Tenant) adminSettingsResponse {
	floor := d.tenantPolicyFloor()
	return adminSettingsResponse{
		TenantID:               t.ID,
		DisplayName:            t.DisplayName,
		PasswordPolicyOverride: t.PasswordPolicyOverride,
		PasswordPolicyDefaults: passwordPolicyDefaults{
			MinLength:    floor.MinLength,
			MaxLength:    floor.MaxLength,
			HistoryDepth: floor.HistoryDepth,
		},
	}
}

func adminSettingsChangedFields(input adminSettingsUpdateRequest) []string {
	fields := []string{}
	if input.DisplayName != nil {
		fields = append(fields, "display_name")
	}
	if input.PasswordPolicyOverride != nil {
		fields = append(fields, "password_policy_override")
	}
	return fields
}
