package http

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/spec"
	tenantusecases "ra-idp-go/internal/tenancy/usecases"

	"github.com/labstack/echo/v5"
)

type tenantCreateRequest struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type tenantUpdateRequest struct {
	DisplayName            *string                      `json:"display_name,omitempty"`
	PasswordPolicyOverride *spec.PasswordPolicyOverride `json:"password_policy_override,omitempty"`
}

func (d Deps) handleListTenants(c *echo.Context) error {
	if _, err := d.requireSystemAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	tenants, err := d.TenantRepo.FindAll(c.Request().Context())
	if err != nil {
		return err
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"tenants": tenants})
}

func (d Deps) handleGetTenant(c *echo.Context) error {
	if _, err := d.requireSystemAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	tenant, err := d.TenantRepo.FindByID(c.Request().Context(), c.Param("tenant_id"))
	if err != nil {
		return err
	}
	if tenant == nil {
		return core.WriteBrowserError(c, http.StatusNotFound, "tenant_not_found", "テナントが存在しません")
	}
	return core.NoStoreJSON(c, http.StatusOK, tenant)
}

func (d Deps) handleCreateTenant(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.requireSystemAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var input tenantCreateRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	now := time.Now().UTC()
	tenant, err := tenantusecases.Create(
		c.Request().Context(), d.TenantRepo, input.ID, input.DisplayName, now,
	)
	if err != nil {
		return d.writeTenantError(c, err)
	}
	if d.Emit != nil {
		d.Emit(&spec.TenantCreated{At: now, ActorSub: actor.Sub, TenantID: tenant.ID})
	}
	return core.NoStoreJSON(c, http.StatusCreated, tenant)
}

func (d Deps) handleUpdateTenant(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.requireSystemAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var input tenantUpdateRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	now := time.Now().UTC()
	tenant, err := tenantusecases.Update(
		c.Request().Context(), d.TenantRepo, c.Param("tenant_id"),
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
			ChangedFields: tenantChangedFields(input),
		})
	}
	return core.NoStoreJSON(c, http.StatusOK, tenant)
}

func tenantChangedFields(input tenantUpdateRequest) []string {
	fields := []string{}
	if input.DisplayName != nil {
		fields = append(fields, "display_name")
	}
	if input.PasswordPolicyOverride != nil {
		fields = append(fields, "password_policy_override")
	}
	return fields
}

func (d Deps) tenantPolicyFloor() tenantusecases.PolicyFloor {
	floor := tenantusecases.PolicyFloor{}
	if d.SCL == nil {
		return floor
	}
	if v, ok := d.SCL.ObjectiveInt("PasswordPolicy", "min_length"); ok {
		floor.MinLength = v
	}
	if v, ok := d.SCL.ObjectiveInt("PasswordPolicy", "max_length"); ok {
		floor.MaxLength = v
	}
	if v, ok := d.SCL.ObjectiveInt("PasswordPolicy", "history_depth"); ok {
		floor.HistoryDepth = v
	}
	return floor
}

func (d Deps) handleDisableTenant(c *echo.Context) error {
	return d.handleSetTenantDisabled(c, true)
}

func (d Deps) handleEnableTenant(c *echo.Context) error {
	return d.handleSetTenantDisabled(c, false)
}

func (d Deps) handleSetTenantDisabled(c *echo.Context, disabled bool) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.requireSystemAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	now := time.Now().UTC()
	tenant, err := tenantusecases.SetDisabled(
		c.Request().Context(), d.TenantRepo, c.Param("tenant_id"), disabled, now,
	)
	if err != nil {
		return d.writeTenantError(c, err)
	}
	if d.Emit != nil {
		if disabled {
			d.Emit(&spec.TenantDisabled{At: now, ActorSub: actor.Sub, TenantID: tenant.ID})
		} else {
			d.Emit(&spec.TenantEnabled{At: now, ActorSub: actor.Sub, TenantID: tenant.ID})
		}
	}
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) requireSystemAdmin(c *echo.Context) (*spec.User, error) {
	authn, err := d.ResolveAuthentication(c)
	if err != nil {
		return nil, err
	}
	if authn == nil || authn.AuthenticationPending {
		return nil, core.ErrAdminAuthenticationRequired
	}
	user, err := d.UserRepo.FindBySub(c.Request().Context(), authn.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil || user.TenantID != spec.DefaultTenantID || !user.IsActive() ||
		!slices.Contains(d.EffectiveRoles(c.Request().Context(), user), "system_admin") {
		return nil, core.ErrAdminAccessDenied
	}
	return user, nil
}

func (d Deps) writeTenantError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, tenantusecases.ErrTenantNotFound):
		return core.WriteBrowserError(c, http.StatusNotFound, "tenant_not_found", "テナントが存在しません")
	case errors.Is(err, tenantusecases.ErrTenantConflict):
		return core.WriteBrowserError(c, http.StatusConflict, "tenant_conflict", "テナントIDは既に使用されています")
	case errors.Is(err, tenantusecases.ErrInvalidTenantID),
		errors.Is(err, tenantusecases.ErrDisplayNameEmpty),
		errors.Is(err, tenantusecases.ErrDefaultTenant):
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, tenantusecases.ErrPolicyOverrideWeaker):
		floor := d.tenantPolicyFloor()
		message := fmt.Sprintf(
			"パスワードポリシーは標準値より弱くできません (min_length≥%d / max_length≤%d / history_depth≥%d)",
			floor.MinLength, floor.MaxLength, floor.HistoryDepth,
		)
		return core.WriteBrowserError(c, http.StatusBadRequest, "policy_override_weaker", message)
	default:
		return err
	}
}
