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

type PasswordChanged struct {
	At  time.Time `json:"-"`
	Sub string    `json:"sub"`
}

func (e *PasswordChanged) EventType() string     { return "PasswordChanged" }
func (e *PasswordChanged) OccurredAt() time.Time { return e.At }

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
