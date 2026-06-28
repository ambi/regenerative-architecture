package usecases

// 管理者向け User ライフサイクル操作 (Create / Update / Disable / Enable)。
// SCL の IdentityManagement bounded context が所有する admin インターフェース群:
// CreateAdminUser / UpdateAdminUser / DisableAdminUser / EnableAdminUser。

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	authnports "ra-idp-go/internal/authentication/ports"
	authusecases "ra-idp-go/internal/authentication/usecases"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/shared/spec"
	"ra-idp-go/internal/tenancy"
	tenantports "ra-idp-go/internal/tenancy/ports"
)

var (
	ErrUsernameConflict = errors.New("preferred username already exists")
	ErrInvalidRole      = errors.New("role must not be empty")
	// ErrSelfDeleteForbidden は admin / system_admin が自身を削除しようとした場合に
	// 返る (ADR-036 の自爆防止)。
	ErrSelfDeleteForbidden = errors.New("admins cannot delete themselves")
	// ErrSelfDisableForbidden は admin / system_admin が自身を無効化しようとした
	// 場合に返る。delete 側 (ErrSelfDeleteForbidden) と対称な自爆防止で、誤操作で
	// 自身の管理画面アクセスを即時遮断する事故を防ぐ。enable 方向には適用しない。
	ErrSelfDisableForbidden = errors.New("admins cannot disable themselves")
	// ErrInvalidAttribute は attributes が実効スキーマ (組み込み ∪ tenant) に
	// 適合しない場合に返る (ADR-040)。
	ErrInvalidAttribute = errors.New("attribute does not conform to schema")
)

// deletedPasswordHashSentinel は ADR-036 の tombstone 用に PasswordHash へ設定する
// 非ハッシュ形式の値。Argon2id のフォーマットと一致しないため、どんなパスワードでも
// 認証に通らないが、`z.String().Required()` の schema 制約は満たす。
const deletedPasswordHashSentinel = "$deleted$"

type AdminUserDeps struct {
	UserRepo            oauthports.UserRepository
	AttrSchemaRepo      tenantports.TenantUserAttributeSchemaRepository
	ConsentRepo         oauthports.ConsentRepository
	RefreshStore        oauthports.RefreshTokenStore
	DeviceCodeStore     oauthports.DeviceCodeStore
	SessionStore        authnports.SessionStore
	MfaFactorRepo       authnports.MfaFactorRepository
	PasswordHasher      authnports.PasswordHasher
	PasswordHistoryRepo authnports.PasswordHistoryRepository
	Emit                func(spec.DomainEvent)
}

type CreateUserInput struct {
	ActorSub          string
	PreferredUsername string
	Password          string
	Name              *string
	Email             *string
	EmailVerified     bool
	Roles             []string
	Now               time.Time
}

func CreateUser(ctx context.Context, deps AdminUserDeps, in CreateUserInput) (*spec.User, error) {
	username := strings.TrimSpace(in.PreferredUsername)
	if username == "" {
		return nil, errors.New("preferred username is required")
	}
	tenantID := tenancy.TenantID(ctx)
	existing, err := deps.UserRepo.FindByUsername(ctx, tenantID, username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUsernameConflict
	}
	result := authusecases.ValidatePassword(in.Password)
	if !result.OK {
		return nil, &authusecases.PasswordPolicyError{Violations: result.Violations}
	}
	roles, err := normalizeRoles(in.Roles)
	if err != nil {
		return nil, err
	}
	passwordHash, err := deps.PasswordHasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}
	id, err := spec.NewUUIDv4()
	if err != nil {
		return nil, err
	}
	now := normalizedNow(in.Now)
	user := &spec.User{
		Sub: "user_" + id, TenantID: tenantID, PreferredUsername: username, PasswordHash: passwordHash,
		Name: in.Name, Email: in.Email, EmailVerified: in.EmailVerified, Roles: roles,
		Lifecycle: spec.UserLifecycle{Status: spec.UserStatusActive},
		CreatedAt: now, UpdatedAt: now,
	}
	if err := user.Validate(); err != nil {
		return nil, err
	}
	if err := deps.UserRepo.Save(ctx, user); err != nil {
		return nil, err
	}
	if err := deps.PasswordHistoryRepo.Add(ctx, user.Sub, passwordHash, now); err != nil {
		return nil, err
	}
	adminEmit(deps.Emit, &spec.UserCreated{At: now, TenantID: user.TenantID, ActorSub: in.ActorSub, TargetSub: user.Sub})
	return user, nil
}

type UpdateUserInput struct {
	ActorSub          string
	Sub               string
	PreferredUsername *string
	Name              *string
	GivenName         *string
	FamilyName        *string
	Email             *string
	EmailVerified     *bool
	Roles             *[]string
	// Attributes は指定時に attributes 全体を置換する (実効スキーマで検証)。
	Attributes *map[string]spec.AttributeValue
	Now        time.Time
}

func UpdateUser(ctx context.Context, deps AdminUserDeps, in UpdateUserInput) (*spec.User, error) {
	user, err := deps.UserRepo.FindBySub(ctx, in.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	if user.TenantID != tenancy.TenantID(ctx) {
		return nil, ErrUserNotFound
	}
	updated := *user
	changed := []string{}
	if in.PreferredUsername != nil {
		username := strings.TrimSpace(*in.PreferredUsername)
		if username == "" {
			return nil, errors.New("preferred username must not be empty")
		}
		if username != user.PreferredUsername {
			existing, err := deps.UserRepo.FindByUsername(ctx, user.TenantID, username)
			if err != nil {
				return nil, err
			}
			if existing != nil && existing.Sub != user.Sub {
				return nil, ErrUsernameConflict
			}
			updated.PreferredUsername = username
			changed = append(changed, "preferred_username")
		}
	}
	if in.Name != nil && !equalOptionalString(user.Name, in.Name) {
		updated.Name = in.Name
		changed = append(changed, "name")
	}
	if in.GivenName != nil && !equalOptionalString(user.GivenName, in.GivenName) {
		updated.GivenName = in.GivenName
		changed = append(changed, "given_name")
	}
	if in.FamilyName != nil && !equalOptionalString(user.FamilyName, in.FamilyName) {
		updated.FamilyName = in.FamilyName
		changed = append(changed, "family_name")
	}
	if in.Attributes != nil {
		defs, err := effectiveUserAttributeDefs(ctx, deps.AttrSchemaRepo, user.TenantID)
		if err != nil {
			return nil, err
		}
		if err := spec.ValidateAttributes(*in.Attributes, defs); err != nil {
			return nil, errors.Join(ErrInvalidAttribute, err)
		}
		updated.Attributes = *in.Attributes
		changed = append(changed, "attributes")
	}
	if in.Email != nil && !equalOptionalString(user.Email, in.Email) {
		updated.Email = in.Email
		changed = append(changed, "email")
	}
	if in.EmailVerified != nil && *in.EmailVerified != user.EmailVerified {
		updated.EmailVerified = *in.EmailVerified
		changed = append(changed, "email_verified")
	}
	if in.Roles != nil {
		roles, err := normalizeRoles(*in.Roles)
		if err != nil {
			return nil, err
		}
		if !slices.Equal(roles, user.Roles) {
			updated.Roles = roles
			changed = append(changed, "roles")
		}
	}
	if len(changed) == 0 {
		return &updated, nil
	}
	now := normalizedNow(in.Now)
	updated.UpdatedAt = now
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	if err := deps.UserRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	adminEmit(deps.Emit, &spec.UserUpdated{
		At: now, TenantID: user.TenantID, ActorSub: in.ActorSub, TargetSub: user.Sub, ChangedFields: changed,
	})
	return &updated, nil
}

func SetUserDisabled(
	ctx context.Context,
	deps AdminUserDeps,
	actorSub, targetSub string,
	disabled bool,
	now time.Time,
) (*spec.User, error) {
	user, err := deps.UserRepo.FindBySub(ctx, targetSub)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	if user.TenantID != tenancy.TenantID(ctx) {
		return nil, ErrUserNotFound
	}
	if disabled && actorSub == user.Sub && hasPrivilegedRole(user.Roles) {
		return nil, ErrSelfDisableForbidden
	}
	updated := *user
	now = normalizedNow(now)
	if disabled {
		if updated.Lifecycle.Status == spec.UserStatusDisabled {
			return &updated, nil
		}
		updated.Lifecycle.Status = spec.UserStatusDisabled
		updated.Lifecycle.StatusChangedAt = &now
	} else {
		if updated.Lifecycle.Status == spec.UserStatusActive {
			return &updated, nil
		}
		updated.Lifecycle.Status = spec.UserStatusActive
		updated.Lifecycle.StatusChangedAt = &now
	}
	updated.UpdatedAt = now
	if err := deps.UserRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	if disabled {
		adminEmit(deps.Emit, &spec.UserDisabled{At: now, TenantID: updated.TenantID, ActorSub: actorSub, TargetSub: targetSub})
	} else {
		adminEmit(deps.Emit, &spec.UserEnabled{At: now, TenantID: updated.TenantID, ActorSub: actorSub, TargetSub: targetSub})
	}
	return &updated, nil
}

// ErrInvalidRequiredAction は RequiredAction enum に無い値が指定された場合に返る。
var ErrInvalidRequiredAction = errors.New("required action is not in enum")

// SetUserRequiredAction は対象ユーザに次回ログイン時の強制アクションを付与する
// (admin 専用 / wi-19)。既に付与済みの場合は冪等に no-op で返す。
func SetUserRequiredAction(
	ctx context.Context,
	deps AdminUserDeps,
	actorSub, targetSub string,
	action spec.RequiredAction,
	now time.Time,
) (*spec.User, error) {
	if !action.Valid() {
		return nil, ErrInvalidRequiredAction
	}
	user, err := loadTenantUser(ctx, deps, targetSub)
	if err != nil {
		return nil, err
	}
	if slices.Contains(user.Lifecycle.RequiredActions, action) {
		return user, nil
	}
	updated := *user
	updated.Lifecycle.RequiredActions = append(slices.Clone(user.Lifecycle.RequiredActions), action)
	now = normalizedNow(now)
	updated.UpdatedAt = now
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	if err := deps.UserRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	adminEmit(deps.Emit, &spec.UserRequiredActionSet{
		At: now, TenantID: updated.TenantID, ActorSub: actorSub, TargetSub: targetSub, Action: string(action),
	})
	return &updated, nil
}

// ClearUserRequiredAction は強制アクションを解除する (admin 専用 / wi-19)。
// 未付与の場合は冪等に no-op で返す。本人のパスワード変更に伴う自動解除は
// clearRequiredAction (change_password.go) を使う。
func ClearUserRequiredAction(
	ctx context.Context,
	deps AdminUserDeps,
	actorSub, targetSub string,
	action spec.RequiredAction,
	now time.Time,
) (*spec.User, error) {
	if !action.Valid() {
		return nil, ErrInvalidRequiredAction
	}
	user, err := loadTenantUser(ctx, deps, targetSub)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(user.Lifecycle.RequiredActions, action) {
		return user, nil
	}
	updated := *user
	updated.Lifecycle.RequiredActions = removeRequiredAction(user.Lifecycle.RequiredActions, action)
	now = normalizedNow(now)
	updated.UpdatedAt = now
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	if err := deps.UserRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	adminEmit(deps.Emit, &spec.UserRequiredActionCleared{
		At: now, TenantID: updated.TenantID, ActorSub: actorSub, TargetSub: targetSub, Action: string(action),
	})
	return &updated, nil
}

// loadTenantUser は現在のテナント内の user を取得する。存在しない / 別テナントなら
// ErrUserNotFound。admin user 操作の共通プレリュード。
func loadTenantUser(ctx context.Context, deps AdminUserDeps, sub string) (*spec.User, error) {
	user, err := deps.UserRepo.FindBySub(ctx, sub)
	if err != nil {
		return nil, err
	}
	if user == nil || user.TenantID != tenancy.TenantID(ctx) {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// removeRequiredAction は action を除いた新しいスライスを返す (元を破壊しない)。
func removeRequiredAction(actions []spec.RequiredAction, action spec.RequiredAction) []spec.RequiredAction {
	out := make([]spec.RequiredAction, 0, len(actions))
	for _, a := range actions {
		if a != action {
			out = append(out, a)
		}
	}
	return out
}

func normalizeRoles(roles []string) ([]string, error) {
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role == "" {
			return nil, ErrInvalidRole
		}
		if !slices.Contains(out, role) {
			out = append(out, role)
		}
	}
	slices.Sort(out)
	return out, nil
}

func normalizedNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

func equalOptionalString(left, right *string) bool {
	return left == nil && right == nil ||
		left != nil && right != nil && *left == *right
}

func adminEmit(sink func(spec.DomainEvent), event spec.DomainEvent) {
	if sink != nil {
		sink(event)
	}
}

// DeleteUserInput は ADR-036 の DeleteUser use case 入力。
type DeleteUserInput struct {
	ActorSub string
	Sub      string
	Reason   string
	Now      time.Time
}

// DeleteUser は ADR-036 の anonymize cascade を実行する。
//   - 対象 user の PII フィールドを tombstone 値で置換する (`deleted_at` 設定)。
//   - 関連 aggregate (Consent / RefreshToken / Session / PasswordHistory /
//     MfaFactor / DeviceAuthorization) を物理削除する。
//   - `user.deleted` を 1 度だけ emit する (冪等)。
//
// 既に削除済の user に対しては no-op で nil を返す (audit event も emit しない)。
// actor.Sub == target.Sub かつ target が admin / system_admin role を持つ場合は
// ErrSelfDeleteForbidden を返し、cascade は実施しない。
func DeleteUser(ctx context.Context, deps AdminUserDeps, in DeleteUserInput) error {
	user, err := deps.UserRepo.FindBySubIncludingDeleted(ctx, in.Sub)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}
	if user.TenantID != tenancy.TenantID(ctx) {
		return ErrUserNotFound
	}
	if user.IsDeleted() {
		return nil
	}
	if in.ActorSub == user.Sub && hasPrivilegedRole(user.Roles) {
		return ErrSelfDeleteForbidden
	}
	now := normalizedNow(in.Now)
	tombstone := anonymizeUser(user, now)
	if err := tombstone.Validate(); err != nil {
		return err
	}
	if err := deps.UserRepo.Save(ctx, tombstone); err != nil {
		return err
	}
	if err := cascadeDeleteForSub(ctx, deps, user.Sub); err != nil {
		return err
	}
	adminEmit(deps.Emit, &spec.UserDeleted{
		At: now, TenantID: user.TenantID, ActorSub: in.ActorSub, TargetSub: user.Sub, Reason: in.Reason,
	})
	return nil
}

func hasPrivilegedRole(roles []string) bool {
	return slices.Contains(roles, "admin") || slices.Contains(roles, "system_admin")
}

// effectiveUserAttributeDefs は組み込み属性 + tenant 固有 schema を結合した実効定義を返す。
// AttrSchemaRepo 未配線 (nil) の場合は組み込み属性のみで検証する。
func effectiveUserAttributeDefs(
	ctx context.Context, repo tenantports.TenantUserAttributeSchemaRepository, tenantID string,
) ([]spec.UserAttributeDef, error) {
	defs := spec.BuiltinUserAttributeDefs()
	if repo == nil {
		return defs, nil
	}
	schema, err := repo.FindByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if schema != nil {
		defs = append(defs, schema.Attributes...)
	}
	return defs, nil
}

func anonymizeUser(user *spec.User, now time.Time) *spec.User {
	tombstone := *user
	tombstone.PreferredUsername = "deleted:" + user.Sub
	tombstone.PasswordHash = deletedPasswordHashSentinel
	tombstone.Name = nil
	tombstone.GivenName = nil
	tombstone.FamilyName = nil
	tombstone.Email = nil
	tombstone.EmailVerified = false
	tombstone.MfaEnrolled = false
	tombstone.Roles = []string{}
	tombstone.Attributes = nil
	tombstone.UpdatedAt = now
	tombstone.Lifecycle = spec.UserLifecycle{Status: spec.UserStatusDeleted, StatusChangedAt: &now}
	return &tombstone
}

func cascadeDeleteForSub(ctx context.Context, deps AdminUserDeps, sub string) error {
	if deps.ConsentRepo != nil {
		if err := deps.ConsentRepo.DeleteAllForSub(ctx, sub); err != nil {
			return err
		}
	}
	if deps.RefreshStore != nil {
		if err := deps.RefreshStore.DeleteAllForSub(ctx, sub); err != nil {
			return err
		}
	}
	if deps.SessionStore != nil {
		if err := deps.SessionStore.DeleteAllForSub(ctx, sub); err != nil {
			return err
		}
	}
	if deps.PasswordHistoryRepo != nil {
		if err := deps.PasswordHistoryRepo.DeleteAllForSub(ctx, sub); err != nil {
			return err
		}
	}
	if deps.MfaFactorRepo != nil {
		if err := deps.MfaFactorRepo.DeleteAllForSub(ctx, sub); err != nil {
			return err
		}
	}
	if deps.DeviceCodeStore != nil {
		if err := deps.DeviceCodeStore.DeleteAllForSub(ctx, sub); err != nil {
			return err
		}
	}
	return nil
}
