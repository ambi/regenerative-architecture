package usecases

import (
	"context"
	"errors"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

var ErrInvalidResetToken = errors.New("reset token is invalid or expired")

type ResetPasswordWithTokenDeps struct {
	UserRepo                oauthports.UserRepository
	TokenStore              authports.PasswordResetTokenStore
	PasswordHasher          authports.PasswordHasher
	PasswordHistoryRepo     authports.PasswordHistoryRepository
	BreachedPasswordChecker authports.BreachedPasswordChecker
	Emit                    func(spec.DomainEvent)
	HistoryDepth            int
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

	result := ValidatePassword(in.NewPassword)
	if !result.OK {
		return nil, &PasswordPolicyError{Violations: result.Violations}
	}
	if deps.BreachedPasswordChecker != nil &&
		deps.BreachedPasswordChecker.IsBreached(ctx, in.NewPassword) {
		return nil, &PasswordPolicyError{Violations: []PasswordPolicyViolation{ViolationBreached}}
	}

	depth := deps.HistoryDepth
	if depth == 0 {
		depth = PasswordPolicyHistoryDepth
	}
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
	if err := deps.UserRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	if err := deps.PasswordHistoryRepo.Add(ctx, user.Sub, encoded, now); err != nil {
		return nil, err
	}
	if deps.Emit != nil {
		deps.Emit(&spec.PasswordChanged{At: now, Sub: user.Sub})
	}
	return &updated, nil
}
