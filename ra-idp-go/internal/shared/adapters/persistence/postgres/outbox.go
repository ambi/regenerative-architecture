package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ra-idp-go/internal/shared/spec"
)

// eventTopics はドメインイベント種別を outbox トピックへ対応づける。
var eventTopics = map[string]string{
	"ClientRegistered": "oauth2.client.lifecycle.v1", "UserAuthenticated": "oauth2.authentication.v1",
	"AuthenticationFailed": "oauth2.authentication.v1", "LoginThrottled": "oauth2.security-incident.v1",
	"PasswordChanged": "oauth2.authentication.v1", "ConsentGranted": "oauth2.consent.v1",
	"ConsentRevoked": "oauth2.consent.v1", "AuthorizationCodeIssued": "oauth2.authorization-code.v1",
	"AuthorizationCodeRedeemed": "oauth2.authorization-code.v1", "AccessTokenIssued": "oauth2.token.v1",
	"RefreshTokenIssued": "oauth2.token.v1", "RefreshTokenRotated": "oauth2.token.v1",
	"TokenRevoked": "oauth2.token.v1", "TokenIntrospected": "oauth2.token.v1",
	"TokenExchanged": "oauth2.token.v1", "TokenExchangeRejected": "oauth2.token.v1",
	"RefreshTokenReuseDetected": "oauth2.security-incident.v1", "SigningKeyRotated": "oauth2.key-management.v1",
	"PARStored": "oauth2.par.v1", "DeviceAuthorizationRequested": "oauth2.device-authorization.v1",
	"DeviceAuthorizationApproved": "oauth2.device-authorization.v1", "DeviceAuthorizationDenied": "oauth2.device-authorization.v1",
	"TenantCreated": "tenancy.lifecycle.v1", "TenantUpdated": "tenancy.lifecycle.v1",
	"TenantDisabled": "tenancy.lifecycle.v1", "TenantEnabled": "tenancy.lifecycle.v1",
	"TenantUserAttributeSchemaUpdated": "tenancy.lifecycle.v1",
	"AdminOAuth2ClientCreated":         "oauth2.administration.v1", "AdminOAuth2ClientUpdated": "oauth2.administration.v1",
	"AdminOAuth2ClientDeleted": "oauth2.administration.v1",
	"GroupCreated":             "iam.groups.v1", "GroupUpdated": "iam.groups.v1", "GroupDeleted": "iam.groups.v1",
	"GroupMemberAdded": "iam.groups.v1", "GroupMemberRemoved": "iam.groups.v1",
	"AgentRegistered": "iam.agents.v1", "AgentUpdated": "iam.agents.v1", "AgentDisabled": "iam.agents.v1",
	"AgentEnabled": "iam.agents.v1", "AgentKilled": "iam.agents.v1", "AgentDeleted": "iam.agents.v1",
	"AgentOwnerChanged": "iam.agents.v1", "AgentCredentialBound": "iam.agents.v1",
	"AgentCredentialUnbound": "iam.agents.v1",
}

// OutboxEventSink はドメインイベントを outbox テーブルへ書き出す EventSink 実装。
type OutboxEventSink struct{ Pool *pgxpool.Pool }

func (s *OutboxEventSink) Emit(ctx context.Context, event spec.DomainEvent) error {
	topic := eventTopics[event.EventType()]
	if topic == "" {
		return fmt.Errorf("no topic mapping for event %s", event.EventType())
	}
	payload, err := spec.MarshalDomainEvent(event)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO outbox(event_type,topic,payload) VALUES ($1,$2,$3)`,
		event.EventType(), topic, string(payload))
	return err
}
