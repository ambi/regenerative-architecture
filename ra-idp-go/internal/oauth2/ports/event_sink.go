package ports

import (
	"context"

	"ra-idp-go/internal/shared/spec"
)

// EventSink はドメインイベントの出力先。observable side-effect の境界。
type EventSink interface {
	Emit(ctx context.Context, e spec.DomainEvent) error
}
