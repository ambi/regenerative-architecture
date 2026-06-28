package spec

import (
	"slices"
	"time"
)

// ===============================================================
// Group 集約 (ADR-038)
// ===============================================================

// Group は tenant-scoped なロール束集約。所属する User に roles[] を一斉付与する。
// 階層・deny ルール・属性自動所属は持たない (effective_roles は union のみ)。
// ID は不変の生成識別子 (group_<uuid>)、Name はテナント内で一意な編集可能ラベル。
type Group struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Roles       []string   `json:"roles"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

func (g Group) Validate() error {
	return validate(groupSchema, &g)
}

// GroupMember は User と Group の所属関係。group_id × user_sub で一意。
type GroupMember struct {
	GroupID string    `json:"group_id"`
	UserSub string    `json:"user_sub"`
	AddedAt time.Time `json:"added_at"`
}

func (m GroupMember) Validate() error {
	return validate(groupMemberSchema, &m)
}

// NewGroupID は不変の Group 識別子 group_<uuid> を生成する。
func NewGroupID() (string, error) {
	id, err := NewUUIDv4()
	if err != nil {
		return "", err
	}
	return "group_" + id, nil
}

// EffectiveRoles は認可で用いる有効ロール集合を返す (ADR-038)。
// effective_roles(user) = user.roles ∪ ⋃_{g ∈ groups} g.roles。
// 結果はソート済みで重複を含まない。所属グループが空なら user.roles に一致する。
func EffectiveRoles(userRoles []string, groups []*Group) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(userRoles))
	add := func(roles []string) {
		for _, role := range roles {
			if role == "" {
				continue
			}
			if _, ok := seen[role]; ok {
				continue
			}
			seen[role] = struct{}{}
			out = append(out, role)
		}
	}
	add(userRoles)
	for _, group := range groups {
		if group != nil {
			add(group.Roles)
		}
	}
	slices.Sort(out)
	return out
}
