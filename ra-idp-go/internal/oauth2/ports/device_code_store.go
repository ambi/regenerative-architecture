package ports

import (
	"context"

	"ra-idp-go/internal/shared/spec"
)

type DeviceCodeStore interface {
	Save(ctx context.Context, rec *spec.DeviceAuthorization) error
	FindByDeviceCodeHash(ctx context.Context, hash string) (*spec.DeviceAuthorization, error)
	FindByUserCode(ctx context.Context, userCode string) (*spec.DeviceAuthorization, error)
	Update(ctx context.Context, rec *spec.DeviceAuthorization) error
	Exchange(ctx context.Context, deviceCodeHash string) (*spec.DeviceAuthorization, error)
	// DeleteAllForSub は ADR-036 の anonymize cascade から呼ばれる。
	// 対象 sub に既に紐付いた DeviceAuthorization を物理削除する。
	DeleteAllForSub(ctx context.Context, sub string) error
}
