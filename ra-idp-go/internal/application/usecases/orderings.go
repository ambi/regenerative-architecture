package usecases

// 利用者ポータルの手動並び順 (wi-70, ADR-069)。手動順は ApplicationCatalog が所有し、
// 既定は name 昇順。手動順を割当済み visible アプリの上に重ねる。

import (
	"context"
	"errors"
	"slices"
	"time"

	"ra-idp-go/internal/application/ports"
	"ra-idp-go/internal/shared/spec"
	"ra-idp-go/internal/tenancy"
)

// ErrUnassignedInOrder は手動順に割当済み visible でない application_id が含まれていた。
var ErrUnassignedInOrder = errors.New("ordering contains an unassigned application")

// GetMyApplicationOrder は利用者の保存済み手動並び順を返す。未保存なら空スライス。
func GetMyApplicationOrder(ctx context.Context, repo ports.ApplicationOrderingRepository, userSub string) ([]string, error) {
	if repo == nil {
		return []string{}, nil
	}
	ordering, err := repo.Get(ctx, tenancy.TenantID(ctx), userSub)
	if err != nil {
		return nil, err
	}
	if ordering == nil {
		return []string{}, nil
	}
	return ordering.ApplicationIDs, nil
}

// ApplyManualOrder は name 昇順の apps に手動順 order を重ねる。order に在るアプリを
// その順で前に置き、order に無い割当アプリは元の (name 昇順) で末尾に付ける。
// order に在って apps に無い (現在は未割当) id は無視する。
func ApplyManualOrder(apps []*spec.Application, order []string) []*spec.Application {
	if len(order) == 0 {
		return apps
	}
	byID := make(map[string]*spec.Application, len(apps))
	for _, app := range apps {
		byID[app.ApplicationID] = app
	}
	out := make([]*spec.Application, 0, len(apps))
	placed := make(map[string]struct{}, len(apps))
	for _, id := range order {
		app, ok := byID[id]
		if !ok {
			continue
		}
		if _, done := placed[id]; done {
			continue
		}
		placed[id] = struct{}{}
		out = append(out, app)
	}
	for _, app := range apps {
		if _, done := placed[app.ApplicationID]; done {
			continue
		}
		out = append(out, app)
	}
	return out
}

// SaveMyApplicationOrder は利用者の手動並び順を検証して保存する。application_ids は
// 利用者に割当済みの visible active アプリのみを含めること。重複は除去し、それ以外の
// id を含む場合は ErrUnassignedInOrder を返す。
func SaveMyApplicationOrder(ctx context.Context, deps AssignmentDeps, userSub string, subjects []ports.SubjectRef, applicationIDs []string, now time.Time) ([]string, error) {
	assigned, err := ListMyApplications(ctx, deps, subjects)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(assigned))
	for _, app := range assigned {
		allowed[app.ApplicationID] = struct{}{}
	}
	cleaned := make([]string, 0, len(applicationIDs))
	seen := make(map[string]struct{}, len(applicationIDs))
	for _, id := range applicationIDs {
		if _, ok := allowed[id]; !ok {
			return nil, ErrUnassignedInOrder
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		cleaned = append(cleaned, id)
	}
	ordering := &spec.ApplicationOrdering{
		TenantID:       tenancy.TenantID(ctx),
		UserSub:        userSub,
		ApplicationIDs: cleaned,
		UpdatedAt:      adminNow(now),
	}
	if err := deps.OrderingRepo.Save(ctx, ordering); err != nil {
		return nil, err
	}
	return slices.Clone(cleaned), nil
}
