package policy

import "context"

type NoopBreachedPasswordChecker struct{}

func (NoopBreachedPasswordChecker) IsBreached(context.Context, string) bool {
	return false
}
