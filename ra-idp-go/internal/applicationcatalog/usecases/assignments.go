package usecases

// Application へのユーザー / グループ割当と、利用者ポータル向けの一覧・割当ゲート (wi-69)。
// 割当はポータル可視性とフェデレーション利用可否を fail-closed で制御する。

import (
	"context"
	"slices"
	"strings"
	"time"

	"ra-idp-go/internal/applicationcatalog/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

type AssignmentDeps struct {
	Repo           ports.ApplicationRepository
	AssignmentRepo ports.AssignmentRepository
	Emit           func(spec.DomainEvent)
}

type AssignApplicationInput struct {
	ActorSub      string
	ApplicationID string
	SubjectType   spec.AssignmentSubjectType
	SubjectID     string
	Visibility    spec.AssignmentVisibility
	Now           time.Time
}

func AssignApplication(ctx context.Context, deps AssignmentDeps, in AssignApplicationInput) (*spec.ApplicationAssignment, error) {
	tenantID := tenancy.TenantID(ctx)
	app, err := deps.Repo.FindByID(ctx, tenantID, in.ApplicationID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}
	if !in.SubjectType.Valid() {
		return nil, ErrInvalidSubjectType
	}
	subjectID := strings.TrimSpace(in.SubjectID)
	if subjectID == "" {
		return nil, ErrSubjectRequired
	}
	visibility := in.Visibility
	if visibility == "" {
		visibility = spec.AssignmentVisible
	}
	if !visibility.Valid() {
		return nil, ErrInvalidVisibility
	}
	assignment := &spec.ApplicationAssignment{
		TenantID:      tenantID,
		ApplicationID: in.ApplicationID,
		SubjectType:   in.SubjectType,
		SubjectID:     subjectID,
		Visibility:    visibility,
		CreatedAt:     adminNow(in.Now),
	}
	if err := deps.AssignmentRepo.Save(ctx, assignment); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.ApplicationAssigned{
		At: assignment.CreatedAt, TenantID: tenantID, ActorSub: in.ActorSub, ApplicationID: in.ApplicationID,
		SubjectType: string(in.SubjectType), SubjectID: subjectID,
	})
	return assignment, nil
}

func UnassignApplication(ctx context.Context, deps AssignmentDeps, actorSub, applicationID string, subjectType spec.AssignmentSubjectType, subjectID string, now time.Time) error {
	tenantID := tenancy.TenantID(ctx)
	if err := deps.AssignmentRepo.Delete(ctx, tenantID, applicationID, subjectType, subjectID); err != nil {
		return err
	}
	emit(deps.Emit, &spec.ApplicationUnassigned{
		At: adminNow(now), TenantID: tenantID, ActorSub: actorSub, ApplicationID: applicationID,
		SubjectType: string(subjectType), SubjectID: subjectID,
	})
	return nil
}

func ListAssignments(ctx context.Context, deps AssignmentDeps, applicationID string) ([]*spec.ApplicationAssignment, error) {
	tenantID := tenancy.TenantID(ctx)
	app, err := deps.Repo.FindByID(ctx, tenantID, applicationID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}
	return deps.AssignmentRepo.ListByApplication(ctx, tenantID, applicationID)
}

// ListMyApplications は subjects (利用者本人 + 所属グループ) に割当済みで visible な
// active Application を name 昇順・重複排除して返す。hidden 割当は除外する (wi-69)。
func ListMyApplications(ctx context.Context, deps AssignmentDeps, subjects []ports.SubjectRef) ([]*spec.Application, error) {
	tenantID := tenancy.TenantID(ctx)
	assignments, err := deps.AssignmentRepo.ListBySubjects(ctx, tenantID, subjects)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	out := make([]*spec.Application, 0, len(assignments))
	for _, assignment := range assignments {
		if assignment.Visibility != spec.AssignmentVisible {
			continue
		}
		if _, ok := seen[assignment.ApplicationID]; ok {
			continue
		}
		app, err := deps.Repo.FindByID(ctx, tenantID, assignment.ApplicationID)
		if err != nil {
			return nil, err
		}
		if app == nil || app.Status != spec.ApplicationActive {
			continue
		}
		seen[assignment.ApplicationID] = struct{}{}
		out = append(out, app)
	}
	slices.SortFunc(out, func(a, b *spec.Application) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// IsSubjectAssigned は subjects のいずれかが当該 Application に割当済みかを返す。
// フェデレーション開始経路の fail-closed 割当ゲートに用いる (wi-69, AssignmentGatesProtocol)。
func IsSubjectAssigned(ctx context.Context, repo ports.AssignmentRepository, tenantID, applicationID string, subjects []ports.SubjectRef) (bool, error) {
	assignments, err := repo.ListBySubjects(ctx, tenantID, subjects)
	if err != nil {
		return false, err
	}
	for _, assignment := range assignments {
		if assignment.ApplicationID == applicationID {
			return true, nil
		}
	}
	return false, nil
}
