package bootstrap

import (
	"testing"
	"time"

	"ra-idp-go/internal/spec"
)

// wi-35: emit 時点で event に載せた tenantId が、監査レコードの TenantID に
// 流れ込むこと。これがテナント所属 admin の監査ビュー絞り込み
// (auditEventMatches) に効く。
func TestNewAuditEventRecordExtractsTenantID(t *testing.T) {
	now := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	rec, err := newAuditEventRecord(&spec.UserAuthenticated{
		At: now, TenantID: "acme", Sub: "user_alice", AMR: []string{"pwd"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Type != "UserAuthenticated" {
		t.Fatalf("type = %q, want UserAuthenticated", rec.Type)
	}
	if rec.TenantID != "acme" {
		t.Fatalf("tenant_id = %q, want acme (emit-time tenantId must reach the audit record)", rec.TenantID)
	}
	if got, _ := rec.Payload["tenantId"].(string); got != "acme" {
		t.Fatalf("payload tenantId = %q, want acme", got)
	}
}

// tenantId を持たない event は従来どおり空テナントで記録される (回帰の境界)。
func TestNewAuditEventRecordWithoutTenantIDStaysEmpty(t *testing.T) {
	now := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	rec, err := newAuditEventRecord(&spec.EmailSent{
		At: now, ToHash: "deadbeef", Purpose: "password_reset", Delivered: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.TenantID != "" {
		t.Fatalf("tenant_id = %q, want empty for an event without tenantId", rec.TenantID)
	}
}
