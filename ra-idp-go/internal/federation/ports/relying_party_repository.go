// Package ports は Federation bounded context の永続境界 (port) を定義する。
package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

// WsFedRelyingPartyRepository は WS-Federation relying party 登録の永続境界 (wi-61)。
// wtrealm で識別し、許可 wreply・audience・claim policy を保持する。
type WsFedRelyingPartyRepository interface {
	// FindByWtrealm は wtrealm に一致する RP を返す。存在しなければ (nil, nil)。
	FindByWtrealm(ctx context.Context, tenantID, wtrealm string) (*spec.WsFedRelyingParty, error)
	// ListByTenant はテナント内の RP を wtrealm 昇順で返す。
	ListByTenant(ctx context.Context, tenantID string) ([]*spec.WsFedRelyingParty, error)
	// Save は RP を upsert する。
	Save(ctx context.Context, rp *spec.WsFedRelyingParty) error
	// Delete は wtrealm に一致する RP を削除する (冪等)。
	Delete(ctx context.Context, tenantID, wtrealm string) error
}
