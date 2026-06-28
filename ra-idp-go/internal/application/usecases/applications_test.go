package usecases_test

import (
	"context"
	"testing"

	"ra-idp-go/internal/application/ports"
	appusecases "ra-idp-go/internal/application/usecases"
	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

func tenantContext(id string) context.Context {
	return tenancy.WithTenant(context.Background(), &spec.Tenant{ID: id}, "https://idp.example", "")
}

func newDeps() (appusecases.ApplicationDeps, appusecases.AssignmentDeps) {
	apps := memory.NewApplicationRepository()
	assignments := memory.NewApplicationAssignmentRepository()
	appDeps := appusecases.ApplicationDeps{Repo: apps, AssignmentRepo: assignments}
	assignDeps := appusecases.AssignmentDeps{Repo: apps, AssignmentRepo: assignments}
	return appDeps, assignDeps
}

func TestCreateAndListMyApplicationsRespectsAssignmentAndVisibility(t *testing.T) {
	ctx := tenantContext("acme")
	appDeps, assignDeps := newDeps()

	app, err := appusecases.CreateApplication(ctx, appDeps, appusecases.CreateApplicationInput{
		ActorSub: "admin", Name: "Payroll", Kind: spec.ApplicationFederated,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	userSubjects := []ports.SubjectRef{{Type: spec.AssignmentSubjectUser, ID: "alice"}}

	// 未割当はポータルに出ず、割当ゲートも閉じる。
	mine, err := appusecases.ListMyApplications(ctx, assignDeps, userSubjects)
	if err != nil {
		t.Fatalf("list mine (unassigned): %v", err)
	}
	if len(mine) != 0 {
		t.Fatalf("unassigned user should see no apps, got %d", len(mine))
	}
	assigned, err := appusecases.IsSubjectAssigned(ctx, assignDeps.AssignmentRepo, "acme", app.ApplicationID, userSubjects)
	if err != nil || assigned {
		t.Fatalf("unassigned subject must fail the gate: assigned=%v err=%v", assigned, err)
	}

	// 割当後はポータルに出て、ゲートが開く。
	if _, err := appusecases.AssignApplication(ctx, assignDeps, appusecases.AssignApplicationInput{
		ActorSub: "admin", ApplicationID: app.ApplicationID, SubjectType: spec.AssignmentSubjectUser, SubjectID: "alice",
	}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	mine, err = appusecases.ListMyApplications(ctx, assignDeps, userSubjects)
	if err != nil || len(mine) != 1 {
		t.Fatalf("assigned user should see 1 app, got %d err=%v", len(mine), err)
	}
	assigned, err = appusecases.IsSubjectAssigned(ctx, assignDeps.AssignmentRepo, "acme", app.ApplicationID, userSubjects)
	if err != nil || !assigned {
		t.Fatalf("assigned subject must pass the gate: assigned=%v err=%v", assigned, err)
	}

	// hidden 割当はポータルから消えるが、ゲートは開いたまま。
	if _, err := appusecases.AssignApplication(ctx, assignDeps, appusecases.AssignApplicationInput{
		ActorSub: "admin", ApplicationID: app.ApplicationID, SubjectType: spec.AssignmentSubjectUser, SubjectID: "alice",
		Visibility: spec.AssignmentHidden,
	}); err != nil {
		t.Fatalf("assign hidden: %v", err)
	}
	mine, err = appusecases.ListMyApplications(ctx, assignDeps, userSubjects)
	if err != nil || len(mine) != 0 {
		t.Fatalf("hidden assignment should hide app from portal, got %d err=%v", len(mine), err)
	}
	assigned, err = appusecases.IsSubjectAssigned(ctx, assignDeps.AssignmentRepo, "acme", app.ApplicationID, userSubjects)
	if err != nil || !assigned {
		t.Fatalf("hidden assignment must still pass the gate: assigned=%v err=%v", assigned, err)
	}
}

func TestWeblinkRequiresLaunchURLAndRejectsBindings(t *testing.T) {
	ctx := tenantContext("acme")
	appDeps, _ := newDeps()

	if _, err := appusecases.CreateApplication(ctx, appDeps, appusecases.CreateApplicationInput{
		ActorSub: "admin", Name: "Wiki", Kind: spec.ApplicationWeblink,
	}); err == nil {
		t.Fatal("weblink without launch_url must be rejected")
	}

	app, err := appusecases.CreateApplication(ctx, appDeps, appusecases.CreateApplicationInput{
		ActorSub: "admin", Name: "Wiki", Kind: spec.ApplicationWeblink, LaunchURL: "https://wiki.example",
	})
	if err != nil {
		t.Fatalf("create weblink: %v", err)
	}
	if _, err := appusecases.AttachBinding(ctx, appDeps, appusecases.AttachBindingInput{
		ActorSub: "admin", ApplicationID: app.ApplicationID,
		Binding: spec.ProtocolBinding{Type: spec.ProtocolBindingOIDC, ClientID: "c1"},
	}); err == nil {
		t.Fatal("weblink must not accept protocol bindings")
	}
}

func TestAttachBindingReplacesSameType(t *testing.T) {
	ctx := tenantContext("acme")
	appDeps, _ := newDeps()
	app, err := appusecases.CreateApplication(ctx, appDeps, appusecases.CreateApplicationInput{
		ActorSub: "admin", Name: "CRM", Kind: spec.ApplicationFederated,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, clientID := range []string{"c1", "c2"} {
		if _, err := appusecases.AttachBinding(ctx, appDeps, appusecases.AttachBindingInput{
			ActorSub: "admin", ApplicationID: app.ApplicationID,
			Binding: spec.ProtocolBinding{Type: spec.ProtocolBindingOIDC, ClientID: clientID},
		}); err != nil {
			t.Fatalf("attach %s: %v", clientID, err)
		}
	}
	got, err := appDeps.Repo.FindByID(ctx, "acme", app.ApplicationID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(got.Bindings) != 1 || got.Bindings[0].ClientID != "c2" {
		t.Fatalf("same-type binding should be replaced, got %+v", got.Bindings)
	}
}
