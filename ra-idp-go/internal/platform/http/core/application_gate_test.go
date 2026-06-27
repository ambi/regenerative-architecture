package core_test

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"
)

func TestApplicationAccessAllowedGatesUnassignedSubjects(t *testing.T) {
	ctx := context.Background()
	apps := memory.NewApplicationRepository()
	assignments := memory.NewApplicationAssignmentRepository()
	now := time.Now().UTC()
	app := &spec.Application{
		TenantID: "default", ApplicationID: "app-1", Name: "Payroll",
		Kind: spec.ApplicationFederated, Status: spec.ApplicationActive,
		Bindings:  []spec.ProtocolBinding{{Type: spec.ProtocolBindingOIDC, ClientID: "c1"}},
		CreatedAt: now, UpdatedAt: now,
	}
	if err := apps.Save(ctx, app); err != nil {
		t.Fatal(err)
	}
	d := core.Deps{ApplicationRepo: apps, ApplicationAssignmentRepo: assignments, GroupRepo: memory.NewGroupRepository()}

	// catalog 外の client は gating 対象外。
	if allowed, err := d.ApplicationAccessAllowed(ctx, "default", spec.ProtocolBindingOIDC, "other", "alice"); err != nil || !allowed {
		t.Fatalf("client outside catalog must be allowed: allowed=%v err=%v", allowed, err)
	}

	// catalog 内・未割当は fail-closed で拒否。
	if allowed, err := d.ApplicationAccessAllowed(ctx, "default", spec.ProtocolBindingOIDC, "c1", "alice"); err != nil || allowed {
		t.Fatalf("unassigned subject must be denied: allowed=%v err=%v", allowed, err)
	}

	// 割当後は許可。
	if err := assignments.Save(ctx, &spec.ApplicationAssignment{
		TenantID: "default", ApplicationID: "app-1", SubjectType: spec.AssignmentSubjectUser,
		SubjectID: "alice", Visibility: spec.AssignmentVisible, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if allowed, err := d.ApplicationAccessAllowed(ctx, "default", spec.ProtocolBindingOIDC, "c1", "alice"); err != nil || !allowed {
		t.Fatalf("assigned subject must be allowed: allowed=%v err=%v", allowed, err)
	}

	// disabled application は割当済みでも拒否。
	app.Status = spec.ApplicationDisabled
	if err := apps.Save(ctx, app); err != nil {
		t.Fatal(err)
	}
	if allowed, err := d.ApplicationAccessAllowed(ctx, "default", spec.ProtocolBindingOIDC, "c1", "alice"); err != nil || allowed {
		t.Fatalf("disabled application must be denied: allowed=%v err=%v", allowed, err)
	}
}
