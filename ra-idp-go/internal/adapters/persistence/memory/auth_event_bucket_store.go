package memory

// AuthEventBucketStore は AuthEventBucketStore (wi-20 スライス 3) の in-memory 実装。
// (tenant, kind, keyHash, windowStart) を鍵に 5 分窓の件数を積む。攻撃時の爆発を抑える
// のが目的なので個別行は持たず、窓ごとの集約 1 件だけを保持する。テスト / memory 構成用。

import (
	"context"
	"sort"
	"sync"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
)

const (
	authEventBucketDefaultListLimit = 100
	authEventBucketMaxListLimit     = 1000
	// authEventBucketMaxBuckets は古い窓が無限に溜まらないようにする上限 (aging)。
	authEventBucketMaxBuckets = 10000
)

type authEventBucketKey struct {
	tenantID    string
	kind        authports.AuthEventBucketKind
	keyHash     string
	windowStart int64
}

type AuthEventBucketStore struct {
	mu      sync.Mutex
	buckets map[authEventBucketKey]*authports.AuthEventBucket
}

func NewAuthEventBucketStore() *AuthEventBucketStore {
	return &AuthEventBucketStore{buckets: map[authEventBucketKey]*authports.AuthEventBucket{}}
}

func (s *AuthEventBucketStore) Record(
	_ context.Context,
	kind authports.AuthEventBucketKind,
	tenantID, keyHash string,
	now time.Time,
) (authports.AuthEventBucketResult, error) {
	windowStart := now.UTC().Truncate(authports.AuthEventBucketWindow)
	key := authEventBucketKey{
		tenantID: tenantID, kind: kind, keyHash: keyHash, windowStart: windowStart.Unix(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket, exists := s.buckets[key]
	first := !exists
	if first {
		bucket = &authports.AuthEventBucket{
			TenantID: tenantID, Kind: kind, KeyHash: keyHash,
			WindowStart: windowStart, FirstSeen: now.UTC(),
		}
		s.buckets[key] = bucket
		s.evictOldestLocked()
	}
	bucket.Count++
	bucket.LastSeen = now.UTC()
	return authports.AuthEventBucketResult{Bucket: *bucket, FirstInWindow: first}, nil
}

func (s *AuthEventBucketStore) List(
	_ context.Context,
	tenantID string,
	limit int,
) ([]authports.AuthEventBucket, error) {
	if limit <= 0 {
		limit = authEventBucketDefaultListLimit
	}
	if limit > authEventBucketMaxListLimit {
		limit = authEventBucketMaxListLimit
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]authports.AuthEventBucket, 0, len(s.buckets))
	for _, bucket := range s.buckets {
		if tenantID != "" && bucket.TenantID != tenantID {
			continue
		}
		out = append(out, *bucket)
	}
	// windowStart 降順 (新しい窓が先)、同窓は count 降順で安定化。
	sort.Slice(out, func(i, j int) bool {
		if !out[i].WindowStart.Equal(out[j].WindowStart) {
			return out[i].WindowStart.After(out[j].WindowStart)
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// evictOldestLocked は bucket 数が上限を超えたら最も古い窓から 1 件落とす。呼び出し前に lock 済み。
func (s *AuthEventBucketStore) evictOldestLocked() {
	if len(s.buckets) <= authEventBucketMaxBuckets {
		return
	}
	var oldestKey authEventBucketKey
	var oldest int64
	found := false
	for k := range s.buckets {
		if !found || k.windowStart < oldest {
			oldestKey, oldest, found = k, k.windowStart, true
		}
	}
	if found {
		delete(s.buckets, oldestKey)
	}
}
