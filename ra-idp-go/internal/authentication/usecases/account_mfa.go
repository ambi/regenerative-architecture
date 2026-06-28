package usecases

// self-service の MFA (現状 TOTP) 登録・解除 (wi-21 / ADR-042)。actor.sub == target.sub
// に固定し、登録は確認コードによる所持証明、解除は有効な TOTP コードによる所持証明を
// 要求する。WebAuthn / SMS OTP は別 WI。step-up auth (ADR-043) は本ステージでは未導入で、
// 所持証明 + CSRF + 認証済みセッションで保護する。
//
// 登録フローは stateless: StartTOTPEnrollment が返した secret をクライアントが保持し、
// ConfirmTOTPEnrollment で secret + コードを送り返す。secret は本人自身の factor なので
// セッションに束縛しなくても他者を害さない (QR に元々露出する値と同じ)。

import (
	"context"
	"encoding/base32"
	"errors"
	"net/url"
	"strings"
	"time"

	authnports "ra-idp-go/internal/authentication/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/shared/spec"
	"ra-idp-go/internal/tenancy"
)

var (
	// ErrMfaAlreadyEnrolled は対象種別の factor が既に存在する状態で登録しようとした場合。
	ErrMfaAlreadyEnrolled = errors.New("mfa factor already enrolled")
	// ErrMfaNotEnrolled は解除対象の factor が存在しない場合。
	ErrMfaNotEnrolled = errors.New("mfa factor not enrolled")
	// ErrInvalidTOTPCode は確認コード / 解除時コードが secret に対して無効な場合。
	ErrInvalidTOTPCode = errors.New("totp code is invalid")
	// ErrInvalidTOTPSecret は confirm に渡された secret が base32 として復号できない場合。
	ErrInvalidTOTPSecret = errors.New("totp secret is invalid")
)

const totpFactorLabel = "Authenticator app"

// AccountMfaDeps は self-service MFA use case の依存。Issuer は otpauth URI の issuer
// ラベル導出に使う (authenticator アプリの表示名)。
type AccountMfaDeps struct {
	UserRepo      oauthports.UserRepository
	MfaFactorRepo authnports.MfaFactorRepository
	Emit          func(spec.DomainEvent)
	Issuer        string
}

// TOTPEnrollmentStart は StartTOTPEnrollment の戻り値。secret はクライアントが confirm まで
// 保持し、otpauth_uri は QR / 手動登録のために提示する。
type TOTPEnrollmentStart struct {
	Secret      string
	OTPAuthURI  string
	AccountName string
	Issuer      string
}

// StartTOTPEnrollment は新しい TOTP secret を発行し、登録用の otpauth URI を組み立てる。
// 既に TOTP factor がある場合は ErrMfaAlreadyEnrolled。永続化はまだ行わない。
func StartTOTPEnrollment(ctx context.Context, deps AccountMfaDeps, sub string) (*TOTPEnrollmentStart, error) {
	user, err := loadSelfUser(ctx, deps.UserRepo, sub)
	if err != nil {
		return nil, err
	}
	existing, err := deps.MfaFactorRepo.Find(ctx, sub, spec.MfaFactorTOTP)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrMfaAlreadyEnrolled
	}
	secret, err := GenerateTOTPSecret()
	if err != nil {
		return nil, err
	}
	issuerLabel := otpauthIssuerLabel(deps.Issuer)
	accountName := user.PreferredUsername
	return &TOTPEnrollmentStart{
		Secret:      secret,
		OTPAuthURI:  BuildOTPAuthURI(secret, accountName, issuerLabel),
		AccountName: accountName,
		Issuer:      issuerLabel,
	}, nil
}

// ConfirmTOTPEnrollmentInput は確認リクエスト。Secret は Start が返した値、Code は
// その secret から生成された現在の TOTP。
type ConfirmTOTPEnrollmentInput struct {
	Sub    string
	Secret string
	Code   string
	Now    time.Time
}

// ConfirmTOTPEnrollment は secret に対するコードの所持証明を検証し、TOTP factor を
// 永続化して user.MfaEnrolled を true にする。
func ConfirmTOTPEnrollment(ctx context.Context, deps AccountMfaDeps, in ConfirmTOTPEnrollmentInput) error {
	now := normalizedNow(in.Now)
	user, err := loadSelfUser(ctx, deps.UserRepo, in.Sub)
	if err != nil {
		return err
	}
	existing, err := deps.MfaFactorRepo.Find(ctx, in.Sub, spec.MfaFactorTOTP)
	if err != nil {
		return err
	}
	if existing != nil {
		return ErrMfaAlreadyEnrolled
	}
	secret := strings.TrimSpace(in.Secret)
	if _, decodeErr := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret); decodeErr != nil {
		return ErrInvalidTOTPSecret
	}
	if !VerifyTOTP(secret, strings.TrimSpace(in.Code), now.Unix(), TOTPWindow) {
		return ErrInvalidTOTPCode
	}
	label := totpFactorLabel
	factor := &spec.MfaFactor{
		Sub: user.Sub, Type: spec.MfaFactorTOTP, Secret: &secret, Label: &label, CreatedAt: now,
	}
	if err := factor.Validate(); err != nil {
		return err
	}
	if err := deps.MfaFactorRepo.Save(ctx, factor); err != nil {
		return err
	}
	user.MfaEnrolled = true
	user.UpdatedAt = now
	if err := deps.UserRepo.Save(ctx, user); err != nil {
		return err
	}
	if deps.Emit != nil {
		deps.Emit(&spec.MfaFactorEnrolled{
			At: now, TenantID: user.TenantID, Sub: user.Sub, FactorType: spec.MfaFactorTOTP,
		})
	}
	return nil
}

// RemoveTOTPFactorInput は TOTP 解除リクエスト。Code は所持証明としての現在の TOTP。
type RemoveTOTPFactorInput struct {
	Sub  string
	Code string
	Now  time.Time
}

// RemoveTOTPFactor は所持証明 (有効な TOTP コード) を検証してから factor を削除し、
// user.MfaEnrolled を false に戻す。factor が無ければ ErrMfaNotEnrolled。
func RemoveTOTPFactor(ctx context.Context, deps AccountMfaDeps, in RemoveTOTPFactorInput) error {
	now := normalizedNow(in.Now)
	user, err := loadSelfUser(ctx, deps.UserRepo, in.Sub)
	if err != nil {
		return err
	}
	factor, err := deps.MfaFactorRepo.Find(ctx, in.Sub, spec.MfaFactorTOTP)
	if err != nil {
		return err
	}
	if factor == nil || factor.Secret == nil || *factor.Secret == "" {
		return ErrMfaNotEnrolled
	}
	if !VerifyTOTP(*factor.Secret, strings.TrimSpace(in.Code), now.Unix(), TOTPWindow) {
		return ErrInvalidTOTPCode
	}
	if err := deps.MfaFactorRepo.Delete(ctx, in.Sub, spec.MfaFactorTOTP); err != nil {
		return err
	}
	user.MfaEnrolled = false
	user.UpdatedAt = now
	if err := deps.UserRepo.Save(ctx, user); err != nil {
		return err
	}
	if deps.Emit != nil {
		deps.Emit(&spec.MfaFactorRemoved{
			At: now, TenantID: user.TenantID, Sub: user.Sub, FactorType: spec.MfaFactorTOTP,
		})
	}
	return nil
}

// loadSelfUser は self 経路で対象 user を取得する。tenant 不一致は ErrUserNotFound に潰す。
func loadSelfUser(ctx context.Context, repo oauthports.UserRepository, sub string) (*spec.User, error) {
	user, err := repo.FindBySub(ctx, sub)
	if err != nil {
		return nil, err
	}
	if user == nil || user.TenantID != tenancy.TenantID(ctx) {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// otpauthIssuerLabel は issuer URL から authenticator 表示用の簡潔なラベルを導く。
// 解析できない場合は "ra-idp" を使う。
func otpauthIssuerLabel(issuer string) string {
	issuer = strings.TrimSpace(issuer)
	if issuer == "" {
		return "ra-idp"
	}
	if parsed, err := url.Parse(issuer); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return issuer
}
