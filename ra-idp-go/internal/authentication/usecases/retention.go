package usecases

// 認証イベント / 監査イベントの保持期間 sweep (ADR-045)。種類ごとに根拠ある保持期間を
// 決め、cutoff より古い行を確実に削除する。partition 化は行わない判断のため、retention が
// 単一テーブルの肥大と PII 滞留を抑える唯一の機構になる。
//
// 種類別の既定: 成功 / 一般監査 365 日・失敗詳細 30 日・bucket 集約 90 日・MFA 90 日・
// セッション 90 日。impersonation は本人保護のため短縮対象外 (global cap 未設定なら無期限)。
// global cap (MaxDays) はどの種類もこれを超えて保持しない上限。

import (
	"context"
	"time"

	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

// RetentionPolicy は種類別の保持日数と global cap。0 以下の日数は「無期限保持」を意味する。
type RetentionPolicy struct {
	SuccessDays    int
	FailDays       int
	AggregatedDays int
	MfaDays        int
	SessionDays    int
	// MaxDays は global cap (0 = 上限なし)。各種類はこれを超えて保持しない。
	MaxDays int
}

// DefaultRetentionPolicy は ADR-045 の既定値。
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		SuccessDays:    365,
		FailDays:       30,
		AggregatedDays: 90,
		MfaDays:        90,
		SessionDays:    90,
		MaxDays:        0,
	}
}

// 保持期間の種類分け。impersonation は短縮対象外なので Keep に入れる。
var (
	retentionFailTypes = []string{
		(&spec.AuthenticationFailed{}).EventType(),
		(&spec.AuthenticationStepFailed{}).EventType(),
	}
	retentionAggregatedTypes = []string{
		(&spec.AuthenticationEventAggregated{}).EventType(),
	}
	retentionMfaTypes = []string{
		(&spec.MfaChallengeIssued{}).EventType(),
		(&spec.MfaChallengeSucceeded{}).EventType(),
		(&spec.MfaChallengeFailed{}).EventType(),
		(&spec.BackupCodeConsumed{}).EventType(),
	}
	retentionSessionTypes = []string{
		(&spec.SessionStarted{}).EventType(),
		(&spec.SessionRefreshed{}).EventType(),
		(&spec.SessionEnded{}).EventType(),
	}
	retentionImpersonationTypes = []string{
		(&spec.SessionImpersonationStarted{}).EventType(),
		(&spec.SessionImpersonationEnded{}).EventType(),
	}
)

// capDays は global cap を適用する。days <= 0 (無期限) は cap があればそれに丸める。
func (p RetentionPolicy) capDays(days int) int {
	if p.MaxDays > 0 {
		if days <= 0 || days > p.MaxDays {
			return p.MaxDays
		}
	}
	return days
}

// AuditCutoff は now を基準に、監査イベント sweep の type 別 cutoff を組み立てる。
func (p RetentionPolicy) AuditCutoff(now time.Time) oauthports.RetentionCutoff {
	byType := map[string]time.Time{}
	assign := func(types []string, days int) {
		days = p.capDays(days)
		if days <= 0 {
			return
		}
		before := now.Add(-time.Duration(days) * 24 * time.Hour)
		for _, t := range types {
			byType[t] = before
		}
	}
	assign(retentionFailTypes, p.FailDays)
	assign(retentionAggregatedTypes, p.AggregatedDays)
	assign(retentionMfaTypes, p.MfaDays)
	assign(retentionSessionTypes, p.SessionDays)

	cutoff := oauthports.RetentionCutoff{ByType: byType}
	// impersonation は short 化対象外。global cap がある場合のみ cap で消す。
	if p.MaxDays > 0 {
		before := now.Add(-time.Duration(p.MaxDays) * 24 * time.Hour)
		for _, t := range retentionImpersonationTypes {
			byType[t] = before
		}
	} else {
		cutoff.Keep = append(cutoff.Keep, retentionImpersonationTypes...)
	}
	if d := p.capDays(p.SuccessDays); d > 0 {
		cutoff.Default = now.Add(-time.Duration(d) * 24 * time.Hour)
	}
	return cutoff
}

// BucketCutoff は authentication_event_buckets の削除境界 (集約は AggregatedDays)。
func (p RetentionPolicy) BucketCutoff(now time.Time) time.Time {
	days := p.capDays(p.AggregatedDays)
	if days <= 0 {
		return time.Time{}
	}
	return now.Add(-time.Duration(days) * 24 * time.Hour)
}

// AuditEventPurger / AuthEventBucketPurger は sweep が要求する削除境界。store の read 契約
// (AuditEventRepository / AuthEventBucketStore) とは分離し、sweep を持たない構成でも動く。
type AuditEventPurger interface {
	DeleteOlderThan(ctx context.Context, cutoff oauthports.RetentionCutoff) (int64, error)
}

type AuthEventBucketPurger interface {
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// RetentionSweepResult は 1 回の sweep で削除した件数。
type RetentionSweepResult struct {
	AuditEvents int64
	Buckets     int64
}

// RunRetentionSweep は監査イベントと bucket を保持期間に従って一括削除する。store が nil の
// 系統はスキップする。idempotent で、1 回で消し切れなくても次回で収束する。
func RunRetentionSweep(
	ctx context.Context,
	audit AuditEventPurger,
	buckets AuthEventBucketPurger,
	policy RetentionPolicy,
	now time.Time,
) (RetentionSweepResult, error) {
	var result RetentionSweepResult
	now = now.UTC()
	if audit != nil {
		deleted, err := audit.DeleteOlderThan(ctx, policy.AuditCutoff(now))
		if err != nil {
			return result, err
		}
		result.AuditEvents = deleted
	}
	if buckets != nil {
		if before := policy.BucketCutoff(now); !before.IsZero() {
			deleted, err := buckets.DeleteOlderThan(ctx, before)
			if err != nil {
				return result, err
			}
			result.Buckets = deleted
		}
	}
	return result, nil
}
