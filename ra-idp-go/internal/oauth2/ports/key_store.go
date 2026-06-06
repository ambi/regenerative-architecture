package ports

import (
	"context"
	"crypto"
	"time"

	"ra-idp-go/internal/spec"
)

// SigningKey は本実装では RSA を想定。alg=PS256 のみ。
// 公開鍵 JWK は JWKS 配布用。
type SigningKey struct {
	Kid        string
	Alg        spec.SignatureAlgorithm
	PrivateKey crypto.PrivateKey
	PublicKey  crypto.PublicKey
	PublicJWK  map[string]any
	Active     bool
	CreatedAt  time.Time
}

type KeyStore interface {
	GetActiveKey(ctx context.Context) (*SigningKey, error)
	GetAllKeys(ctx context.Context) ([]*SigningKey, error)
	FindByKID(ctx context.Context, kid string) (*SigningKey, error)
	Rotate(ctx context.Context) (*SigningKey, error)
}
