package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ra-idp-go/internal/spec"
)

// AgentRepository は ADR-048 の Agent 集約と OAuth2Client 束縛を PostgreSQL に永続化
// する。すべての参照はテナント境界に閉じる。agent_credential_bindings は agents への
// ON DELETE CASCADE FK を持つため、DeleteAgent の cascade は DB 側でも保証される。
type AgentRepository struct{ Pool *pgxpool.Pool }

const agentSelect = `SELECT id,tenant_id,name,description,kind,owner_sub,status,roles,
created_at,updated_at,disabled_at,killed_at FROM agents`

func scanAgent(row rowScanner) (*spec.Agent, error) {
	var a spec.Agent
	err := row.Scan(&a.ID, &a.TenantID, &a.Name, &a.Description, &a.Kind, &a.OwnerSub, &a.Status,
		&a.Roles, &a.CreatedAt, &a.UpdatedAt, &a.DisabledAt, &a.KilledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if a.Roles == nil {
		a.Roles = []string{}
	}
	return &a, a.Validate()
}

func (r *AgentRepository) ListByTenant(ctx context.Context, tenantID string) ([]*spec.Agent, error) {
	rows, err := r.Pool.Query(ctx, agentSelect+" WHERE tenant_id=$1 ORDER BY name", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*spec.Agent{}
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, agent)
	}
	return out, rows.Err()
}

func (r *AgentRepository) FindByID(ctx context.Context, tenantID, id string) (*spec.Agent, error) {
	return scanAgent(r.Pool.QueryRow(ctx, agentSelect+" WHERE tenant_id=$1 AND id=$2", tenantID, id))
}

func (r *AgentRepository) Save(ctx context.Context, agent *spec.Agent) error {
	roles := agent.Roles
	if roles == nil {
		roles = []string{}
	}
	_, err := r.Pool.Exec(ctx, `
INSERT INTO agents (id,tenant_id,name,description,kind,owner_sub,status,roles,
 created_at,updated_at,disabled_at,killed_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,
 kind=EXCLUDED.kind,owner_sub=EXCLUDED.owner_sub,status=EXCLUDED.status,roles=EXCLUDED.roles,
 updated_at=EXCLUDED.updated_at,disabled_at=EXCLUDED.disabled_at,killed_at=EXCLUDED.killed_at`,
		agent.ID, agent.TenantID, agent.Name, agent.Description, agent.Kind, agent.OwnerSub,
		agent.Status, roles, agent.CreatedAt, agent.UpdatedAt, agent.DisabledAt, agent.KilledAt)
	return err
}

func (r *AgentRepository) Delete(ctx context.Context, tenantID, id string) error {
	_, err := r.Pool.Exec(ctx, "DELETE FROM agents WHERE tenant_id=$1 AND id=$2", tenantID, id)
	return err
}

func (r *AgentRepository) ListBindings(ctx context.Context, tenantID, agentID string) ([]*spec.AgentCredentialBinding, error) {
	rows, err := r.Pool.Query(ctx, `
SELECT b.agent_id,b.client_id,b.tenant_id,b.created_at
FROM agent_credential_bindings b JOIN agents a ON a.id=b.agent_id
WHERE a.tenant_id=$1 AND b.agent_id=$2 ORDER BY b.client_id`, tenantID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*spec.AgentCredentialBinding{}
	for rows.Next() {
		var b spec.AgentCredentialBinding
		if err := rows.Scan(&b.AgentID, &b.ClientID, &b.TenantID, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &b)
	}
	return out, rows.Err()
}

func (r *AgentRepository) AddBinding(ctx context.Context, binding *spec.AgentCredentialBinding) (bool, error) {
	tag, err := r.Pool.Exec(ctx, `
INSERT INTO agent_credential_bindings (agent_id,client_id,tenant_id,created_at)
VALUES ($1,$2,$3,$4)
ON CONFLICT DO NOTHING`,
		binding.AgentID, binding.ClientID, binding.TenantID, binding.CreatedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *AgentRepository) RemoveBinding(ctx context.Context, tenantID, agentID, clientID string) (bool, error) {
	tag, err := r.Pool.Exec(ctx, `
DELETE FROM agent_credential_bindings
WHERE agent_id=$2 AND client_id=$3
  AND agent_id IN (SELECT id FROM agents WHERE tenant_id=$1 AND id=$2)`,
		tenantID, agentID, clientID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *AgentRepository) FindByClientID(ctx context.Context, tenantID, clientID string) (*spec.Agent, error) {
	return scanAgent(r.Pool.QueryRow(ctx, agentSelect+`
WHERE tenant_id=$1 AND id IN (
  SELECT agent_id FROM agent_credential_bindings WHERE tenant_id=$1 AND client_id=$2
) LIMIT 1`, tenantID, clientID))
}
