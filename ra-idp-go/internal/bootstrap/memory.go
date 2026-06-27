package bootstrap

import (
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/platform/crypto"
	"ra-idp-go/internal/platform/eventsink"
	"ra-idp-go/internal/platform/persistence/memory"
)

func assembleMemory() (*Dependencies, error) {
	keyStore, err := crypto.NewInMemoryKeyStore()
	if err != nil {
		return nil, err
	}
	return &Dependencies{
		TenantRepo:                memory.NewTenantRepository(),
		AttrSchemaRepo:            memory.NewTenantUserAttributeSchemaRepository(),
		ClientRepo:                memory.NewClientRepository(),
		UserRepo:                  memory.NewUserRepository(),
		GroupRepo:                 memory.NewGroupRepository(),
		AgentRepo:                 memory.NewAgentRepository(),
		MfaFactorRepo:             memory.NewMfaFactorRepository(),
		PasswordHistoryRepo:       memory.NewPasswordHistoryRepository(),
		PasswordResetTokenStore:   memory.NewPasswordResetTokenStore(),
		EmailChangeTokenStore:     memory.NewEmailChangeTokenStore(),
		ConsentRepo:               memory.NewConsentRepository(),
		AuthzDetailTypeRepo:       memory.NewAuthorizationDetailTypeRepository(),
		RequestStore:              memory.NewAuthorizationRequestStore(),
		CodeStore:                 memory.NewAuthorizationCodeStore(),
		PARStore:                  memory.NewPARStore(),
		RefreshStore:              memory.NewRefreshTokenStore(),
		DeviceCodeStore:           memory.NewDeviceCodeStore(),
		DpopReplay:                memory.NewDpopReplayStore(),
		ClientAssertionReplay:     memory.NewClientAssertionReplayStore(),
		AccessTokenDenylist:       memory.NewAccessTokenDenylist(),
		SessionStore:              memory.NewSessionStore(),
		KeyStore:                  oauthports.KeyStore(keyStore),
		EventSink:                 eventsink.NewConsoleSink(),
		AuditEventRepo:            memory.NewAuditEventStore(0),
		AuthEventBucketStore:      memory.NewAuthEventBucketStore(),
		WsFedRPRepo:               memory.NewWsFedRelyingPartyRepository(),
		ApplicationRepo:           memory.NewApplicationRepository(),
		ApplicationAssignmentRepo: memory.NewApplicationAssignmentRepository(),
		Close:                     func() {},
	}, nil
}
