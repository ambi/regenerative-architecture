package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

type DeviceCodeStore interface {
	Save(ctx context.Context, rec *spec.DeviceAuthorization) error
	FindByDeviceCodeHash(ctx context.Context, hash string) (*spec.DeviceAuthorization, error)
	FindByUserCode(ctx context.Context, userCode string) (*spec.DeviceAuthorization, error)
	Update(ctx context.Context, rec *spec.DeviceAuthorization) error
	Exchange(ctx context.Context, deviceCodeHash string) (*spec.DeviceAuthorization, error)
}
