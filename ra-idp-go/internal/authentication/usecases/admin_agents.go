package usecases

// 管理者向け Agent ライフサイクル操作と OAuth2Client 資格情報束縛 (ADR-048)。
// SCL Authentication component が所有する admin インターフェース群:
// ListAgents / GetAgent / RegisterAgent / UpdateAgent / DisableAgent /
// EnableAgent / KillAgent / DeleteAgent / BindAgentCredential /
// UnbindAgentCredential。
//
// すべての操作は tenancy.TenantID(ctx) のテナント境界に閉じ、cross-tenant な参照・
// 束縛は reject する。Agent は自身の資格情報を持たず、AgentCredentialBinding で既存
// OAuth2Client に束縛してトークンを得る。Status は Active / Disabled / Killed の三状態で、
// Killed は一方向終端 (緊急停止) であり復帰できない。

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

var (
	ErrAgentNotFound       = errors.New("agent not found")
	ErrAgentNameConflict   = errors.New("agent name already exists")
	ErrAgentNameEmpty      = errors.New("agent name is required")
	ErrAgentOwnerRequired  = errors.New("agent owner is required")
	ErrAgentKilled         = errors.New("agent is killed and cannot be modified")
	ErrAgentClientNotFound = errors.New("client not found")
)

type AdminAgentDeps struct {
	AgentRepo  authports.AgentRepository
	ClientRepo oauthports.ClientRepository
	Emit       func(spec.DomainEvent)
}

// AgentView は一覧・詳細で Agent と束縛済み client id をまとめて返す。
type AgentView struct {
	Agent     *spec.Agent
	ClientIDs []string
}

func ListAgents(ctx context.Context, deps AdminAgentDeps) ([]AgentView, error) {
	tenantID := tenancy.TenantID(ctx)
	agents, err := deps.AgentRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]AgentView, 0, len(agents))
	for _, agent := range agents {
		clientIDs, err := agentClientIDs(ctx, deps, tenantID, agent.ID)
		if err != nil {
			return nil, err
		}
		views = append(views, AgentView{Agent: agent, ClientIDs: clientIDs})
	}
	return views, nil
}

// GetAgent は Agent 本体と束縛済み client id を返す。別テナントの Agent は未存在
// として扱う。
func GetAgent(ctx context.Context, deps AdminAgentDeps, id string) (*AgentView, error) {
	tenantID := tenancy.TenantID(ctx)
	agent, err := deps.AgentRepo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, ErrAgentNotFound
	}
	clientIDs, err := agentClientIDs(ctx, deps, tenantID, agent.ID)
	if err != nil {
		return nil, err
	}
	return &AgentView{Agent: agent, ClientIDs: clientIDs}, nil
}

type RegisterAgentInput struct {
	ActorSub    string
	Name        string
	Description *string
	Kind        *spec.AgentKind
	OwnerSub    string
	Roles       []string
	Now         time.Time
}

func RegisterAgent(ctx context.Context, deps AdminAgentDeps, in RegisterAgentInput) (*spec.Agent, error) {
	tenantID := tenancy.TenantID(ctx)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrAgentNameEmpty
	}
	if err := ensureAgentNameAvailable(ctx, deps, tenantID, name, ""); err != nil {
		return nil, err
	}
	owner := strings.TrimSpace(in.OwnerSub)
	if owner == "" {
		owner = strings.TrimSpace(in.ActorSub)
	}
	if owner == "" {
		return nil, ErrAgentOwnerRequired
	}
	roles, err := normalizeRoles(in.Roles)
	if err != nil {
		return nil, err
	}
	id, err := spec.NewAgentID()
	if err != nil {
		return nil, err
	}
	now := normalizedNow(in.Now)
	agent := &spec.Agent{
		ID: id, TenantID: tenantID, Name: name, Description: normalizeDescription(in.Description),
		Kind: normalizeAgentKind(in.Kind), OwnerSub: owner, Status: spec.AgentStatusActive,
		Roles: roles, CreatedAt: now,
	}
	if err := agent.Validate(); err != nil {
		return nil, err
	}
	if err := deps.AgentRepo.Save(ctx, agent); err != nil {
		return nil, err
	}
	adminEmit(deps.Emit, &spec.AgentRegistered{At: now, TenantID: agent.TenantID, ActorSub: in.ActorSub, AgentID: agent.ID})
	return agent, nil
}

type UpdateAgentInput struct {
	ActorSub    string
	ID          string
	Name        *string
	Description *string
	Kind        *spec.AgentKind
	OwnerSub    *string
	Roles       *[]string
	Now         time.Time
}

func UpdateAgent(ctx context.Context, deps AdminAgentDeps, in UpdateAgentInput) (*spec.Agent, error) {
	tenantID := tenancy.TenantID(ctx)
	agent, err := deps.AgentRepo.FindByID(ctx, tenantID, in.ID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, ErrAgentNotFound
	}
	if agent.Status == spec.AgentStatusKilled {
		return nil, ErrAgentKilled
	}
	updated := *agent
	changed := []string{}
	previousOwner := agent.OwnerSub
	ownerChanged := false
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, ErrAgentNameEmpty
		}
		if name != agent.Name {
			if err := ensureAgentNameAvailable(ctx, deps, tenantID, name, agent.ID); err != nil {
				return nil, err
			}
			updated.Name = name
			changed = append(changed, "name")
		}
	}
	if in.Description != nil {
		desc := normalizeDescription(in.Description)
		if !equalOptionalString(agent.Description, desc) {
			updated.Description = desc
			changed = append(changed, "description")
		}
	}
	if in.Kind != nil {
		kind := normalizeAgentKind(in.Kind)
		if kind != agent.Kind {
			updated.Kind = kind
			changed = append(changed, "kind")
		}
	}
	if in.OwnerSub != nil {
		owner := strings.TrimSpace(*in.OwnerSub)
		if owner == "" {
			return nil, ErrAgentOwnerRequired
		}
		if owner != agent.OwnerSub {
			updated.OwnerSub = owner
			changed = append(changed, "owner_sub")
			ownerChanged = true
		}
	}
	if in.Roles != nil {
		roles, err := normalizeRoles(*in.Roles)
		if err != nil {
			return nil, err
		}
		if !slices.Equal(roles, agent.Roles) {
			updated.Roles = roles
			changed = append(changed, "roles")
		}
	}
	if len(changed) == 0 {
		return &updated, nil
	}
	now := normalizedNow(in.Now)
	updated.UpdatedAt = &now
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	if err := deps.AgentRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	adminEmit(deps.Emit, &spec.AgentUpdated{
		At: now, TenantID: agent.TenantID, ActorSub: in.ActorSub, AgentID: agent.ID, ChangedFields: changed,
	})
	if ownerChanged {
		adminEmit(deps.Emit, &spec.AgentOwnerChanged{
			At: now, TenantID: agent.TenantID, ActorSub: in.ActorSub, AgentID: agent.ID,
			PreviousOwnerSub: previousOwner, NewOwnerSub: updated.OwnerSub,
		})
	}
	return &updated, nil
}

// SetAgentDisabled は Agent を運用停止 (disabled=true) / 再稼働 (disabled=false) する。
// Killed の Agent は復帰できないため reject する (一方向終端)。
func SetAgentDisabled(ctx context.Context, deps AdminAgentDeps, actorSub, id string, disabled bool, now time.Time) (*spec.Agent, error) {
	tenantID := tenancy.TenantID(ctx)
	agent, err := deps.AgentRepo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, ErrAgentNotFound
	}
	if agent.Status == spec.AgentStatusKilled {
		return nil, ErrAgentKilled
	}
	now = normalizedNow(now)
	updated := *agent
	updated.UpdatedAt = &now
	if disabled {
		updated.Status = spec.AgentStatusDisabled
		updated.DisabledAt = &now
	} else {
		updated.Status = spec.AgentStatusActive
		updated.DisabledAt = nil
	}
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	if err := deps.AgentRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	if disabled {
		adminEmit(deps.Emit, &spec.AgentDisabled{At: now, TenantID: tenantID, ActorSub: actorSub, AgentID: agent.ID})
	} else {
		adminEmit(deps.Emit, &spec.AgentEnabled{At: now, TenantID: tenantID, ActorSub: actorSub, AgentID: agent.ID})
	}
	return &updated, nil
}

// KillAgent は Agent を緊急停止し Killed (一方向終端) に遷移させる。既に Killed なら
// reject する (冪等ではなく irreversible なため明示エラー)。
func KillAgent(ctx context.Context, deps AdminAgentDeps, actorSub, id string, now time.Time) (*spec.Agent, error) {
	tenantID := tenancy.TenantID(ctx)
	agent, err := deps.AgentRepo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, ErrAgentNotFound
	}
	if agent.Status == spec.AgentStatusKilled {
		return nil, ErrAgentKilled
	}
	now = normalizedNow(now)
	updated := *agent
	updated.Status = spec.AgentStatusKilled
	updated.KilledAt = &now
	updated.UpdatedAt = &now
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	if err := deps.AgentRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	adminEmit(deps.Emit, &spec.AgentKilled{At: now, TenantID: tenantID, ActorSub: actorSub, AgentID: agent.ID})
	return &updated, nil
}

// DeleteAgent は Agent を物理削除し、束縛は cascade で解除する。
func DeleteAgent(ctx context.Context, deps AdminAgentDeps, actorSub, id string, now time.Time) error {
	tenantID := tenancy.TenantID(ctx)
	agent, err := deps.AgentRepo.FindByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if agent == nil {
		return ErrAgentNotFound
	}
	now = normalizedNow(now)
	if err := deps.AgentRepo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	adminEmit(deps.Emit, &spec.AgentDeleted{At: now, TenantID: tenantID, ActorSub: actorSub, AgentID: id})
	return nil
}

// BindCredential は Agent に同一テナントの OAuth2Client を束縛する。既束縛なら no-op
// で event も emit しない (冪等)。
func BindCredential(ctx context.Context, deps AdminAgentDeps, actorSub, agentID, clientID string, now time.Time) error {
	tenantID := tenancy.TenantID(ctx)
	agent, err := deps.AgentRepo.FindByID(ctx, tenantID, agentID)
	if err != nil {
		return err
	}
	if agent == nil {
		return ErrAgentNotFound
	}
	if agent.Status == spec.AgentStatusKilled {
		return ErrAgentKilled
	}
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return ErrAgentClientNotFound
	}
	if deps.ClientRepo != nil {
		client, err := deps.ClientRepo.FindByID(ctx, tenantID, clientID)
		if err != nil {
			return err
		}
		if client == nil {
			return ErrAgentClientNotFound
		}
	}
	now = normalizedNow(now)
	added, err := deps.AgentRepo.AddBinding(ctx, &spec.AgentCredentialBinding{
		AgentID: agentID, ClientID: clientID, TenantID: tenantID, CreatedAt: now,
	})
	if err != nil {
		return err
	}
	if added {
		adminEmit(deps.Emit, &spec.AgentCredentialBound{
			At: now, TenantID: tenantID, ActorSub: actorSub, AgentID: agentID, ClientID: clientID,
		})
	}
	return nil
}

// UnbindCredential は Agent から OAuth2Client の束縛を解除する。非束縛なら no-op で
// event も emit しない (冪等)。
func UnbindCredential(ctx context.Context, deps AdminAgentDeps, actorSub, agentID, clientID string, now time.Time) error {
	tenantID := tenancy.TenantID(ctx)
	agent, err := deps.AgentRepo.FindByID(ctx, tenantID, agentID)
	if err != nil {
		return err
	}
	if agent == nil {
		return ErrAgentNotFound
	}
	now = normalizedNow(now)
	removed, err := deps.AgentRepo.RemoveBinding(ctx, tenantID, agentID, clientID)
	if err != nil {
		return err
	}
	if removed {
		adminEmit(deps.Emit, &spec.AgentCredentialUnbound{
			At: now, TenantID: tenantID, ActorSub: actorSub, AgentID: agentID, ClientID: clientID,
		})
	}
	return nil
}

func agentClientIDs(ctx context.Context, deps AdminAgentDeps, tenantID, agentID string) ([]string, error) {
	bindings, err := deps.AgentRepo.ListBindings(ctx, tenantID, agentID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, binding.ClientID)
	}
	return out, nil
}

func ensureAgentNameAvailable(ctx context.Context, deps AdminAgentDeps, tenantID, name, excludeID string) error {
	agents, err := deps.AgentRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, agent := range agents {
		if agent.ID != excludeID && strings.EqualFold(agent.Name, name) {
			return ErrAgentNameConflict
		}
	}
	return nil
}

func normalizeAgentKind(kind *spec.AgentKind) spec.AgentKind {
	if kind == nil || !kind.Valid() {
		return spec.AgentKindSupervised
	}
	return *kind
}
