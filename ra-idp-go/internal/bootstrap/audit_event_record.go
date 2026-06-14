package bootstrap

// newAuditEventRecord は DomainEvent を AuditEventRepository に保存可能な
// 形 (SCL `AdminAuditEventResponse` の双子) に変換する。tenant_id は
// payload に tenantId が存在する場合のみ抽出し、無い場合は空文字を残す
// (admin の所属テナント絞り込みでは引っかからない)。
//
// SCL events セクションで TenantID を持つのは Tenant ライフサイクル系のみ。
// それ以外の event は将来 TenantID を載せれば、本変換だけで audit 経路から
// 見えるようになる。

import (
	"encoding/json"
	"fmt"

	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

func newAuditEventRecord(e spec.DomainEvent) (*oauthports.AuditEventRecord, error) {
	wire, err := spec.MarshalDomainEvent(e)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(wire, &payload); err != nil {
		return nil, err
	}
	id, err := spec.NewUUIDv4()
	if err != nil {
		return nil, err
	}
	rec := &oauthports.AuditEventRecord{
		ID:         id,
		Type:       e.EventType(),
		OccurredAt: e.OccurredAt(),
		Payload:    payload,
	}
	if tenantID, ok := payload["tenantId"].(string); ok {
		rec.TenantID = tenantID
	}
	if rec.Payload == nil {
		return nil, fmt.Errorf("audit event %s: empty payload", e.EventType())
	}
	return rec, nil
}
