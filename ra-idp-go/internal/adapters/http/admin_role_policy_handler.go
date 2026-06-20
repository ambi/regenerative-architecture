package http

import (
	"net/http"
	"regexp"
	"slices"
	"strings"

	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

// adrReference は SCL の説明文に埋め込まれた ADR 参照 (例 " (ADR-031 / ADR-032)")。
// 設計ドキュメントの語彙なので、管理者向けレスポンスからは取り除く。
var adrReference = regexp.MustCompile(`\s*\(ADR-[^)]*\)`)

// sanitizeAdminCopy は SCL 由来の説明文を管理 UI 向けに整える。内部の ADR 参照を
// 取り除くだけで文意は保つ。allow_when 式 (requirements) は別途レスポンスに含めない。
func sanitizeAdminCopy(text string) string {
	return strings.TrimSpace(adrReference.ReplaceAllString(text, ""))
}

type adminRolePolicyResponse struct {
	Name        string                        `json:"name"`
	Description string                        `json:"description"`
	Aliases     []string                      `json:"aliases"`
	Permissions []adminRolePermissionResponse `json:"permissions"`
}

type adminRolePermissionResponse struct {
	Name        string                       `json:"name"`
	Action      string                       `json:"action"`
	Description string                       `json:"description"`
	Interfaces  []adminRoleInterfaceResponse `json:"interfaces"`
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
			Name: permission.Name, Action: permission.Action,
			Description: sanitizeAdminCopy(permission.Description), Interfaces: interfaces,
		}
	}
	return adminRolePolicyResponse{
		Name: role.Name, Description: sanitizeAdminCopy(role.Description), Aliases: slices.Clone(role.Aliases),
		Permissions: permissions,
	}
}
