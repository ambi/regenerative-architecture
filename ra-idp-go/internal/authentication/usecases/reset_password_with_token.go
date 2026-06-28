package usecases

import (
	"context"
	"errors"
	"slices"
	"time"

	authnports "ra-idp-go/internal/authentication/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

var ErrInvalidResetToken = errors.New("reset token is invalid or expired")

type ResetPasswordWithTokenDeps struct {
	UserRepo                oauthports.UserRepository
	TokenStore              authnports.PasswordResetTokenStore
	PasswordHasher          authnports.PasswordHasher
	PasswordHistoryRepo     authnports.PasswordHistoryRepository
	BreachedPasswordChecker authnports.BreachedPasswordChecker
	Emit                    func(spec.DomainEvent)
	HistoryDepth            int                    // Deprecated: use Policy 指定。後方互換のためのフォールバック。
	Policy                  PasswordPolicySnapshot // テナント解決済みのしきい値。ゼロ値は global default。
}

type ResetPasswordWithTokenInput struct {
	Token       string
	NewPassword string
	Now         time.Time
}

func ResetPasswordWithToken(
	ctx context.Context,
	deps ResetPasswordWithTokenDeps,
	in ResetPasswordWithTokenInput,
) (*spec.User, error) {
	now := in.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	record, err := deps.TokenStore.Consume(ctx, sha256Hex(in.Token), now)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrInvalidResetToken
	}
	user, err := deps.UserRepo.FindBySub(ctx, record.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil || user.TenantID != tenancy.TenantID(ctx) {
		return nil, ErrInvalidResetToken
	}

	snap := resolveSnapshot(deps.Policy, deps.HistoryDepth)
	result := ValidatePasswordWith(in.NewPassword, snap)
	if !result.OK {
		return nil, &PasswordPolicyError{Violations: result.Violations}
	}
	if deps.BreachedPasswordChecker != nil &&
		deps.BreachedPasswordChecker.IsBreached(ctx, in.NewPassword) {
		return nil, &PasswordPolicyError{Violations: []PasswordPolicyViolation{ViolationBreached}}
	}

	depth := snap.HistoryDepth
	recent, err := deps.PasswordHistoryRepo.Recent(ctx, user.Sub, depth)
	if err != nil {
		return nil, err
	}
	for _, entry := range recent {
		matched, err := deps.PasswordHasher.Verify(in.NewPassword, entry.Encoded)
		if err != nil {
			return nil, err
		}
		if matched {
			return nil, ErrPasswordReused
		}
	}
	matched, err := deps.PasswordHasher.Verify(in.NewPassword, user.PasswordHash)
	if err != nil {
		return nil, err
	}
	if matched {
		return nil, ErrPasswordReused
	}

	encoded, err := deps.PasswordHasher.Hash(in.NewPassword)
	if err != nil {
		return nil, err
	}
	updated := *user
	updated.PasswordHash = encoded
	updated.UpdatedAt = now
	updated.Lifecycle.PasswordChangedAt = &now
	// リセットで新パスワードを設定したので update_password 強制アクションを自動解除する。
	clearedUpdatePassword := slices.Contains(updated.Lifecycle.RequiredActions, spec.RequiredActionUpdatePassword)
	if clearedUpdatePassword {
		updated.Lifecycle.RequiredActions = removeRequiredAction(
			updated.Lifecycle.RequiredActions, spec.RequiredActionUpdatePassword,
		)
	}
	if err := deps.UserRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	if err := deps.PasswordHistoryRepo.Add(ctx, user.Sub, encoded, now); err != nil {
		return nil, err
	}
	if deps.Emit != nil {
		deps.Emit(&spec.PasswordChanged{At: now, TenantID: user.TenantID, Sub: user.Sub})
		if clearedUpdatePassword {
			deps.Emit(&spec.UserRequiredActionCleared{
				At: now, TenantID: user.TenantID, ActorSub: user.Sub, TargetSub: user.Sub,
				Action: string(spec.RequiredActionUpdatePassword),
			})
		}
	}
	return &updated, nil
}
