package ports

// AuditEventRepository は管理 API 用の DomainEvent 読み出し境界 (SCL OAuth2
// bounded context: ListAdminAuditEvents / GetAdminAuditEvent)。EventSink が
// fire-and-forget の側に対して、本 port は時系列降順検索とテナント境界フィルタを提供する。
//
// 本実装は監査保管 (SCL objectives.AuditLogRetention 7y) ではなく、
// 直近のオペレーション可視化を目的としたショートウィンドウ用途を想定する。
// 永続化アダプタは Phase 4 以降で Outbox / SIEM クエリ等に差し替えられる。

import (
	"context"
	"time"
)

// AuditEventRecord はストアに格納された 1 イベントの管理者向け表現。
// SCL `AdminAuditEventResponse` の双子定義。
type AuditEventRecord struct {
	ID         string
	TenantID   string
	Type       string
	OccurredAt time.Time
	Payload    map[string]any
}

// AuditEventQuery は ListAdminAuditEvents のフィルタ。SCL `AuditEventQuery` の双子。
// AllTenants は system_admin が default tenant 経路で全テナント横断する時にのみ true。
// 通常呼び出し側 (admin) は TenantID で絞り、AllTenants=false。
type AuditEventQuery struct {
	TenantID   string
	AllTenants bool
	Type       string
	// Types は複数 type のいずれかに一致する行のみを返す (空なら制限なし)。認証イベント
	// 検索が success/fail/aggregated に対応する type 群へ絞り込むのに使う (wi-44)。
	Types  []string
	Sub    string
	After  time.Time
	Before time.Time
	Limit  int
}

type AuditEventRepository interface {
	// Append は新規イベントを保存する。EventSink 側からも並列に呼ばれる想定。
	Append(ctx context.Context, rec *AuditEventRecord) error
	// List は OccurredAt 降順で検索結果を返す。
	List(ctx context.Context, q AuditEventQuery) ([]*AuditEventRecord, error)
	// FindByID は ID で 1 件取得。テナント境界フィルタは呼び出し側の責務。
	FindByID(ctx context.Context, id string) (*AuditEventRecord, error)
}

// RetentionCutoff は ADR-045 の保持期間 sweep が「どの type をいつより前に消すか」を
// 表す。Default はそれ以外 (成功 / 一般監査) の cutoff。ByType は type 別の上書き
// (失敗詳細 30 日など)。Keep に挙げた type は cutoff 対象外 (impersonation など、
// global cap 未設定時は無期限保持) とする。OccurredAt < cutoff の行を削除する。
type RetentionCutoff struct {
	Default time.Time
	ByType  map[string]time.Time
	Keep    []string
}
