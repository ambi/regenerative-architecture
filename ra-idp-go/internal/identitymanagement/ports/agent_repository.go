package ports

import (
	"context"

	"ra-idp-go/internal/shared/spec"
)

// AgentRepository は tenant-scoped な Agent 集約とその OAuth2Client 束縛を永続化する
// (ADR-048)。すべての操作はテナント境界に閉じ、cross-tenant 参照は use case 側で
// reject する。Agent は自身の資格情報を持たず、AgentCredentialBinding で既存
// OAuth2Client に束縛してトークンを得る。
type AgentRepository interface {
	ListByTenant(ctx context.Context, tenantID string) ([]*spec.Agent, error)
	FindByID(ctx context.Context, tenantID, id string) (*spec.Agent, error)
	Save(ctx context.Context, agent *spec.Agent) error
	Delete(ctx context.Context, tenantID, id string) error

	ListBindings(ctx context.Context, tenantID, agentID string) ([]*spec.AgentCredentialBinding, error)
	// AddBinding は束縛を追加し、新規追加なら true を返す。既に束縛済みなら false を
	// 返し no-op とする (冪等)。
	AddBinding(ctx context.Context, binding *spec.AgentCredentialBinding) (bool, error)
	// RemoveBinding は束縛を削除し、削除されたなら true を返す。非束縛なら false を
	// 返し no-op とする (冪等)。
	RemoveBinding(ctx context.Context, tenantID, agentID, clientID string) (bool, error)
	// FindByClientID は指定 client に束縛された Agent を返す。束縛がなければ nil を
	// 返す。トークン発行経路の status gate が呼ぶ (ADR-048)。
	FindByClientID(ctx context.Context, tenantID, clientID string) (*spec.Agent, error)
}
