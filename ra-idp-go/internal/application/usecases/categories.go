package usecases

// 管理者向け ApplicationCategory の CRUD と Application への付与 (wi-70, ADR-069)。
// カテゴリは ApplicationCatalog が tenant 単位で所有し、ポータルのセクション分類に使う。
// 付与は Application.CategoryIDs に持ち、カテゴリ削除時はアプリ側からも除く。

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"ra-idp-go/internal/application/ports"
	"ra-idp-go/internal/shared/spec"
	"ra-idp-go/internal/tenancy"
)

var (
	// ErrCategoryNotFound は対象カテゴリが存在しない。
	ErrCategoryNotFound = errors.New("application category not found")
	// ErrCategoryNameRequired はカテゴリ名が空。
	ErrCategoryNameRequired = errors.New("application category name is required")
	// ErrUnknownCategory は付与しようとしたカテゴリが所属テナントに存在しない。
	ErrUnknownCategory = errors.New("application categories contain an unknown category")
)

type CategoryDeps struct {
	Repo    ports.ApplicationCategoryRepository
	AppRepo ports.ApplicationRepository
	Emit    func(spec.DomainEvent)
}

func ListCategories(ctx context.Context, deps CategoryDeps) ([]*spec.ApplicationCategory, error) {
	return deps.Repo.ListByTenant(ctx, tenancy.TenantID(ctx))
}

type CreateCategoryInput struct {
	ActorSub string
	Name     string
	Position *int
	Now      time.Time
}

func CreateCategory(ctx context.Context, deps CategoryDeps, in CreateCategoryInput) (*spec.ApplicationCategory, error) {
	tenantID := tenancy.TenantID(ctx)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrCategoryNameRequired
	}
	position, err := resolvePosition(ctx, deps, tenantID, in.Position)
	if err != nil {
		return nil, err
	}
	id, err := spec.NewUUIDv4()
	if err != nil {
		return nil, err
	}
	now := adminNow(in.Now)
	category := &spec.ApplicationCategory{
		TenantID: tenantID, CategoryID: id, Name: name, Position: position,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := deps.Repo.Save(ctx, category); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.ApplicationCategoryCreated{At: now, TenantID: tenantID, ActorSub: in.ActorSub, CategoryID: id})
	return category, nil
}

// resolvePosition は position 省略時に末尾 (既存数) を採用する。
func resolvePosition(ctx context.Context, deps CategoryDeps, tenantID string, requested *int) (int, error) {
	if requested != nil {
		return *requested, nil
	}
	existing, err := deps.Repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	return len(existing), nil
}

type UpdateCategoryInput struct {
	ActorSub   string
	CategoryID string
	Name       *string
	Position   *int
	Now        time.Time
}

func UpdateCategory(ctx context.Context, deps CategoryDeps, in UpdateCategoryInput) (*spec.ApplicationCategory, error) {
	tenantID := tenancy.TenantID(ctx)
	category, err := deps.Repo.FindByID(ctx, tenantID, in.CategoryID)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, ErrCategoryNotFound
	}
	updated := *category
	changed := false
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, ErrCategoryNameRequired
		}
		if name != category.Name {
			updated.Name = name
			changed = true
		}
	}
	if in.Position != nil && *in.Position != category.Position {
		updated.Position = *in.Position
		changed = true
	}
	if !changed {
		return &updated, nil
	}
	updated.UpdatedAt = adminNow(in.Now)
	if err := deps.Repo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.ApplicationCategoryUpdated{
		At: updated.UpdatedAt, TenantID: tenantID, ActorSub: in.ActorSub, CategoryID: category.CategoryID,
	})
	return &updated, nil
}

func DeleteCategory(ctx context.Context, deps CategoryDeps, actorSub, categoryID string, now time.Time) error {
	tenantID := tenancy.TenantID(ctx)
	category, err := deps.Repo.FindByID(ctx, tenantID, categoryID)
	if err != nil {
		return err
	}
	if category == nil {
		return ErrCategoryNotFound
	}
	if err := deps.Repo.Delete(ctx, tenantID, categoryID); err != nil {
		return err
	}
	if err := deps.AppRepo.RemoveCategory(ctx, tenantID, categoryID); err != nil {
		return err
	}
	emit(deps.Emit, &spec.ApplicationCategoryDeleted{At: adminNow(now), TenantID: tenantID, ActorSub: actorSub, CategoryID: categoryID})
	return nil
}

type SetApplicationCategoriesInput struct {
	ActorSub      string
	ApplicationID string
	CategoryIDs   []string
	Now           time.Time
}

// SetApplicationCategories は Application に付与するカテゴリ集合を置き換える。
// category_ids は所属テナントの既存カテゴリのみを許し、重複は除去する。
func SetApplicationCategories(ctx context.Context, deps CategoryDeps, in SetApplicationCategoriesInput) (*spec.Application, error) {
	tenantID := tenancy.TenantID(ctx)
	app, err := deps.AppRepo.FindByID(ctx, tenantID, in.ApplicationID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}
	categories, err := deps.Repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	known := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		known[category.CategoryID] = struct{}{}
	}
	cleaned := make([]string, 0, len(in.CategoryIDs))
	seen := make(map[string]struct{}, len(in.CategoryIDs))
	for _, id := range in.CategoryIDs {
		if _, ok := known[id]; !ok {
			return nil, ErrUnknownCategory
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		cleaned = append(cleaned, id)
	}
	updated := *app
	updated.Bindings = slices.Clone(app.Bindings)
	updated.CategoryIDs = cleaned
	updated.UpdatedAt = adminNow(in.Now)
	if err := deps.AppRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.ApplicationUpdated{
		At: updated.UpdatedAt, TenantID: tenantID, ActorSub: in.ActorSub,
		ApplicationID: app.ApplicationID, ChangedFields: []string{"category_ids"},
	})
	return &updated, nil
}
