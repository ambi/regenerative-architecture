package spec

import (
	"testing"
	"time"
)

func TestAdminConsentPolicyRequiresRoleAuthenticationAndTenantMatch(t *testing.T) {
	request := AuthZRequest{
		Subject: AuthZSubject{
			Type: "User",
			ID:   "admin",
			Properties: AuthZSubjectProps{
				Roles: []string{"admin"}, TenantID: "acme",
			},
		},
		Action: ActionAdminConsentsManage,
		Resource: AuthZResource{
			Type: "Consent", Properties: AuthZResourceProps{TenantID: "acme"},
		},
		Context: AuthZContext{Authenticated: true},
	}
	if result := Evaluate(request); !result.Permit {
		t.Fatalf("same-tenant admin denied: %v", result.Reasons)
	}

	request.Resource.Properties.TenantID = "other"
	if result := Evaluate(request); result.Permit {
		t.Fatal("cross-tenant admin was permitted")
	}

	request.Resource.Properties.TenantID = "acme"
	request.Subject.Properties.DisabledAt = ptrTime(time.Now())
	if result := Evaluate(request); result.Permit {
		t.Fatal("disabled admin was permitted")
	}
}

func TestSCLPermissionsHaveGoActionMappings(t *testing.T) {
	scl, err := LoadSCL()
	if err != nil {
		t.Fatal(err)
	}
	missing, extra := scl.SCLPermissionsCoverage()
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("permission mappings missing=%v extra=%v", missing, extra)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
