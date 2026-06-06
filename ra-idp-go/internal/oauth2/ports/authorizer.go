package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

type Authorizer interface {
	Authorize(ctx context.Context, req spec.AuthZRequest) (spec.AuthZResponse, error)
}
