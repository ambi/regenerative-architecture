// ApplicationCategory の管理 API (wi-70, ADR-069)。RequireAdmin で保護し、テナント境界に閉じる。
package http

import (
	"errors"
	"net/http"
	"time"

	appusecases "ra-idp-go/internal/application/usecases"
	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type categoryResponse struct {
	CategoryID string    `json:"category_id"`
	Name       string    `json:"name"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type categoryRequest struct {
	Name     string `json:"name"`
	Position *int   `json:"position"`
}

type applicationCategoriesRequest struct {
	CategoryIDs []string `json:"category_ids"`
}

func (d Deps) handleListCategories(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	categories, err := appusecases.ListCategories(c.Request().Context(), d.categoryDeps())
	if err != nil {
		return err
	}
	out := make([]categoryResponse, len(categories))
	for i, category := range categories {
		out[i] = toCategoryResponse(category)
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"categories": out})
}

func (d Deps) handleCreateCategory(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var req categoryRequest
	if err := core.DecodeJSON(c.Request(), &req); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	category, err := appusecases.CreateCategory(c.Request().Context(), d.categoryDeps(), appusecases.CreateCategoryInput{
		ActorSub: actor.Sub, Name: req.Name, Position: req.Position, Now: time.Now().UTC(),
	})
	if err != nil {
		return d.writeCategoryError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusCreated, map[string]any{"category": toCategoryResponse(category)})
}

func (d Deps) handleUpdateCategory(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var req categoryRequest
	if err := core.DecodeJSON(c.Request(), &req); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	name := &req.Name
	if req.Name == "" {
		name = nil
	}
	category, err := appusecases.UpdateCategory(c.Request().Context(), d.categoryDeps(), appusecases.UpdateCategoryInput{
		ActorSub: actor.Sub, CategoryID: c.Param("category_id"), Name: name, Position: req.Position, Now: time.Now().UTC(),
	})
	if err != nil {
		return d.writeCategoryError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"category": toCategoryResponse(category)})
}

func (d Deps) handleDeleteCategory(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if err := appusecases.DeleteCategory(
		c.Request().Context(), d.categoryDeps(), actor.Sub, c.Param("category_id"), time.Now().UTC(),
	); err != nil {
		return d.writeCategoryError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleSetApplicationCategories(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var req applicationCategoriesRequest
	if err := core.DecodeJSON(c.Request(), &req); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	app, err := appusecases.SetApplicationCategories(c.Request().Context(), d.categoryDeps(), appusecases.SetApplicationCategoriesInput{
		ActorSub: actor.Sub, ApplicationID: c.Param("application_id"), CategoryIDs: req.CategoryIDs, Now: time.Now().UTC(),
	})
	if err != nil {
		return d.writeCategoryError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusOK, toApplicationResponse(app))
}

func (d Deps) categoryDeps() appusecases.CategoryDeps {
	return appusecases.CategoryDeps{Repo: d.ApplicationCategoryRepo, AppRepo: d.ApplicationRepo, Emit: d.Emit}
}

func (d Deps) writeCategoryError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, appusecases.ErrCategoryNotFound):
		return core.WriteBrowserError(c, http.StatusNotFound, "category_not_found", "カテゴリが存在しません")
	case errors.Is(err, appusecases.ErrApplicationNotFound):
		return core.WriteBrowserError(c, http.StatusNotFound, "application_not_found", "アプリケーションが存在しません")
	case errors.Is(err, appusecases.ErrCategoryNameRequired):
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "カテゴリ名を入力してください")
	case errors.Is(err, appusecases.ErrUnknownCategory):
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "存在しないカテゴリは付与できません")
	default:
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", err.Error())
	}
}

func toCategoryResponse(category *spec.ApplicationCategory) categoryResponse {
	return categoryResponse{
		CategoryID: category.CategoryID, Name: category.Name, Position: category.Position,
		CreatedAt: category.CreatedAt, UpdatedAt: category.UpdatedAt,
	}
}
