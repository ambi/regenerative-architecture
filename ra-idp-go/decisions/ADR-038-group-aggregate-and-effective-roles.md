# ADR-038: グループ集約と実効ロール (effective roles)

## ステータス

採用

## コンテキスト

これまで RBAC は `User.roles []string` のみで表現されてきた (ADR-031)。
「営業チーム = `catalog:read` + `invoice:read`」のようなロール束を一つの単位
として定義し、入社・異動・退職に合わせてまとめて付与・剥奪する手段が無い。
ロールをユーザーごとに個別管理すると、組織変更のたびに全該当ユーザーを
個別更新する必要があり、監査も困難になる。

Keycloak (groups + group roles)、Okta (groups → role/app assignment)、
Google IAM (groups as principals) はいずれもグループによるロール集約を
一級機能として持つ。ra-idp にも tenant-scoped なグループ集約を導入する。

本 ADR は WI-9 の決定を記録する。WI のテキストは新 ADR を `ADR-037` と
呼んでいたが、`ADR-037-use-case-layer-only-when-it-carries-domain-work.md`
が既に存在するため番号を `ADR-038` に繰り下げた。

## 決定

1. **Group 集約**を新規導入する。フィールドは
   `(id, tenant_id, name, description?, roles[], created_at, updated_at?)`。
   `id` は生成される不変の `group_<uuid>`、`name` はテナント内で一意な
   編集可能の表示名 (Keycloak の id=UUID / name 編集可に倣う)。
2. **テナント境界**: Group は `tenant_id` を持ち、テナントに閉じる
   (ADR-032〜034 と整合)。クロステナントのメンバーシップは拒否する
   (`AddMember` は対象 User をロードし、不在・別テナントなら拒否)。
3. **`name` のテナント内一意性**: `(tenant_id, name)` に unique index を張り、
   `CreateGroup`/`UpdateGroup` は衝突時 `ErrGroupNameConflict` を返す
   (`admin_users.go` の username 衝突と同じ扱い)。
4. **実効ロール (effective roles)** を
   `effective_roles(user) = user.roles ∪ ⋃_{g ∈ user.groups} g.roles`
   と定義する。和集合のみ。`spec.EffectiveRoles` はソート済み・重複排除した
   union を返す。グループが空なら `user.roles` と同一になり、既存挙動を保つ。
5. **適用面 (surface)** は二つ:
   - 管理コンソールの RBAC ゲート (`requireAdmin` / `requireSystemAdmin` /
     `resolveAdminActor` / `requireAuditReader`) — 内部的に実効ロールへ切替。
   - `/account` セルフビュー (`authorize_handler.go`, `resp.Roles`) —
     ユーザー自身がグループ由来ロールを確認できる。
   ワイヤフォーマットは不変。グループが空なら従来と完全に同一の挙動。
6. **メンバーシップ操作は冪等**とする (Okta/Keycloak の membership API に倣う)。
   既存メンバーへの `AddMember`、非メンバーへの `RemoveMember` は no-op で、
   重複イベントを発行しない。
7. **監査保証**: メンバーシップ操作とグループ CRUD は `AdminAuditEvent` と
   ドメインイベント (`GroupCreated`/`GroupUpdated`/`GroupDeleted`/
   `GroupMemberAdded`/`GroupMemberRemoved`) を発行する。`DeleteGroup` は
   メンバーシップを cascade 削除し、削除メンバーごとに `GroupMemberRemoved`、
   最後に `GroupDeleted` を発行する。
8. **`admin.groups.read` / `admin.groups.write`** 権限で保護し、`admin` ロール
   に束ねる (SCL `permissions`)。
9. **`User.roles` は維持する**。グループに束ねられないユーザー個別の override
   経路として残す。テナント ID は埋め込まない (ADR-031 §7 / ADR-032 §6 と整合)。
10. **トークンへの groups/roles claim は既定で出さない**。本 IdP は現状
    role→scope / role→claim マッピングを持たず、ロールは (a) 管理 RBAC ゲートと
    (b) `/account` コンテキスト表示のみを駆動する。グループ由来ロールも
    この二面で効く。token への claim 投入は opt-in として別 WI に送る。

## 影響

- `groups` / `group_members` テーブルを追加 (`infra/migrations/0008_groups.sql`)。
  `groups.tenant_id` は `tenants(id)` へ FK (RESTRICT)、`(tenant_id, name)` に
  unique index、`group_members` は `groups(id)` へ FK ON DELETE CASCADE、
  `users(sub)` へ FK ON DELETE CASCADE。`roles` は JSONB。
- 5 つのドメインイベントを outbox / 監査経路に流す
  (topic `iam.groups.v1`)。
- 管理 UI に `/admin/groups` ページを追加。`AdminUsersPage` の詳細パネルに
  「所属グループ」セクションを追加し、ロール表示を
  明示ロール / グループ由来ロール / 実効ロールに分割する。
- RBAC ゲートは実効ロールを参照するため `GroupRepo` を HTTP `Deps` に注入する。
  未注入 (`GroupRepo == nil`) の場合は raw `user.Roles` を返し、後方互換を保つ。

## 検討したが見送った代替案 (considered & deferred)

Keycloak / Okta / Google IAM との比較で機能ギャップを洗い出し、RA-minimal の
方針で以下を本 WI の scope 外とした。

| 項目 | 参照製品 | 判断 | 理由 |
| --- | --- | --- | --- |
| グループ階層 / サブグループ | Keycloak nested groups | 見送り | 継承順序と evaluation が複雑化。フラットな union で要求を満たす |
| 動的 / ルールベース所属 | Okta group rules, Keycloak | 見送り | 属性ベース自動メンバーシップは別 WI。明示メンバーシップに限定 |
| 既定 / 自動参加グループ | Keycloak default groups, Okta Everyone | 見送り (別 WI) | 2 つの scope 質問でユーザーと合意済み |
| deny / minus ルール | — | 見送り | union のみ。減算は評価順序の複雑性を招く |
| メンバーシップ・ロール / 委譲グループ管理 | Okta group admin roles | 見送り | グループ単位の delegated admin は別関心事 |
| 期限付きメンバーシップ | — | 見送り | time-bound membership は別 WI |
| 自由形式グループ属性 | Keycloak group attributes | 見送り | スキーマレス属性は本フェーズで不要 |
| groups / roles トークン claim | Keycloak group membership mapper | 見送り (opt-in) | role→claim マッピング自体が未実装。別 WI |
| ロール付与時の昇格防止ガード | — | 見送り | 既存 `user.roles` の無制限付与と対称に保つ。横断的関心事として別 WI に明記 |
| SCIM プロビジョニング | Okta/Azure SCIM | 見送り | 外部プロビジョニングは Phase 外 |
