package usecases

// admin 向けの認証イベント集約 (bucket) 一覧 (wi-20 スライス 3)。攻撃時に個別行へ
// 落とさず集約した failed_login などの窓別件数を、新しい窓順で射影する。閾値超過時の
// 記録自体は recordLoginFailure が AuthEventBucketStore に対して行う。

import (
	"context"
	"time"

	authnports "ra-idp-go/internal/authentication/ports"
)

const (
	// AuthEventBucketDefaultLimit は limit 未指定時に返す件数。
	AuthEventBucketDefaultLimit = 50
	// AuthEventBucketMaxLimit は 1 回の取得上限。
	AuthEventBucketMaxLimit = 200
)

// AuthEventBucketView は bucket 1 件の admin 表示用射影。
type AuthEventBucketView struct {
	Kind        string
	KeyHash     string
	WindowStart time.Time
	Count       int
	FirstSeen   time.Time
	LastSeen    time.Time
}

// ListAuthEventBuckets は tenant 境界内の集約 bucket を新しい窓順で最大 limit 件返す。
// limit は [1, AuthEventBucketMaxLimit] にクランプし、0 以下は既定値。
func ListAuthEventBuckets(
	ctx context.Context,
	store authnports.AuthEventBucketStore,
	tenantID string,
	limit int,
) ([]AuthEventBucketView, error) {
	if store == nil {
		return []AuthEventBucketView{}, nil
	}
	limit = clampAuthEventBucketLimit(limit)
	buckets, err := store.List(ctx, tenantID, limit)
	if err != nil {
		return nil, err
	}
	views := make([]AuthEventBucketView, 0, len(buckets))
	for _, bucket := range buckets {
		views = append(views, AuthEventBucketView{
			Kind:        string(bucket.Kind),
			KeyHash:     bucket.KeyHash,
			WindowStart: bucket.WindowStart,
			Count:       bucket.Count,
			FirstSeen:   bucket.FirstSeen,
			LastSeen:    bucket.LastSeen,
		})
	}
	return views, nil
}

func clampAuthEventBucketLimit(limit int) int {
	if limit <= 0 {
		return AuthEventBucketDefaultLimit
	}
	if limit > AuthEventBucketMaxLimit {
		return AuthEventBucketMaxLimit
	}
	return limit
}
