# ADR-036: ユーザー削除と即時匿名化 (anonymize cascade)

## ステータス

採用。

## コンテキスト

ADR-031 で `/admin/users` の Disable / Enable は実装したが、削除経路が無い
ため次が成立しない。

- データ主体 (end user) の削除要求 (GDPR Art.17 right-to-erasure)。
- 退職処理。Disable のみだと audit / consent / refresh token / session が
  残り、攻撃時に「無効化された旧アカウントを起点に refresh token を再
  活性化される」リスクと運用衛生上の懸念が同居する。
- tenant 内のテスト用 user の本格的な掃除 (デモシードと衝突する)。

単純な hard delete は採用しない。

- `AdminAuditEvent` などの append-only ログが `sub` を参照しており、
  参照整合性 (概念上) を壊す。
- 削除と無効化の差を運用上見分けたい。
- GDPR 文脈でも "anonymize で sub + 一意化トークンを残す" 形が一般的。

## 決定

1. `User` aggregate に `deleted_at: Timestamp?` を導入する (DB スキーマには
   既存)。`deleted_at != null` を **tombstone 状態** と呼び、SCL の
   `UserLifecycle` 状態機械の `Deleted` 終端状態と対応させる。`Active` /
   `Disabled` のいずれからも `Deleted` に遷移できるが、`Deleted` から戻る
   遷移は持たない。
2. 削除は **物理削除しない**。`sub` は audit 参照のため不変で残す。
   削除時に以下の tombstone 置換を atomic に適用する。
   - `preferred_username = "deleted:<sub>"`
   - `name = nil` / `given_name = nil` / `family_name = nil`
   - `email = nil` / `email_verified = false`
   - `password_hash = ""` (login 不可)
   - `mfa_enrolled = false`
   - `roles = []`
   - `deleted_at = now`
   - `disabled_at` はそのまま (削除前が disabled なら disabled のまま、
     再有効化はできない)
3. 関連 aggregate を cascade で消す。
   - `Consent` (DELETE all rows for `sub`)
   - `RefreshTokenRecord` (DELETE all rows for `sub`)
   - `LoginSession` (DELETE all sessions for `sub`)
   - `PasswordHistory` (DELETE all entries for `sub`)
   - `MfaFactor` (DELETE all factors for `sub`)
   - `DeviceAuthorization` (DELETE active records bound to `sub`)
   実装は use case 側で順次呼び出す。PostgreSQL 経路は `pgx.BeginTx` で
   1 トランザクションに束ね、Valkey 経路は per-store 削除を順に呼ぶ
   (Valkey は session / device code / replay 程度の volatile state なので
   transactional 不整合の窓は短い)。
4. `sub` の再利用は禁止する。新規 User 作成時に `sub` 生成器は既存衝突
   チェックを継承する (本 ADR で新規制約は追加しないが、tombstone も
   `users` テーブルに残るので結果的に衝突しない)。
5. `preferred_username` は **削除後に再利用可** とする。`preferred_username =
   "deleted:<sub>"` への置換と `users_preferred_username_active_idx`
   (`WHERE deleted_at IS NULL` の部分一意 index) によって、生存中の user
   とだけ unique になる。
6. 削除済 user の認証・トークン経路は以下に固定する。
   - login (`/api/auth/login`) は `invalid_credentials` (DisabledAt と同じ
     uniformity を維持。anti-enumeration)。
   - `/authorize` は session が無くなっているので login にリダイレクト。
   - `/token` (refresh / authorization_code / device_code) は
     `invalid_grant` ("ユーザーは利用できません")。
   - `/introspect` は `active=false`。
   - `/userinfo` は 401 `invalid_token`。
   既発行 short-lived JWT は expiry まで RS で検証は通る。RP 通知
   (Back-Channel Logout / CAEP) は Phase 3 のスコープで本 ADR の対象外。
7. 削除は冪等。既に `deleted_at != null` の user に対して再度 `DeleteUser`
   を呼んでも 200 / no-op (audit event は重複発行しない)。
8. 自爆防止。actor.Sub == target.Sub かつ target.Roles に `admin` または
   `system_admin` が含まれる場合は reject (`invalid_request`)。
   tenant 内 admin が 0 になる削除を許す判断は本 ADR ではしない。
9. 監査保全。`UserDeleted` event を `AdminAuditEvent` として永続化する。
   payload は `actorSub` / `targetSub` / `reason` (任意 free-text) /
   `occurredAt`。`sub` は anonymize で残るため、後続の調査で
   「削除日時・削除者・対象 sub」を再現できる。

## 影響

- SCL に `UserLifecycle` 状態機械、`Delete` vocabulary、`Deleted` 終端
  状態、`DeleteUser` interface、`UserDeleted` event、`AdminUserDelete`
  permission が追加される。
- Authentication component の `owns_states` / `owns_events` /
  `owns_interfaces` / `owns_permissions` が更新される。
- `PiiPurgeAfterDeletion` objective は「削除時に即 PII 匿名化」を
  根拠に retention を `0s` に変更し、`+30日` の物理消去予定は撤回する
  (anonymize 自体が物理消去と同等の PII 排除を提供するため)。
- Go の repository port に `DeleteAllForSub(ctx, sub)` (cascade 対象) と
  `MarkDeleted(ctx, sub, now, tombstone)` (User) が増える。memory /
  postgres / valkey の各 adapter に新規メソッドが実装される。
- HTTP に `DELETE /api/admin/users/{sub}` が追加される。CSRF + Origin
  + `requireAdmin` の既存ガードを継承し、新規 RBAC 経路は作らない。
- UI に削除ダイアログ (preferred_username typing confirm + reason) が
  追加される。AdminUsersPage の `FindAll` は既に `deleted_at IS NULL` の
  user だけ返すため、削除後は一覧から自動的に消える。
- 既存 `disabled_at` 経路には影響なし。Disable と Delete は独立した
  終端で、`disabled_at != null` の user は restoration 可能なまま。
- Hard delete は debug 用 CLI も含めて提供しない。後の "30 日 grace
  期間 + 物理消去" の要件が来た場合は本 ADR を改定する。
