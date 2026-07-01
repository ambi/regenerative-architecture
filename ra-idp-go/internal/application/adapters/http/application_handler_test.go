package http_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"
	httpadapter "ra-idp-go/internal/shared/adapters/http/server"
	"ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/shared/adapters/persistence/memory"
	"ra-idp-go/internal/shared/spec"

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
	httpadapter.Register(e, support.Deps{
		Issuer: "http://idp.test", UserRepo: users, GroupRepo: memory.NewGroupRepository(),
		ApplicationRepo:           memory.NewApplicationRepository(),
		ApplicationIconStore:      memory.NewApplicationIconStore(),
		ApplicationAssignmentRepo: memory.NewApplicationAssignmentRepository(),
		ApplicationOrderingRepo:   memory.NewApplicationOrderingRepository(),
		ApplicationCategoryRepo:   memory.NewApplicationCategoryRepository(),
		SamlSPRepo:                memory.NewSamlServiceProviderRepository(),
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

func adminMultipart(t *testing.T, e *echo.Echo, path, csrf string, cookie *http.Cookie, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
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

// claim 規則を持たない SAML SP の詳細では rules が null ではなく [] になり、
// UI が rules.length を参照してもクラッシュしない (作成直後の "認証を続行できません" 回帰)。
func TestSamlApplicationDetailReturnsEmptyRulesNotNull(t *testing.T) {
	e := newApplicationHandler(t)
	csrf, cookie := appCSRF(t, e)

	create := adminJSON(t, e, http.MethodPost, "/api/admin/applications", csrf, cookie, map[string]any{
		"name":           "SAML App",
		"type":           "saml",
		"entity_id":      "https://sp.example.com",
		"acs_urls":       []string{"https://sp.example.com/acs"},
		"name_id_format": "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
		"name_id_source": "sub",
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

	detail := adminJSON(t, e, http.MethodGet,
		"/api/admin/applications/"+created.Application.ApplicationID, csrf, cookie, nil)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	// 生 JSON に "rules":null が現れてはならない。
	if bytes.Contains(detail.Body.Bytes(), []byte(`"rules":null`)) {
		t.Fatalf("saml rules must serialize as [] not null: %s", detail.Body.String())
	}
	var body struct {
		Saml *struct {
			Rules []spec.ClaimMappingRule `json:"rules"`
		} `json:"saml"`
	}
	if err := json.Unmarshal(detail.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Saml == nil {
		t.Fatalf("saml config missing: %s", detail.Body.String())
	}
	if body.Saml.Rules == nil {
		t.Fatal("saml.rules decoded as nil; expected empty array")
	}
}

func TestApplicationIconUploadServeRejectAndDelete(t *testing.T) {
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
	appID := created.Application.ApplicationID

	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	upload := adminMultipart(t, e, "/api/admin/applications/"+appID+"/icon", csrf, cookie, "icon.png", png)
	if upload.Code != http.StatusOK {
		t.Fatalf("upload status=%d body=%s", upload.Code, upload.Body.String())
	}
	var uploaded struct {
		Application struct {
			IconURL       string `json:"icon_url"`
			IconObjectKey string `json:"icon_object_key"`
		} `json:"application"`
	}
	if err := json.Unmarshal(upload.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}
	if uploaded.Application.IconURL == "" || uploaded.Application.IconObjectKey == "" {
		t.Fatalf("missing icon fields: %s", upload.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, uploaded.Application.IconURL, http.NoBody)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, get)
	if response.Code != http.StatusOK {
		t.Fatalf("icon get status=%d body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content-type=%q", got)
	}
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("nosniff=%q", got)
	}
	if !bytes.Equal(response.Body.Bytes(), png) {
		t.Fatalf("icon body mismatch: %v", response.Body.Bytes())
	}

	reject := adminMultipart(t, e, "/api/admin/applications/"+appID+"/icon", csrf, cookie, "icon.txt", []byte("not an image"))
	if reject.Code != http.StatusBadRequest {
		t.Fatalf("reject status=%d body=%s", reject.Code, reject.Body.String())
	}

	deleted := adminJSON(t, e, http.MethodDelete, "/api/admin/applications/"+appID+"/icon", csrf, cookie, nil)
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete icon status=%d body=%s", deleted.Code, deleted.Body.String())
	}
	response = httptest.NewRecorder()
	e.ServeHTTP(response, get)
	if response.Code != http.StatusNotFound {
		t.Fatalf("deleted icon status=%d body=%s", response.Code, response.Body.String())
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
