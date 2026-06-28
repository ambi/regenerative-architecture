package support

// フェデレーション開始経路の割当ゲート (wi-69, invariant AssignmentGatesProtocol)。
//
// protocol binding (OIDC client_id / WS-Fed wtrealm) を所有する Application に対し、
// 解決された subject (本人 + 所属グループ) が割当済みかを fail-closed で判定する。
// catalog に属さない client (binding 未登録) は gating 対象外とし、既存挙動を保つ。

import (
	"context"

	appports "ra-idp-go/internal/application/ports"
	"ra-idp-go/internal/shared/spec"
)

// ApplicationAccessAllowed は binding 経由のフェデレーション開始を許可してよいかを返す。
// Application が見つからない (catalog 外) なら true。見つかった場合は active かつ
// subject が割当済みのときのみ true。判定不能・未割当・disabled は false (fail-closed)。
func (d Deps) ApplicationAccessAllowed(
	ctx context.Context,
	tenantID string,
	bindingType spec.ProtocolBindingType,
	bindingKey, sub string,
) (bool, error) {
	if d.ApplicationRepo == nil {
		return true, nil
	}
	app, err := d.ApplicationRepo.FindByBinding(ctx, tenantID, bindingType, bindingKey)
	if err != nil {
		return false, err
	}
	if app == nil {
		return true, nil
	}
	if app.Status != spec.ApplicationActive {
		return false, nil
	}
	if d.ApplicationAssignmentRepo == nil {
		return false, nil
	}
	subjects := []appports.SubjectRef{{Type: spec.AssignmentSubjectUser, ID: sub}}
	if d.GroupRepo != nil {
		groups, err := d.GroupRepo.ListGroupsByUser(ctx, tenantID, sub)
		if err != nil {
			return false, err
		}
		for _, g := range groups {
			subjects = append(subjects, appports.SubjectRef{Type: spec.AssignmentSubjectGroup, ID: g.ID})
		}
	}
	assignments, err := d.ApplicationAssignmentRepo.ListBySubjects(ctx, tenantID, subjects)
	if err != nil {
		return false, err
	}
	for _, a := range assignments {
		if a.ApplicationID == app.ApplicationID {
			return true, nil
		}
	}
	return false, nil
}
