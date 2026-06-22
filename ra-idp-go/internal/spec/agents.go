package spec

import (
	"time"
)

// ===============================================================
// Agent 集約 (ADR-048)
// ===============================================================

// Agent は tenant-scoped な非人間 (non-human) identity principal。自身の資格情報は
// 持たず、AgentCredentialBinding で既存 OAuth2Client に束縛してトークンを得る。
// owner_sub (所有者 User の sub) は必須。Status は Active / Disabled / Killed の
// 三状態で、Killed は一方向終端 (緊急停止)。Active 以外は新規トークンを発行しない
// (fail-closed)。
type Agent struct {
	ID          string      `json:"id"`
	TenantID    string      `json:"tenant_id"`
	Name        string      `json:"name"`
	Description *string     `json:"description,omitempty"`
	Kind        AgentKind   `json:"kind"`
	OwnerSub    string      `json:"owner_sub"`
	Status      AgentStatus `json:"status"`
	Roles       []string    `json:"roles"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   *time.Time  `json:"updated_at,omitempty"`
	DisabledAt  *time.Time  `json:"disabled_at,omitempty"`
	KilledAt    *time.Time  `json:"killed_at,omitempty"`
}

func (a Agent) Validate() error {
	return validate(agentSchema, &a)
}

// IsActive は Agent が新規トークン発行可能な状態かを返す (ADR-048)。Status が Active
// かつ disabled_at / killed_at がいずれも未設定の場合のみ true。
func (a Agent) IsActive() bool {
	return a.Status == AgentStatusActive && a.DisabledAt == nil && a.KilledAt == nil
}

// AgentCredentialBinding は Agent と OAuth2Client の束縛関係 (ADR-048)。
// agent_id × client_id で一意。
type AgentCredentialBinding struct {
	AgentID   string    `json:"agent_id"`
	ClientID  string    `json:"client_id"`
	TenantID  string    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (b AgentCredentialBinding) Validate() error {
	return validate(agentCredentialBindingSchema, &b)
}

// NewAgentID は不変の Agent 識別子 agent_<uuid> を生成する。
func NewAgentID() (string, error) {
	id, err := NewUUIDv4()
	if err != nil {
		return "", err
	}
	return "agent_" + id, nil
}
