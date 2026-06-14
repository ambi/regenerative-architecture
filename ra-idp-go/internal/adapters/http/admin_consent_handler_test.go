package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpadapter "ra-idp-go/internal/adapters/http"
	"ra-idp-go/internal/adapters/persistence/memory"
	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func TestAdminConsentListsGetsAndRevokesWithinTenant(t *testing.T) {
	e, consents, events := newAdminConsentHandler()
	now := time.Now().UTC()
	for _, consent := range []*spec.Consent{
		{
			TenantID: spec.DefaultTenantID, Sub: "alice", ClientID: "portal",
			Scopes: []string{"openid", "profile"}, State: spec.ConsentGranted,
			GrantedAt: now, ExpiresAt: now.Add(24 * time.Hour),
		},
		{
			TenantID: "acme", Sub: "alice", ClientID: "portal",
			Scopes: []string{"openid"}, State: spec.ConsentGranted,
			GrantedAt: now, ExpiresAt: now.Add(24 * time.Hour),
		},
	} {
		if err := consents.Save(context.Background(), consent); err != nil {
			t.Fatal(err)
		}
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/admin/consents", http.NoBody)
	listRequest.Header.Set("X-Demo-Sub", "admin")
	listResponse := httptest.NewRecorder()
	e.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	var list struct {
		Consents []adminConsentBody `json:"consents"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Consents) != 1 || list.Consents[0].TenantID != spec.DefaultTenantID {
		t.Fatalf("cross-tenant consent leaked: %+v", list.Consents)
	}

	getRequest := httptest.NewRequest(
		http.MethodGet, "/admin/consents/alice/portal", http.NoBody,
	)
	getRequest.Header.Set("X-Demo-Sub", "admin")
	getResponse := httptest.NewRecorder()
	e.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getResponse.Code, getResponse.Body.String())
	}

	csrf, cookie := adminCSRF(t, e, "admin")
	revokeResponse := adminJSONRequest(
		t, e, http.MethodDelete, "/admin/consents/alice/portal", csrf, cookie, nil,
	)
	if revokeResponse.Code != http.StatusNoContent {
		t.Fatalf("revoke status=%d body=%s", revokeResponse.Code, revokeResponse.Body.String())
	}
	revoked, err := consents.Find(context.Background(), spec.DefaultTenantID, "alice", "portal")
	if err != nil {
		t.Fatal(err)
	}
	if revoked == nil || revoked.State != spec.ConsentRevoked || revoked.RevokedAt == nil {
		t.Fatalf("consent not revoked: %+v", revoked)
	}
	if len(*events) != 1 || (*events)[0].EventType() != "ConsentRevoked" {
		t.Fatalf("events=%v", *events)
	}
	event, ok := (*events)[0].(*spec.ConsentRevokedEvent)
	if !ok || event.ActorSub != "admin" {
		t.Fatalf("event=%+v", (*events)[0])
	}
}

func TestAdminConsentRequiresAdminAndHidesOtherTenant(t *testing.T) {
	e, consents, _ := newAdminConsentHandler()
	now := time.Now().UTC()
	if err := consents.Save(context.Background(), &spec.Consent{
		TenantID: "acme", Sub: "alice", ClientID: "portal", Scopes: []string{"openid"},
		State: spec.ConsentGranted, GrantedAt: now, ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/admin/consents/alice/portal", http.NoBody)
	request.Header.Set("X-Demo-Sub", "admin")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant status=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/admin/consents", http.NoBody)
	request.Header.Set("X-Demo-Sub", "regular")
	response = httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-admin status=%d body=%s", response.Code, response.Body.String())
	}
}

type adminConsentBody struct {
	TenantID string `json:"tenant_id"`
}

func newAdminConsentHandler() (*echo.Echo, *memory.ConsentRepository, *[]spec.DomainEvent) {
	users := memory.NewUserRepository()
	consents := memory.NewConsentRepository()
	now := time.Now().UTC()
	users.Seed(&spec.User{
		Sub: "admin", TenantID: spec.DefaultTenantID, PreferredUsername: "admin",
		PasswordHash: "unused", Roles: []string{"admin"}, CreatedAt: now, UpdatedAt: now,
	})
	users.Seed(&spec.User{
		Sub: "regular", TenantID: spec.DefaultTenantID, PreferredUsername: "regular",
		PasswordHash: "unused", CreatedAt: now, UpdatedAt: now,
	})
	events := []spec.DomainEvent{}
	e := echo.New()
	httpadapter.Register(e, httpadapter.Deps{
		Issuer: "http://idp.test", UserRepo: users, ConsentRepo: consents,
		AuthnResolver: authusecases.DemoHeaderResolver{},
		Emit: func(event spec.DomainEvent) {
			events = append(events, event)
		},
	})
	return e, consents, &events
}
