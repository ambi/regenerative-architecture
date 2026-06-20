# ADR-040: 属性スキーマとカスタム属性ポリシー

## ステータス

採用。wi-19 の基盤 (PR i)。[[ADR-039]] の attribute bag を統治する。

## コンテキスト

[[ADR-039]] で、core 以外のプロフィール属性を `User.attributes:
Map<String, AttributeValue>` に sparse に持つと決めた。本 ADR はその属性を
統治する schema の形と運用規約を定める。OIDC 標準クレームの組み込み属性と、
tenant 定義のカスタム属性を **同一の `AttributeDef` 機構**で扱う点が要点。
Keycloak UserProfile / Okta Custom Profile / Google customSchemas 相当。

## 決定

1. **2 階建ての属性定義**。属性は `AttributeDef` で定義し、出所を 2 つ持つ。
   - **組み込みカタログ** `BuiltinAttributeDefs()` (コード、全テナント共通)。
     OIDC §5.1 optional claim と SCIM enterprise:User 拡張相当の組織属性を
     定義する。OIDC claim は `claim_name` + `oidc_scope` + `visibility:
     ClaimExposed` を持ち、組織属性は `visibility: AdminReadable` で claim
     露出しない。
   - **tenant 定義** `TenantAttributeSchema` (独立 aggregate、identity:
     `tenant_id`)。tenant 固有の custom 属性を足す。tenant aggregate には
     embed せず別 aggregate とする理由は、(a) schema 変更が tenant 本体より
     頻繁、(b) 後続 PR で別テーブル化したい、(c) tenant 削除時の cascade を
     明示したい、ため。tenant 削除時は
     `TenantAttributeSchemaRepository.Delete(tenantID)` で cascade する
     ([[ADR-034]] のテナント境界に従い全テーブルが `tenant_id` を持つ)。
   - 実効定義 = 組み込み ∪ tenant。custom key が組み込み key と衝突する schema
     は拒否する。

2. **`AttributeDef` のフィールド**。
   - `key`: **snake_case、英字始まり** (`^[a-z][a-z0-9_]{0,62}$`)。
   - `type`: `string` / `number` / `boolean` / `date` / `string_array`
     (`AttributeValue` の sum type discriminator と一致)。
   - `multi_valued`: `type == string_array` のときのみ true になる規約。
   - `required`: 必須か。
   - `editable_by_user`: self-service 経路で end-user が編集できるか
     (false なら admin 専用)。
   - `claim_name` / `oidc_scope`: 露出する OIDC claim 名と、それを解禁する
     scope。`visibility == ClaimExposed` のときに効く。
   - `visibility`: `private` / `self_readable` / `admin_readable` /
     `claim_exposed` の 4 段。**`claim_exposed` のみ** RP に開示できる。
   - `pii`: PII 扱いするか。**省略時 true** (安全側 default)。

3. **値検証フロー**。`User.attributes` を保存する際、実効定義に対して
   `ValidateAttributes` で検証する。
   - 定義に無い key は拒否。
   - `required` な属性の欠落は拒否。
   - 値の `type` が定義の `type` と一致しない場合は拒否。
   - `AttributeValue` は Type が示すフィールドだけが設定されている
     (sum type の単一充足) ことを保証する。
   self-service 経路は加えて `editable_by_user=true` の属性しか触れない
   (use case は後続 PR)。

4. **PII と監査の切り替え**。`pii=true` の属性値は [[ADR-018]] に従い保管・
   監査で **SHA-256 hash 化**する。`pii=false` のみ平文を許す。default が
   true なので、明示的に false にしない限り常に hash 化される。GDPR 上の
   sensitive attribute を schema に定義するかは tenant 側の運用責任とし、
   本システムは hash 化で最小限の防御を提供する。

## 影響

- 本 PR では `AttributeValue` / `AttributeDef` / `TenantAttributeSchema` の
  spec 型・zog 検証・`BuiltinAttributeDefs()` カタログ・in-memory repository
  まで。schema を編集する admin API、custom attribute scope での claim 露出、
  UI は後続 PR。
- `ValidateAttributes` は usecase 層から呼ぶ前提の純関数として spec に置く。
  admin / self の権限差は actor の permission で吸収する (後続 PR)。