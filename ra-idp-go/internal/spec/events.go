package spec

import (
	"encoding/json"
	"time"
)

// DomainEvent は集計対象のドメインイベント。TS の z.discriminatedUnion('type', ...) に対応。
// Go では tagged interface + 各イベント型構造体で表現する。OccurredAt は各構造体が公開フィールド At を持つ。

type DomainEvent interface {
	EventType() string
	OccurredAt() time.Time
}

type ClientRegistered struct {
	At         time.Time  `json:"-"`
	ClientID   string     `json:"clientId"`
	ClientType ClientType `json:"clientType"`
}

func (e *ClientRegistered) EventType() string     { return "ClientRegistered" }
func (e *ClientRegistered) OccurredAt() time.Time { return e.At }

type UserAuthenticated struct {
	At  time.Time `json:"-"`
	Sub string    `json:"sub"`
	AMR []string  `json:"amr"`
}

func (e *UserAuthenticated) EventType() string     { return "UserAuthenticated" }
func (e *UserAuthenticated) OccurredAt() time.Time { return e.At }

type AuthenticationFailed struct {
	At       time.Time `json:"-"`
	Username string    `json:"username"`
	Reason   string    `json:"reason"`
}

func (e *AuthenticationFailed) EventType() string     { return "AuthenticationFailed" }
func (e *AuthenticationFailed) OccurredAt() time.Time { return e.At }

type LoginThrottled struct {
	At                time.Time `json:"-"`
	Kind              string    `json:"kind"`
	KeyHash           string    `json:"keyHash"`
	RetryAfterSeconds int       `json:"retryAfterSeconds"`
}

func (e *LoginThrottled) EventType() string     { return "LoginThrottled" }
func (e *LoginThrottled) OccurredAt() time.Time { return e.At }

type PasswordChanged struct {
	At  time.Time `json:"-"`
	Sub string    `json:"sub"`
}

func (e *PasswordChanged) EventType() string     { return "PasswordChanged" }
func (e *PasswordChanged) OccurredAt() time.Time { return e.At }

type PasswordResetRequested struct {
	At        time.Time `json:"-"`
	EmailHash string    `json:"emailHash"`
}

func (e *PasswordResetRequested) EventType() string     { return "PasswordResetRequested" }
func (e *PasswordResetRequested) OccurredAt() time.Time { return e.At }

type EmailSent struct {
	At        time.Time `json:"-"`
	ToHash    string    `json:"toHash"`
	Purpose   string    `json:"purpose"`
	Delivered bool      `json:"delivered"`
}

func (e *EmailSent) EventType() string     { return "EmailSent" }
func (e *EmailSent) OccurredAt() time.Time { return e.At }

type UserCreated struct {
	At        time.Time `json:"-"`
	ActorSub  string    `json:"actorSub"`
	TargetSub string    `json:"targetSub"`
}

func (e *UserCreated) EventType() string     { return "UserCreated" }
func (e *UserCreated) OccurredAt() time.Time { return e.At }

type UserUpdated struct {
	At            time.Time `json:"-"`
	ActorSub      string    `json:"actorSub"`
	TargetSub     string    `json:"targetSub"`
	ChangedFields []string  `json:"changedFields"`
}

func (e *UserUpdated) EventType() string     { return "UserUpdated" }
func (e *UserUpdated) OccurredAt() time.Time { return e.At }

type UserDisabled struct {
	At        time.Time `json:"-"`
	ActorSub  string    `json:"actorSub"`
	TargetSub string    `json:"targetSub"`
}

func (e *UserDisabled) EventType() string     { return "UserDisabled" }
func (e *UserDisabled) OccurredAt() time.Time { return e.At }

type UserEnabled struct {
	At        time.Time `json:"-"`
	ActorSub  string    `json:"actorSub"`
	TargetSub string    `json:"targetSub"`
}

func (e *UserEnabled) EventType() string     { return "UserEnabled" }
func (e *UserEnabled) OccurredAt() time.Time { return e.At }

type UserDeleted struct {
	At        time.Time `json:"-"`
	ActorSub  string    `json:"actorSub"`
	TargetSub string    `json:"targetSub"`
	Reason    string    `json:"reason,omitempty"`
}

func (e *UserDeleted) EventType() string     { return "UserDeleted" }
func (e *UserDeleted) OccurredAt() time.Time { return e.At }

type AdminClientCreated struct {
	At       time.Time `json:"-"`
	ActorSub string    `json:"actorSub"`
	ClientID string    `json:"clientId"`
}

func (e *AdminClientCreated) EventType() string     { return "AdminClientCreated" }
func (e *AdminClientCreated) OccurredAt() time.Time { return e.At }

type AdminClientUpdated struct {
	At            time.Time `json:"-"`
	ActorSub      string    `json:"actorSub"`
	ClientID      string    `json:"clientId"`
	ChangedFields []string  `json:"changedFields"`
}

func (e *AdminClientUpdated) EventType() string     { return "AdminClientUpdated" }
func (e *AdminClientUpdated) OccurredAt() time.Time { return e.At }

type AdminClientDeleted struct {
	At       time.Time `json:"-"`
	ActorSub string    `json:"actorSub"`
	ClientID string    `json:"clientId"`
}

func (e *AdminClientDeleted) EventType() string     { return "AdminClientDeleted" }
func (e *AdminClientDeleted) OccurredAt() time.Time { return e.At }

type ConsentGrantedEvent struct {
	At       time.Time `json:"-"`
	Sub      string    `json:"sub"`
	ClientID string    `json:"clientId"`
	Scopes   []string  `json:"scopes"`
}

func (e *ConsentGrantedEvent) EventType() string     { return "ConsentGranted" }
func (e *ConsentGrantedEvent) OccurredAt() time.Time { return e.At }

type ConsentRevokedEvent struct {
	At       time.Time `json:"-"`
	ActorSub string    `json:"actorSub,omitempty"`
	Sub      string    `json:"sub"`
	ClientID string    `json:"clientId"`
}

func (e *ConsentRevokedEvent) EventType() string     { return "ConsentRevoked" }
func (e *ConsentRevokedEvent) OccurredAt() time.Time { return e.At }

type AuthorizationCodeIssued struct {
	At                  time.Time           `json:"-"`
	ClientID            string              `json:"clientId"`
	Sub                 string              `json:"sub"`
	Scopes              []string            `json:"scopes"`
	CodeChallengeMethod CodeChallengeMethod `json:"codeChallengeMethod"`
}

func (e *AuthorizationCodeIssued) EventType() string     { return "AuthorizationCodeIssued" }
func (e *AuthorizationCodeIssued) OccurredAt() time.Time { return e.At }

type AuthorizationCodeRedeemed struct {
	At       time.Time `json:"-"`
	ClientID string    `json:"clientId"`
	Sub      string    `json:"sub"`
}

func (e *AuthorizationCodeRedeemed) EventType() string     { return "AuthorizationCodeRedeemed" }
func (e *AuthorizationCodeRedeemed) OccurredAt() time.Time { return e.At }

type AccessTokenIssued struct {
	At               time.Time `json:"-"`
	JTI              string    `json:"jti"`
	ClientID         string    `json:"clientId"`
	Sub              string    `json:"sub"`
	Scopes           []string  `json:"scopes"`
	SenderConstraint string    `json:"senderConstraint"` // "none" | "dpop" | "mtls"
}

func (e *AccessTokenIssued) EventType() string     { return "AccessTokenIssued" }
func (e *AccessTokenIssued) OccurredAt() time.Time { return e.At }

type RefreshTokenIssued struct {
	At       time.Time `json:"-"`
	TokenID  string    `json:"tokenId"`
	FamilyID string    `json:"familyId"`
	ParentID string    `json:"parentId,omitempty"`
	ClientID string    `json:"clientId"`
	Sub      string    `json:"sub"`
}

func (e *RefreshTokenIssued) EventType() string     { return "RefreshTokenIssued" }
func (e *RefreshTokenIssued) OccurredAt() time.Time { return e.At }

type RefreshTokenRotated struct {
	At         time.Time `json:"-"`
	OldTokenID string    `json:"oldTokenId"`
	NewTokenID string    `json:"newTokenId"`
	FamilyID   string    `json:"familyId"`
}

func (e *RefreshTokenRotated) EventType() string     { return "RefreshTokenRotated" }
func (e *RefreshTokenRotated) OccurredAt() time.Time { return e.At }

type TokenRevoked struct {
	At        time.Time `json:"-"`
	TokenType string    `json:"tokenType"` // "access_token" | "refresh_token"
	TokenID   string    `json:"tokenId"`
	Reason    string    `json:"reason"`
}

func (e *TokenRevoked) EventType() string     { return "TokenRevoked" }
func (e *TokenRevoked) OccurredAt() time.Time { return e.At }

type TokenIntrospected struct {
	At         time.Time `json:"-"`
	RSClientID string    `json:"rsClientId"`
	TokenID    string    `json:"tokenId"`
	Active     bool      `json:"active"`
}

func (e *TokenIntrospected) EventType() string     { return "TokenIntrospected" }
func (e *TokenIntrospected) OccurredAt() time.Time { return e.At }

type RefreshTokenReuseDetected struct {
	At       time.Time `json:"-"`
	FamilyID string    `json:"familyId"`
	TokenID  string    `json:"tokenId"`
	ClientID string    `json:"clientId"`
}

func (e *RefreshTokenReuseDetected) EventType() string     { return "RefreshTokenReuseDetected" }
func (e *RefreshTokenReuseDetected) OccurredAt() time.Time { return e.At }

type SigningKeyRotated struct {
	At          time.Time `json:"-"`
	NewKID      string    `json:"newKid"`
	PreviousKID string    `json:"previousKid"`
}

func (e *SigningKeyRotated) EventType() string     { return "SigningKeyRotated" }
func (e *SigningKeyRotated) OccurredAt() time.Time { return e.At }

type PARStored struct {
	At         time.Time `json:"-"`
	RequestURI string    `json:"requestUri"`
	ClientID   string    `json:"clientId"`
}

func (e *PARStored) EventType() string     { return "PARStored" }
func (e *PARStored) OccurredAt() time.Time { return e.At }

type DeviceAuthorizationRequested struct {
	At       time.Time `json:"-"`
	ClientID string    `json:"clientId"`
	Scopes   []string  `json:"scopes"`
}

func (e *DeviceAuthorizationRequested) EventType() string     { return "DeviceAuthorizationRequested" }
func (e *DeviceAuthorizationRequested) OccurredAt() time.Time { return e.At }

type DeviceAuthorizationApproved struct {
	At       time.Time `json:"-"`
	ClientID string    `json:"clientId"`
	Sub      string    `json:"sub"`
}

func (e *DeviceAuthorizationApproved) EventType() string     { return "DeviceAuthorizationApproved" }
func (e *DeviceAuthorizationApproved) OccurredAt() time.Time { return e.At }

type DeviceAuthorizationDenied struct {
	At       time.Time `json:"-"`
	ClientID string    `json:"clientId"`
	Sub      string    `json:"sub"`
}

func (e *DeviceAuthorizationDenied) EventType() string     { return "DeviceAuthorizationDenied" }
func (e *DeviceAuthorizationDenied) OccurredAt() time.Time { return e.At }

type TenantCreated struct {
	At       time.Time `json:"-"`
	ActorSub string    `json:"actorSub"`
	TenantID string    `json:"tenantId"`
}

func (e *TenantCreated) EventType() string     { return "TenantCreated" }
func (e *TenantCreated) OccurredAt() time.Time { return e.At }

type TenantUpdated struct {
	At            time.Time `json:"-"`
	ActorSub      string    `json:"actorSub"`
	TenantID      string    `json:"tenantId"`
	ChangedFields []string  `json:"changedFields"`
}

func (e *TenantUpdated) EventType() string     { return "TenantUpdated" }
func (e *TenantUpdated) OccurredAt() time.Time { return e.At }

type TenantDisabled struct {
	At       time.Time `json:"-"`
	ActorSub string    `json:"actorSub"`
	TenantID string    `json:"tenantId"`
}

func (e *TenantDisabled) EventType() string     { return "TenantDisabled" }
func (e *TenantDisabled) OccurredAt() time.Time { return e.At }

type TenantEnabled struct {
	At       time.Time `json:"-"`
	ActorSub string    `json:"actorSub"`
	TenantID string    `json:"tenantId"`
}

func (e *TenantEnabled) EventType() string     { return "TenantEnabled" }
func (e *TenantEnabled) OccurredAt() time.Time { return e.At }

type GroupCreated struct {
	At       time.Time `json:"-"`
	ActorSub string    `json:"actorSub"`
	GroupID  string    `json:"groupId"`
}

func (e *GroupCreated) EventType() string     { return "GroupCreated" }
func (e *GroupCreated) OccurredAt() time.Time { return e.At }

type GroupUpdated struct {
	At            time.Time `json:"-"`
	ActorSub      string    `json:"actorSub"`
	GroupID       string    `json:"groupId"`
	ChangedFields []string  `json:"changedFields"`
}

func (e *GroupUpdated) EventType() string     { return "GroupUpdated" }
func (e *GroupUpdated) OccurredAt() time.Time { return e.At }

type GroupDeleted struct {
	At       time.Time `json:"-"`
	ActorSub string    `json:"actorSub"`
	GroupID  string    `json:"groupId"`
}

func (e *GroupDeleted) EventType() string     { return "GroupDeleted" }
func (e *GroupDeleted) OccurredAt() time.Time { return e.At }

type GroupMemberAdded struct {
	At       time.Time `json:"-"`
	ActorSub string    `json:"actorSub"`
	GroupID  string    `json:"groupId"`
	UserSub  string    `json:"userSub"`
}

func (e *GroupMemberAdded) EventType() string     { return "GroupMemberAdded" }
func (e *GroupMemberAdded) OccurredAt() time.Time { return e.At }

type GroupMemberRemoved struct {
	At       time.Time `json:"-"`
	ActorSub string    `json:"actorSub"`
	GroupID  string    `json:"groupId"`
	UserSub  string    `json:"userSub"`
}

func (e *GroupMemberRemoved) EventType() string     { return "GroupMemberRemoved" }
func (e *GroupMemberRemoved) OccurredAt() time.Time { return e.At }

func MarshalDomainEvent(event DomainEvent) ([]byte, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	var wire map[string]any
	if err := json.Unmarshal(payload, &wire); err != nil {
		return nil, err
	}
	wire["type"] = event.EventType()
	wire["occurredAt"] = event.OccurredAt().UTC().Format(time.RFC3339Nano)
	return json.Marshal(wire)
}
