# ADR-072: ユーザー削除を soft-delete + 復元 + 完全削除の 2 段階にする

## ステータス

採用。ADR-036 の即時 anonymize cascade を「完全削除 (purge)」として維持しつつ、
その前段に復元可能な soft-delete 状態 `PendingDeletion` を追加する。既定の
DELETE は soft-delete になる。

## コンテキスト

ADR-036 の `DELETE /api/admin/users/{sub}` は 1 クリックで `deleted_at` 設定 +
即時 PII 匿名化 + 関連 aggregate の cascade 削除をまとめて実施する片方向操作だった。
GDPR Art.17 の即時履行としては強い保証だが、運用上 2 つの問題がある。

1. 管理者の誤操作を救えない。削除した瞬間に Consent / RefreshToken / Session /
   MFA / PasswordHistory がすべて消え、「対象を間違えた」を取り消せない。
   Google / Microsoft / Apple は 7〜30 日の復元可能期間を持つ。
2. 「とりあえず触らせたくない」と「完全に消したい」が同じ動線になっている。
   Disabled は復活可能、Deleted は終端だが、その中間の「削除予約中・PII は残るが
   見えない」状態 (退職処理の宙ぶらり、本人申請の取消受付期間) を表現できない。

## 決定

1. `UserStatus` に `PendingDeletion` を追加する。soft-delete された user は
   この状態になり、`status_changed_at` が予約時刻を保持する。専用の日時カラムは
   増やさない (ADR-039 の status 統合方針を踏襲)。
2. 状態機械 `UserLifecycle` を拡張する。`Active`/`Disabled` から `SoftDelete` で
   `PendingDeletion` に入り、`Restore` で `Active` に戻る。`Purge` で `Deleted`
   (anonymize cascade) に落ちる。`Active`/`Disabled` からの緊急 `Purge` も残す。
   `Delete` event は `Purge` に置き換える。
3. `DELETE /api/admin/users/{sub}` の既定挙動を soft-delete にする。`?purge=true`
   (または body `force=true`) のときだけ ADR-036 の anonymize cascade を即時実行する。
   `POST /api/admin/users/{sub}/restore` を復元経路として追加する。
4. soft-delete は PII / Consent / RefreshToken / Session を温存し cascade しない。
   認証系 (`/authorize` `/login` `/token` `/userinfo` `/introspect`) は
   `IsActive()` ゲートにより Disabled / Deleted と同様に fail-close で拒否する。
5. 復元可能期間は SCL objective `UserSoftDeleteGracePeriod` (30 日) で固定する。
   Go 定数 `UserSoftDeleteGracePeriodSeconds` と coherence test で二重定義の乖離を防ぐ。
6. 期間経過後の完全削除は本 ADR では lazy-on-access で行う。管理者のユーザー一覧
   取得時に期限切れの `PendingDeletion` user を検出し、その場で Purge して
   `UserDeleted` (reason=`auto_purge`) を記録する。専用スケジューラは別 WI に切り出す。
7. 自爆防止は soft-delete / restore / purge のすべてに適用する。actor が対象本人かつ
   admin / system_admin role を持つ場合は `ErrSelfDeleteForbidden` で拒否する。

## 却下した代替案

- 専用の `pending_deletion_at` カラムを追加する:
  status を唯一の真実とした ADR-039 の統合方針に反し、状態と日時の二重管理になる。
  `status_changed_at` で予約時刻を表現できるため不要。
- 復元可能期間経過後の purge を専用 cron / scheduler で行う:
  デモ IdP の規模では lazy-on-access で十分で、常駐ジョブと監視を増やさない。
  一覧取得の latency が問題化した段階で別 WI として cron 化する。
- 既定 DELETE を即時 purge のまま維持し、soft-delete を別 endpoint にする:
  業界標準 (Auth0 / Okta / Keycloak) は既定を復元可能側に倒す。誤操作救済を
  既定にする方が安全側で、完全削除を明示操作にする方が事故を減らせる。

## 影響

- SCL に `PendingDeletion` 状態、`SoftDelete` / `Restore` / `Purge` vocabulary、
  `UserSoftDeleted` / `UserRestored` event、`RestoreAdminUser` interface、
  `AdminUserRestore` / `AdminUserPurge` permission、`UserSoftDeleteGracePeriod`
  objective、不変条件 `SoftDeletedUserBlockedFromAuth` が加わる。
- ADR-036 の anonymize cascade (`DeleteUser`) は「完全削除 (purge)」専用の内部処理
  として維持され、soft-delete / auto-purge / 明示 force purge の 3 経路から呼ばれる。
- JSONB `lifecycle.status` に新しい enum 値が入るだけなので schema migration は不要
  (ADR-071 の宣言的 schema と整合)。既存の `deleted` 前提の可視性フィルタも変更不要。
- 既存 wi-8 (ADR-036) の「DELETE = 即 anonymize」を前提にした外部スクリプトは
  挙動が変わる。デモ IdP のため後方互換は維持せず、`?purge=true` への移行を要する。
