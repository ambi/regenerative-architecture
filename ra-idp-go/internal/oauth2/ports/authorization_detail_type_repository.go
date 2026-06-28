package ports

import (
	"context"

	"ra-idp-go/internal/shared/spec"
)

// AuthorizationDetailTypeRepository はテナント登録済みの authorization_details
// type 定義 (RFC 9396 / ADR-050) の永続境界。受理する type のスキーマ・表示
// テンプレート・運用状態を保持する。
type AuthorizationDetailTypeRepository interface {
	ListByTenant(ctx context.Context, tenantID string) ([]*spec.AuthorizationDetailType, error)
	FindByType(ctx context.Context, tenantID, detailType string) (*spec.AuthorizationDetailType, error)
	Save(ctx context.Context, t *spec.AuthorizationDetailType) error
	Delete(ctx context.Context, tenantID, detailType string) error
}
