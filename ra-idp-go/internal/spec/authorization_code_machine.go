package spec

import "fmt"

// ===============================================================
// AuthorizationCodeFlow 状態機械
//
// SCL `states.AuthorizationCodeFlow` を Go の値で表現する。
// 不変条件: TransitionAuthorizationCodeFlow は SCL に宣言されていない
// 遷移を返さないこと（AuthCodeStateSafety プロパティに対応）。
// ===============================================================

type AuthorizationCodeFlowEvent string

const (
	EventStartAuthorization  AuthorizationCodeFlowEvent = "StartAuthorization"
	EventStartAuthentication AuthorizationCodeFlowEvent = "StartAuthentication"
	EventAuthenticateUser    AuthorizationCodeFlowEvent = "AuthenticateUser"
	EventRequestConsent      AuthorizationCodeFlowEvent = "RequestConsent"
	EventGrantConsent        AuthorizationCodeFlowEvent = "GrantConsent"
	EventIssueCode           AuthorizationCodeFlowEvent = "IssueCode"
	EventRedeemCode          AuthorizationCodeFlowEvent = "RedeemCode"
	EventRejectAuthorization AuthorizationCodeFlowEvent = "RejectAuthorization"
	EventExpireRequest       AuthorizationCodeFlowEvent = "ExpireRequest"
)

type authCodeFlowTransition struct {
	From  AuthorizationCodeFlowState
	Event AuthorizationCodeFlowEvent
	To    AuthorizationCodeFlowState
}

// 遷移表。SCL の states.AuthorizationCodeFlow.transitions と一致させる。
var authCodeFlowTransitions = []authCodeFlowTransition{
	{AuthFlowReceived, EventStartAuthentication, AuthFlowAuthenticationPending},
	{AuthFlowAuthenticationPending, EventAuthenticateUser, AuthFlowAuthenticated},
	{AuthFlowAuthenticated, EventRequestConsent, AuthFlowConsentPending},
	{AuthFlowConsentPending, EventGrantConsent, AuthFlowConsented},
	{AuthFlowAuthenticated, EventIssueCode, AuthFlowCodeIssued}, // 既存同意あり時の skip-consent
	{AuthFlowConsented, EventIssueCode, AuthFlowCodeIssued},
	{AuthFlowCodeIssued, EventRedeemCode, AuthFlowExchanged},
	{AuthFlowReceived, EventRejectAuthorization, AuthFlowRejected},
	{AuthFlowAuthenticationPending, EventRejectAuthorization, AuthFlowRejected},
	{AuthFlowConsentPending, EventRejectAuthorization, AuthFlowRejected},
	{AuthFlowReceived, EventExpireRequest, AuthFlowExpired},
	{AuthFlowAuthenticationPending, EventExpireRequest, AuthFlowExpired},
	{AuthFlowConsentPending, EventExpireRequest, AuthFlowExpired},
	{AuthFlowCodeIssued, EventExpireRequest, AuthFlowExpired},
}

func TransitionAuthorizationCodeFlow(from AuthorizationCodeFlowState, event AuthorizationCodeFlowEvent) (AuthorizationCodeFlowState, error) {
	for _, t := range authCodeFlowTransitions {
		if t.From == from && t.Event == event {
			return t.To, nil
		}
	}
	return "", fmt.Errorf("no transition from %q on event %q", from, event)
}

func IsAuthorizationCodeFlowTerminal(s AuthorizationCodeFlowState) bool {
	switch s {
	case AuthFlowExchanged, AuthFlowRejected, AuthFlowExpired:
		return true
	}
	return false
}

// ===============================================================
// AuthorizationCodeRecordLifecycle 状態機械
//
// AuthorizationCodeFlow とは別軸で、コード本体のライフサイクル。
// ===============================================================

type AuthorizationCodeRecordEvent string

const (
	RecordEventRedeem AuthorizationCodeRecordEvent = "RedeemCode"
	RecordEventExpire AuthorizationCodeRecordEvent = "ExpireCode"
)

type authCodeRecordTransition struct {
	From  AuthorizationCodeRecordState
	Event AuthorizationCodeRecordEvent
	To    AuthorizationCodeRecordState
}

var authCodeRecordTransitions = []authCodeRecordTransition{
	{AuthCodeRecordIssued, RecordEventRedeem, AuthCodeRecordRedeemed},
	{AuthCodeRecordIssued, RecordEventExpire, AuthCodeRecordExpired},
}

func TransitionAuthorizationCodeRecord(from AuthorizationCodeRecordState, event AuthorizationCodeRecordEvent) (AuthorizationCodeRecordState, error) {
	for _, t := range authCodeRecordTransitions {
		if t.From == from && t.Event == event {
			return t.To, nil
		}
	}
	return "", fmt.Errorf("no transition from %q on event %q", from, event)
}

func IsAuthorizationCodeRecordTerminal(s AuthorizationCodeRecordState) bool {
	switch s {
	case AuthCodeRecordRedeemed, AuthCodeRecordExpired:
		return true
	}
	return false
}
