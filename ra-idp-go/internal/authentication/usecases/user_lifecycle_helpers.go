package usecases

import (
	"time"

	"ra-idp-go/internal/spec"
)

// removeRequiredAction は action を除いた新しいスライスを返す (元を破壊しない)。
func removeRequiredAction(actions []spec.RequiredAction, action spec.RequiredAction) []spec.RequiredAction {
	out := make([]spec.RequiredAction, 0, len(actions))
	for _, a := range actions {
		if a != action {
			out = append(out, a)
		}
	}
	return out
}

func normalizedNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}
