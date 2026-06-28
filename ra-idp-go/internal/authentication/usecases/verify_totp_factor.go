package usecases

import (
	"context"
	"time"

	authnports "ra-idp-go/internal/authentication/ports"
	"ra-idp-go/internal/spec"
)

type VerifyTOTPFactorResult struct {
	OK     bool
	Reason string
}

func VerifyTOTPFactor(
	ctx context.Context,
	repo authnports.MfaFactorRepository,
	sub string,
	code string,
	now time.Time,
) (*VerifyTOTPFactorResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	factor, err := repo.Find(ctx, sub, spec.MfaFactorTOTP)
	if err != nil {
		return nil, err
	}
	if factor == nil || factor.Secret == nil || *factor.Secret == "" {
		return &VerifyTOTPFactorResult{OK: false, Reason: "no_factor"}, nil
	}
	if !VerifyTOTP(*factor.Secret, code, now.Unix(), TOTPWindow) {
		return &VerifyTOTPFactorResult{OK: false, Reason: "invalid_code"}, nil
	}
	usedAt := now.UTC()
	factor.LastUsedAt = &usedAt
	if err := repo.Save(ctx, factor); err != nil {
		return nil, err
	}
	return &VerifyTOTPFactorResult{OK: true}, nil
}
