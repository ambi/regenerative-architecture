package usecases

import (
	"fmt"
	"slices"
	"sort"

	"ra-idp-go/internal/spec"
)

type RolePolicy struct {
	Name        string
	Description string
	Aliases     []string
	Permissions []RolePermission
}

type RolePermission struct {
	Name         string
	Action       string
	Description  string
	Requirements []string
	Interfaces   []RoleInterface
}

type RoleInterface struct {
	Name   string
	Method string
	Path   string
}

var rolePermissionInterfaces = map[string][]string{
	"AdminUserRead":        {"ListAdminUsers", "GetAdminUser"},
	"AdminUserCreate":      {"CreateAdminUser"},
	"AdminUserUpdate":      {"UpdateAdminUser", "DisableAdminUser", "EnableAdminUser"},
	"AdminUserDelete":      {"DeleteAdminUser"},
	"AdminClientsManage":   {"ListAdminClients", "GetAdminClient", "CreateAdminClient", "UpdateAdminClient", "DeleteAdminClient"},
	"AdminConsentsManage":  {"ListAdminConsents", "GetAdminConsent", "RevokeAdminConsent"},
	"AdminTenantsManage":   {"ListTenants", "GetTenant", "CreateTenant", "UpdateTenant", "DisableTenant", "EnableTenant"},
	"AdminAuditEventsRead": {"ListAdminAuditEvents", "GetAdminAuditEvent"},
	"AdminKeysRead":        {"ListAdminKeys", "GetAdminKey"},
	"AdminKeysRotate":      {"RotateAdminKey"},
}

func ListRolePolicies(scl *spec.SCL, actorRoles []string, controlPlane bool) ([]RolePolicy, error) {
	if scl == nil {
		return nil, fmt.Errorf("SCL is required")
	}
	roleDefinitions := []struct {
		name       string
		vocabulary string
	}{
		{name: "admin", vocabulary: "Administrator"},
		{name: "system_admin", vocabulary: "SystemAdministrator"},
	}
	roles := make([]RolePolicy, 0, len(roleDefinitions))
	for _, definition := range roleDefinitions {
		vocabulary, ok := scl.Vocabulary[definition.vocabulary]
		if !ok {
			return nil, fmt.Errorf("vocabulary %s is missing", definition.vocabulary)
		}
		role := RolePolicy{
			Name:        definition.name,
			Description: vocabulary.Definition,
			Aliases:     slices.Clone(vocabulary.Aliases),
		}
		for permissionName, permission := range scl.Permissions {
			if !permissionAppliesToRole(permission.AllowWhen, definition.name) {
				continue
			}
			if definition.name == "system_admin" &&
				(!slices.Contains(actorRoles, "system_admin") || !controlPlane) {
				continue
			}
			interfaces, err := rolePolicyInterfaces(scl, permissionName)
			if err != nil {
				return nil, err
			}
			action, ok := spec.ActionNameForPermission(permissionName)
			if !ok {
				return nil, fmt.Errorf("action for permission %s is not mapped", permissionName)
			}
			role.Permissions = append(role.Permissions, RolePermission{
				Name:         permissionName,
				Action:       action,
				Description:  permission.Description,
				Requirements: flattenConditions(permission.AllowWhen),
				Interfaces:   interfaces,
			})
		}
		sort.Slice(role.Permissions, func(i, j int) bool {
			return role.Permissions[i].Name < role.Permissions[j].Name
		})
		roles = append(roles, role)
	}
	return roles, nil
}

func permissionAppliesToRole(condition any, role string) bool {
	for _, requirement := range flattenConditions(condition) {
		switch role {
		case "admin":
			if requirement == "admin in actor.roles" ||
				requirement == "(admin in actor.roles) or (system_admin in actor.roles)" {
				return true
			}
		case "system_admin":
			if requirement == "system_admin in actor.roles" ||
				requirement == "(admin in actor.roles) or (system_admin in actor.roles)" {
				return true
			}
		}
	}
	return false
}

func flattenConditions(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []any:
		var out []string
		for _, item := range typed {
			out = append(out, flattenConditions(item)...)
		}
		return out
	case map[string]any:
		var out []string
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			out = append(out, flattenConditions(typed[key])...)
		}
		return out
	default:
		return nil
	}
}

func rolePolicyInterfaces(scl *spec.SCL, permissionName string) ([]RoleInterface, error) {
	names, ok := rolePermissionInterfaces[permissionName]
	if !ok {
		return nil, fmt.Errorf("interfaces for permission %s are not mapped", permissionName)
	}
	interfaces := make([]RoleInterface, 0, len(names))
	for _, name := range names {
		iface, ok := scl.Interfaces[name]
		if !ok {
			return nil, fmt.Errorf("interface %s for permission %s is missing", name, permissionName)
		}
		binding, ok := scl.HTTPBinding(iface)
		if !ok {
			return nil, fmt.Errorf("HTTP binding for interface %s is missing", name)
		}
		interfaces = append(interfaces, RoleInterface{
			Name:   name,
			Method: binding.String("method"),
			Path:   binding.String("path"),
		})
	}
	return interfaces, nil
}
