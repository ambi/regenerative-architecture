package usecases_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	idmusecases "ra-idp-go/internal/identitymanagement/usecases"
	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

func newAgentDeps(t *testing.T) (idmusecases.AdminAgentDeps, *[]spec.DomainEvent) {
	t.Helper()
	clientRepo := memory.NewClientRepository()
	userRepo := memory.NewUserRepository()
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	_ = clientRepo.Save(context.Background(), &spec.OAuth2Client{
		TenantID: "default", ClientID: "svc_client", ClientType: spec.ClientConfidential,
		RedirectURIs:             []string{"https://app.example/cb"},
		GrantTypes:               []spec.GrantType{spec.GrantClientCredentials},
		TokenEndpointAuthMethod:  spec.AuthMethodClientSecretBasic,
		IDTokenSignedResponseAlg: spec.SigAlgPS256, FapiProfile: spec.FapiNone, CreatedAt: now,
	})
	userRepo.Seed(&spec.User{
		Sub: "operator", TenantID: "default", PreferredUsername: "operator",
		PasswordHash: "hash", CreatedAt: now, UpdatedAt: now,
	})
	userRepo.Seed(&spec.User{
		Sub: "user_new", TenantID: "default", PreferredUsername: "user-new",
		PasswordHash: "hash", CreatedAt: now, UpdatedAt: now,
	})
	events := &[]spec.DomainEvent{}
	deps := idmusecases.AdminAgentDeps{
		AgentRepo:  memory.NewAgentRepository(),
		ClientRepo: clientRepo,
		UserRepo:   userRepo,
		Emit:       func(e spec.DomainEvent) { *events = append(*events, e) },
	}
	return deps, events
}

func agentEventTypes(events []spec.DomainEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.EventType()
	}
	return out
}

// defaultTenantCtx は tenancy.TenantID が "default" を返す素の context。
func defaultTenantCtx() context.Context {
	return context.Background()
}

func tenantCtx(id string) context.Context {
	return tenancy.WithTenant(context.Background(), &spec.Tenant{ID: id}, "https://idp.example", "")
}

func TestRegisterAgentNameUniquenessAndOwnerDefault(t *testing.T) {
	ctx := defaultTenantCtx()
	deps, events := newAgentDeps(t)
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)

	agent, err := idmusecases.RegisterAgent(ctx, deps, idmusecases.RegisterAgentInput{
		ActorSub: "operator", Name: "deploy-bot", Roles: []string{"deploy:run"}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if agent.OwnerSub != "operator" {
		t.Fatalf("owner_sub default = %q, want operator", agent.OwnerSub)
	}
	if agent.Status != spec.AgentStatusActive || agent.Kind != spec.AgentKindSupervised {
		t.Fatalf("unexpected defaults: status=%q kind=%q", agent.Status, agent.Kind)
	}

	// 名前一意性 (大文字小文字無視)
	if _, err := idmusecases.RegisterAgent(ctx, deps, idmusecases.RegisterAgentInput{
		ActorSub: "operator", Name: "Deploy-Bot", Now: now,
	}); !errors.Is(err, idmusecases.ErrAgentNameConflict) {
		t.Fatalf("expected name conflict, got %v", err)
	}

	if !slices.Equal(agentEventTypes(*events), []string{"AgentRegistered"}) {
		t.Fatalf("events = %v", agentEventTypes(*events))
	}
}

func TestGetAgentRejectsCrossTenant(t *testing.T) {
	deps, _ := newAgentDeps(t)
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	agent, err := idmusecases.RegisterAgent(defaultTenantCtx(), deps, idmusecases.RegisterAgentInput{
		ActorSub: "operator", Name: "deploy-bot", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	otherCtx := tenantCtx("acme")
	if _, err := idmusecases.GetAgent(otherCtx, deps, agent.ID); !errors.Is(err, idmusecases.ErrAgentNotFound) {
		t.Fatalf("expected ErrAgentNotFound cross-tenant, got %v", err)
	}
}

func TestSetAgentDisabledThenEnable(t *testing.T) {
	ctx := defaultTenantCtx()
	deps, events := newAgentDeps(t)
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	agent, err := idmusecases.RegisterAgent(ctx, deps, idmusecases.RegisterAgentInput{
		ActorSub: "operator", Name: "deploy-bot", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	*events = (*events)[:0]

	disabled, err := idmusecases.SetAgentDisabled(ctx, deps, "operator", agent.ID, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Status != spec.AgentStatusDisabled || disabled.DisabledAt == nil || disabled.IsActive() {
		t.Fatalf("disabled agent unexpected: %+v", disabled)
	}
	enabled, err := idmusecases.SetAgentDisabled(ctx, deps, "operator", agent.ID, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if enabled.Status != spec.AgentStatusActive || enabled.DisabledAt != nil || !enabled.IsActive() {
		t.Fatalf("enabled agent unexpected: %+v", enabled)
	}
	if !slices.Equal(agentEventTypes(*events), []string{"AgentDisabled", "AgentEnabled"}) {
		t.Fatalf("events = %v", agentEventTypes(*events))
	}
}

func TestKillAgentIsIrreversible(t *testing.T) {
	ctx := defaultTenantCtx()
	deps, events := newAgentDeps(t)
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	agent, err := idmusecases.RegisterAgent(ctx, deps, idmusecases.RegisterAgentInput{
		ActorSub: "operator", Name: "deploy-bot", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	*events = (*events)[:0]

	killed, err := idmusecases.KillAgent(ctx, deps, "operator", agent.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if killed.Status != spec.AgentStatusKilled || killed.KilledAt == nil || killed.IsActive() {
		t.Fatalf("killed agent unexpected: %+v", killed)
	}
	// 再 kill / enable / update は reject
	if _, err := idmusecases.KillAgent(ctx, deps, "operator", agent.ID, now); !errors.Is(err, idmusecases.ErrAgentKilled) {
		t.Fatalf("expected ErrAgentKilled on re-kill, got %v", err)
	}
	if _, err := idmusecases.SetAgentDisabled(ctx, deps, "operator", agent.ID, false, now); !errors.Is(err, idmusecases.ErrAgentKilled) {
		t.Fatalf("expected ErrAgentKilled on enable-after-kill, got %v", err)
	}
	if _, err := idmusecases.UpdateAgent(ctx, deps, idmusecases.UpdateAgentInput{
		ActorSub: "operator", ID: agent.ID, Name: ptr("x"),
	}); !errors.Is(err, idmusecases.ErrAgentKilled) {
		t.Fatalf("expected ErrAgentKilled on update-after-kill, got %v", err)
	}
	if !slices.Equal(agentEventTypes(*events), []string{"AgentKilled"}) {
		t.Fatalf("events = %v", agentEventTypes(*events))
	}
}

func TestUpdateAgentOwnerChangeEmitsOwnerChanged(t *testing.T) {
	ctx := defaultTenantCtx()
	deps, events := newAgentDeps(t)
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	agent, err := idmusecases.RegisterAgent(ctx, deps, idmusecases.RegisterAgentInput{
		ActorSub: "operator", Name: "deploy-bot", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	*events = (*events)[:0]

	updated, err := idmusecases.UpdateAgent(ctx, deps, idmusecases.UpdateAgentInput{
		ActorSub: "operator", ID: agent.ID, OwnerSub: ptr("user_new"), Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.OwnerSub != "user_new" {
		t.Fatalf("owner not changed: %q", updated.OwnerSub)
	}
	if !slices.Equal(agentEventTypes(*events), []string{"AgentUpdated", "AgentOwnerChanged"}) {
		t.Fatalf("events = %v", agentEventTypes(*events))
	}
}

func TestRegisterAndUpdateAgentRejectUnknownOwner(t *testing.T) {
	ctx := defaultTenantCtx()
	deps, _ := newAgentDeps(t)
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	if _, err := idmusecases.RegisterAgent(ctx, deps, idmusecases.RegisterAgentInput{
		ActorSub: "operator", Name: "deploy-bot", OwnerSub: "ghost", Now: now,
	}); !errors.Is(err, idmusecases.ErrAgentOwnerNotFound) {
		t.Fatalf("expected ErrAgentOwnerNotFound on register, got %v", err)
	}
	agent, err := idmusecases.RegisterAgent(ctx, deps, idmusecases.RegisterAgentInput{
		ActorSub: "operator", Name: "deploy-bot", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idmusecases.UpdateAgent(ctx, deps, idmusecases.UpdateAgentInput{
		ActorSub: "operator", ID: agent.ID, OwnerSub: ptr("ghost"), Now: now,
	}); !errors.Is(err, idmusecases.ErrAgentOwnerNotFound) {
		t.Fatalf("expected ErrAgentOwnerNotFound on update, got %v", err)
	}
}

func TestBindUnbindCredentialAndFindByClientID(t *testing.T) {
	ctx := defaultTenantCtx()
	deps, events := newAgentDeps(t)
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	agent, err := idmusecases.RegisterAgent(ctx, deps, idmusecases.RegisterAgentInput{
		ActorSub: "operator", Name: "deploy-bot", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	*events = (*events)[:0]

	if err := idmusecases.BindCredential(ctx, deps, "operator", agent.ID, "svc_client", now); err != nil {
		t.Fatal(err)
	}
	// 冪等再束縛は event を増やさない
	if err := idmusecases.BindCredential(ctx, deps, "operator", agent.ID, "svc_client", now); err != nil {
		t.Fatal(err)
	}
	// 未知 client は reject
	if err := idmusecases.BindCredential(ctx, deps, "operator", agent.ID, "ghost", now); !errors.Is(err, idmusecases.ErrAgentClientNotFound) {
		t.Fatalf("expected ErrAgentClientNotFound, got %v", err)
	}

	found, err := deps.AgentRepo.FindByClientID(ctx, "default", "svc_client")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ID != agent.ID {
		t.Fatalf("FindByClientID returned %+v", found)
	}

	view, err := idmusecases.GetAgent(ctx, deps, agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(view.ClientIDs, []string{"svc_client"}) {
		t.Fatalf("client_ids = %v", view.ClientIDs)
	}

	if err := idmusecases.UnbindCredential(ctx, deps, "operator", agent.ID, "svc_client", now); err != nil {
		t.Fatal(err)
	}
	found, err = deps.AgentRepo.FindByClientID(ctx, "default", "svc_client")
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatalf("expected nil after unbind, got %+v", found)
	}

	if !slices.Equal(agentEventTypes(*events), []string{"AgentCredentialBound", "AgentCredentialUnbound"}) {
		t.Fatalf("events = %v", agentEventTypes(*events))
	}
}

func TestBindCredentialRejectsClientAlreadyBoundToAnotherAgent(t *testing.T) {
	ctx := defaultTenantCtx()
	deps, _ := newAgentDeps(t)
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	first, err := idmusecases.RegisterAgent(ctx, deps, idmusecases.RegisterAgentInput{
		ActorSub: "operator", Name: "deploy-bot", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := idmusecases.RegisterAgent(ctx, deps, idmusecases.RegisterAgentInput{
		ActorSub: "operator", Name: "report-bot", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := idmusecases.BindCredential(ctx, deps, "operator", first.ID, "svc_client", now); err != nil {
		t.Fatal(err)
	}
	if err := idmusecases.BindCredential(ctx, deps, "operator", second.ID, "svc_client", now); !errors.Is(err, idmusecases.ErrAgentClientBound) {
		t.Fatalf("expected ErrAgentClientBound, got %v", err)
	}
}

func TestDeleteAgentEmitsAgentDeleted(t *testing.T) {
	ctx := defaultTenantCtx()
	deps, events := newAgentDeps(t)
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	agent, err := idmusecases.RegisterAgent(ctx, deps, idmusecases.RegisterAgentInput{
		ActorSub: "operator", Name: "deploy-bot", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	*events = (*events)[:0]
	if err := idmusecases.DeleteAgent(ctx, deps, "operator", agent.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := idmusecases.GetAgent(ctx, deps, agent.ID); !errors.Is(err, idmusecases.ErrAgentNotFound) {
		t.Fatalf("expected ErrAgentNotFound after delete, got %v", err)
	}
	if !slices.Equal(agentEventTypes(*events), []string{"AgentDeleted"}) {
		t.Fatalf("events = %v", agentEventTypes(*events))
	}
}

func TestDeleteKilledAgentIsRejected(t *testing.T) {
	ctx := defaultTenantCtx()
	deps, _ := newAgentDeps(t)
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	agent, err := idmusecases.RegisterAgent(ctx, deps, idmusecases.RegisterAgentInput{
		ActorSub: "operator", Name: "deploy-bot", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idmusecases.KillAgent(ctx, deps, "operator", agent.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := idmusecases.DeleteAgent(ctx, deps, "operator", agent.ID, now); !errors.Is(err, idmusecases.ErrAgentKilled) {
		t.Fatalf("expected ErrAgentKilled, got %v", err)
	}
	found, err := idmusecases.GetAgent(ctx, deps, agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.Agent.Status != spec.AgentStatusKilled {
		t.Fatalf("agent was deleted or changed: %+v", found.Agent)
	}
}

func ptr[T any](v T) *T { return &v }
