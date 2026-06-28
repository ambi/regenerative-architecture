package postgres

// AuthEventBucketStore は AuthEventBucketStore (ADR-041 / wi-44) を PostgreSQL に永続化する。
// 攻撃時に個別の AuthenticationFailed を 1 行ずつ書かず、(tenant_id, kind, key_hash, 5 分窓)
// 単位の 1 行へ畳み込む。Record は upsert 1 回で「窓ごとの件数を積む」+「その窓で最初の記録
// だったか」を返し、最初の記録だけが集約イベントを emit する。xmax=0 は当該 upsert が INSERT
// だったこと (= 窓内で最初) を示す PostgreSQL の慣用。

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	authnports "ra-idp-go/internal/authentication/ports"
)

const (
	authEventBucketDefaultListLimit = 100
	authEventBucketMaxListLimit     = 1000
)

type AuthEventBucketStore struct{ Pool *pgxpool.Pool }

func (s *AuthEventBucketStore) Record(
	ctx context.Context,
	kind authnports.AuthEventBucketKind,
	tenantID, keyHash string,
	now time.Time,
) (authnports.AuthEventBucketResult, error) {
	nowUTC := now.UTC()
	windowStart := nowUTC.Truncate(authnports.AuthEventBucketWindow)
	var (
		count     int64
		firstSeen time.Time
		lastSeen  time.Time
		inserted  bool
	)
	err := s.Pool.QueryRow(ctx, `
INSERT INTO authentication_event_buckets (tenant_id,kind,key_hash,window_start,count,first_seen,last_seen)
VALUES ($1,$2,$3,$4,1,$5,$5)
ON CONFLICT (tenant_id,kind,key_hash,window_start)
DO UPDATE SET count = authentication_event_buckets.count + 1, last_seen = EXCLUDED.last_seen
RETURNING count, first_seen, last_seen, (xmax = 0)`,
		tenantID, string(kind), keyHash, windowStart, nowUTC,
	).Scan(&count, &firstSeen, &lastSeen, &inserted)
	if err != nil {
		return authnports.AuthEventBucketResult{}, err
	}
	return authnports.AuthEventBucketResult{
		Bucket: authnports.AuthEventBucket{
			TenantID:    tenantID,
			Kind:        kind,
			KeyHash:     keyHash,
			WindowStart: windowStart,
			Count:       int(count),
			FirstSeen:   firstSeen.UTC(),
			LastSeen:    lastSeen.UTC(),
		},
		FirstInWindow: inserted,
	}, nil
}

// DeleteOlderThan は window_start が before より前の bucket を削除し、削除件数を返す
// (ADR-045 の保持期間 sweep / 既定 90 日)。idempotent。
func (s *AuthEventBucketStore) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.Pool.Exec(ctx,
		"DELETE FROM authentication_event_buckets WHERE window_start < $1", before.UTC())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *AuthEventBucketStore) List(
	ctx context.Context,
	tenantID string,
	limit int,
) ([]authnports.AuthEventBucket, error) {
	if limit <= 0 {
		limit = authEventBucketDefaultListLimit
	}
	if limit > authEventBucketMaxListLimit {
		limit = authEventBucketMaxListLimit
	}
	rows, err := s.Pool.Query(ctx, `
SELECT tenant_id,kind,key_hash,window_start,count,first_seen,last_seen
FROM authentication_event_buckets
WHERE ($1 = '' OR tenant_id = $1)
ORDER BY window_start DESC, count DESC
LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []authnports.AuthEventBucket{}
	for rows.Next() {
		var (
			b     authnports.AuthEventBucket
			kind  string
			count int64
		)
		if err := rows.Scan(
			&b.TenantID, &kind, &b.KeyHash, &b.WindowStart, &count, &b.FirstSeen, &b.LastSeen,
		); err != nil {
			return nil, err
		}
		b.Kind = authnports.AuthEventBucketKind(kind)
		b.Count = int(count)
		b.WindowStart = b.WindowStart.UTC()
		b.FirstSeen = b.FirstSeen.UTC()
		b.LastSeen = b.LastSeen.UTC()
		out = append(out, b)
	}
	return out, rows.Err()
}
