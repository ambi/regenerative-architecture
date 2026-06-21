package bootstrap

// 認証 / 監査イベントの保持期間 sweep を周期 job として動かす (ADR-045)。store が削除境界
// (DeleteOlderThan) を実装していない構成ではスキップする。job は ctx のキャンセル
// (SIGINT/SIGTERM) で停止する。起動直後に 1 回走らせ、以後は interval ごとに回す。

import (
	"context"
	"log"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"
)

// startRetentionSweep は保持期間 sweep の goroutine を起動する。
func startRetentionSweep(ctx context.Context, deps *Dependencies, interval time.Duration) {
	audit, _ := deps.AuditEventRepo.(authusecases.AuditEventPurger)
	buckets, _ := deps.AuthEventBucketStore.(authusecases.AuthEventBucketPurger)
	if audit == nil && buckets == nil {
		return
	}
	if interval <= 0 {
		interval = time.Hour
	}
	policy := authusecases.DefaultRetentionPolicy()
	sweep := func() {
		res, err := authusecases.RunRetentionSweep(ctx, audit, buckets, policy, time.Now().UTC())
		if err != nil {
			log.Printf("retention sweep: %v", err)
			return
		}
		if res.AuditEvents > 0 || res.Buckets > 0 {
			log.Printf("retention sweep: deleted %d audit events, %d buckets", res.AuditEvents, res.Buckets)
		}
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		sweep()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweep()
			}
		}
	}()
}
