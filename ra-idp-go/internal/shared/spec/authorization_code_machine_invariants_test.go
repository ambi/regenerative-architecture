package spec

// AuthorizationCodeFlow 状態機械の不変条件 (property-based)。
// TS 側 invariants.test.ts の fast-check による property test を Go の
// math/rand ベースのランダム探索で再現する。
//
// 検証する不変条件:
//   1. 終端状態には発火可能なイベントが存在しない。
//   2. 遷移は決定論的（同じ (state, event) は常に同じ結果）。
//   3. 任意のイベント列を適用しても常に合法な状態に留まる。
//   4. exchanged から redeem_code は発火できない（重複交換不可）。
//   5. 開始状態から全状態が到達可能。

import (
	"math/rand/v2"
	"testing"
)

var allAuthCodeFlowStates = []AuthorizationCodeFlowState{
	AuthFlowReceived, AuthFlowAuthenticationPending, AuthFlowAuthenticated,
	AuthFlowConsentPending, AuthFlowConsented, AuthFlowCodeIssued,
	AuthFlowExchanged, AuthFlowRejected, AuthFlowExpired,
}

var allAuthCodeFlowEvents = []AuthorizationCodeFlowEvent{
	EventStartAuthorization, EventStartAuthentication, EventAuthenticateUser,
	EventRequestConsent, EventGrantConsent, EventIssueCode, EventRedeemCode,
	EventRejectAuthorization, EventExpireRequest,
}

func TestInvariantTerminalStatesHaveNoOutgoingTransitions(t *testing.T) {
	for _, s := range allAuthCodeFlowStates {
		if !IsAuthorizationCodeFlowTerminal(s) {
			continue
		}
		for _, e := range allAuthCodeFlowEvents {
			if _, err := TransitionAuthorizationCodeFlow(s, e); err == nil {
				t.Fatalf("terminal state %q accepted event %q", s, e)
			}
		}
	}
}

func TestInvariantTransitionIsDeterministic(t *testing.T) {
	for _, s := range allAuthCodeFlowStates {
		for _, e := range allAuthCodeFlowEvents {
			a, errA := TransitionAuthorizationCodeFlow(s, e)
			b, errB := TransitionAuthorizationCodeFlow(s, e)
			if (errA == nil) != (errB == nil) || a != b {
				t.Fatalf("non-deterministic at (%q, %q): %v/%v vs %v/%v", s, e, a, errA, b, errB)
			}
		}
	}
}

func TestInvariantAnyEventSequenceStaysInLegalStates(t *testing.T) {
	legal := make(map[AuthorizationCodeFlowState]struct{}, len(allAuthCodeFlowStates))
	for _, s := range allAuthCodeFlowStates {
		legal[s] = struct{}{}
	}
	r := rand.New(rand.NewPCG(1, 2))
	for trial := range 500 {
		state := AuthFlowReceived
		steps := r.IntN(30)
		for range steps {
			e := allAuthCodeFlowEvents[r.IntN(len(allAuthCodeFlowEvents))]
			if next, err := TransitionAuthorizationCodeFlow(state, e); err == nil {
				state = next
			}
		}
		if _, ok := legal[state]; !ok {
			t.Fatalf("trial %d ended in illegal state %q", trial, state)
		}
	}
}

func TestInvariantExchangedCannotRedeemAgain(t *testing.T) {
	if _, err := TransitionAuthorizationCodeFlow(AuthFlowExchanged, EventRedeemCode); err == nil {
		t.Fatal("expected error: exchanged → redeem_code must not transition")
	}
}

func TestInvariantAllStatesReachableFromReceived(t *testing.T) {
	reached := map[AuthorizationCodeFlowState]struct{}{AuthFlowReceived: {}}
	queue := []AuthorizationCodeFlowState{AuthFlowReceived}
	for len(queue) > 0 {
		head := queue[0]
		queue = queue[1:]
		for _, e := range allAuthCodeFlowEvents {
			next, err := TransitionAuthorizationCodeFlow(head, e)
			if err != nil {
				continue
			}
			if _, seen := reached[next]; seen {
				continue
			}
			reached[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	for _, s := range allAuthCodeFlowStates {
		if _, ok := reached[s]; !ok {
			t.Fatalf("state %q is unreachable from %q", s, AuthFlowReceived)
		}
	}
}
