package http

import (
	"net/http"
	"slices"

	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type adminRolePolicyResponse struct {
	Name        string                        `json:"name"`
	Description string                        `json:"description"`
	Aliases     []string                      `json:"aliases"`
	Permissions []adminRolePermissionResponse `json:"permissions"`
}

type adminRolePermissionResponse struct {
	Name         string                       `json:"name"`
	Action       string                       `json:"action"`
	Description  string                       `json:"description"`
	Requirements []string                     `json:"requirements"`
	Interfaces   []adminRoleInterfaceResponse `json:"interfaces"`
}

type adminRoleInterfaceResponse struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Path   string `json:"path"`
}

func (d Deps) handleListAdminRolePolicies(c *echo.Context) error {
	actor, err := d.resolveAdminActor(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	if !slices.Contains(actor.Roles, "admin") && !slices.Contains(actor.Roles, "system_admin") {
		return d.writeAdminAccessError(c, errAdminAccessDenied)
	}
	roles, err := usecases.ListRolePolicies(
		d.SCL,
		actor.Roles,
		requestTenantID(c) == spec.DefaultTenantID && actor.TenantID == spec.DefaultTenantID,
	)
	if err != nil {
		return err
	}
	response := make([]adminRolePolicyResponse, len(roles))
	for i, role := range roles {
		response[i] = toAdminRolePolicyResponse(role)
	}
	return noStoreJSON(c, http.StatusOK, map[string]any{"roles": response})
}

func toAdminRolePolicyResponse(role usecases.RolePolicy) adminRolePolicyResponse {
	permissions := make([]adminRolePermissionResponse, len(role.Permissions))
	for i, permission := range role.Permissions {
		interfaces := make([]adminRoleInterfaceResponse, len(permission.Interfaces))
		for j, iface := range permission.Interfaces {
			interfaces[j] = adminRoleInterfaceResponse(iface)
		}
		permissions[i] = adminRolePermissionResponse{
			Name: permission.Name, Action: permission.Action, Description: permission.Description,
			Requirements: slices.Clone(permission.Requirements), Interfaces: interfaces,
		}
	}
	return adminRolePolicyResponse{
		Name: role.Name, Description: role.Description, Aliases: slices.Clone(role.Aliases),
		Permissions: permissions,
	}
}
