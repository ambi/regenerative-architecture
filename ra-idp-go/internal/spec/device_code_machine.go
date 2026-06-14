package spec

import "fmt"

// DeviceCodeFlow 状態機械 (RFC 8628)。
// SCL states.DeviceCodeFlow と一致する遷移表。

type DeviceCodeFlowEvent string

const (
	DeviceEventEnterUserCode DeviceCodeFlowEvent = "EnterUserCode"
	DeviceEventApprove       DeviceCodeFlowEvent = "Approve"
	DeviceEventDeny          DeviceCodeFlowEvent = "Deny"
	DeviceEventExchange      DeviceCodeFlowEvent = "Exchange"
	DeviceEventExpire        DeviceCodeFlowEvent = "Expire"
)

type deviceCodeFlowTransition struct {
	From  DeviceCodeFlowState
	Event DeviceCodeFlowEvent
	To    DeviceCodeFlowState
}

var deviceCodeFlowTransitions = []deviceCodeFlowTransition{
	{DeviceFlowIssued, DeviceEventEnterUserCode, DeviceFlowUserCodeEntered},
	{DeviceFlowUserCodeEntered, DeviceEventApprove, DeviceFlowApproved},
	{DeviceFlowUserCodeEntered, DeviceEventDeny, DeviceFlowDenied},
	{DeviceFlowApproved, DeviceEventExchange, DeviceFlowExchanged},
	{DeviceFlowIssued, DeviceEventExpire, DeviceFlowExpired},
	{DeviceFlowUserCodeEntered, DeviceEventExpire, DeviceFlowExpired},
	{DeviceFlowApproved, DeviceEventExpire, DeviceFlowExpired},
}

func TransitionDeviceCodeFlow(from DeviceCodeFlowState, event DeviceCodeFlowEvent) (DeviceCodeFlowState, error) {
	for _, t := range deviceCodeFlowTransitions {
		if t.From == from && t.Event == event {
			return t.To, nil
		}
	}
	return "", fmt.Errorf("no transition from %q on event %q", from, event)
}

func IsDeviceCodeFlowTerminal(s DeviceCodeFlowState) bool {
	switch s {
	case DeviceFlowDenied, DeviceFlowExchanged, DeviceFlowExpired:
		return true
	}
	return false
}

// DeviceCodePolling は RFC 8628 §3.5 のポーリング動作仕様。
type DeviceCodePolling struct {
	DefaultIntervalSeconds   int
	SlowDownIncrementSeconds int
}

func DefaultDeviceCodePolling() DeviceCodePolling {
	return DeviceCodePolling{DefaultIntervalSeconds: 5, SlowDownIncrementSeconds: 5}
}
