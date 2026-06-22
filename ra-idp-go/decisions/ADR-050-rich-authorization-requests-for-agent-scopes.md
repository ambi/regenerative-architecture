# ADR-050: Rich Authorization Requests でエージェント権限を細粒度に束縛する

## ステータス

提案 (draft)。[[wi-51-rich-authorization-requests-agent-scopes]] の意思決定を先行して起草する。
wi-51 の実装着手とともに「採用」へ移す。[[ADR-048]] (エージェント一級プリンシパル)・
[[ADR-049]] (token exchange によるダウンスコープ)・[[ADR-007]] (同意モデル)・
[[ADR-010]] (AuthZEN ポリシー)・[[ADR-012]] (opaque / JWT access token) を前提に、
AI エージェントへ「必要な分だけ」を付与するための **構造化された最小権限要求の安全境界** を確定する。
本 ADR は CIBA による人間承認 ([[wi-52-ciba-async-human-approval]]) と
ガバナンス ([[wi-59-agent-governance-guardrails-audit-inventory]]) と組み合わさり、
[[wi-50-token-exchange-delegation-actor-chain]] のダウンスコープ規則を構造化詳細へ拡張する。

## コンテキスト

AI エージェントへの権限付与を粗い OAuth scope (例: `payments`) で行うと、エージェントは
「支払い API のすべて」を握ってしまい、最小権限が成り立たない。本当に束縛したいのは
「口座 X から最大 $100 まで送金」「この 3 文書のみ閲覧」のように、対象・上限・条件を
構造的に表した権限である。これを標準化したのが Rich Authorization Requests (RFC 9396) の
`authorization_details` で、`type` で識別される構造化 JSON オブジェクトの配列として、
要求・同意・トークン反映を一貫して扱える。

ra-idp-go は現状 scope ベースの同意までしか持たず、構造化された細粒度権限を表現できない。
`authorization_details` を導入すれば、エージェントは要求段階で対象と上限を宣言し、ユーザーは
同意段階でそれを人間が読める形で確認し、resource server は検証段階でトークンに束縛された
詳細を信頼できる。さらに [[ADR-049]] の token exchange と組み合わせると「上限つき委譲」
(交換のたびに詳細を縮小する) が成立する。

ここには 3 つの危険がある。

1. **過大な権限付与 (over-grant)**: 自由形式の詳細をそのまま受理すると、検証が緩いぶん
   エージェントが要求しただけ広い権限を得てしまう。
2. **同意逸脱 (consent drift)**: 発行トークンの詳細が、ユーザーが同意した範囲を超える。
3. **解釈の不整合**: resource server ごとに `type` を私的に解釈すると、IdP と RS、RS 同士で
   同じ `type` の意味がずれ、横断的な統制・監査ができない。

そこで本 ADR は、受理する `type` を事前登録スキーマに限定し、同意との部分集合判定と
交換時のダウンスコープを **保証義務 (fail-closed)** として確定する。

## 決定

1. **`authorization_details` を /authorize・/par・/token で受理する**。認可リクエスト
   (/authorize)、Pushed Authorization Requests (/par)、token endpoint (/token) で
   RFC 9396 の `authorization_details` パラメータを受け付ける。token endpoint には
   [[ADR-049]] の token-exchange grant も含み、交換時に詳細を縮小 (ダウンスコープ) する
   経路をここで担う。discovery には対応 `type` の一覧 (`authorization_details_types_supported`) を
   広告する。

2. **受理する `type` をテナント登録済みスキーマに限定し、Zog で fail-closed に検証する**。
   テナントが事前登録した `authorization_details` の `type` スキーマだけを受理する。各詳細は
   (a) 構文 (JSON 構造・必須フィールド)、(b) 登録済み `type` への適合 (型・列挙・上限制約) を
   Zog schema で検証する。未登録 `type`・スキーマ不適合・検証不能はすべて拒否する。
   判定漏れがあっても「受理しない」側へ倒す。

3. **発行・交換は部分集合に限る (ダウンスコープ規則)**。発行されるトークンの詳細は、ユーザーが
   同意した詳細の **部分集合** でなければならない。さらに [[ADR-049]] の token exchange では、
   交換後トークンの詳細は元 (source) トークンの詳細の **部分集合** でなければならない。
   部分集合関係は登録スキーマ上で定義された半順序 (対象の包含・上限の単調減少) で判定し、
   超過・拡大・等価未満を満たさない要求は拒否する (fail-closed)。詳細を伴わない交換は
   元トークンの詳細を保持または縮小するのみとし、決して拡張しない。

4. **同意 UI は詳細を人間が読める形で提示する**。end-user 同意 UI ([[ADR-007]] の同意モデル) は、
   生の JSON ではなく登録スキーマに紐づく表示テンプレートを用い、「どの対象に・どの操作を・
   どの上限まで」を日本語等で人間が読める形に変換して提示する。ユーザーは詳細単位で内容を
   確認したうえで同意する。同意した詳細は [[ADR-007]] の同意レコードに構造化して保存し、
   以後の発行・更新・取消の基準とする。

5. **詳細を `AccessTokenClaims` へ optional で反映し、検証点を定義する**。発行トークンには
   `authorization_details` claim を **optional** で反映する ([[ADR-012]] のトークン claim 方針に従い
   後方互換を保つ)。JWT access token では claim としてトークン内に、opaque access token では
   `/introspect` 応答として詳細を返す。resource server が信頼する **検証点** は「IdP が発行・署名
   (JWT) または introspect (opaque) した詳細」であり、RS は自前で詳細を拡大解釈してはならない。
   RS は受け取った詳細を業務操作へ突き合わせ、合致しないアクセスを拒否する (解釈実装は RS 側、
   ra-idp は発行と検証点の提供まで)。

6. **粗い `scope` と構造化 `authorization_details` の優先順位・共存規則を定める**。両者は共存でき、
   `authorization_details` が表す権限はその `type` が扱う領域において `scope` に **優先** する。
   ある領域を `authorization_details` で束縛したときは、同領域を粗い `scope` で広く付与し直す
   解釈を許さない (構造化詳細が上限を画定する)。重複する `type` と `scope` が同時に来た場合は、
   より狭いほうを採り、矛盾は拒否する (fail-closed)。

7. **管理操作を新規 permission で保護し、判定は AuthZEN を通す**。`authorization_details` の
   `type` スキーマの登録・更新・無効化は新規 permission `AdminAuthorizationDetailTypesManage` で
   保護する。この管理操作も、発行時・交換時の詳細許可判定も、すべて [[ADR-010]] の AuthZEN
   `authorize()` を経由して決定し、[[ADR-010]] の網羅性テストでルール実装漏れを検知する。

## 影響

- 新規 model として `AuthorizationDetail` / `AuthorizationDetailType` / `AuthorizationDetailsSchema` を
  追加し、`AuthorizationRequest` / `Consent` ([[ADR-007]]) / `AccessTokenClaims` ([[ADR-012]]) に
  `authorization_details` を持たせる。`AccessTokenClaims` への追加は optional で後方互換を保つ。
- 新規イベント `AuthorizationDetailsRequested` / `AuthorizationDetailsConsented` /
  `AuthorizationDetailsRejected` を既存監査経路へ emit し、要求・同意・拒否の来歴を残す。
- /authorize・/par・/token に Zog による詳細検証ゲートが入り、token-exchange ([[ADR-049]]) には
  ダウンスコープ部分集合判定が追加される。検証点は JWT claim と `/introspect` の双方で一貫する。
- 同意 UI に詳細表示が加わり、`type` スキーマごとの表示テンプレートが必要になる。
- AuthZEN ポリシーに詳細許可ルールと `AdminAuthorizationDetailTypesManage` が追加される。
- CIBA ([[wi-52-ciba-async-human-approval]]) とガバナンス
  ([[wi-59-agent-governance-guardrails-audit-inventory]]) は本詳細を再利用し、人間承認と
  上限監査を構造化詳細の上に積む。

## 却下した代替案

- **粗い `scope` のみで表現する**: 実装は最小だが、`scope` は領域単位の二値許可しか表せず、
  「口座 X から $100 まで」のような対象・上限・条件を構造化できない。エージェントには粗すぎ、
  最小権限が成り立たない。RFC 9396 の `authorization_details` で細粒度に束縛する。
- **自由形式の `authorization_details` を無検証で受理する**: RFC 9396 の柔軟性をそのまま使うと、
  検証が緩いぶん過大な権限付与 (over-grant) と同意逸脱 (consent drift) を招く。受理する `type` を
  テナント登録スキーマに限定し、構文・型適合・部分集合を fail-closed で検証する。
- **resource server ごとに `type` を私的解釈する (中央登録なし)**: IdP に登録せず RS が独自に
  詳細を解釈すると、同じ `type` の意味が RS ごとにずれ、IdP と RS・RS 同士で一貫性が失われ、
  横断的な同意・ダウンスコープ・監査が成立しない。`type` スキーマは中央 (テナント) 登録し、
  検証点を IdP に集約する。
