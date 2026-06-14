package usecases

// 管理者向け User ライフサイクル操作 (Create / Update / Disable / Enable)。
// SCL の Authentication component が所有する admin インターフェース群:
// CreateAdminUser / UpdateAdminUser / DisableAdminUser / EnableAdminUser。

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

var (
	ErrUsernameConflict = errors.New("preferred username already exists")
	ErrInvalidRole      = errors.New("role must not be empty")
)

type AdminUserDeps struct {
	UserRepo            oauthports.UserRepository
	PasswordHasher      authports.PasswordHasher
	PasswordHistoryRepo authports.PasswordHistoryRepository
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
	result := ValidatePassword(in.Password)
	if !result.OK {
		return nil, &PasswordPolicyError{Violations: result.Violations}
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
	adminEmit(deps.Emit, &spec.UserCreated{At: now, ActorSub: in.ActorSub, TargetSub: user.Sub})
	return user, nil
}

type UpdateUserInput struct {
	ActorSub          string
	Sub               string
	PreferredUsername *string
	Name              *string
	Email             *string
	EmailVerified     *bool
	Roles             *[]string
	Now               time.Time
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
		At: now, ActorSub: in.ActorSub, TargetSub: user.Sub, ChangedFields: changed,
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
	updated := *user
	now = normalizedNow(now)
	if disabled {
		if updated.DisabledAt != nil {
			return &updated, nil
		}
		updated.DisabledAt = &now
	} else {
		if updated.DisabledAt == nil {
			return &updated, nil
		}
		updated.DisabledAt = nil
	}
	updated.UpdatedAt = now
	if err := deps.UserRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	if disabled {
		adminEmit(deps.Emit, &spec.UserDisabled{At: now, ActorSub: actorSub, TargetSub: targetSub})
	} else {
		adminEmit(deps.Emit, &spec.UserEnabled{At: now, ActorSub: actorSub, TargetSub: targetSub})
	}
	return &updated, nil
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
