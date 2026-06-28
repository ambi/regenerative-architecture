package spec

import (
	"testing"
	"time"
)

func validUser() User {
	now := time.Now().UTC()
	return User{
		Sub:               "user_alice",
		PreferredUsername: "alice",
		PasswordHash:      "$argon2id$v=19$m=19456,t=2,p=1$...",
		EmailVerified:     true,
		MfaEnrolled:       false,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func TestUserValidateAcceptsMinimumValidShape(t *testing.T) {
	if err := validUser().Validate(); err != nil {
		t.Fatalf("expected valid user, got %v", err)
	}
}

func TestUserValidateRejectsEmptySub(t *testing.T) {
	u := validUser()
	u.Sub = ""
	if err := u.Validate(); err == nil {
		t.Fatal("expected error for empty sub")
	}
}

func TestUserValidateRejectsOversizedUsername(t *testing.T) {
	u := validUser()
	long := make([]byte, 101)
	for i := range long {
		long[i] = 'x'
	}
	u.PreferredUsername = string(long)
	if err := u.Validate(); err == nil {
		t.Fatal("expected error for >100-char preferred_username")
	}
}

func TestUserValidateRejectsMalformedEmail(t *testing.T) {
	u := validUser()
	bad := "not-an-email"
	u.Email = &bad
	if err := u.Validate(); err == nil {
		t.Fatal("expected error for malformed email")
	}
}

func TestClientValidateRequiresGrantTypes(t *testing.T) {
	c := OAuth2Client{
		ClientID:                 "demo",
		ClientType:               ClientConfidential,
		RedirectURIs:             []string{"https://app.example.com/cb"},
		GrantTypes:               nil,
		TokenEndpointAuthMethod:  AuthMethodClientSecretBasic,
		IDTokenSignedResponseAlg: SigAlgPS256,
		FapiProfile:              FapiNone,
		CreatedAt:                time.Now().UTC(),
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for empty grant_types")
	}
}

func TestTransitionAuthorizationCodeFlowHappyPath(t *testing.T) {
	steps := []struct {
		from  AuthorizationCodeFlowState
		event AuthorizationCodeFlowEvent
		to    AuthorizationCodeFlowState
	}{
		{AuthFlowReceived, EventStartAuthentication, AuthFlowAuthenticationPending},
		{AuthFlowAuthenticationPending, EventAuthenticateUser, AuthFlowAuthenticated},
		{AuthFlowAuthenticated, EventRequestConsent, AuthFlowConsentPending},
		{AuthFlowConsentPending, EventGrantConsent, AuthFlowConsented},
		{AuthFlowConsented, EventIssueCode, AuthFlowCodeIssued},
		{AuthFlowCodeIssued, EventRedeemCode, AuthFlowExchanged},
	}
	for _, s := range steps {
		got, err := TransitionAuthorizationCodeFlow(s.from, s.event)
		if err != nil {
			t.Fatalf("transition %q on %q failed: %v", s.from, s.event, err)
		}
		if got != s.to {
			t.Fatalf("transition %q on %q: got %q, want %q", s.from, s.event, got, s.to)
		}
	}
}

func TestTransitionAuthorizationCodeFlowRejectsInvalidEdge(t *testing.T) {
	if _, err := TransitionAuthorizationCodeFlow(AuthFlowReceived, EventRedeemCode); err == nil {
		t.Fatal("expected error: cannot redeem from Received")
	}
}

func TestTransitionAuthorizationCodeRecordRejectsDoubleRedeem(t *testing.T) {
	mid, err := TransitionAuthorizationCodeRecord(AuthCodeRecordIssued, RecordEventRedeem)
	if err != nil {
		t.Fatalf("first redeem failed: %v", err)
	}
	if _, err := TransitionAuthorizationCodeRecord(mid, RecordEventRedeem); err == nil {
		t.Fatal("expected error: cannot redeem already-redeemed code")
	}
}
