package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

type AccessTokenInput struct {
	Client           *spec.Client
	Sub              string
	Scopes           []string
	SenderConstraint *spec.SenderConstraint
	AuthTime         int64
	AMR              []string
	ACR              string
}

type IDTokenInput struct {
	Client    *spec.Client
	User      *spec.User
	Scopes    []string
	Nonce     *string
	AuthTime  int64
	AMR       []string
	ACR       string
	AtHashFor string // access token whose hash goes into at_hash
	// ResolveAttributeDefs はユーザのテナントに有効な属性定義 (builtin + custom) を
	// 返す。nil の場合は属性ベースの claim 生成をスキップする (wi-19)。
	ResolveAttributeDefs func(ctx context.Context, tenantID string) ([]spec.UserAttributeDef, error)
}

type TokenIssuer interface {
	SignAccessToken(ctx context.Context, in AccessTokenInput) (token, jti string, err error)
	SignIDToken(ctx context.Context, in IDTokenInput) (string, error)
	AccessTokenTTLSeconds() int
	IDTokenTTLSeconds() int
}
