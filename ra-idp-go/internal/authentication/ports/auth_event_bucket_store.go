package ports

// AuthEventBucketStore は、攻撃時にログイン失敗イベントを 1 行ずつ書かず 5 分窓の
// bucket に集約するための境界 (wi-20 スライス 3 / ADR-029 と同じ leaky-bucket 系の
// 語彙)。閾値超過 (throttle が Locked) の判定は呼び出し側 (recordLoginFailure) が
// 持ち、本 store は「窓ごとの件数を積み、その窓で最初の記録だったか」を返すことに
// 専念する。最初の記録だけが集約イベント (AuthenticationEventAggregated) を発火し、
// 以後の増分はここで count に積む。

import (
	"context"
	"time"
)

// AuthEventBucketWindow は集約の時間窓。bucket は now を本値で切り捨てた windowStart で束ねる。
const AuthEventBucketWindow = 5 * time.Minute

// AuthEventBucketKind は集約対象の種別。本スライスで実際に発火するのは failed_login のみ。
// throttled / mfa_failed は後続スライス用に語彙だけ確保する。
type AuthEventBucketKind string

const (
	AuthEventBucketFailedLogin AuthEventBucketKind = "failed_login"
	AuthEventBucketThrottled   AuthEventBucketKind = "throttled"
	AuthEventBucketMfaFailed   AuthEventBucketKind = "mfa_failed"
)

// AuthEventBucket は (tenant, kind, keyHash, windowStart) で一意な集約 1 件。
// keyHash は throttle の LoginThrottled.KeyHash と同じ sha256(key) hex で、tenant 境界内に閉じる。
type AuthEventBucket struct {
	TenantID    string
	Kind        AuthEventBucketKind
	KeyHash     string
	WindowStart time.Time
	Count       int
	FirstSeen   time.Time
	LastSeen    time.Time
}

// AuthEventBucketResult は Record の結果。FirstInWindow は当該窓で最初の記録だったかを示し、
// 呼び出し側はこれが true のときだけ集約イベントを emit する。
type AuthEventBucketResult struct {
	Bucket        AuthEventBucket
	FirstInWindow bool
}

type AuthEventBucketStore interface {
	// Record は now を含む 5 分窓の bucket に 1 件を積み、更新後の bucket と FirstInWindow を返す。
	Record(
		ctx context.Context,
		kind AuthEventBucketKind,
		tenantID, keyHash string,
		now time.Time,
	) (AuthEventBucketResult, error)
	// List は tenant 境界内の bucket を新しい窓順 (windowStart 降順) で最大 limit 件返す。
	List(ctx context.Context, tenantID string, limit int) ([]AuthEventBucket, error)
}
