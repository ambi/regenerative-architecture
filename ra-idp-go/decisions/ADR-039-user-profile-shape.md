# ADR-039: ユーザープロフィールの形 (thin core + sparse attribute bag)

## ステータス

採用。wi-19 の基盤 (PR i)。

## コンテキスト

`spec.User` は OIDC Core の最小プロファイル (`sub` / `preferred_username` /
`name` / `given_name` / `family_name` / `email` / `email_verified`) と運用
フラグ (`roles` / `mfa_enrolled` / `disabled_at` / `deleted_at` / timestamps)
しか持たない。本番 IdP (Keycloak / Okta / Google Workspace) と比べると、
OIDC §5.1 の残りの標準クレーム、SCIM `enterprise:User` 拡張相当の組織属性、
ライフサイクル属性、tenant 定義カスタム属性が欠落していた (動機は wi-19)。

当初これらを `spec.User` に **OIDC optional claim を個別フィールドで全部足し、
組織属性/連絡先/検証済み claim も専用構造体で持つ**設計を試みた。しかし:

- どんなテナントでも実際に使う属性は一部だけ。全ユーザーに ~25 個の
  optional フィールドを持たせるのはモデルも DB も無駄に肥大する。
- `last_login_at` 等を `UserLifecycle` に置きつつ `disabled_at` / `deleted_at`
  を `User` 直下に残すのは一貫性が無い。
- 多値連絡先や `verified_claims` (OIDC4IDA、本 WI では out_of_scope) は
  過剰設計だった。

## 決定

1. **thin core + sparse attribute bag**。`User` は識別・認証・表示名・RBAC・
   ライフサイクルだけを型付きで持ち、それ以外のプロフィール属性は単一の
   sparse な `attributes: Map<String, AttributeValue>` に格納する。値を入れた
   key だけが保存されるため、使わない属性は領域を消費しない。
   - **型付き core**: `sub` / `tenant_id` / `preferred_username` /
     `password_hash` / `email` / `email_verified` / `mfa_enrolled` / `roles` /
     `name` / `given_name` / `family_name` / `lifecycle` / timestamps。
     login・lookup に使う識別子と、ID Token / 匿名化で頻用する表示名まで。
   - **attribute bag**: OIDC §5.1 optional claim (`middle_name` / `nickname` /
     `picture` / `phone_number` / `address_*` 等)、SCIM 組織属性
     (`title` / `department` / `manager_sub` 等)、tenant 定義 custom。

2. **属性は schema 駆動**。属性定義 `UserAttributeDef` を 2 階建てで持つ。
   - **組み込みカタログ** `BuiltinUserAttributeDefs()` (コード) が OIDC §5.1
     optional claim と SCIM 組織属性を全テナント共通で定義する。各定義が
     `claim_name` / `oidc_scope` / `visibility` を持ち、claim 露出の規則を
     与える (claim 生成は spec.ClaimsForScopes で実装済み)。
   - **tenant 定義** `TenantUserAttributeSchema` (集約) が tenant 固有の custom
     属性を足す。形は [[ADR-040]] で規定する。
   - 実効定義 = 組み込み ∪ tenant。`ValidateAttributes` が `User.attributes`
     を実効定義に対して検証する (未定義 key 拒否 / 型一致 / required 充足)。

3. **`address` はフラット key に分解**。OIDC `address` Claim (§5.1.1) は
   構造体にせず `address_formatted` / `address_locality` / … の string key で
   保持し、UserInfo / ID Token 生成時に address オブジェクトへ再構成する (spec.ClaimsForScopes)
   。`AttributeValue` の sum type を string / number /
   boolean / date / string[] のフラットなままに保てる。

4. **ライフサイクルは status 単一化**。`UserLifecycle` を `User` 直下の値と
   して必ず持ち、`status: UserStatus` を **無効化・削除の唯一の真実**とする。
   旧 `disabled_at` / `deleted_at` フィールドは廃止し、`status` (Disabled /
   Deleted) + `status_changed_at` に統合した。「いつ遷移したか」の詳細は
   既存の監査イベント (UserDisabled / UserDeleted) が時刻付きで持つため、
   専用タイムスタンプ列の二重持ちは不要。`IsActive()` は `status == Active`、
   `IsDeleted()` は `status == Deleted`。`Active` 以外 (Disabled / Locked /
   Staged / Suspended / Deleted) はすべて認証不可。zero-value (空 status) は
   既定で Active として解決する (SCL の `status: { default: Active }` と整合)。

5. **後方互換は不要 (pre-release)**。既存 DB 構造は作り変える。`users` から
   `disabled_at` / `deleted_at` 列を落とし、`lifecycle` / `attributes` を
   JSONB 列で持つ (migration 0009)。admin API レスポンスは新たに `status` を
   返し、現行 admin UI 用に `disabled_at` を status から導出して併載する
   (UI の status 対応は後続 PR)。

6. **監査と削除への波及** (既存 ADR の追記)。
   - [[ADR-031]] (admin-user-api-and-rbac): 監査ログに平文で残すのは
     "changedKeys" まで。属性値は [[ADR-018]] に従い PII フラグに基づき
     SHA-256 hash 化する。
   - [[ADR-036]] (user-deletion-and-anonymization): tombstone 時に
     `attributes` も全消去し、`lifecycle.status = Deleted` を設定する。`sub` と
     監査参照のみ残す。

## 影響

- `spec.User` のフィールド数が大幅に減り、プロフィール拡張は `attributes` の
  key 追加だけで済む。既存 RP のクレーム集合は変わらない (core は据え置き、
  optional claim は値が入った時だけ露出)。
- 本 PR では in-memory adapter + Postgres (JSONB) が追従。多値属性の別テーブル
  化と self API・UI は後続 PR
  ([[wi-19-rich-user-attributes]] の scope_split)。