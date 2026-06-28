package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	oauth2http "ra-idp-go/internal/oauth2/adapters/http"

	"ra-idp-go/internal/shared/spec"
)

func TestAdminRolePoliciesOmitInternalDocReferences(t *testing.T) {
	e, _, _ := newKeyAdminServer(t, keyAdminUser("admin", "acme", []string{"admin"}))
	rec := getAdminRolePolicies(e, "/realms/acme/api/admin/policy/roles")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, leak := range []string{"ADR-", "actor.roles", "allow_when", `"requirements"`} {
		if strings.Contains(body, leak) {
			t.Fatalf("response leaks internal token %q: %s", leak, body)
		}
	}
	// 説明文 (description) には設計者向けの内部語を出さない。interface 名/path は
	// 技術的識別子なので対象外とし、description のみを検査する。
	internalTerms := []string{
		"User.roles", "SystemAdministrator", "tombstone", "reject", "Consent", "SCL",
	}
	for _, role := range decodeAdminRolePolicies(t, rec) {
		descriptions := []string{role.Description}
		for _, permission := range role.Permissions {
			descriptions = append(descriptions, permission.Description)
		}
		for _, description := range descriptions {
			for _, term := range internalTerms {
				if strings.Contains(description, term) {
					t.Fatalf("description leaks internal term %q: %q", term, description)
				}
			}
		}
	}
}

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

func decodeAdminRolePolicies(t *testing.T, rec *httptest.ResponseRecorder) []oauth2http.AdminRolePolicyResponse {
	t.Helper()
	var body struct {
		Roles []oauth2http.AdminRolePolicyResponse `json:"roles"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Roles
}

func hasAdminRolePermission(roles []oauth2http.AdminRolePolicyResponse, roleName, permissionName string) bool {
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
