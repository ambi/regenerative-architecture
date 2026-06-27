// Package usecases は ApplicationCatalog の admin / account 操作を実装する (wi-69)。
package usecases

import (
	"errors"
	"time"

	"ra-idp-go/internal/spec"
)

var (
	ErrInvalidSubjectType = errors.New("invalid assignment subject type")
	ErrSubjectRequired    = errors.New("assignment subject id is required")
	ErrInvalidVisibility  = errors.New("invalid assignment visibility")
)

func emit(sink func(spec.DomainEvent), event spec.DomainEvent) {
	if sink != nil {
		sink(event)
	}
}

func adminNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}
