# ADR-052: 関係ベースの細粒度認可 (ReBAC) を導入する

## ステータス

提案 (draft)。[[wi-53-rebac-fine-grained-authorization]] の意思決定を先行して起草する。
wi-53 の実装着手とともに「採用」へ移す。[[ADR-010]] (AuthZEN ポリシーを仕様核に置く)・
[[ADR-038]] (グループ集約と実効ロール)・[[ADR-034]] (テナント単位の永続化境界)・
[[ADR-049]] (token exchange による委譲と actor チェーン) を前提に、エージェント
(およびユーザー代行) のデータアクセスを**リソース単位 (per-resource) で判定する関係ベース
認可 (ReBAC / Fine-Grained Authorization)** の核を確定する。本 ADR は委譲
([[wi-50-token-exchange-delegation-actor-chain]]) で確立する actor チェーンを判定に取り込み、
ガバナンス ([[wi-59-agent-governance-guardrails-audit-inventory]]) が「誰が何にアクセスできるか」を
追跡できる土台となる。

## コンテキスト

AI エージェント、とりわけ RAG (Retrieval-Augmented Generation) パイプラインは、大量の
文書・レコードへ横断的にアクセスして応答を生成する。ここで決定的に重要なのは「代行している
ユーザーが本来見られるものだけを取得する」ことであり、リソース単位の細粒度認可が不可欠になる。
1 件の文書を漏らすことが、そのまま情報漏洩になる。

ra-idp-go は既に AuthZEN スタイルの PDP を持つ ([[ADR-010]]) が、その判定は client 認可
ルール (グラントタイプ保持・redirect URI 検証・sender constraint など) と、RBAC の実効ロール
([[ADR-038]]) を中心としており、いずれも**主体に紐づく粗い権限**を扱う。これらでは
「ユーザー U が文書 D を読めるか」「フォルダ F の配下を継承して読めるか」「チーム T のメンバーだから
共有された資料を見られるか」といった**リソース×主体の関係に依存する判定**を表現できない。

Google Zanzibar に始まる関係ベースアクセス制御 (ReBAC) と、その実装である OpenFGA
(Auth0 / Okta の Fine-Grained Authorization) は、まさにこの per-resource 判定の標準的手法に
なっている。関係を `(object, relation, subject)` のタプルとして格納し、関係グラフを辿って
「この主体はこのリソースに対しこの関係を持つか」を判定する。継承 (フォルダ→文書)・グループ
(チーム→メンバー)・委譲を、グラフの辺として自然に表現できる点が ABAC や RBAC に対する優位である。

さらにエージェント文脈では、判定は単一主体では完結しない。[[ADR-049]] の token exchange で
「エージェント A がユーザー U を代行する」関係 (actor チェーン) が成立するため、アクセスは
**「ユーザー U として、かつエージェント A を経由して」**評価され、U と A の双方が許可する場合に
のみ通らねばならない。U が見られる文書でも、A に委ねられていなければ取得させない。

本 ADR は、この ReBAC 判定を [[ADR-010]] の仕様核・adapter 境界の流儀に従って導入し、
ローカル評価エンジンと外部 PDP (OpenFGA 等) を同一契約で差し替え可能にしつつ、既定拒否
(default-deny) / fail-closed を保証義務として確定する。

## 決定

1. **Zanzibar 風の authorization model を採用する**。`ResourceType` (例: `document` /
   `folder` / `index`)・`RelationType` (例: `owner` / `editor` / `viewer` / `parent` /
   `member`)・`RelationTuple` `(object, relation, subject)` の三要素で関係を表現する。
   subject は直接の主体 (`user:U` / `agent:A`) と、別タプルへの参照 (userset, 例
   `folder:F#viewer`) の双方を取れる。relation は他 relation から導出する書き換え規則
   (例 `viewer = editor ∪ parent.viewer`) を持ち、継承・グループ・含意をグラフで表す。
   このモデル定義自体を仕様核に宣言的に置き、唯一の権威とする ([[ADR-010]] と同じ思想)。

2. **AuthZEN インターフェースを tuple ベース判定へ拡張する**。[[ADR-010]] の
   `authorize({ subject, action, resource, context })` を保ったまま、`action` を ReBAC の
   relation に対応づけ、judgement を関係タプルのグラフ探索で解決する経路を加える。
   client 認可・RBAC 実効ロールの既存判定はそのまま残し、リソース単位判定だけを ReBAC へ
   委ねる。呼び出し側 (ユースケース層) はインターフェースの背後がローカルエンジンか外部 PDP かを
   知らない。adapter 境界は [[ADR-010]] の `local-authzen-adapter` を踏襲し、`local-rebac-adapter`
   を既定実装、将来の OpenFGA / Zanzibar サービス接続を HTTP 差し替えとする。

3. **判定 context に委譲・actor チェーンを載せる**。[[ADR-049]] の `act` チェーンを判定 context に
   明示的に渡し、アクセスを**「as user U, via agent A」**として評価する。`CheckAccess` は
   subject (代行されるユーザー U) と actor チェーン (経由するエージェント A …) の**両方が許可する**
   場合にのみ許可を返す (logical AND)。actor 側の許可が確認できない、または context に actor が
   欠落している場合は拒否側へ倒す。これにより「ユーザーが見られても、そのエージェントには
   委ねていない」リソースを確実に遮断する。

4. **ローカル ReBAC 評価エンジンを実装する**。tuple ストア + 関係グラフ探索からなるローカル
   エンジンを Postgres adapter 上に構築する。タプルは tenant-scoped に格納し ([[ADR-034]])、
   探索・書き込み・列挙はすべて `tenant_id` 境界内に限定する。cross-tenant な関係タプルは
   作らせず、探索が境界を越える辺に到達したら拒否する。判定は **default-deny / fail-closed** を
   核とする: 許可タプルへ到達したときのみ許可し、グラフ探索の打ち切り・タイムアウト・エラー・
   モデル未定義はすべて拒否として扱う。グラフ探索の循環・深さは bounded にする。

5. **既存 RBAC 実効ロールを置換せず補完する**。[[ADR-038]] の実効ロールは「管理操作を行えるか」
   「テナント横断の admin 権限を持つか」といった**粗い (coarse) 権限**を引き続き担う。ReBAC は
   「この文書を読めるか」という**細粒度 (fine-grained) のリソース判定**を担う。両者は直交し、
   ReBAC は RBAC を置き換えない。管理 API などは RBAC permission で守り、データアクセスは
   ReBAC で守る、という役割分担を確立する。

6. **新規インターフェースと permission を定義する**。`WriteRelationTuples` (タプルの追加・削除を
   atomic に適用)・`CheckAccess` (単一の許可判定、actor チェーン考慮・fail-closed)・
   `ListAccessibleResources` (主体がアクセスできるリソースの列挙、RAG の事前絞り込み用) を
   新設する。authorization model とタプル空間の管理操作は新規 permission
   `AdminAuthorizationModelManage` で保護し、判定は [[ADR-010]] の `authorize()` 経由とする
   (ReBAC の運用自体も既存ポリシーの統制下に置く)。

7. **整合性モデル (consistency) を明示する**。`WriteRelationTuples` でタプルを書いた直後の
   判定が古い状態を見ないよう、Zanzibar の "new-enemy problem" を避ける read-after-write
   整合性を既定とする。具体的には、書き込みは consistency token (Zanzibar の Zookie 相当) を
   返し、`CheckAccess` は「少なくともこの token 以降の状態で評価する」最小鮮度を要求できる。
   既定では権限**剥奪**は即時反映 (剥奪タプルの削除を観測するまで判定を遅らせる) を fail-closed
   寄りに優先し、付与の遅延より剥奪の取りこぼしを許さない。外部 PDP 差し替え時も同じ整合性契約を
   adapter に課す。

8. **観測と監査を流す**。`RelationTupleWritten` / `RelationTupleDeleted` / `FgaCheckEvaluated`
   を既存経路 ([[ADR-018]]) へ emit し、ガバナンス ([[wi-59-agent-governance-guardrails-audit-inventory]])
   が「誰がどの関係を書き、どの判定がどう転んだか」を追跡できるようにする。

## 影響

- 仕様核に authorization model 定義 (ResourceType / RelationType / 書き換え規則) と ReBAC 判定
  ルールが加わり、[[ADR-010]] の網羅性テストの流儀に倣ってモデルと実装の乖離を検知する。
- 新規 model `RelationTuple` / `RelationType` / `ResourceType` / `FgaCheckRequest` /
  `FgaCheckResult` を SCL に追加し、AuthZEN 判定 context に actor チェーンを追加する。
- Postgres に tenant-scoped な関係タプルテーブル (`tenant_id` 必須、[[ADR-034]]) と
  グラフ探索クエリが加わる。`local-rebac-adapter` を既定、外部 PDP を差し替え点とする。
- HTTP に関係タプル管理 API と `CheckAccess` / `ListAccessibleResources` エンドポイントが加わる。
  データアクセス判定は ReBAC、管理操作は RBAC permission、という二層が並立する。
- `CheckAccess` は subject と actor チェーンの双方を要求し、欠落・エラーは拒否へ倒す。RAG は
  `ListAccessibleResources` で取得対象を事前に絞り、許可されない文書を検索対象に入れない。
- 外部 OpenFGA / Zanzibar の本番接続実装、RAG パイプライン本体、大規模 tuple の
  シャーディング / キャッシュ最適化は本 ADR の対象外とし、まず正しさ (fail-closed) を優先する。

## 却下した代替案

- **粗い RBAC ([[ADR-038]]) のみで賄う**: 実装は最小だが、ロールは主体に紐づく粗い権限であり
  「ユーザー U が文書 D を読めるか」をリソース単位で判定できない。RAG の per-document 認可が
  成立せず、エージェントが見てはならない文書を取得しうる。ReBAC で補完する (置換ではない)。
- **認可をアプリケーション層に散在させる**: 各データアクセス経路に `if` で許可判定を書くと、
  再生成時に脱落するリスクがあり、テストで網羅できない。これは [[ADR-010]] が
  「セキュリティポリシーは仕様核に置く」と決めた理由とまったく同じであり、リソース判定だけを
  例外にする理由はない。判定は仕様核 + adapter 境界へ集約する。
- **最初から外部 OpenFGA サービスへ hard-couple する**: 成熟した ReBAC 実装を即時に得られるが、
  起動時から外部サービスへのランタイム依存が生じ、参照実装としての自己完結性と即時テスト可能性を
  損なう ([[ADR-010]] が Cedar / OPA を仕様核へ直結しなかった判断と同じ)。adapter 境界を保ち、
  ローカルエンジンを既定、OpenFGA は差し替え先とする。
- **関係を持たない純粋な ABAC で表現する**: 属性ルールだけでも一部の判定は書けるが、
  フォルダ→文書の継承やチーム→メンバーの所属といった**関係 / 継承のグラフ**を属性条件で
  表現するのは煩雑で、推移的な含意が爆発しやすく可読性も低い。関係をタプルとして一級に扱う
  ReBAC の方が、エージェントのデータアクセスに必要な継承・委譲を素直に表せる。
- **独自の関係スキーマを定義する**: Zanzibar / OpenFGA の type / relation / tuple 語彙に倣わず
  自前モデルにすると、外部 PDP への差し替え (本 ADR の adapter 境界の目的) が崩れ、知見・ツールも
  流用できない。Zanzibar 風モデルに従う。
