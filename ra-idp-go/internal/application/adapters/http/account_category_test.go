package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func createCategory(t *testing.T, e *echo.Echo, csrf string, cookie *http.Cookie, name string) string {
	t.Helper()
	create := adminJSON(t, e, http.MethodPost, "/api/admin/application-categories", csrf, cookie, map[string]any{"name": name})
	if create.Code != http.StatusCreated {
		t.Fatalf("create category %s status=%d body=%s", name, create.Code, create.Body.String())
	}
	var created struct {
		Category struct {
			CategoryID string `json:"category_id"`
		} `json:"category"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	return created.Category.CategoryID
}

func TestApplicationCategoryCRUDAndPortalGrouping(t *testing.T) {
	e := newApplicationHandler(t)
	csrf, cookie := appCSRF(t, e)

	catID := createCategory(t, e, csrf, cookie, "Work")
	appID := createAndAssignWeblink(t, e, csrf, cookie, "Payroll")

	// カテゴリを付与する。
	set := adminJSON(t, e, http.MethodPut, "/api/admin/applications/"+appID+"/categories", csrf, cookie, map[string]any{
		"category_ids": []string{catID},
	})
	if set.Code != http.StatusOK {
		t.Fatalf("set categories status=%d body=%s", set.Code, set.Body.String())
	}

	// ポータル応答にカテゴリ定義とアプリの category_ids が含まれる。
	request := httptest.NewRequest(http.MethodGet, "/api/account/applications", http.NoBody)
	request.Header.Set("X-Demo-Sub", "admin")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, request)
	if rec.Code != http.StatusOK {
		t.Fatalf("portal status=%d body=%s", rec.Code, rec.Body.String())
	}
	var portal struct {
		Applications []struct {
			ApplicationID string   `json:"application_id"`
			CategoryIDs   []string `json:"category_ids"`
		} `json:"applications"`
		Categories []struct {
			CategoryID string `json:"category_id"`
			Name       string `json:"name"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &portal); err != nil {
		t.Fatal(err)
	}
	if len(portal.Categories) != 1 || portal.Categories[0].Name != "Work" {
		t.Fatalf("portal categories want [Work], got %+v", portal.Categories)
	}
	if len(portal.Applications) != 1 || len(portal.Applications[0].CategoryIDs) != 1 || portal.Applications[0].CategoryIDs[0] != catID {
		t.Fatalf("app should carry category_ids: %+v", portal.Applications)
	}

	// カテゴリを削除するとアプリの category_ids からも除かれる。
	del := adminJSON(t, e, http.MethodDelete, "/api/admin/application-categories/"+catID, csrf, cookie, nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete category status=%d body=%s", del.Code, del.Body.String())
	}
	apps := myApplications(t, e, "admin")
	if len(apps) != 1 {
		t.Fatalf("want 1 app, got %d", len(apps))
	}
	if ids, _ := apps[0]["category_ids"].([]any); len(ids) != 0 {
		t.Fatalf("category must be scrubbed after delete, got %v", ids)
	}
}
