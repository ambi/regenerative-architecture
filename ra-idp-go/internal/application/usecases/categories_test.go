package usecases_test

import (
	"errors"
	"testing"
	"time"

	appusecases "ra-idp-go/internal/application/usecases"
	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/spec"
)

func newCategoryDeps() (appusecases.CategoryDeps, appusecases.ApplicationDeps) {
	apps := memory.NewApplicationRepository()
	assignments := memory.NewApplicationAssignmentRepository()
	categories := memory.NewApplicationCategoryRepository()
	return appusecases.CategoryDeps{Repo: categories, AppRepo: apps},
		appusecases.ApplicationDeps{Repo: apps, AssignmentRepo: assignments}
}

func TestCreateCategoryAssignsTrailingPosition(t *testing.T) {
	ctx := tenantContext()
	deps, _ := newCategoryDeps()

	first, err := appusecases.CreateCategory(ctx, deps, appusecases.CreateCategoryInput{ActorSub: "admin", Name: "Work"})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if first.Position != 0 {
		t.Fatalf("first position want 0, got %d", first.Position)
	}
	second, err := appusecases.CreateCategory(ctx, deps, appusecases.CreateCategoryInput{ActorSub: "admin", Name: "Personal"})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if second.Position != 1 {
		t.Fatalf("second position want 1, got %d", second.Position)
	}
}

func TestCreateCategoryRejectsBlankName(t *testing.T) {
	ctx := tenantContext()
	deps, _ := newCategoryDeps()
	if _, err := appusecases.CreateCategory(ctx, deps, appusecases.CreateCategoryInput{ActorSub: "admin", Name: "  "}); !errors.Is(err, appusecases.ErrCategoryNameRequired) {
		t.Fatalf("expected ErrCategoryNameRequired, got %v", err)
	}
}

func TestSetApplicationCategoriesValidatesAndDedups(t *testing.T) {
	ctx := tenantContext()
	deps, appDeps := newCategoryDeps()

	work, err := appusecases.CreateCategory(ctx, deps, appusecases.CreateCategoryInput{ActorSub: "admin", Name: "Work"})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	app, err := appusecases.CreateApplication(ctx, appDeps, appusecases.CreateApplicationInput{
		ActorSub: "admin", Name: "Payroll", Kind: spec.ApplicationFederated,
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}

	// 重複を含めても 1 件に正規化される。
	updated, err := appusecases.SetApplicationCategories(ctx, deps, appusecases.SetApplicationCategoriesInput{
		ActorSub: "admin", ApplicationID: app.ApplicationID, CategoryIDs: []string{work.CategoryID, work.CategoryID},
	})
	if err != nil {
		t.Fatalf("set categories: %v", err)
	}
	if len(updated.CategoryIDs) != 1 || updated.CategoryIDs[0] != work.CategoryID {
		t.Fatalf("category_ids should dedup to one: %v", updated.CategoryIDs)
	}

	// 未知のカテゴリは拒否する。
	if _, err := appusecases.SetApplicationCategories(ctx, deps, appusecases.SetApplicationCategoriesInput{
		ActorSub: "admin", ApplicationID: app.ApplicationID, CategoryIDs: []string{"nope"},
	}); !errors.Is(err, appusecases.ErrUnknownCategory) {
		t.Fatalf("expected ErrUnknownCategory, got %v", err)
	}
}

func TestDeleteCategoryScrubsFromApplications(t *testing.T) {
	ctx := tenantContext()
	deps, appDeps := newCategoryDeps()

	work, err := appusecases.CreateCategory(ctx, deps, appusecases.CreateCategoryInput{ActorSub: "admin", Name: "Work"})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	app, err := appusecases.CreateApplication(ctx, appDeps, appusecases.CreateApplicationInput{
		ActorSub: "admin", Name: "Payroll", Kind: spec.ApplicationFederated,
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	if _, err := appusecases.SetApplicationCategories(ctx, deps, appusecases.SetApplicationCategoriesInput{
		ActorSub: "admin", ApplicationID: app.ApplicationID, CategoryIDs: []string{work.CategoryID},
	}); err != nil {
		t.Fatalf("set categories: %v", err)
	}

	if err := appusecases.DeleteCategory(ctx, deps, "admin", work.CategoryID, time.Time{}); err != nil {
		t.Fatalf("delete category: %v", err)
	}
	got, err := appDeps.Repo.FindByID(ctx, "acme", app.ApplicationID)
	if err != nil {
		t.Fatalf("find app: %v", err)
	}
	if len(got.CategoryIDs) != 0 {
		t.Fatalf("deleted category must be scrubbed from app, got %v", got.CategoryIDs)
	}
}
