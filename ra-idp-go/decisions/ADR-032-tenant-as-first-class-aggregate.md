# ADR-032: テナント (realm) を一級集約として導入する

## ステータス

採用

## コンテキスト

ra-idp はこれまで単一論理空間で動作してきた。client / user / consent /
ポリシー / ブランディングはすべて単一の global スコープに乗っており、
複数組織を 1 つの IdP インスタンスでホストする道が無かった。

エンタープライズ用途（B2B SaaS、複数事業部、買収統合）と Phase 5–6 の
派生機能（同意元帳の組織別保持、SAML metadata の組織別配信、Federation
broker の組織別 IdP 紐付け）はテナント境界を前提に成立する。Phase 6 まで
延期すると admin API / DCR / Federation を全て retrofit する必要がある
（README §197 で「Phase 4 が分水嶺」と明示）。

ADR-031 §7 はこの増分を予期して
「tenant role や tenant membership は次の増分で独立モデルとして追加する。
`roles` に tenant ID を埋め込まない」と明記した。本 ADR がその増分である。

## 決定

1. **Tenant 集約**を新規導入する。フィールドは
   `(id, display_name, status, created_at, updated_at, disabled_at?)`。
   `id` は URL-safe slug `^[a-z0-9][a-z0-9-]{0,62}$`。予約語 `admin` は
   `id` として使用不可（path `/admin/...` との衝突回避）。
2. **`default` テナント**が起動時に idempotent に upsert される。
   既存単一テナント運用と bare route の可用性を守るため、削除・disable
   ともに不可で常時 active とする。
3. ライフサイクルイベント `TenantCreated / TenantUpdated /
   TenantDisabled / TenantEnabled` を既存 outbox / 監査経路に流す。
4. 無効化されたテナントへの `/authorize` `/token` `/login` `/par`
   `/device_authorization` は generic `invalid_request` で拒否する。
   テナントの存在は応答から漏らさない（enumeration 防止）。
5. テナント物理削除は本フェーズでは scope 外。Phase 5 の DSAR / PII purge
   とまとめて実装する。本フェーズは reversible disable のみ。
6. 認可境界は二層とする:
   - `admin` ロール — 自テナント内のユーザー / クライアント管理に閉じる
   - `system_admin` ロール — テナント CRUD と cross-tenant 操作を許可
   - `system_admin` を持つ User は `default` control-plane tenant に所属する。
     テナント CRUD endpoint は `/realms/default/admin/tenants/...` に置き、default
     tenant の session cookie がそのまま path 一致する形で認証する (ADR-033 §1)
   - `roles` フィールドは ADR-031 の `string[]` 構造を維持し、テナント ID
     を埋め込まない。所属テナントは `User.tenant_id` で表す。
7. 既存 demo は `default` テナントに収める。isolation テストでは第二
   テナント `acme` をテスト fixture として作成する。本番 seed には含めない。

## 影響

- すべての aggregate root (Client / User / Consent / 認可コード / refresh
  token / PAR / device code / 認可リクエスト) に `tenant_id` 列が必須化
  する (実カラム配置は ADR-034)。
- HTTP 境界には URL 規約が必要 (ADR-033)。
- 管理 UI / 管理 API は本フェーズでは tenant 自体の CRUD は PR-C 送り。
  本 ADR は集約と境界の確立のみを規定する。
- demo seed は `default` テナントに alice / demo-web-app を配置する。
  既存 demo.sh は無変更で通る。
- Phase 8 の HSM / KMS 採用時に per-tenant 鍵を入れるかは別 ADR で判断する。
  本 ADR は鍵を tenant に紐付けない決定をしている。

## 却下した代替案

- **テナント ID を `roles` 配列に埋め込む** — ADR-031 が明示的に却下済み。
  ロール配列のフィルタロジックがテナント比較と混ざり、SCL の権威性が落ちる。
- **テナント無しのまま multi-tenant に見せる SaaS パターン** (例: 全レコード
  に `org_id` を持つが境界は app 層 if 文) — 集約として扱わないと
  isolation のテストが書きにくく、Phase 5–6 で retrofit が発生する。
- **テナントを `Realm` と呼ぶ** — Keycloak 互換で識別性は高いが、SCL の
  vocabulary では `Tenant` を canonical、`Realm` を alias と扱う
  （URL 上は `/realms/{id}` を採用し ADR-033 で扱う）。
