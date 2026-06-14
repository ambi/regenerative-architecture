package spec

import "fmt"

type AssuranceVerification struct {
	File  string
	Check string
}

var AssuranceManifest = map[string][]AssuranceVerification{
	"PkcePropertyTests": {
		{File: "ra-idp-go/internal/oauth2/domain/pkce_test.go", Check: "TestPKCES256Verifies"},
		{File: "ra-idp/src/spec-bindings/property/pkce.property.test.ts", Check: "PKCE round-trip property"},
	},
	"AuthorizationCodeStoreContract": {
		{File: "ra-idp-go/internal/adapters/persistence/memory/memory_test.go", Check: "TestAuthorizationCodeRedeemIsAtomic"},
		{File: "ra-idp-go/internal/adapters/persistence/redis/redis_test.go", Check: "TestAuthorizationCodeRedeemOnce"},
	},
	"AuthorizationPolicyTests": {
		{File: "ra-idp-go/internal/oauth2/usecases/exchange_code_test.go", Check: "TestExchangeCodePKCEFailureDoesNotConsumeCode"},
		{File: "ra-idp/src/spec-bindings/policy/authorization.test.ts", Check: "token:grant_authorization_code"},
	},
	"RefreshRotationPropertyTests": {
		{File: "ra-idp/src/spec-bindings/property/refresh-rotation.property.test.ts", Check: "Refresh family transitive revoke"},
	},
	"RefreshStoreContract": {
		{File: "ra-idp-go/internal/oauth2/usecases/exchange_code_test.go", Check: "TestExchangeCodeReplayRevokesRefreshFamily"},
		{File: "ra-idp/src/spec-bindings/persistence-contract.test.ts", Check: "RefreshTokenStore"},
	},
	"TenantUseCaseTests": {
		{File: "ra-idp-go/internal/tenancy/usecases/manage_tenants_test.go", Check: "TestTenantLifecycle"},
	},
	"TenantHttpBoundaryTests": {
		{File: "ra-idp-go/internal/adapters/http/admin_client_handler_test.go", Check: "TestAdminClientCannotCrossTenantBoundary"},
	},
	"TenantOAuthBoundaryTests": {
		{File: "ra-idp-go/internal/oauth2/usecases/tenant_isolation_test.go", Check: "TestAuthorizationCodeCannotCrossTenantBoundary"},
	},
	"PasswordProtectionTests": {
		{File: "ra-idp-go/internal/authentication/usecases/change_password_test.go", Check: "TestChangePasswordRejectsPasswordReuse"},
		{File: "ra-idp-go/internal/authentication/usecases/password_policy_test.go", Check: "TestValidatePasswordRejectsTooShort"},
	},
	"ResetTokenStorageTests": {
		{File: "ra-idp-go/internal/adapters/persistence/memory/password_reset_token_store_test.go", Check: "TestPasswordResetTokenStoreConsumeSucceedsOnceConcurrently"},
		{File: "ra-idp-go/internal/authentication/usecases/password_reset_test.go", Check: "TestResetPasswordWithTokenConsumesTokenAndUpdatesPassword"},
	},
	"PersistenceSecretContracts": {
		{File: "ra-idp-go/internal/oauth2/usecases/register_client_test.go", Check: "TestRegisterClientHashesSecret"},
		{File: "ra-idp/src/spec-bindings/persistence-contract.test.ts", Check: "RefreshTokenStore"},
	},
	"SpecificationBindingTests": {
		{File: "ra-idp-go/internal/spec/coherence_test.go", Check: "TestCurrentSCLLoadsAllNormativeSections"},
		{File: "ra-idp/src/spec-bindings/invariants.test.ts", Check: "SCL ↔ TypeScript"},
	},
	"CoherenceCheck": {
		{File: "ra-idp-go/internal/spec/coherence_test.go", Check: "TestCurrentSCLIsInternallyCoherent"},
		{File: ".github/workflows/ra-idp-ci.yaml", Check: "Spec cross-reference coherence"},
	},
	"GeneratedArtifactDriftCheck": {
		{File: ".github/workflows/ra-idp-ci.yaml", Check: "Drift detection"},
	},
}

func (s *SCL) ValidateAssuranceManifest() error {
	expected := map[string]struct{}{}
	for _, obligation := range s.Assurance {
		for evidenceID := range obligation.Evidence {
			expected[evidenceID] = struct{}{}
		}
	}
	for evidenceID := range expected {
		if len(AssuranceManifest[evidenceID]) == 0 {
			return fmt.Errorf("assurance evidence %s has no verification binding", evidenceID)
		}
	}
	for evidenceID := range AssuranceManifest {
		if _, ok := expected[evidenceID]; !ok {
			return fmt.Errorf("assurance manifest contains unknown evidence %s", evidenceID)
		}
	}
	return nil
}
