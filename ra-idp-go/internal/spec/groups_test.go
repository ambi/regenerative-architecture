package spec

import (
	"slices"
	"testing"
	"time"
)

func TestEffectiveRolesUnionSortedDedup(t *testing.T) {
	groups := []*Group{
		{Roles: []string{"catalog:read", "invoice:read"}},
		{Roles: []string{"catalog:read", "support:read"}},
		nil,
	}
	got := EffectiveRoles([]string{"admin", "support:read"}, groups)
	want := []string{"admin", "catalog:read", "invoice:read", "support:read"}
	if !slices.Equal(got, want) {
		t.Fatalf("EffectiveRoles = %v, want %v", got, want)
	}
}

func TestEffectiveRolesEmptyGroupsEqualsUserRoles(t *testing.T) {
	got := EffectiveRoles([]string{"admin", "auditor"}, nil)
	want := []string{"admin", "auditor"}
	if !slices.Equal(got, want) {
		t.Fatalf("EffectiveRoles = %v, want %v", got, want)
	}
}

func TestGroupValidate(t *testing.T) {
	now := time.Now().UTC()
	valid := Group{ID: "group_x", TenantID: "default", Name: "engineering", Roles: []string{"catalog:read"}, CreatedAt: now}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid group rejected: %v", err)
	}
	missingName := Group{ID: "group_x", TenantID: "default", CreatedAt: now}
	if err := missingName.Validate(); err == nil {
		t.Fatal("group without name was accepted")
	}
}

func TestNewGroupIDPrefix(t *testing.T) {
	id, err := NewGroupID()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) <= len("group_") || id[:len("group_")] != "group_" {
		t.Fatalf("unexpected group id %q", id)
	}
}
