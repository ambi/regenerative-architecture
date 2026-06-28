// Package memory: 永続化アダプタの in-memory 実装（デモ・テスト用）。
// TS adapters/persistence/memory/*.ts に対応。
//
// リポジトリ実装は境界づけられたコンテキスト単位でファイル分割している
// (tenants.go / clients.go / users.go ...)。共有インフラとして bootstrap が
// 全コンテキストへ配線するが、コンテキスト境界は各 ports が担保する。
package memory

import "ra-idp-go/internal/shared/spec"

func defaultTenant(tenantID *string) {
	if *tenantID == "" {
		*tenantID = spec.DefaultTenantID
	}
}

func tenantKey(tenantID, id string) string {
	if tenantID == "" {
		tenantID = spec.DefaultTenantID
	}
	return tenantID + "|" + id
}
