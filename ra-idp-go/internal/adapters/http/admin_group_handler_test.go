package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpadapter "ra-idp-go/internal/adapters/http"
	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func newAdminGroupHandler(t *testing.T) (*echo.Echo, *memory.GroupRepository) {
	t.Helper()
	userRepo := memory.NewUserRepository()
	groupRepo := memory.NewGroupRepository()
	now := time.Now().UTC()
	userRepo.Seed(&spec.User{
		Sub: "admin", PreferredUsername: "admin", PasswordHash: "unused",
		Roles: []string{"admin"}, CreatedAt: now, UpdatedAt: now,
	})
	userRepo.Seed(&spec.User{
		Sub: "alice", PreferredUsername: "alice", PasswordHash: "unused",
		Roles: []string{}, CreatedAt: now, UpdatedAt: now,
	})
	e := echo.New()
	httpadapter.Register(e, httpadapter.Deps{
		Issuer: "http://idp.test", UserRepo: userRepo, GroupRepo: groupRepo,
		AuthnResolver: authusecases.DemoHeaderResolver{},
	})
	return e, groupRepo
}

func TestAdminGroupAPIRequiresAdminRole(t *testing.T) {
	e, _ := newAdminGroupHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/api/admin/groups", http.NoBody)
	request.Header.Set("X-Demo-Sub", "alice")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAdminGroupAPICreateAddMemberAndEffectiveRoles(t *testing.T) {
	e, _ := newAdminGroupHandler(t)
	csrf, cookie := adminCSRF(t, e)

	create := adminJSONRequest(t, e, http.MethodPost, "/api/admin/groups", csrf, cookie, map[string]any{
		"name": "engineering", "roles": []string{"catalog:read"},
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	// 名前一意性: 409
	conflict := adminJSONRequest(t, e, http.MethodPost, "/api/admin/groups", csrf, cookie, map[string]any{"name": "engineering"})
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d body=%s", conflict.Code, conflict.Body.String())
	}

	add := adminJSONRequest(t, e, http.MethodPost, "/api/admin/groups/"+created.ID+"/members/alice", csrf, cookie, nil)
	if add.Code != http.StatusNoContent {
		t.Fatalf("add member status=%d body=%s", add.Code, add.Body.String())
	}
	// 冪等な再追加も 204
	if again := adminJSONRequest(t, e, http.MethodPost, "/api/admin/groups/"+created.ID+"/members/alice", csrf, cookie, nil); again.Code != http.StatusNoContent {
		t.Fatalf("idempotent add status=%d", again.Code)
	}

	groupsResp := httptest.NewRequest(http.MethodGet, "/api/admin/users/alice/groups", http.NoBody)
	groupsResp.Header.Set("X-Demo-Sub", "admin")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, groupsResp)
	if rec.Code != http.StatusOK {
		t.Fatalf("user groups status=%d body=%s", rec.Code, rec.Body.String())
	}
	var view struct {
		EffectiveRoles []string `json:"effective_roles"`
		GroupRoles     []string `json:"group_roles"`
		DirectRoles    []string `json:"direct_roles"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.EffectiveRoles) != 1 || view.EffectiveRoles[0] != "catalog:read" {
		t.Fatalf("effective roles=%v", view.EffectiveRoles)
	}
	if len(view.DirectRoles) != 0 {
		t.Fatalf("direct roles=%v", view.DirectRoles)
	}
}

// alice をグループ経由で admin にすると admin API を通過できる (effective roles)。
func TestGroupDerivedAdminRolePassesRBAC(t *testing.T) {
	e, groupRepo := newAdminGroupHandler(t)
	ctx := httptest.NewRequest(http.MethodGet, "/", http.NoBody).Context()
	group := &spec.Group{ID: "group_admins", TenantID: "default", Name: "admins", Roles: []string{"admin"}, CreatedAt: time.Now().UTC()}
	if err := groupRepo.Save(ctx, group); err != nil {
		t.Fatal(err)
	}
	if _, err := groupRepo.AddMember(ctx, &spec.GroupMember{GroupID: group.ID, UserSub: "alice", AddedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/admin/groups", http.NoBody)
	request.Header.Set("X-Demo-Sub", "alice")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("group-derived admin denied: status=%d body=%s", response.Code, response.Body.String())
	}
}
