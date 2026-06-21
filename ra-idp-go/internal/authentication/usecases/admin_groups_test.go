package usecases_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"
)

func newGroupDeps(t *testing.T) (authusecases.AdminGroupDeps, *[]spec.DomainEvent) {
	t.Helper()
	userRepo := memory.NewUserRepository()
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	userRepo.Seed(&spec.User{
		Sub: "user_alice", TenantID: "default", PreferredUsername: "alice",
		PasswordHash: "x", Roles: []string{}, CreatedAt: now, UpdatedAt: now,
	})
	userRepo.Seed(&spec.User{
		Sub: "user_other", TenantID: "acme", PreferredUsername: "other",
		PasswordHash: "x", Roles: []string{}, CreatedAt: now, UpdatedAt: now,
	})
	events := &[]spec.DomainEvent{}
	deps := authusecases.AdminGroupDeps{
		GroupRepo: memory.NewGroupRepository(),
		UserRepo:  userRepo,
		Emit:      func(e spec.DomainEvent) { *events = append(*events, e) },
	}
	return deps, events
}

func eventTypes(events []spec.DomainEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.EventType()
	}
	return out
}

func TestGroupCreateAddMemberEffectiveRoles(t *testing.T) {
	ctx := context.Background()
	deps, events := newGroupDeps(t)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)

	group, err := authusecases.CreateGroup(ctx, deps, authusecases.CreateGroupInput{
		ActorSub: "operator", Name: "engineering", Roles: []string{"catalog:read"}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 名前一意性
	if _, err := authusecases.CreateGroup(ctx, deps, authusecases.CreateGroupInput{
		ActorSub: "operator", Name: "Engineering", Now: now,
	}); !errors.Is(err, authusecases.ErrGroupNameConflict) {
		t.Fatalf("expected name conflict, got %v", err)
	}

	if err := authusecases.AddMember(ctx, deps, "operator", group.ID, "user_alice", now); err != nil {
		t.Fatal(err)
	}
	// 冪等: 再追加では event は増えない
	if err := authusecases.AddMember(ctx, deps, "operator", group.ID, "user_alice", now); err != nil {
		t.Fatal(err)
	}

	view, err := authusecases.UserGroups(ctx, deps, "user_alice")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(view.EffectiveRoles, []string{"catalog:read"}) {
		t.Fatalf("effective roles = %v", view.EffectiveRoles)
	}
	if !slices.Equal(view.GroupRoles, []string{"catalog:read"}) || len(view.DirectRoles) != 0 {
		t.Fatalf("direct=%v group=%v", view.DirectRoles, view.GroupRoles)
	}

	got := eventTypes(*events)
	want := []string{"GroupCreated", "GroupMemberAdded"}
	if !slices.Equal(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestAddMemberRejectsCrossTenantUser(t *testing.T) {
	ctx := context.Background()
	deps, _ := newGroupDeps(t)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	group, err := authusecases.CreateGroup(ctx, deps, authusecases.CreateGroupInput{
		ActorSub: "operator", Name: "engineering", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := authusecases.AddMember(ctx, deps, "operator", group.ID, "user_other", now); !errors.Is(err, authusecases.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound for cross-tenant user, got %v", err)
	}
}

func TestDeleteGroupCascadesMembership(t *testing.T) {
	ctx := context.Background()
	deps, events := newGroupDeps(t)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	group, err := authusecases.CreateGroup(ctx, deps, authusecases.CreateGroupInput{
		ActorSub: "operator", Name: "engineering", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := authusecases.AddMember(ctx, deps, "operator", group.ID, "user_alice", now); err != nil {
		t.Fatal(err)
	}
	*events = (*events)[:0]
	if err := authusecases.DeleteGroup(ctx, deps, "operator", group.ID, now); err != nil {
		t.Fatal(err)
	}
	got := eventTypes(*events)
	want := []string{"GroupMemberRemoved", "GroupDeleted"}
	if !slices.Equal(got, want) {
		t.Fatalf("delete events = %v, want %v", got, want)
	}
	if _, _, err := authusecases.GetGroup(ctx, deps, group.ID); !errors.Is(err, authusecases.ErrGroupNotFound) {
		t.Fatalf("expected ErrGroupNotFound after delete, got %v", err)
	}
}
