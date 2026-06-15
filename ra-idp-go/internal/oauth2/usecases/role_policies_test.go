package usecases

import (
	"slices"
	"testing"

	"ra-idp-go/internal/spec"
)

func TestListRolePoliciesFiltersControlPlanePermissions(t *testing.T) {
	scl := spec.MustLoadSCL()

	adminRoles, err := ListRolePolicies(scl, []string{"admin"}, false)
	if err != nil {
		t.Fatal(err)
	}
	admin := findRole(t, adminRoles, "admin")
	for _, permission := range []string{
		"AdminUserRead",
		"AdminUserCreate",
		"AdminUserUpdate",
		"AdminUserDelete",
	} {
		if !hasPermission(admin, permission) {
			t.Fatalf("admin permission %s is missing", permission)
		}
	}
	systemAdmin := findRole(t, adminRoles, "system_admin")
	if len(systemAdmin.Permissions) != 0 {
		t.Fatalf("system_admin permissions visible to plain admin: %+v", systemAdmin.Permissions)
	}

	controlPlaneRoles, err := ListRolePolicies(scl, []string{"system_admin"}, true)
	if err != nil {
		t.Fatal(err)
	}
	systemAdmin = findRole(t, controlPlaneRoles, "system_admin")
	for _, permission := range []string{"AdminTenantsManage", "AdminKeysRotate"} {
		if !hasPermission(systemAdmin, permission) {
			t.Fatalf("system_admin permission %s is missing", permission)
		}
	}
	if hasPermission(findRole(t, controlPlaneRoles, "admin"), "AdminTenantsManage") {
		t.Fatal("AdminTenantsManage must not appear under admin")
	}
}

func TestListRolePoliciesIncludesHTTPInterfaces(t *testing.T) {
	roles, err := ListRolePolicies(spec.MustLoadSCL(), []string{"admin"}, false)
	if err != nil {
		t.Fatal(err)
	}
	admin := findRole(t, roles, "admin")
	index := slices.IndexFunc(admin.Permissions, func(permission RolePermission) bool {
		return permission.Name == "AdminUserCreate"
	})
	if index < 0 {
		t.Fatal("AdminUserCreate is missing")
	}
	permission := admin.Permissions[index]
	if len(permission.Interfaces) != 1 ||
		permission.Action != spec.ActionAdminUserCreate ||
		permission.Interfaces[0].Name != "CreateAdminUser" ||
		permission.Interfaces[0].Method != "POST" ||
		permission.Interfaces[0].Path != "/api/admin/users" {
		t.Fatalf("unexpected interfaces: %+v", permission.Interfaces)
	}
}

func findRole(t *testing.T, roles []RolePolicy, name string) RolePolicy {
	t.Helper()
	index := slices.IndexFunc(roles, func(role RolePolicy) bool { return role.Name == name })
	if index < 0 {
		t.Fatalf("role %s is missing", name)
	}
	return roles[index]
}

func hasPermission(role RolePolicy, name string) bool {
	return slices.ContainsFunc(role.Permissions, func(permission RolePermission) bool {
		return permission.Name == name
	})
}
