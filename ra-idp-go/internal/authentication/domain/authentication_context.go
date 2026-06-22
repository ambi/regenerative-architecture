// Authentication bounded context の境界。OAuth2/OIDC ユースケースはこの context を消費するだけで、
// password 検証・user lookup・session cookie の詳細には踏み込まない。
package domain

import "context"

type AuthenticationContext struct {
	Sub                   string
	AuthTime              int64
	AMR                   []string
	ACR                   string
	SessionID             string
	AuthenticationPending bool
	// StepUpAt は直近の step-up 再認証時刻 (Unix 秒、未実施は 0)。高 sensitivity 操作の
	// recency gate (ADR-043) が AuthTime と合わせて評価する。
	StepUpAt int64
}

type AuthenticationContextResolver interface {
	Resolve(ctx context.Context, headers Headers) (*AuthenticationContext, error)
}

// Headers は HTTP framework 非依存の薄い抽象 (key → first value)。
type Headers interface {
	Get(key string) string
}

// HTTPHeadersAdapter は標準 http.Header から Headers への変換。
type HTTPHeadersAdapter struct {
	H interface{ Get(string) string }
}

func (h HTTPHeadersAdapter) Get(k string) string { return h.H.Get(k) }
