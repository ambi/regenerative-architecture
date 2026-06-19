package bootstrap

import (
	"ra-idp-go/internal/adapters/crypto"
	"ra-idp-go/internal/adapters/eventsink"
	"ra-idp-go/internal/adapters/persistence/memory"
	oauthports "ra-idp-go/internal/oauth2/ports"
)

func assembleMemory() (*Dependencies, error) {
	keyStore, err := crypto.NewInMemoryKeyStore()
	if err != nil {
		return nil, err
	}
	return &Dependencies{
		TenantRepo:              memory.NewTenantRepository(),
		ClientRepo:              memory.NewClientRepository(),
		UserRepo:                memory.NewUserRepository(),
		GroupRepo:               memory.NewGroupRepository(),
		MfaFactorRepo:           memory.NewMfaFactorRepository(),
		PasswordHistoryRepo:     memory.NewPasswordHistoryRepository(),
		PasswordResetTokenStore: memory.NewPasswordResetTokenStore(),
		ConsentRepo:             memory.NewConsentRepository(),
		RequestStore:            memory.NewAuthorizationRequestStore(),
		CodeStore:               memory.NewAuthorizationCodeStore(),
		PARStore:                memory.NewPARStore(),
		RefreshStore:            memory.NewRefreshTokenStore(),
		DeviceCodeStore:         memory.NewDeviceCodeStore(),
		DpopReplay:              memory.NewDpopReplayStore(),
		ClientAssertionReplay:   memory.NewClientAssertionReplayStore(),
		AccessTokenDenylist:     memory.NewAccessTokenDenylist(),
		SessionStore:            memory.NewSessionStore(),
		KeyStore:                oauthports.KeyStore(keyStore),
		EventSink:               eventsink.NewConsoleSink(),
		AuditEventRepo:          memory.NewAuditEventStore(0),
		Close:                   func() {},
	}, nil
}
