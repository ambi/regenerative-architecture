package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

// IntrospectionResult は RFC 7662 のレスポンス。
type IntrospectionResult struct {
	Active           bool
	JTI              string
	ClientID         string
	Sub              string
	Scope            string
	Exp              int64
	Iat              int64
	TokenType        string
	SenderConstraint *spec.SenderConstraint
	// Aud / Act / MayAct は RFC 8693 トークン交換のために検証済みペイロードから抽出する。
	// Aud は単一文字列でも配列でも常に []string に正規化する。
	Aud    []string
	Act    map[string]any
	MayAct map[string]any
	// AuthorizationDetails は RFC 9396 の構造化詳細 (ADR-050)。検証済みペイロードから
	// 抽出し、introspection 応答とトークン交換のダウンスコープ判定に使う。
	AuthorizationDetails []spec.AuthorizationDetail
}

type TokenIntrospector interface {
	IntrospectAccessToken(ctx context.Context, token string) (*IntrospectionResult, error)
}
