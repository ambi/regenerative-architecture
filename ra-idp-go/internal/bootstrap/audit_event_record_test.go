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

// wi-36: oauth2 / token / consent / client 系の event も emit 時の tenantId が
// 監査レコードに流れ込む。代表として ClientRegistered と AccessTokenIssued。
func TestNewAuditEventRecordExtractsTenantIDForOAuth2Events(t *testing.T) {
	now := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	for _, ev := range []spec.DomainEvent{
		&spec.ClientRegistered{At: now, TenantID: "acme", ClientID: "demo-client"},
		&spec.AccessTokenIssued{At: now, TenantID: "acme", JTI: "jti", ClientID: "demo-client", Sub: "user_alice"},
		&spec.ConsentGrantedEvent{At: now, TenantID: "acme", Sub: "user_alice", ClientID: "demo-client"},
	} {
		rec, err := newAuditEventRecord(ev)
		if err != nil {
			t.Fatalf("%s: %v", ev.EventType(), err)
		}
		if rec.TenantID != "acme" {
			t.Fatalf("%s: tenant_id = %q, want acme", ev.EventType(), rec.TenantID)
		}
	}
}

// tenantId を持たない event は従来どおり空テナントで記録される (回帰の境界)。
// wi-36: SigningKeyRotated は per-tenant 鍵が無いため意図的に tenant_id 空。
func TestNewAuditEventRecordWithoutTenantIDStaysEmpty(t *testing.T) {
	now := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	for _, ev := range []spec.DomainEvent{
		&spec.EmailSent{At: now, ToHash: "deadbeef", Purpose: "password_reset", Delivered: true},
		&spec.SigningKeyRotated{At: now, NewKID: "kid-2", PreviousKID: "kid-1"},
	} {
		rec, err := newAuditEventRecord(ev)
		if err != nil {
			t.Fatalf("%s: %v", ev.EventType(), err)
		}
		if rec.TenantID != "" {
			t.Fatalf("%s: tenant_id = %q, want empty", ev.EventType(), rec.TenantID)
		}
	}
}
