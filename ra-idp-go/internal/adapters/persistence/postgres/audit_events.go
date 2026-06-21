package postgres

// AuditEventRepository は AuditEventRepository (SCL Trust component) を PostgreSQL に
// 永続化する読み出しモデル。in-memory 実装 (memory.AuditEventStore) と同じ port 契約を
// 満たし、admin の時系列調査 / 本人サインイン履歴 / wi-44 の認証イベント検索が共有する。
// 付加属性 (ip_truncated / ip_hash / session_id 等) は payload JSONB に載るため、本テーブルは
// 構造化カラムを増やさず type / sub / occurred_at の絞り込みだけを担う (ADR-041)。

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ra-idp-go/internal/oauth2/ports"
)

const (
	auditDefaultListLimit = 100
	auditMaxListLimit     = 1000
)

type AuditEventRepository struct{ Pool *pgxpool.Pool }

const auditEventSelect = `SELECT id,tenant_id,type,occurred_at,payload FROM audit_events`

func scanAuditEvent(row rowScanner) (*ports.AuditEventRecord, error) {
	var rec ports.AuditEventRecord
	err := row.Scan(&rec.ID, &rec.TenantID, &rec.Type, &rec.OccurredAt, &rec.Payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if rec.Payload == nil {
		rec.Payload = map[string]any{}
	}
	return &rec, nil
}

func (r *AuditEventRepository) Append(ctx context.Context, rec *ports.AuditEventRecord) error {
	if rec == nil || rec.ID == "" || rec.Type == "" {
		return nil
	}
	var sub *string
	if s, ok := rec.Payload["sub"].(string); ok && s != "" {
		sub = &s
	}
	payload := rec.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	_, err := r.Pool.Exec(ctx, `
INSERT INTO audit_events (id,tenant_id,type,sub,occurred_at,payload)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (id) DO NOTHING`,
		rec.ID, rec.TenantID, rec.Type, sub, rec.OccurredAt, payload)
	return err
}

func (r *AuditEventRepository) List(ctx context.Context, q ports.AuditEventQuery) ([]*ports.AuditEventRecord, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = auditDefaultListLimit
	}
	if limit > auditMaxListLimit {
		limit = auditMaxListLimit
	}
	var conds []string
	var args []any
	add := func(expr string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(expr, len(args)))
	}
	if !q.AllTenants && q.TenantID != "" {
		add("tenant_id = $%d", q.TenantID)
	}
	if q.Type != "" {
		add("type = $%d", q.Type)
	}
	if q.Sub != "" {
		add("sub = $%d", q.Sub)
	}
	if !q.After.IsZero() {
		add("occurred_at >= $%d", q.After)
	}
	if !q.Before.IsZero() {
		add("occurred_at <= $%d", q.Before)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit)
	query := auditEventSelect + where + fmt.Sprintf(" ORDER BY occurred_at DESC LIMIT $%d", len(args))
	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*ports.AuditEventRecord{}
	for rows.Next() {
		rec, err := scanAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *AuditEventRepository) FindByID(ctx context.Context, id string) (*ports.AuditEventRecord, error) {
	return scanAuditEvent(r.Pool.QueryRow(ctx, auditEventSelect+" WHERE id=$1", id))
}
