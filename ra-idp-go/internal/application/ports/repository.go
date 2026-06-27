// Package ports は Application bounded context の永続境界 (port) を定義する (wi-69)。
package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

// ApplicationRepository は Application aggregate の永続境界 (wi-69)。
type ApplicationRepository interface {
	// ListByTenant はテナント内の Application を name 昇順で返す。
	ListByTenant(ctx context.Context, tenantID string) ([]*spec.Application, error)
	// FindByID は application_id に一致する Application を返す。存在しなければ (nil, nil)。
	FindByID(ctx context.Context, tenantID, applicationID string) (*spec.Application, error)
	// FindByBinding は指定 protocol binding (種別 + key: oidc は client_id / wsfed は wtrealm)
	// を持つ Application を返す。割当ゲートの解決に使う。存在しなければ (nil, nil)。
	FindByBinding(ctx context.Context, tenantID string, bindingType spec.ProtocolBindingType, key string) (*spec.Application, error)
	// Save は Application を upsert する。
	Save(ctx context.Context, app *spec.Application) error
	// Delete は application_id に一致する Application を削除する (冪等)。
	Delete(ctx context.Context, tenantID, applicationID string) error
}

// SubjectRef は割当の対象 (user / group) を表す参照 (wi-69)。
type SubjectRef struct {
	Type spec.AssignmentSubjectType
	ID   string
}

// AssignmentRepository は Application 割当の永続境界 (wi-69)。
type AssignmentRepository interface {
	// ListByApplication は Application の割当を subject 昇順で返す。
	ListByApplication(ctx context.Context, tenantID, applicationID string) ([]*spec.ApplicationAssignment, error)
	// ListBySubjects は指定 subject 群に一致する割当を返す (ポータル一覧・割当ゲート用)。
	ListBySubjects(ctx context.Context, tenantID string, subjects []SubjectRef) ([]*spec.ApplicationAssignment, error)
	// Save は割当を upsert する。
	Save(ctx context.Context, assignment *spec.ApplicationAssignment) error
	// Delete は 1 件の割当を削除する (冪等)。
	Delete(ctx context.Context, tenantID, applicationID string, subjectType spec.AssignmentSubjectType, subjectID string) error
	// DeleteByApplication は Application の全割当を削除する (Application 削除時のクリーンアップ)。
	DeleteByApplication(ctx context.Context, tenantID, applicationID string) error
}
