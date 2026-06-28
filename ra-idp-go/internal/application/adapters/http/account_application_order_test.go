package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func createAndAssignWeblink(t *testing.T, e *echo.Echo, csrf string, cookie *http.Cookie, name string) string {
	t.Helper()
	create := adminJSON(t, e, http.MethodPost, "/api/admin/applications", csrf, cookie, map[string]any{
		"name": name, "type": "weblink", "launch_url": "https://" + name + ".example",
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create %s status=%d body=%s", name, create.Code, create.Body.String())
	}
	var created struct {
		Application struct {
			ApplicationID string `json:"application_id"`
		} `json:"application"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	assign := adminJSON(t, e, http.MethodPost,
		"/api/admin/applications/"+created.Application.ApplicationID+"/assignments",
		csrf, cookie, map[string]any{"subject_type": "user", "subject_id": "admin"})
	if assign.Code != http.StatusCreated {
		t.Fatalf("assign %s status=%d body=%s", name, assign.Code, assign.Body.String())
	}
	return created.Application.ApplicationID
}

func portalAppNames(t *testing.T, e *echo.Echo) []string {
	t.Helper()
	apps := myApplications(t, e, "admin")
	names := make([]string, len(apps))
	for i, a := range apps {
		names[i], _ = a["name"].(string)
	}
	return names
}

func TestAccountApplicationReorderAndDefaultSort(t *testing.T) {
	e := newApplicationHandler(t)
	csrf, cookie := appCSRF(t, e)

	idAlpha := createAndAssignWeblink(t, e, csrf, cookie, "Alpha")
	idBeta := createAndAssignWeblink(t, e, csrf, cookie, "Beta")

	// 既定は name 昇順。
	if got := portalAppNames(t, e); len(got) != 2 || got[0] != "Alpha" || got[1] != "Beta" {
		t.Fatalf("default sort want [Alpha Beta], got %v", got)
	}

	// 手動順 [Beta, Alpha] を保存。
	reorder := adminJSON(t, e, http.MethodPut, "/api/account/applications/order", csrf, cookie, map[string]any{
		"application_ids": []string{idBeta, idAlpha},
	})
	if reorder.Code != http.StatusOK {
		t.Fatalf("reorder status=%d body=%s", reorder.Code, reorder.Body.String())
	}

	// ポータル一覧が手動順を反映する。
	if got := portalAppNames(t, e); len(got) != 2 || got[0] != "Beta" || got[1] != "Alpha" {
		t.Fatalf("after reorder want [Beta Alpha], got %v", got)
	}

	// GET order が保存済み順を返す。
	getOrder := httptest.NewRequest(http.MethodGet, "/api/account/applications/order", http.NoBody)
	getOrder.Header.Set("X-Demo-Sub", "admin")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, getOrder)
	if rec.Code != http.StatusOK {
		t.Fatalf("get order status=%d", rec.Code)
	}
	var order struct {
		ApplicationIDs []string `json:"application_ids"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &order); err != nil {
		t.Fatal(err)
	}
	if len(order.ApplicationIDs) != 2 || order.ApplicationIDs[0] != idBeta {
		t.Fatalf("get order mismatch: %v", order.ApplicationIDs)
	}

	// 割当外の id を含む順序は 400 で拒否する。
	bad := adminJSON(t, e, http.MethodPut, "/api/account/applications/order", csrf, cookie, map[string]any{
		"application_ids": []string{"not-assigned"},
	})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("unassigned reorder want 400, got %d body=%s", bad.Code, bad.Body.String())
	}
}
