// DemoHeaderResolver: テスト用に "X-Demo-Sub" ヘッダから authn context を作る。
// TS src/authentication/usecases/demo-header-resolver.ts に対応。
package usecases

import (
	"context"
	"time"

	"ra-idp-go/internal/authentication/domain"
)

type DemoHeaderResolver struct{}

func (DemoHeaderResolver) Resolve(_ context.Context, h domain.Headers) (*domain.AuthenticationContext, error) {
	sub := h.Get("X-Demo-Sub")
	if sub == "" {
		return nil, nil
	}
	return &domain.AuthenticationContext{
		Sub:      sub,
		AuthTime: time.Now().Unix(),
		AMR:      []string{"pwd"},
	}, nil
}
