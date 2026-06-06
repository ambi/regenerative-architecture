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
}

type TokenIntrospector interface {
	IntrospectAccessToken(ctx context.Context, token string) (*IntrospectionResult, error)
}
