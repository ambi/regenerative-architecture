// リプレイ防止 (DPoP / private_key_jwt assertion)。
package ports

import (
	"context"
	"time"
)

type DpopReplayStore interface {
	RecordIfNew(ctx context.Context, jti string, windowSeconds int, now time.Time) (bool, error)
}

type ClientAssertionReplayStore interface {
	RecordIfNew(ctx context.Context, jti string, windowSeconds int, now time.Time) (bool, error)
}
