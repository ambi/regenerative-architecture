package usecases

// 高 sensitivity な self-service 操作のための step-up 再認証 (ADR-043 / wi-43)。
// パスワード変更・MFA factor 解除・primary email 変更・全セッション失効などは、
// セッションが乗っ取られた場合の被害を抑えるため「直近 N 分以内に password / MFA で
// 再認証済み」であることを要求する。判定の recency ソースは max(auth_time, step_up_at)。
// 新規ログイン直後 (auth_time が新しい) はそのまま step-up 済みとして扱う。

import (
	"context"
	"errors"
	"time"

	"ra-idp-go/internal/authentication/domain"
	authports "ra-idp-go/internal/authentication/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

// StepUpRecencySeconds は step-up が有効とみなされる窓 (5 分)。
const StepUpRecencySeconds = 300

// StepUpMethod は再認証に使える factor。
type StepUpMethod string

const (
	StepUpMethodPassword StepUpMethod = "password"
	StepUpMethodTOTP     StepUpMethod = "totp"
)

var (
	// ErrStepUpRequired は recency 窓を外れており再認証が必要なことを表す (handler が 403 に写す)。
	ErrStepUpRequired = errors.New("step-up authentication required")
	// ErrStepUpFailed は提示された factor (パスワード / TOTP コード) の検証に失敗したことを表す。
	ErrStepUpFailed = errors.New("step-up authentication failed")
	// ErrStepUpUnsupportedMethod は未対応 / 未登録の method を要求したことを表す。
	ErrStepUpUnsupportedMethod = errors.New("step-up method unsupported")
)

// StepUpSatisfied は authn が recency 窓内に強い (再)認証を済ませているかを判定する。
func StepUpSatisfied(authn *domain.AuthenticationContext, now time.Time) bool {
	if authn == nil || authn.AuthenticationPending {
		return false
	}
	recent := max(authn.AuthTime, authn.StepUpAt)
	if recent <= 0 {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return now.Unix()-recent <= StepUpRecencySeconds
}

// AvailableStepUpMethods は user が step-up に使える method を返す。password は常に利用可能、
// totp は enrolled の場合のみ。
func AvailableStepUpMethods(user *spec.User) []StepUpMethod {
	methods := []StepUpMethod{StepUpMethodPassword}
	if user != nil && user.MfaEnrolled {
		methods = append(methods, StepUpMethodTOTP)
	}
	return methods
}

// StepUpDeps は CompleteStepUp の依存。SessionManager は step_up_at の刻印に使う。
type StepUpDeps struct {
	UserRepo       oauthports.UserRepository
	PasswordHasher authports.PasswordHasher
	MfaFactorRepo  authports.MfaFactorRepository
	SessionManager *SessionManager
	Emit           func(spec.DomainEvent)
}

// StepUpStart は利用可能な method を返し StepUpRequested を emit する。
func StepUpStart(
	ctx context.Context,
	deps StepUpDeps,
	sub, sessionID string,
) ([]StepUpMethod, error) {
	user, err := deps.UserRepo.FindBySub(ctx, sub)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	if deps.Emit != nil {
		deps.Emit(&spec.StepUpRequested{
			At: time.Now().UTC(), TenantID: tenancy.TenantID(ctx), Sub: sub, SessionID: sessionID,
		})
	}
	return AvailableStepUpMethods(user), nil
}

// CompleteStepUpInput は再認証の検証材料。method に応じて Password か Code を使う。
type CompleteStepUpInput struct {
	Sub       string
	SessionID string
	Method    StepUpMethod
	Password  string
	Code      string
	Now       time.Time
}

// CompleteStepUp は提示された factor を検証し、成功すれば session に step_up_at を刻んで
// StepUpCompleted を emit する。検証失敗は ErrStepUpFailed。
func CompleteStepUp(ctx context.Context, deps StepUpDeps, in CompleteStepUpInput) error {
	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	user, err := deps.UserRepo.FindBySub(ctx, in.Sub)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}
	switch in.Method {
	case StepUpMethodPassword:
		if user.PasswordHash == "" {
			return ErrStepUpUnsupportedMethod
		}
		ok, verr := deps.PasswordHasher.Verify(in.Password, user.PasswordHash)
		if verr != nil {
			return verr
		}
		if !ok {
			return ErrStepUpFailed
		}
	case StepUpMethodTOTP:
		if !user.MfaEnrolled {
			return ErrStepUpUnsupportedMethod
		}
		result, verr := VerifyTOTPFactor(ctx, deps.MfaFactorRepo, in.Sub, in.Code, now)
		if verr != nil {
			return verr
		}
		if result == nil || !result.OK {
			return ErrStepUpFailed
		}
	default:
		return ErrStepUpUnsupportedMethod
	}
	if deps.SessionManager != nil && in.SessionID != "" {
		if _, err := deps.SessionManager.RecordStepUp(ctx, in.SessionID, now); err != nil {
			return err
		}
	}
	if deps.Emit != nil {
		deps.Emit(&spec.StepUpCompleted{
			At: now, TenantID: tenancy.TenantID(ctx), Sub: in.Sub,
			SessionID: in.SessionID, Method: string(in.Method),
		})
	}
	return nil
}
