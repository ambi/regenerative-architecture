package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ra-idp-go/internal/spec"
)

// GroupRepository は ADR-038 の Group 集約とメンバーシップを PostgreSQL に永続化する。
// すべての参照はテナント境界に閉じる。group_members は groups への ON DELETE CASCADE
// FK を持つため、DeleteGroup の cascade は DB 側でも保証される。
type GroupRepository struct{ Pool *pgxpool.Pool }

const groupSelect = `SELECT id,tenant_id,name,description,roles,created_at,updated_at FROM groups`

func scanGroup(row rowScanner) (*spec.Group, error) {
	var g spec.Group
	err := row.Scan(&g.ID, &g.TenantID, &g.Name, &g.Description, &g.Roles, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if g.Roles == nil {
		g.Roles = []string{}
	}
	return &g, g.Validate()
}

func (r *GroupRepository) ListByTenant(ctx context.Context, tenantID string) ([]*spec.Group, error) {
	rows, err := r.Pool.Query(ctx, groupSelect+" WHERE tenant_id=$1 ORDER BY name", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*spec.Group{}
	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, group)
	}
	return out, rows.Err()
}

func (r *GroupRepository) FindByID(ctx context.Context, tenantID, id string) (*spec.Group, error) {
	return scanGroup(r.Pool.QueryRow(ctx, groupSelect+" WHERE tenant_id=$1 AND id=$2", tenantID, id))
}

func (r *GroupRepository) Save(ctx context.Context, group *spec.Group) error {
	roles := group.Roles
	if roles == nil {
		roles = []string{}
	}
	_, err := r.Pool.Exec(ctx, `
INSERT INTO groups (id,tenant_id,name,description,roles,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,
 roles=EXCLUDED.roles,updated_at=EXCLUDED.updated_at`,
		group.ID, group.TenantID, group.Name, group.Description, roles, group.CreatedAt, group.UpdatedAt)
	return err
}

func (r *GroupRepository) Delete(ctx context.Context, tenantID, id string) error {
	_, err := r.Pool.Exec(ctx, "DELETE FROM groups WHERE tenant_id=$1 AND id=$2", tenantID, id)
	return err
}

func (r *GroupRepository) ListMembersByGroup(ctx context.Context, tenantID, groupID string) ([]*spec.GroupMember, error) {
	rows, err := r.Pool.Query(ctx, `
SELECT gm.group_id,gm.user_sub,gm.added_at
FROM group_members gm JOIN groups g ON g.id=gm.group_id
WHERE g.tenant_id=$1 AND gm.group_id=$2 ORDER BY gm.user_sub`, tenantID, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*spec.GroupMember{}
	for rows.Next() {
		var m spec.GroupMember
		if err := rows.Scan(&m.GroupID, &m.UserSub, &m.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (r *GroupRepository) ListGroupsByUser(ctx context.Context, tenantID, userSub string) ([]*spec.Group, error) {
	rows, err := r.Pool.Query(ctx, `
SELECT g.id,g.tenant_id,g.name,g.description,g.roles,g.created_at,g.updated_at
FROM groups g JOIN group_members gm ON gm.group_id=g.id
WHERE g.tenant_id=$1 AND gm.user_sub=$2 ORDER BY g.name`, tenantID, userSub)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*spec.Group{}
	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, group)
	}
	return out, rows.Err()
}

func (r *GroupRepository) CountMembers(ctx context.Context, tenantID, groupID string) (int, error) {
	var count int
	err := r.Pool.QueryRow(ctx, `
SELECT count(*) FROM group_members gm JOIN groups g ON g.id=gm.group_id
WHERE g.tenant_id=$1 AND gm.group_id=$2`, tenantID, groupID).Scan(&count)
	return count, err
}

func (r *GroupRepository) AddMember(ctx context.Context, member *spec.GroupMember) (bool, error) {
	tag, err := r.Pool.Exec(ctx, `
INSERT INTO group_members (group_id,user_sub,added_at) VALUES ($1,$2,$3)
ON CONFLICT (group_id,user_sub) DO NOTHING`,
		member.GroupID, member.UserSub, member.AddedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *GroupRepository) RemoveMember(ctx context.Context, tenantID, groupID, userSub string) (bool, error) {
	tag, err := r.Pool.Exec(ctx, `
DELETE FROM group_members
WHERE group_id=$2 AND user_sub=$3
  AND group_id IN (SELECT id FROM groups WHERE tenant_id=$1 AND id=$2)`,
		tenantID, groupID, userSub)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
