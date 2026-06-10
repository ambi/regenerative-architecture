package usecases

import (
	"context"
	"errors"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

var ErrCurrentPasswordMismatch = errors.New("current password does not match")
var ErrPasswordReused = errors.New("new password matches a recent password")
var ErrUserNotFound = errors.New("user not found")

type ChangePasswordInput struct {
	Sub             string
	CurrentPassword string
	NewPassword     string
	Now             time.Time
}

type ChangePasswordDeps struct {
	UserRepo            oauthports.UserRepository
	PasswordHasher      authports.PasswordHasher
	PasswordHistoryRepo authports.PasswordHistoryRepository
	Emit                func(spec.DomainEvent)
	HistoryDepth        int
}

func ChangePassword(ctx context.Context, deps ChangePasswordDeps, in ChangePasswordInput) (*spec.User, error) {
	user, err := deps.UserRepo.FindBySub(ctx, in.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	ok, err := deps.PasswordHasher.Verify(in.CurrentPassword, user.PasswordHash)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrCurrentPasswordMismatch
	}

	result := ValidatePassword(in.NewPassword)
	if !result.OK {
		return nil, &PasswordPolicyError{Violations: result.Violations}
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

	now := in.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
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
