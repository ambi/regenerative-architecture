package usecases

import "ra-idp-go/internal/spec"

// emit はイベントを emit-callback に流す。Event 構造体側で At を埋めた状態で渡す前提。
func emit(f func(spec.DomainEvent), e spec.DomainEvent) {
	if f == nil {
		return
	}
	f(e)
}

func senderConstraintTag(sc *spec.SenderConstraint) string {
	if sc == nil {
		return "none"
	}
	return string(sc.Type)
}
