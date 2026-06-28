package usecases

// ユーザー本人のサインイン履歴 (wi-20 スライス 1)。本スライスは新テーブルを作らず、
// 既存の監査イベントストア (AuditEventRepository) に蓄積済みの UserAuthenticated を
// 発生時刻の降順で読み出して射影する。IP / UA / sessionId などの付加属性は、イベント側を
// 拡張する後続スライス (ADR-041) で足す。

import (
	"context"
	"time"

	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/shared/spec"
)

const (
	// SignInActivityDefaultLimit は limit 未指定時に返す件数。
	SignInActivityDefaultLimit = 10
	// SignInActivityMaxLimit は 1 回の取得上限。
	SignInActivityMaxLimit = 50
)

// SignInActivity はサインイン履歴 1 件。occurred_at と amr のみ (本スライスの射影)。
type SignInActivity struct {
	OccurredAt time.Time
	AMR        []string
}

// ListSignInActivity は tenant 境界内で sub の UserAuthenticated を発生時刻の降順で
// 最大 limit 件返す。limit は [1, SignInActivityMaxLimit] にクランプし、0 以下は既定値。
func ListSignInActivity(
	ctx context.Context,
	repo oauthports.AuditEventRepository,
	tenantID, sub string,
	limit int,
) ([]SignInActivity, error) {
	if repo == nil {
		return []SignInActivity{}, nil
	}
	limit = clampSignInActivityLimit(limit)
	records, err := repo.List(ctx, oauthports.AuditEventQuery{
		TenantID: tenantID,
		Type:     (&spec.UserAuthenticated{}).EventType(),
		Sub:      sub,
		Limit:    limit,
	})
	if err != nil {
		return nil, err
	}
	activities := make([]SignInActivity, 0, len(records))
	for _, rec := range records {
		activities = append(activities, SignInActivity{
			OccurredAt: rec.OccurredAt,
			AMR:        stringSliceFromPayload(rec.Payload["amr"]),
		})
	}
	return activities, nil
}

func clampSignInActivityLimit(limit int) int {
	if limit <= 0 {
		return SignInActivityDefaultLimit
	}
	if limit > SignInActivityMaxLimit {
		return SignInActivityMaxLimit
	}
	return limit
}

// stringSliceFromPayload は JSON 由来の []any (各要素 string) を []string に変換する。
// 形が合わない要素は無視する。
func stringSliceFromPayload(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
