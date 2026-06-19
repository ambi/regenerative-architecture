package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

// GroupRepository は tenant-scoped な Group 集約とそのメンバーシップを永続化する
// (ADR-038)。すべての操作はテナント境界に閉じ、cross-tenant 参照は use case 側で
// reject する。
type GroupRepository interface {
	ListByTenant(ctx context.Context, tenantID string) ([]*spec.Group, error)
	FindByID(ctx context.Context, tenantID, id string) (*spec.Group, error)
	Save(ctx context.Context, group *spec.Group) error
	Delete(ctx context.Context, tenantID, id string) error

	ListMembersByGroup(ctx context.Context, tenantID, groupID string) ([]*spec.GroupMember, error)
	// ListGroupsByUser は指定 User が所属するグループを返す。認可経路 (effective
	// roles の解決) と admin UI の両方から呼ばれる。
	ListGroupsByUser(ctx context.Context, tenantID, userSub string) ([]*spec.Group, error)
	CountMembers(ctx context.Context, tenantID, groupID string) (int, error)
	// AddMember は membership を追加し、新規追加なら true を返す。既に所属済みなら
	// false を返し no-op とする (冪等)。
	AddMember(ctx context.Context, member *spec.GroupMember) (bool, error)
	// RemoveMember は membership を削除し、削除されたなら true を返す。非所属なら
	// false を返し no-op とする (冪等)。
	RemoveMember(ctx context.Context, tenantID, groupID, userSub string) (bool, error)
}
