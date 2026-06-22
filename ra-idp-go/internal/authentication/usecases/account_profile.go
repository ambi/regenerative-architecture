package usecases

// エンドユーザ自身によるプロフィール参照・編集 (self-service)。
// SCL の IdentityManagement bounded context が所有する self インターフェース:
// GetUserProfile / UpdateUserProfile。actor.sub == target.sub を前提とし、
// 編集できるのは editable_by_user=true の属性と表示名のみ (status / roles /
// organization は admin 専用、ADR-040 の affected_guarantees)。

import (
	"context"
	"errors"
	"time"

	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
	tenantports "ra-idp-go/internal/tenancy/ports"
)

// ErrAttributeNotEditable は self-service で editable_by_user=false の属性を
// 変更しようとした場合に返る (ADR-040)。
var ErrAttributeNotEditable = errors.New("attribute is not user-editable")

type AccountProfileDeps struct {
	UserRepo       oauthports.UserRepository
	AttrSchemaRepo tenantports.TenantUserAttributeSchemaRepository
	Emit           func(spec.DomainEvent)
}

// GetUserProfile は呼び出しユーザ自身のプロフィールと実効属性定義を返す。
// 返した定義は handler 側で self 可視属性の絞り込み・編集可能属性の提示に使う。
func GetUserProfile(
	ctx context.Context, deps AccountProfileDeps, sub string,
) (*spec.User, []spec.UserAttributeDef, error) {
	user, err := loadSelf(ctx, deps.UserRepo, sub)
	if err != nil {
		return nil, nil, err
	}
	defs, err := effectiveUserAttributeDefs(ctx, deps.AttrSchemaRepo, user.TenantID)
	if err != nil {
		return nil, nil, err
	}
	return user, defs, nil
}

type UpdateUserProfileInput struct {
	Sub        string
	Name       *string
	GivenName  *string
	FamilyName *string
	// Attributes は editable_by_user=true の属性の部分更新 (指定 key のみ upsert)。
	// 未指定 key は現状維持。admin 管理属性は保持される。
	Attributes *map[string]spec.AttributeValue
	Now        time.Time
}

// UpdateUserProfile は呼び出しユーザ自身の表示名と編集可能属性を更新する。
// 属性は全置換ではなく key 単位の merge とし、editable_by_user=false の key は拒否する。
func UpdateUserProfile(
	ctx context.Context, deps AccountProfileDeps, in UpdateUserProfileInput,
) (*spec.User, []spec.UserAttributeDef, error) {
	user, err := loadSelf(ctx, deps.UserRepo, in.Sub)
	if err != nil {
		return nil, nil, err
	}
	defs, err := effectiveUserAttributeDefs(ctx, deps.AttrSchemaRepo, user.TenantID)
	if err != nil {
		return nil, nil, err
	}

	updated := *user
	changed := []string{}
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
		merged, err := mergeEditableAttributes(user.Attributes, *in.Attributes, defs)
		if err != nil {
			return nil, nil, err
		}
		updated.Attributes = merged
		changed = append(changed, "attributes")
	}
	if len(changed) == 0 {
		return &updated, defs, nil
	}
	now := normalizedNow(in.Now)
	updated.UpdatedAt = now
	if err := updated.Validate(); err != nil {
		return nil, nil, err
	}
	if err := deps.UserRepo.Save(ctx, &updated); err != nil {
		return nil, nil, err
	}
	// self 編集は actorSub == targetSub。changedFields の粒度で記録する (ADR-018)。
	adminEmit(deps.Emit, &spec.UserUpdated{
		At: now, TenantID: user.TenantID, ActorSub: user.Sub, TargetSub: user.Sub, ChangedFields: changed,
	})
	return &updated, defs, nil
}

func loadSelf(ctx context.Context, repo oauthports.UserRepository, sub string) (*spec.User, error) {
	user, err := repo.FindBySub(ctx, sub)
	if err != nil {
		return nil, err
	}
	if user == nil || user.TenantID != tenancy.TenantID(ctx) {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// mergeEditableAttributes は既存属性を保ったまま、editable_by_user=true の key だけを
// 上書きする。未定義 key は ErrInvalidAttribute、編集不可 key は ErrAttributeNotEditable。
func mergeEditableAttributes(
	current map[string]spec.AttributeValue,
	patch map[string]spec.AttributeValue,
	defs []spec.UserAttributeDef,
) (map[string]spec.AttributeValue, error) {
	byKey := make(map[string]spec.UserAttributeDef, len(defs))
	for _, def := range defs {
		byKey[def.Key] = def
	}
	merged := make(map[string]spec.AttributeValue, len(current)+len(patch))
	for key, value := range current {
		merged[key] = value
	}
	for key, value := range patch {
		def, ok := byKey[key]
		if !ok {
			return nil, errors.Join(ErrInvalidAttribute, errors.New("attribute "+key+" is not defined"))
		}
		if !def.EditableByUser {
			return nil, errors.Join(ErrAttributeNotEditable, errors.New("attribute "+key+" is admin-managed"))
		}
		if err := spec.ValidateAttributeValue(value, def); err != nil {
			return nil, errors.Join(ErrInvalidAttribute, err)
		}
		merged[key] = value
	}
	return merged, nil
}

// SelfReadableAttributes は self が読める属性 (self_readable / claim_exposed) の値だけを
// 抽出する。private / admin_readable は除外する。
func SelfReadableAttributes(
	attributes map[string]spec.AttributeValue, defs []spec.UserAttributeDef,
) map[string]spec.AttributeValue {
	visibility := make(map[string]spec.AttrVisibility, len(defs))
	for _, def := range defs {
		visibility[def.Key] = def.Visibility
	}
	out := map[string]spec.AttributeValue{}
	for key, value := range attributes {
		switch visibility[key] {
		case spec.AttrVisibilitySelfReadable, spec.AttrVisibilityClaimExposed:
			out[key] = value
		}
	}
	return out
}

// SelfReadableAttributeDefs は self が読める属性定義を返す。
func SelfReadableAttributeDefs(defs []spec.UserAttributeDef) []spec.UserAttributeDef {
	out := []spec.UserAttributeDef{}
	for _, def := range defs {
		switch def.Visibility {
		case spec.AttrVisibilitySelfReadable, spec.AttrVisibilityClaimExposed:
			out = append(out, def)
		}
	}
	return out
}

// EditableAttributeDefs は self が編集できる属性定義 (editable_by_user=true) を返す。
func EditableAttributeDefs(defs []spec.UserAttributeDef) []spec.UserAttributeDef {
	out := []spec.UserAttributeDef{}
	for _, def := range defs {
		if def.EditableByUser {
			out = append(out, def)
		}
	}
	return out
}
