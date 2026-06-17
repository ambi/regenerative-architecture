package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ra-idp-go/internal/spec"
)

func TestAdminRolePoliciesRequireAdminRole(t *testing.T) {
	e, _, _ := newKeyAdminServer(t, keyAdminUser("plain", "acme", nil))
	rec := getAdminRolePolicies(e, "/realms/acme/api/admin/policy/roles")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminRolePoliciesHideSystemAdminPermissionsFromAdmin(t *testing.T) {
	e, _, _ := newKeyAdminServer(t, keyAdminUser("admin", "acme", []string{"admin"}))
	rec := getAdminRolePolicies(e, "/realms/acme/api/admin/policy/roles")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	roles := decodeAdminRolePolicies(t, rec)
	if hasAdminRolePermission(roles, "system_admin", "AdminTenantsManage") {
		t.Fatal("AdminTenantsManage is visible to plain admin")
	}
	if !hasAdminRolePermission(roles, "admin", "AdminUserRead") {
		t.Fatal("AdminUserRead is missing")
	}
}

func TestAdminRolePoliciesIncludeControlPlanePermissionsForSystemAdmin(t *testing.T) {
	e, _, _ := newKeyAdminServer(
		t,
		keyAdminUser("ops", spec.DefaultTenantID, []string{"system_admin"}),
	)
	rec := getAdminRolePolicies(e, "/realms/default/api/admin/policy/roles")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	roles := decodeAdminRolePolicies(t, rec)
	for _, permission := range []string{"AdminTenantsManage", "AdminKeysRotate"} {
		if !hasAdminRolePermission(roles, "system_admin", permission) {
			t.Fatalf("%s is missing", permission)
		}
	}
}

func getAdminRolePolicies(e interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}, path string,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func decodeAdminRolePolicies(t *testing.T, rec *httptest.ResponseRecorder) []adminRolePolicyResponse {
	t.Helper()
	var body struct {
		Roles []adminRolePolicyResponse `json:"roles"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Roles
}

func hasAdminRolePermission(roles []adminRolePolicyResponse, roleName, permissionName string) bool {
	for _, role := range roles {
		if role.Name != roleName {
			continue
		}
		for _, permission := range role.Permissions {
			if permission.Name == permissionName {
				return true
			}
		}
	}
	return false
}
