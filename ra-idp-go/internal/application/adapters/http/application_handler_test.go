package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"
	httpadapter "ra-idp-go/internal/platform/http"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func newApplicationHandler(t *testing.T) *echo.Echo {
	t.Helper()
	users := memory.NewUserRepository()
	now := time.Now().UTC()
	users.Seed(&spec.User{
		Sub: "admin", TenantID: spec.DefaultTenantID, PreferredUsername: "admin",
		PasswordHash: "unused", Roles: []string{"admin"}, CreatedAt: now, UpdatedAt: now,
	})
	users.Seed(&spec.User{
		Sub: "regular", TenantID: spec.DefaultTenantID, PreferredUsername: "regular",
		PasswordHash: "unused", CreatedAt: now, UpdatedAt: now,
	})
	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer: "http://idp.test", UserRepo: users, GroupRepo: memory.NewGroupRepository(),
		ApplicationRepo:           memory.NewApplicationRepository(),
		ApplicationAssignmentRepo: memory.NewApplicationAssignmentRepository(),
		AuthnResolver:             authusecases.DemoHeaderResolver{},
		Emit:                      func(spec.DomainEvent) {},
	})
	return e
}

func appCSRF(t *testing.T, e *echo.Echo) (string, *http.Cookie) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/account", http.NoBody)
	request.Header.Set("X-Demo-Sub", "admin")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("account status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("csrf cookie missing")
	}
	return body.CSRFToken, cookies[0]
}

func adminJSON(t *testing.T, e *echo.Echo, method, path, csrf string, cookie *http.Cookie, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://idp.test")
	request.Header.Set("X-CSRF-Token", csrf)
	request.Header.Set("X-Demo-Sub", "admin")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	return response
}

func myApplications(t *testing.T, e *echo.Echo, sub string) []map[string]any {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/account/applications", http.NoBody)
	request.Header.Set("X-Demo-Sub", sub)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("account applications status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Applications []map[string]any `json:"applications"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Applications
}

func TestApplicationAdminCRUDAndAccountVisibility(t *testing.T) {
	e := newApplicationHandler(t)
	csrf, cookie := appCSRF(t, e)

	create := adminJSON(t, e, http.MethodPost, "/api/admin/applications", csrf, cookie, map[string]any{
		"name": "Payroll", "type": "weblink", "launch_url": "https://payroll.example",
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		Application struct {
			ApplicationID string `json:"application_id"`
		} `json:"application"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Application.ApplicationID == "" {
		t.Fatalf("missing application_id: %s", create.Body.String())
	}
	appID := created.Application.ApplicationID

	// 未割当の regular はポータルに出ない。
	if apps := myApplications(t, e, "regular"); len(apps) != 0 {
		t.Fatalf("unassigned user should see no apps, got %d", len(apps))
	}

	// 割当すると出る。
	assign := adminJSON(t, e, http.MethodPost, "/api/admin/applications/"+appID+"/assignments", csrf, cookie, map[string]any{
		"subject_type": "user", "subject_id": "regular",
	})
	if assign.Code != http.StatusCreated {
		t.Fatalf("assign status=%d body=%s", assign.Code, assign.Body.String())
	}
	if apps := myApplications(t, e, "regular"); len(apps) != 1 {
		t.Fatalf("assigned user should see 1 app, got %d", len(apps))
	}

	// hidden 割当に上書きするとポータルから消える。
	hidden := adminJSON(t, e, http.MethodPost, "/api/admin/applications/"+appID+"/assignments", csrf, cookie, map[string]any{
		"subject_type": "user", "subject_id": "regular", "visibility": "hidden",
	})
	if hidden.Code != http.StatusCreated {
		t.Fatalf("hidden assign status=%d body=%s", hidden.Code, hidden.Body.String())
	}
	if apps := myApplications(t, e, "regular"); len(apps) != 0 {
		t.Fatalf("hidden assignment should hide app from portal, got %d", len(apps))
	}
}

func TestApplicationCreateRejectsNonAdmin(t *testing.T) {
	e := newApplicationHandler(t)
	csrf, cookie := appCSRF(t, e)
	request := httptest.NewRequest(http.MethodPost, "/api/admin/applications", bytes.NewReader([]byte(`{"name":"X","kind":"federated"}`)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://idp.test")
	request.Header.Set("X-CSRF-Token", csrf)
	request.Header.Set("X-Demo-Sub", "regular")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-admin create status=%d body=%s", response.Code, response.Body.String())
	}
}
