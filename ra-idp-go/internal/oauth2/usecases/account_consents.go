package usecases

// エンドユーザー自身の Consent 操作 (self-service, wi-21)。
// SCL Authentication component の self インターフェース ListMyConsents / RevokeMyConsent。
// 取り消しは admin の RevokeConsent と同じく Consent レコードの論理撤回 + ConsentRevoked
// イベントで、actor.sub == target.sub に固定する。

import (
	"context"

	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

// ListConsentsForSub は指定 sub の active な (granted) Consent のみを返す。
// 接続済みアプリ一覧の用途で、revoked / expired は除外する。
func ListConsentsForSub(ctx context.Context, deps ConsentDeps, sub string) ([]*spec.Consent, error) {
	all, err := deps.ConsentRepo.FindAll(ctx, tenancy.TenantID(ctx))
	if err != nil {
		return nil, err
	}
	mine := make([]*spec.Consent, 0)
	for _, consent := range all {
		if consent.Sub == sub && consent.State == spec.ConsentGranted {
			mine = append(mine, consent)
		}
	}
	return mine, nil
}
