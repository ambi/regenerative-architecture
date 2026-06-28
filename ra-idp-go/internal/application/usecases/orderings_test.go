package usecases_test

import (
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/application/ports"
	appusecases "ra-idp-go/internal/application/usecases"
	"ra-idp-go/internal/shared/adapters/persistence/memory"
	"ra-idp-go/internal/shared/spec"
)

func newOrderingDeps() appusecases.AssignmentDeps {
	apps := memory.NewApplicationRepository()
	assignments := memory.NewApplicationAssignmentRepository()
	orderings := memory.NewApplicationOrderingRepository()
	return appusecases.AssignmentDeps{Repo: apps, AssignmentRepo: assignments, OrderingRepo: orderings}
}

func TestApplyManualOrderOverlaysAndAppendsRemainder(t *testing.T) {
	apps := []*spec.Application{
		{ApplicationID: "a", Name: "Alpha"},
		{ApplicationID: "b", Name: "Beta"},
		{ApplicationID: "g", Name: "Gamma"},
	}
	got := appusecases.ApplyManualOrder(apps, []string{"g", "a"})
	want := []string{"g", "a", "b"}
	if len(got) != len(want) {
		t.Fatalf("len=%d, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ApplicationID != id {
			t.Fatalf("position %d: got %s, want %s", i, got[i].ApplicationID, id)
		}
	}
}

func TestApplyManualOrderIgnoresUnknownAndEmpty(t *testing.T) {
	apps := []*spec.Application{{ApplicationID: "a", Name: "Alpha"}, {ApplicationID: "b", Name: "Beta"}}
	// 空の order は元の並びを保つ。
	if got := appusecases.ApplyManualOrder(apps, nil); len(got) != 2 || got[0].ApplicationID != "a" {
		t.Fatalf("empty order should keep input order")
	}
	// order の未知 id は無視され、現存アプリだけが並ぶ。
	got := appusecases.ApplyManualOrder(apps, []string{"zzz", "b"})
	if len(got) != 2 || got[0].ApplicationID != "b" || got[1].ApplicationID != "a" {
		t.Fatalf("unknown ids must be ignored: %+v", got)
	}
}

func TestSaveAndGetMyApplicationOrder(t *testing.T) {
	ctx := tenantContext()
	deps := newOrderingDeps()
	subjects := []ports.SubjectRef{{Type: spec.AssignmentSubjectUser, ID: "alice"}}

	ids := map[string]string{}
	for _, name := range []string{"Alpha", "Beta", "Gamma"} {
		app, err := appusecases.CreateApplication(ctx,
			appusecases.ApplicationDeps{Repo: deps.Repo, AssignmentRepo: deps.AssignmentRepo},
			appusecases.CreateApplicationInput{ActorSub: "admin", Name: name, Kind: spec.ApplicationFederated})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		ids[name] = app.ApplicationID
		if _, err := appusecases.AssignApplication(ctx, deps, appusecases.AssignApplicationInput{
			ActorSub: "admin", ApplicationID: app.ApplicationID,
			SubjectType: spec.AssignmentSubjectUser, SubjectID: "alice",
		}); err != nil {
			t.Fatalf("assign %s: %v", name, err)
		}
	}

	// 割当済みの部分集合を任意順で保存できる。
	saved, err := appusecases.SaveMyApplicationOrder(ctx, deps, "alice", subjects,
		[]string{ids["Gamma"], ids["Alpha"], ids["Gamma"]}, time.Time{})
	if err != nil {
		t.Fatalf("save order: %v", err)
	}
	if len(saved) != 2 || saved[0] != ids["Gamma"] || saved[1] != ids["Alpha"] {
		t.Fatalf("save should dedup and preserve order: %v", saved)
	}

	order, err := appusecases.GetMyApplicationOrder(ctx, deps.OrderingRepo, "alice")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if len(order) != 2 || order[0] != ids["Gamma"] {
		t.Fatalf("get order mismatch: %v", order)
	}

	// ポータル一覧に適用すると Gamma, Alpha が前、Beta が name 昇順で末尾。
	apps, err := appusecases.ListMyApplications(ctx, deps, subjects)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	ordered := appusecases.ApplyManualOrder(apps, order)
	if ordered[0].Name != "Gamma" || ordered[1].Name != "Alpha" || ordered[2].Name != "Beta" {
		t.Fatalf("unexpected portal order: %s,%s,%s", ordered[0].Name, ordered[1].Name, ordered[2].Name)
	}
}

func TestSaveMyApplicationOrderRejectsUnassigned(t *testing.T) {
	ctx := tenantContext()
	deps := newOrderingDeps()
	subjects := []ports.SubjectRef{{Type: spec.AssignmentSubjectUser, ID: "alice"}}

	_, err := appusecases.SaveMyApplicationOrder(ctx, deps, "alice", subjects, []string{"not-assigned"}, time.Time{})
	if !errors.Is(err, appusecases.ErrUnassignedInOrder) {
		t.Fatalf("expected ErrUnassignedInOrder, got %v", err)
	}
}
