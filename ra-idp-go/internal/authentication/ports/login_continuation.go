// LoginContinuation: ログイン成功後に OAuth2 フローを再開するための境界。
// authentication 側はこの port を呼ぶだけで、authorize 側の状態機械詳細を知らない。
package ports

import (
	"context"
	"net/http"

	"ra-idp-go/internal/authentication/domain"
)

type LoginContinuation interface {
	ContinueAfterLogin(ctx context.Context, requestID string, authn *domain.AuthenticationContext, opts ContinuationOptions, w http.ResponseWriter, r *http.Request) error
}

type ContinuationOptions struct {
	PromptLoginSatisfied bool
}
