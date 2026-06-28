package spec

// AssuranceVerification は assurance evidence ID を実行可能な検証 (テスト関数 / CI ジョブ) に
// 束ねる Go 側レジストリ。SCL 2.0 では assurance セクションは spec から外れたため、本マニフェストは
// テストと検証ファイルの対応を保つ独立した台帳として残す。
type AssuranceVerification struct {
	File  string
	Check string
}

var AssuranceManifest = map[string][]AssuranceVerification{
	"PkcePropertyTests": {
		{File: "ra-idp-go/internal/oauth2/domain/pkce_test.go", Check: "TestPKCES256Verifies"},
		{File: "ra-idp-go/internal/oauth2/domain/pkce_test.go", Check: "TestPKCES256RejectsMismatch"},
	},
	"AuthorizationCodeStoreContract": {
		{File: "ra-idp-go/internal/infrastructure/persistence/memory/memory_test.go", Check: "TestAuthorizationCodeRedeemIsAtomic"},
		{File: "ra-idp-go/internal/infrastructure/persistence/valkey/valkey_test.go", Check: "TestAuthorizationCodeRedeemOnce"},
	},
	"AuthorizationPolicyTests": {
		{File: "ra-idp-go/internal/oauth2/usecases/exchange_code_test.go", Check: "TestExchangeCodePKCEFailureDoesNotConsumeCode"},
		{File: "ra-idp-go/internal/oauth2/adapters/http/authorize_handler_test.go", Check: "TestAuthorize"},
	},
	"RefreshRotationPropertyTests": {
		{File: "ra-idp-go/internal/oauth2/usecases/exchange_code_test.go", Check: "TestExchangeCodeReplayRevokesRefreshFamily"},
		{File: "ra-idp-go/internal/oauth2/usecases/refresh_tokens_test.go", Check: "TestRefreshTokensRejectsAbsoluteTTLExpired"},
	},
	"RefreshStoreContract": {
		{File: "ra-idp-go/internal/oauth2/usecases/exchange_code_test.go", Check: "TestExchangeCodeReplayRevokesRefreshFamily"},
	},
	"TenantUseCaseTests": {
		{File: "ra-idp-go/internal/tenancy/usecases/manage_tenants_test.go", Check: "TestTenantLifecycle"},
	},
	"TenantHttpBoundaryTests": {
		{File: "ra-idp-go/internal/oauth2/adapters/http/admin_client_handler_test.go", Check: "TestAdminClientCannotCrossTenantBoundary"},
	},
	"TenantOAuthBoundaryTests": {
		{File: "ra-idp-go/internal/oauth2/usecases/tenant_isolation_test.go", Check: "TestAuthorizationCodeCannotCrossTenantBoundary"},
	},
	"PasswordProtectionTests": {
		{File: "ra-idp-go/internal/authentication/usecases/change_password_test.go", Check: "TestChangePasswordRejectsPasswordReuse"},
		{File: "ra-idp-go/internal/authentication/usecases/password_policy_test.go", Check: "TestValidatePasswordRejectsTooShort"},
	},
	"ResetTokenStorageTests": {
		{File: "ra-idp-go/internal/infrastructure/persistence/memory/password_reset_token_store_test.go", Check: "TestPasswordResetTokenStoreConsumeSucceedsOnceConcurrently"},
		{File: "ra-idp-go/internal/authentication/usecases/password_reset_test.go", Check: "TestResetPasswordWithTokenConsumesTokenAndUpdatesPassword"},
	},
	"PersistenceSecretContracts": {
		{File: "ra-idp-go/internal/oauth2/usecases/register_client_test.go", Check: "TestRegisterClientHashesSecret"},
		{File: "ra-idp-go/internal/oauth2/usecases/exchange_code_test.go", Check: "TestExchangeCodeReplayRevokesRefreshFamily"},
	},
	"SpecificationBindingTests": {
		{File: "ra-idp-go/internal/spec/coherence_test.go", Check: "TestCurrentSCLLoadsAllNormativeSections"},
		{File: "ra-idp-go/internal/spec/admin_policy_test.go", Check: "TestSCLPermissionsHaveGoActionMappings"},
	},
	"CoherenceCheck": {
		{File: "ra-idp-go/internal/spec/coherence_test.go", Check: "TestCurrentSCLIsInternallyCoherent"},
		{File: ".github/workflows/ra-idp-go-ci.yaml", Check: "Go tests and SCL coherence"},
	},
}
