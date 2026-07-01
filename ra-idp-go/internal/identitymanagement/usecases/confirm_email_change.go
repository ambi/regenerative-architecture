package usecases

import (
	"context"
	"errors"
	"slices"
	"time"

	authnports "ra-idp-go/internal/authentication/ports"
	idmports "ra-idp-go/internal/identitymanagement/ports"
	"ra-idp-go/internal/shared/spec"
	"ra-idp-go/internal/tenancy"
)

var ErrInvalidEmailChangeToken = errors.New("email change token is invalid or expired")

// ConfirmEmailChangeDeps / Input は新アドレスへ送ったワンタイムトークンを消費し、
// primary email を確定する (self-service, wi-21)。トークンが所有確認の証左なので
// 認証済みセッションは要求しない (reset password と同方針)。
type ConfirmEmailChangeDeps struct {
	UserRepo   idmports.UserRepository
	TokenStore authnports.EmailChangeTokenStore
	Emit       func(spec.DomainEvent)
}

type ConfirmEmailChangeInput struct {
	Token string
	Now   time.Time
}

func ConfirmEmailChange(ctx context.Context, deps ConfirmEmailChangeDeps, in ConfirmEmailChangeInput) (*spec.User, error) {
	now := in.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	record, err := deps.TokenStore.Consume(ctx, sha256Hex(in.Token), now)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrInvalidEmailChangeToken
	}
	user, err := deps.UserRepo.FindBySub(ctx, record.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil || user.TenantID != tenancy.TenantID(ctx) {
		return nil, ErrInvalidEmailChangeToken
	}
	// 起票から確定までの間に別ユーザが同アドレスを確定していないか再チェックする。
	existing, err := deps.UserRepo.FindByEmail(ctx, user.TenantID, record.NewEmail)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.Sub != user.Sub {
		return nil, ErrEmailTaken
	}

	updated := *user
	email := record.NewEmail
	updated.Email = &email
	updated.EmailVerified = true
	updated.UpdatedAt = now
	clearedVerifyEmail := slices.Contains(updated.Lifecycle.RequiredActions, spec.RequiredActionVerifyEmail)
	if clearedVerifyEmail {
		updated.Lifecycle.RequiredActions = removeRequiredAction(
			updated.Lifecycle.RequiredActions, spec.RequiredActionVerifyEmail,
		)
	}
	if err := deps.UserRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	if deps.Emit != nil {
		deps.Emit(&spec.EmailChanged{At: now, TenantID: user.TenantID, Sub: user.Sub})
		if clearedVerifyEmail {
			deps.Emit(&spec.UserRequiredActionCleared{
				At: now, TenantID: user.TenantID, ActorSub: user.Sub, TargetSub: user.Sub,
				Action: string(spec.RequiredActionVerifyEmail),
			})
		}
	}
	return &updated, nil
}
