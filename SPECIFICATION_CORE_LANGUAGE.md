# Specification Core Language (SCL)

Specification Core Language (SCL) は、Regenerative Architecture の第1層 *Specification Core* を記述するための単一の形式である。システムの規範仕様として保存するのは SCL であり、契約・コード・図・テスト・監視ルールは SCL からの派生物として扱われる。コンセプション・ベースラインと ADR は、意図と決定を保持する記録として別に保存する。ワークアイテムと完了レポートは、変更の実施と承認を管理するために扱う。

## 1 目的

第1層には、契約・状態機械・行動仕様・不変条件・認可・非機能目標という多面性のすべてが必要である。これらを別々の形式（OpenAPI / JSON Schema / Protobuf / Gherkin / EARS / Cedar / Rego / OpenSLO / TLA+ など）で並行して手書き保守すると、ドリフトが生まれ、どれが真実か分からなくなる。SCL はこの並行保守を排除し、第1層を単一の上流ソースに集約する。

SCL は以下を満たす。

- 実装言語・フレームワーク・データベース・ランタイムに依存しない
- 機械実行可能（生成・検証・実行の合否が判定可能）
- AI が解釈・生成・変換可能
- 人間が読める
- 長期保存可能（ベンダ依存の形式を採らない）
- 単一上流ソース（下流のインタフェース・スキーマ・言語バインディング・実装・図はここから派生する）
- 保証可能（規範要件ごとに合否基準と必要な検証を宣言できる）

## 2 文書構造

SCL ドキュメントは先頭にシステム識別子と SCL 自身のバージョンを置き、続いて
中核9セクションと、適用する標準・ユーザー体験を記述する2つの任意セクションを持つ。

```yaml
system: TaskTracker        # 必須: システム名
spec_version: "1.0"        # 必須: SCL バージョン
annotations:     { ... }   # 任意: 文書全体への補助情報

standards:       { ... }   # 任意: 外部標準と採用する規範要件
components:      { ... }   # 任意: 単一ドキュメント内のモジュール分割
vocabulary:      { ... }   # 用語の定義
models:          { ... }   # データの形と同一性
interfaces:      { ... }   # 外部との契約（インターフェース）
states:          { ... }   # 状態と遷移
invariants:      { ... }   # 普遍的に成り立つ不変条件
scenarios:       { ... }   # 自然文ステップで書く受け入れ例
permissions:     { ... }   # 認可ルール
objectives:      { ... }   # 非機能目標
assurance:       { ... }   # 保証義務と合否基準
user_experience: { ... }   # 任意: 画面、遷移、利用品質
```

すべてのセクションで現れる名前（モデル名・フィールド名・状態名・イベント名・アクション名）はそのコンテキストの `vocabulary` に登録された語彙と一対一で対応していなければならない。CIで名前の整合性を自動検証する。

`annotations` は 9 つのセクションには含めない。文書全体に対する任意の補助情報であり、型は [§3.2 Annotation](#32-models--ドメインモデル) と同じ `map[string, any]` とする。

### 2.1 standards — 外部標準との対応

システムが従う外部仕様と、そのうち採用する規範要件を宣言する。

```yaml
standards:
  RFC7636:
    title: Proof Key for Code Exchange by OAuth Public Clients
    version: RFC 7636
    url: https://www.rfc-editor.org/rfc/rfc7636.html
    roles: [AuthorizationServer]
    scope: Authorization Code Grant の横取り攻撃対策
    requirements:
      - id: RFC7636-S256
        section: "§4.2"
        strength: MUST
        adoption: required
        statement: code_challenge_method は S256 を使用する
        relates_to:
          interfaces: [Authorize, Token]
          invariants: [PkceRoundTrip]
      - id: RFC7636-PLAIN
        section: "§4.2"
        strength: MAY
        adoption: excluded
        statement: plain code challenge method
        reason: S256 のみに限定するため
```

`adoption` はシステム仕様としての採用方針であり、実装状態ではない。

| 値         | 意味                                           |
| ---------- | ---------------------------------------------- |
| `required` | 常に満たすシステム要件                         |
| `optional` | 構成、プロファイル、クライアント能力により適用 |
| `excluded` | 意図的に仕様対象外。`reason` 必須              |

`relates_to` は `vocabulary`、`models`、`interfaces`、`states`、`invariants`、`scenarios`、`permissions`、`objectives`、`assurance` の名前を参照できる。

### 2.2 components — モジュール

1 つの SCL ドキュメント内で、モデル・状態・イベント・インターフェース・不変条件・認可・目標を所有関係で束ね、論理的なモジュール（DDD のサブドメイン）に分割する。コンテキスト全体を縦割りにする必要が出てきたら §3.10 のコンテキストマップに移行する。

```yaml
components:
  TaskAuthoring:
    description: タスクの作成・編集と担当者割り当てを所有する
    owns_models: [Task, TaskState]
    owns_events: [TaskCreated, TaskUpdated]
    owns_interfaces: [CreateTask, UpdateTask]
  TaskExecution:
    description: タスクの開始・完了・中断ライフサイクルを所有する
    owns_states: [TaskLifecycle]
    owns_events: [TaskStarted, TaskCompleted]
    owns_interfaces: [StartTask, CompleteTask]
    depends_on:
      - { component: TaskAuthoring, reason: 開始・完了は TaskAuthoring が所有する Task に対する操作である }
```

**マップキー**: モジュール名 (`<Name>`)。PascalCase。

**プロパティ**:

| プロパティ         | 型             | 必須 | 説明                                                              |
| ------------------ | -------------- | ---- | ----------------------------------------------------------------- |
| `description`      | `string`       | ✓    | モジュールの責務                                                  |
| `owns_models`      | `string[]`     | –    | 所有する `models` 名のリスト（`kind: event` の event 含む）       |
| `owns_states`      | `string[]`     | –    | 所有する `states` 名のリスト                                      |
| `owns_events`      | `string[]`     | –    | 所有する `models` のうち `kind: event` の名前のリスト             |
| `owns_interfaces`  | `string[]`     | –    | 所有する `interfaces` 名のリスト                                  |
| `owns_invariants`  | `string[]`     | –    | 所有する `invariants` 名のリスト                                  |
| `owns_permissions` | `string[]`     | –    | 所有する `permissions` 名のリスト                                 |
| `owns_objectives`  | `string[]`     | –    | 所有する `objectives` 名のリスト                                  |
| `depends_on`       | `Dependency[]` | –    | 依存先モジュールとその理由                                        |
| `annotations`      | `Annotation`   | –    | モジュールへの補助情報                                            |

**Dependency**:

| プロパティ  | 型       | 必須 | 説明                 |
| ----------- | -------- | ---- | -------------------- |
| `component` | `string` | ✓    | 依存先モジュール名   |
| `reason`    | `string` | ✓    | 依存の根拠           |

`owns_*` に挙げる名前は対応するセクションに登録されていなければならず、1 つの要素を所有するモジュールは高々 1 つである。`depends_on` は有向非循環で、循環はモジュール境界の見直しを示す。`components` を宣言しないドキュメントは「単一の暗黙モジュール」を持つものとして扱う。

### 2.3 user_experience — 画面と利用品質

人間が操作する画面、画面遷移、セキュリティ・アクセシビリティ・ローカライズ等の横断要件を宣言する。

```yaml
user_experience:
  accessibility:
    standard: WCAG22
    level: AA
  locales: [ja, en]
  screens:
    Login:
      route: /login
      purpose: ResourceOwner を認証する
      interfaces: [GetBrowserTransaction, SubmitBrowserLogin]
      states: [ready, submitting, error]
  transitions:
    - { from: Login, to: Consent, trigger: authentication_succeeded, interface: SubmitBrowserLogin }
  requirements:
    - id: UX-CSRF
      category: security
      adoption: required
      statement: 状態変更要求はCSRF検証を通過しなければならない
      screens: [Login, Consent]
      interfaces: [SubmitBrowserLogin, SubmitBrowserConsent]
```

画面名は`user_experience.screens`内で一意とする。遷移の`from`と`to`は画面名、`interface`は`interfaces`の名前を参照する。外部クライアントへの遷移は`external: true` を指定し、`to`を省略できる。

## 3 セクションリファレンス

### 3.1 vocabulary — 意味の語彙

ユビキタス言語の定義。第1層の他セクションに現れる全ての概念名はここに登録される。

```yaml
vocabulary:
  Task:
    definition: 担当者一名により独立に完了可能な作業単位
    aliases: [タスク]
    not_to_confuse_with:
      - term: Project
        reason: Project は複数の Task を束ねるが、それ自体は完了状態を持たない
  Backlog:
    definition: 着手前のタスクが置かれる状態
  Order@Sales:
    context: Sales
    definition: 顧客が確定した購入意思
  Order@Fulfillment:
    context: Fulfillment
    definition: 倉庫に対する出荷指示
```

**マップキー**: 用語名 (`<Name>`)。PascalCase を推奨。マルチコンテキストで同名を区別する必要があるときはマップキーを `<Name>@<Context>` の形にし、`context` も明示する。他セクションからの参照は完全なキー（`Order@Sales`）を使う。

**プロパティ**:

| プロパティ                     | 型           | 必須 | 説明                                               |
| ------------------------------ | ------------ | ---- | -------------------------------------------------- |
| `definition`                   | `string`     | ✓    | 用語の定義                                         |
| `description`                  | `string`     | -    | 用語の説明。通常 `definition` で十分なので省略する |
| `aliases`                      | `string[]`   | –    | 別表記・略称・他言語表記のリスト                   |
| `context`                      | `string`     | –    | コンテキスト名（マルチコンテキスト時のみ）         |
| `not_to_confuse_with`          | `object[]`   | –    | 混同しやすい類義語                                 |
| `not_to_confuse_with[].term`   | `string`     | ✓    | 混同しやすい用語の名前                             |
| `not_to_confuse_with[].reason` | `string`     | ✓    | なぜ混同してはいけないか                           |
| `annotations`                  | `Annotation` | –    | 用語への補助情報                                   |

### 3.2 models — ドメインモデル

エンティティ・値オブジェクト・イベント・列挙・エラーの宣言。

```yaml
models:
  Task:
    kind: entity
    identity: id
    fields:
      id:    { type: UUID }
      title: { type: String, constraints: [non_empty, { max_length: 200 }] }
      state: { type: TaskState }
      assignee_id: { type: UserId, optional: true }
      created_at:  { type: Timestamp }

  TaskState:
    kind: enum
    values: [Backlog, InProgress, Done]

  TaskStarted:
    kind: event
    payload:
      task_id: { type: UUID }
      started_by: { type: UserId }
      at: { type: Timestamp }

  NotFound:
    kind: error
    payload:
      target: { type: String }
```

**マップキー**: モデル名。`vocabulary` に登録されていなければならない。

**プロパティ（共通）**:

| プロパティ    | 型                                                         | 必須 | 説明                   |
| ------------- | ---------------------------------------------------------- | ---- | ---------------------- |
| `kind`        | `entity` \| `value_object` \| `event` \| `enum` \| `error` | ✓    | モデルの種別           |
| `description` | `string`                                                   | 推奨 | モデルの説明           |
| `annotations` | `Annotation`                                               | –    | モデル全体への補助情報 |

**`kind: entity` 固有**:

| プロパティ | 型                      | 必須 | 説明                         |
| ---------- | ----------------------- | ---- | ---------------------------- |
| `identity` | `string`                | ✓    | 同一性を判定するフィールド名 |
| `fields`   | `map[string, FieldDef]` | ✓    | フィールド定義               |

**`kind: value_object` 固有**:

| プロパティ | 型                      | 必須 | 説明                                               |
| ---------- | ----------------------- | ---- | -------------------------------------------------- |
| `fields`   | `map[string, FieldDef]` | ✓    | フィールド定義（全フィールドの値が等しければ等価） |

**`kind: enum` 固有**:

| プロパティ | 型         | 必須 | 説明                                                               |
| ---------- | ---------- | ---- | ------------------------------------------------------------------ |
| `values`   | `string[]` | ✓    | 列挙値のリスト。各値は `vocabulary` に登録されていなければならない |

**`kind: event` / `kind: error` 固有**:

| プロパティ | 型                      | 必須 | 説明                     |
| ---------- | ----------------------- | ---- | ------------------------ |
| `payload`  | `map[string, FieldDef]` | –    | 付随情報のフィールド定義 |

**FieldDef**:

| プロパティ    | 型             | 必須 | 説明                                                    |
| ------------- | -------------- | ---- | ------------------------------------------------------- |
| `type`        | `<Type>`       | ✓    | フィールドの型（[§4 型システム](#4-型システム) 参照）   |
| `optional`    | `bool`         | –    | 値なし許容。既定 `false`                                |
| `default`     | `any`          | –    | 既定値                                                  |
| `constraints` | `Constraint[]` | –    | 値制約のリスト（[§4.3 制約](#43-制約-constraint) 参照） |
| `description` | `string`       | –    | 補足説明                                                |
| `annotations` | `Annotation`   | -    | アノテーション                                          |

**Annotation**:

生成・検証・ドキュメント化のための任意の補助情報。SCL の中核意味論を変更しない。型は `map[string, any]` とする。SCL 処理系は、認識しないキーを無視してよい。ただし、特定の処理系や生成器が解釈するキーは、その処理系側の仕様または ADR に記録する。

### 3.3 interfaces — 外部との契約

外部世界に対する入出力の契約。HTTP・gRPC・CLI・メッセージング・GraphQL などのインタフェース・スキーマはここから生成される。インターフェースは **論理的な契約（input / output / errors / emits）** と、それを露出する **トランスポート（bindings）** に分かれる。同一の論理インターフェースを複数トランスポートで同時に露出してよい。

```yaml
interfaces:
  StartTask:
    description: バックログのタスクを開始する
    steps:
      - "{task} を開始する"
    input:
      task_id: { type: UUID }
    output:
      task: { type: Task }
    errors: [NotFound, InvalidTransition, Forbidden]
    emits:  [TaskStarted]
    idempotent: true
    bindings:
      - kind: http
        method: POST
        path: /tasks/{task_id}/start
        successful_status_codes: ["200"]
      - kind: grpc
        service: TaskService
        method: StartTask
      - kind: cli
        command: task start
        args:
          - { name: task_id, position: 1 }
        exit_codes: { success: 0, NotFound: 64, InvalidTransition: 65, Forbidden: 77 }
```

**マップキー**: インターフェース名。`vocabulary` に登録されていなければならない。

**プロパティ**:

| プロパティ    | 型                      | 必須 | 説明                                                                                                                                                                       |
| ------------- | ----------------------- | ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `description` | `string`                | 推奨 | このインターフェースが何を行うか                                                                                                                                           |
| `steps`       | `string[]`              | –    | scenarios の自然文ステップが束縛する文テンプレートの列。`{field}`=input、`{result}`=出力束縛。同一インターフェースが文脈により異なる自然文で参照される場合は複数並べてよい |
| `input`       | `map[string, FieldDef]` | –    | 入力パラメータ                                                                                                                                                             |
| `output`      | `map[string, FieldDef]` | –    | 正常系の出力                                                                                                                                                               |
| `errors`      | `string[]`              | –    | 発生しうるエラー。各要素は `kind: error` のモデル名                                                                                                                        |
| `emits`       | `string[]`              | –    | 発行するイベント。各要素は `kind: event` のモデル名                                                                                                                        |
| `idempotent`  | `bool`                  | –    | 同一入力での再実行が安全か。既定 `false`                                                                                                                                   |
| `read_only`   | `bool`                  | –    | 状態を変更しないか。既定 `false`                                                                                                                                           |
| `bindings`    | `Binding[]`             | –    | このインターフェースを公開するトランスポート群（0 個以上）                                                                                                                 |
| `annotations` | `Annotation`            | –    | インターフェース全体への補助情報                                                                                                                                           |

`bindings` を空にしておくと「設計段階の論理インターフェース」を意味する。露出時にトランスポートを追加する。

**Binding（共通）**:

| プロパティ    | 型                                                                       | 必須 | 説明                         |
| ------------- | ------------------------------------------------------------------------ | ---- | ---------------------------- |
| `kind`        | `http` \| `grpc` \| `cli` \| `event` \| `graphql` \| `sdk` \| `schedule` | ✓    | バインディング種別           |
| `description` | `string`                                                                 | –    | このバインディング固有の補足 |

**`kind: http` 固有**:

| プロパティ                | 型                          | 必須 | 説明                                          |
| ------------------------- | --------------------------- | ---- | --------------------------------------------- |
| `method`                  | `string`                    | ✓    | HTTP メソッド (`GET`, `POST` など)            |
| `path`                    | `string`                    | ✓    | URL パス（テンプレ可、例 `/tasks/{task_id}`） |
| `successful_status_codes` | `string[]`                  | –    | 正常応答ステータス                            |
| `request_form`            | `body` \| `query` \| `form` | –    | 入力の搬送形式。既定 `body`                   |
| `headers`                 | `map[string, FieldDef]`     | –    | リクエストヘッダ                              |

**`kind: grpc` 固有**:

| プロパティ  | 型                                        | 必須 | 説明                             |
| ----------- | ----------------------------------------- | ---- | -------------------------------- |
| `service`   | `string`                                  | ✓    | gRPC サービス名                  |
| `method`    | `string`                                  | ✓    | RPC メソッド名                   |
| `streaming` | `unary` \| `client` \| `server` \| `bidi` | –    | ストリーミング種別。既定 `unary` |

**`kind: cli` 固有**:

| プロパティ   | 型                 | 必須 | 説明                                                  |
| ------------ | ------------------ | ---- | ----------------------------------------------------- |
| `command`    | `string`           | ✓    | コマンド名。サブコマンド含む（例 `task start`）       |
| `args`       | `Arg[]`            | –    | 位置引数                                              |
| `flags`      | `Flag[]`           | –    | 名前付きフラグ                                        |
| `stdin`      | `<Type>`           | –    | 標準入力で受け取る型                                  |
| `stdout`     | `<Type>`           | –    | 標準出力で返す型                                      |
| `exit_codes` | `map[string, int]` | –    | `success` または `errors[]` のメンバー名 → 終了コード |

**Arg / Flag**:

| プロパティ   | 型       | 必須       | 説明                                   |
| ------------ | -------- | ---------- | -------------------------------------- |
| `name`       | `string` | ✓          | input の対応フィールド名               |
| `position`   | `int`    | Arg のみ ✓ | 1-based の位置                         |
| `short`      | `string` | –          | 短縮形（例 `a` → `-a`）                |
| `required`   | `bool`   | –          | 必須フラグ。既定はフィールド定義に従う |
| `repeatable` | `bool`   | –          | 繰り返し可能か。既定 `false`           |

**`kind: event` 固有** (pub/sub・メッセージキュー):

| プロパティ      | 型                                                  | 必須 | 説明                           |
| --------------- | --------------------------------------------------- | ---- | ------------------------------ |
| `channel`       | `string`                                            | ✓    | トピック / キュー / Subject 名 |
| `direction`     | `produce` \| `consume`                              | ✓    | 発行か購読か                   |
| `delivery`      | `at_most_once` \| `at_least_once` \| `exactly_once` | –    | 配送保証。既定 `at_least_once` |
| `ordering`      | `none` \| `per_key` \| `global`                     | –    | 順序保証                       |
| `partition_key` | `string`                                            | –    | input フィールド名             |

**`kind: graphql` 固有**:

| プロパティ  | 型                                      | 必須 | 説明                     |
| ----------- | --------------------------------------- | ---- | ------------------------ |
| `operation` | `query` \| `mutation` \| `subscription` | ✓    | 操作種別                 |
| `field`     | `string`                                | ✓    | スキーマ上のフィールド名 |

**`kind: sdk` 固有** (プロセス内関数 / ライブラリ API):

| プロパティ | 型       | 必須 | 説明                                           |
| ---------- | -------- | ---- | ---------------------------------------------- |
| `function` | `string` | ✓    | パッケージ修飾の関数識別子（例 `tasks.start`） |

**`kind: schedule` 固有** (定期起動 / cron):

| プロパティ | 型         | 必須   | 説明                                                  |
| ---------- | ---------- | ------ | ----------------------------------------------------- |
| `cron`     | `string`   | 条件付 | cron 式（例 `* * * * *`）。`cron`・`every` のいずれか |
| `every`    | `Duration` | 条件付 | 起動間隔（例 `1m`, `1h`）。`cron`・`every` のいずれか |

`kind: schedule` は input を取らない（暗黙の「現在時刻」のみ）。発火は scenarios の clock 刺激で検証する。

### 3.4 states — 状態遷移

状態を持つモデルの遷移を宣言的に記述する。`switch` 文や workflow DSL に埋め込まない。

```yaml
states:
  TaskLifecycle:
    target: Task
    initial: Backlog
    transitions:
      - { from: Backlog,    event: Start,    to: InProgress }
      - { from: InProgress, event: Complete, to: Done }
      - { from: InProgress, event: Cancel,   to: Backlog,
          guard: { not: { exists: assignee_id } } }
```

**マップキー**: 状態機械名。

**プロパティ**:

| プロパティ    | 型             | 必須 | 説明                                                 |
| ------------- | -------------- | ---- | ---------------------------------------------------- |
| `description` | `string`       | 推奨 | この状態機械の説明                                   |
| `target`      | `string`       | ✓    | 対象となる `kind: entity` のモデル                   |
| `initial`     | `string`       | ✓    | 初期状態のステート名。対応する `kind: enum` の値     |
| `terminal`    | `string[]`     | -    | 終端状態のステート名一覧。対応する `kind: enum` の値 |
| `transitions` | `Transition[]` | ✓    | 遷移のリスト                                         |
| `annotations` | `Annotation`   | -    | 状態機械全体への補助情報                             |

**Transition**:

| プロパティ | 型           | 必須 | 説明                                                        |
| ---------- | ------------ | ---- | ----------------------------------------------------------- |
| `from`     | `string`     | ✓    | 遷移元の状態のステート名                                    |
| `event`    | `string`     | ✓    | 引き金となるイベント名。`vocabulary` に登録                 |
| `to`       | `string`     | ✓    | 遷移先の状態のステート名                                    |
| `guard`    | `Expression` | –    | 遷移を許可する条件（[§5 式](#5-式-expression-の文法) 参照） |
| `effect`   | `string[]`   | –    | 遷移時に発行されるイベント                                  |

`from`・`to` の状態名は、`target` モデルの状態フィールドが参照する `kind: enum` の値と一致しなければならない。

### 3.5 invariants — 不変条件と liveness

「どんな入力・どんな実行履歴でも常に成り立つべき性質」を述べる。プロパティベーステスト・監査ルール・フォーマル検証の証明義務がここから派生する。

性質は二系統に分かれる:

- **safety** — *悪いことは決して起こらない*。`always`（常に真）/ `never`（決して真でない）で書く。
- **liveness** — *良いことはいずれ必ず起こる*。`eventually` で書き、必要なら `within` で上限時間を与える。

各主張には `assuming`（前提条件）を付けられる。前提が偽の場合は vacuously true。

```yaml
invariants:

  # safety: 不変
  StateAlwaysValid:
    description: いかなるイベント列を適用しても状態は宣言された集合に留まる
    target: Task
    always: { in: [state, TaskState.values] }

  DoneIsTerminal:
    description: Done に到達した Task は他の状態に戻らない
    target: Task
    always:
      not: { and: ["prev.state == Done", "state != Done"] }

  AuthorizationCodeSingleUse:
    target: AuthorizationCode
    never: { and: ["state == Redeemed", "event == RedeemCode"] }

  # 前提付き safety
  AccessTokenIssuedOnlyAfterConsent:
    description: AccessToken は、対応する Consent が granted である Client にのみ発行される
    assuming: "event == AccessTokenIssued"
    always:
      exists:
        in: Consents
        satisfies: "x.client_id == event.client_id and x.state == Granted"

  # 集合に対する全称量化
  AllAccessTokensCarryAudience:
    target: AccessToken
    always:
      forall:
        in: audience
        satisfies: "x != null and x != ''"

  # liveness: いずれ必ず到達する（上限時間つき）
  AuthorizationCodeEventuallyResolves:
    description: 発行された AuthorizationCode は Redeemed または Expired のいずれかに必ず到達する
    target: AuthorizationCode
    eventually: { in: [state, [Redeemed, Expired]] }
    within: 60s

  # 多重主張: 同じ前提下で同時に課す
  RefreshTokenRotationIsAtomic:
    assuming: "event == RefreshTokenExchanged"
    always: "next.old_token.state == Revoked"
    eventually: "has(next.new_token) and next.new_token.state == Active"
    within: 1s
```

**マップキー**: プロパティ名。

**プロパティ**:

| プロパティ    | 型                 | 必須 | 説明                                                          |
| ------------- | ------------------ | ---- | ------------------------------------------------------------- |
| `description` | `string`           | 推奨 | 性質の意図                                                    |
| `target`      | `string`           | –    | 対象モデル名または `interfaces.<name>`。省略時はシステム全体  |
| `assuming`    | `Expression`       | –    | 前提条件。真であるときのみ後続の主張を評価する                |
| `always`      | `Expression`       | †    | 常に真である式（safety）                                      |
| `never`       | `Expression`       | †    | 決して真にならない式（safety、`always: { not: ... }` の糖衣） |
| `eventually`  | `Expression`       | †    | いずれ真になる式（liveness）                                  |
| `within`      | `Duration`         | –    | `eventually` の上限時間。省略時は無限                         |
| `severity`    | `must` \| `should` | –    | 違反時の扱い。既定 `must`                                     |
| `annotations` | `Annotation`       | –    | プロパティへの補助情報                                        |

† `always` / `never` / `eventually` のうち少なくとも 1 つが必要。複数同時に書けば同じ `assuming` 配下での AND になる。

### 3.6 scenarios — 受け入れ例

特定の状況での期待振る舞いを、**受け入れテストとして人間が読める自然文ステップ**で記述する。`invariants` が *普遍*（常に成り立つ法則）を、`scenarios` が *個別*（具体的な振る舞いの例）を表し、両者は補完関係にある。

scenarios は**ブラックボックス**である。内部のデータモデルの値を直接組み立て・直接覗くことはしない。観測できるのは **インターフェースを通したものだけ**——呼び出しの応答・エラー・発行イベント——であり、事前状態も「作成」系インターフェースの呼び出しで組む。これにより内部表現を変えてもシナリオは壊れない。

各ステップは1つの文（文字列）であり、次のいずれかに決定的に解決される。

- **行動 (action)** — システムへの刺激。`interfaces` の `steps` テンプレートに束縛される。グルーコードは不要で、interface 定義そのものが step 定義になる。刺激は2種類：
  - **invoke** — interface の呼び出し（API・到来イベントの配信・スケジュール処理の直接呼び出し）。`steps` のいずれかのテンプレートにマッチする。
  - **clock** — 時間を進める。満期のスケジュール・TTL が発火する。組み込み形 `時刻が "<ts>" になる` / `"<duration>" 経過する`。
- **表明 (assertion)** — 観測結果の確認。model・event・error 定義から導かれる少数固定の形にだけ束縛される。
- **逃がし弁** — 重い事前準備のための `seed`（内部モデルを直接構築）と、外部依存の応答を用意する `stub`。

ステップ内の引数は `"…"` で括る。`where` を添えるとデータ表になり、ステップ内で `<列名>` として参照する。

```yaml
scenarios:

  # ── 基本：作成し、操作し、観測する ──────────────────
  Backlog のタスクを開始すると InProgress になる:
    steps:
      - タスク "買い物" を作成して "t" とする
      - "t" を開始する
      - "t" の状態は "InProgress"
      - "TaskStarted" が発行される

  # ── 逃がし弁（seed）とエラー ────────────────────────
  完了済みのタスクは開始できない:
    steps:
      - 状態が "Done" のタスクを "t" として用意する
      - "t" を開始すると エラー "InvalidTransition"

  # ── clock（定期バッチ・TTL）───────────────────────
  期限切れのタスクは毎分のバッチで Expired になる:
    steps:
      - 状態 "InProgress"・期限 "2026-01-01T00:00:00Z" のタスクを "t" として用意する
      - 時刻が "2026-01-01T00:01:00Z" になる
      - "TaskExpired" が発行される
      - "t" の状態は "Expired"

  # ── データ表（where）で網羅 ────────────────────────
  Backlog 以外のタスクは開始できない:
    where:
      - { 状態: Done }
      - { 状態: InProgress }
    steps:
      - 状態が "<状態>" のタスクを "t" として用意する
      - "t" を開始すると エラー "InvalidTransition"

  # ── 多段・外部依存（stub）──────────────────────────
  Introspect は上流 IdP に委譲する:
    steps:
      - アクセストークン "tok" を用意する
      - 上流 "UpstreamIdp.Introspect" が "{ active: true, scope: read }" を返すようにする
      - "tok" を Introspect して "r" とする
      - "r" は "{ active: true }"
```

**マップキー**: シナリオ名。**自然文の見出し**（受け入れ基準そのもの）を推奨する。

**プロパティ**:

| プロパティ    | 型           | 必須 | 説明                                                            |
| ------------- | ------------ | ---- | --------------------------------------------------------------- |
| `steps`       | `string[]`   | ✓    | 自然文ステップの列。上から順に実行・評価される                  |
| `where`       | `object[]`   | –    | データ表。各行で `<列名>` を束縛し、行ごとに `steps` を反復する |
| `tags`        | `string[]`   | –    | 分類タグ                                                        |
| `description` | `string`     | –    | シナリオの補足説明                                              |
| `annotations` | `Annotation` | –    | シナリオへの補助情報                                            |

**ステップの種別**: 各ステップ文字列は次のいずれかに解決される。

| 種別             | 束縛先                               | 例                                                         |
| ---------------- | ------------------------------------ | ---------------------------------------------------------- |
| invoke           | `interfaces.<name>.steps` のいずれか | `タスク "買い物" を作成して "t" とする`                    |
| clock            | 組み込み形                           | `時刻が "2026-01-01T00:01:00Z" になる` / `"120s" 経過する` |
| seed（逃がし弁） | 組み込み形（内部モデルを直接構築）   | `状態が "Done" のタスクを "t" として用意する`              |
| stub（逃がし弁） | 組み込み形（外部応答を用意）         | `上流 "UpstreamIdp.Introspect" が "…" を返すようにする`    |
| 表明             | model / event / error から導く固定形 | 下表                                                       |

**表明形**: 観測できるものだけを表明する（表明したい状態は、それを返す取得インターフェースが存在しなければならない）。すべて既定で **部分マッチ**。

| 形                                  | 意味                                                    |
| ----------------------------------- | ------------------------------------------------------- |
| `"<alias>" の状態は "<value>"`      | 観測した状態の一致                                      |
| `"<alias>" の <field> は "<value>"` | 観測したフィールド値の一致                              |
| `"<alias>" は "<partial>"`          | 応答の部分マッチ                                        |
| `"<Event>" が発行される`            | イベント発行（ペイロード条件は `… で "<expr>"` を付与） |
| `… すると エラー "<Error>"`         | 直前の行動がそのエラーで失敗する（行動文への接尾糖衣）  |

**結果の参照**: 行動文の `{result}` スロット（interface の `steps` で宣言）に `"<alias>"` を与えると、その応答を後続ステップから参照できる。`{result}` の値は input フィールドではなく、scenarios 側で任意に与えるエイリアス名。一方それ以外の `{field}` 部分はすべて input フィールド名と一致する。

### 3.7 permissions — 認可ルール

誰がどのリソースに対してどの操作を行えるかを宣言する。下流のポリシーエンジン（Cedar / OPA / Cerbos など）と認可 API（AuthZEN など）はここから生成される。

```yaml
permissions:
  TaskOwnerCanComplete:
    actor: User
    action: Complete
    resource: Task
    allow_when: resource.assignee_id == actor.id

  AdminCanForceCancel:
    actor: User
    action: Cancel
    resource: Task
    allow_when: actor.role == Admin

  ReadAllowedInOwnTenant:
    actor: User
    action: Read
    resource: Task
    allow_when: resource.tenant_id == actor.tenant_id
```

**マップキー**: ルール名 (`<Name>`)。

**プロパティ**:

| プロパティ    | 型           | 必須 | 説明                                                               |
| ------------- | ------------ | ---- | ------------------------------------------------------------------ |
| `actor`       | `string`     | ✓    | 主体のモデル名                                                     |
| `action`      | `string`     | ✓    | アクション名（`vocabulary` に登録、`interfaces` 名と対応してよい） |
| `resource`    | `string`     | ✓    | 対象リソースのモデル名                                             |
| `allow_when`  | `Expression` | –    | 許可する条件。省略時は無条件許可                                   |
| `deny_when`   | `Expression` | –    | 拒否する条件（`allow_when` より優先）                              |
| `description` | `string`     | 推奨 | 認可ルールの説明                                                   |
| `annotations` | `Annotation` | –    | 認可ルールへの補助情報                                             |

「認可をどう呼ぶか」（API 形式）と「どう判定するか」（ポリシー）の双方が SCL から導出されるため、ポリシーエンジンの差し替えは `permissions` の保存性を損なわない。

### 3.8 objectives — 非機能目標

SLO・性能・保持・ライフタイム・セキュリティなどの非機能要件。負荷テスト・監視ルール・アラート設定・保管ポリシーがここから派生する。`kind` によって複数系統に分かれる。

```yaml
objectives:
  StartTaskLatency:
    kind: slo
    metric: latency_p95
    interface: StartTask
    target: "<200ms"
    window: 30d

  AvailabilityCore:
    kind: slo
    metric: availability
    target: ">=99.9%"
    window: 30d

  TaskRetention:
    kind: retention
    target: Task
    policy: keep_indefinitely

  AuditLogIntegrity:
    kind: retention
    target: TaskStarted
    policy: append_only
    retention: "7y"

  TaskLifetime:
    kind: lifetime
    target: Task
    ttl: 30d

  TaskRateLimit:
    kind: security
    policy: rate_limit_per_minute
    target: StartTask
    value: 60
```

**マップキー**: 目標名。

**プロパティ（共通）**:

| プロパティ    | 型                                               | 必須 | 説明             |
| ------------- | ------------------------------------------------ | ---- | ---------------- |
| `kind`        | `slo` \| `retention` \| `lifetime` \| `security` | ✓    | 目標の種別       |
| `description` | `string`                                         | 推奨 | 非機能目標の説明 |
| `annotations` | `Annotation`                                     | –    | 目標への補助情報 |

**`kind: slo` 固有**:

| プロパティ  | 型                                                                                                | 必須 | 説明                                               |
| ----------- | ------------------------------------------------------------------------------------------------- | ---- | -------------------------------------------------- |
| `metric`    | `latency_p50` \| `latency_p95` \| `latency_p99` \| `availability` \| `error_rate` \| `throughput` | ✓    | 計測する指標                                       |
| `target`    | `string`                                                                                          | ✓    | 比較式（例: `"<200ms"`, `">=99.9%"`）              |
| `interface` | `string`                                                                                          | –    | 計測対象のインターフェース名。省略時はシステム全体 |
| `window`    | `string`                                                                                          | –    | 評価期間（例: `30d`, `7d`）                        |

**`kind: retention` 固有**:

| プロパティ  | 型                                                                                                                    | 必須   | 説明                                                                                                                                                                  |
| ----------- | --------------------------------------------------------------------------------------------------------------------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `target`    | `string`                                                                                                              | ✓      | 対象モデル名                                                                                                                                                          |
| `policy`    | `keep_indefinitely` \| `keep` \| `append_only` \| `delete_after` \| `purge_pii_after` \| `archive_after` \| `archive` | ✓      | 保持ポリシー                                                                                                                                                          |
| `retention` | `string`                                                                                                              | 条件付 | 保持期間（例: `30d`, `7d`）。有限期間を持つ `append_only`・`keep`・`delete_after`・`purge_pii_after`・`archive_after`・`archive` で必須。`keep_indefinitely` では不要 |

**`kind: lifetime` 固有**:

| プロパティ   | 型         | 必須 | 説明                             |
| ------------ | ---------- | ---- | -------------------------------- |
| `target`     | `string`   | ✓    | 対象モデル名                     |
| `ttl`        | `Duration` | ✓    | 有効期間                         |
| `single_use` | `bool`     | –    | 一度だけ使用可能か。既定 `false` |
| `reference`  | `string`   | –    | RFC・ADR・規制などの根拠         |

**`kind: security` 固有**:

| プロパティ  | 型       | 必須 | 説明                                             |
| ----------- | -------- | ---- | ------------------------------------------------ |
| `policy`    | `string` | ✓    | セキュリティ方針名（例 `rate_limit_per_minute`） |
| `target`    | `string` | –    | 対象モデル・インターフェース・イベント           |
| `value`     | `any`    | ✓    | 方針のしきい値または設定値                       |
| `reference` | `string` | –    | 根拠となる ADR・標準                             |

`security.policy` はアプリケーション・業界規格・組織ルールに依存するため、SCL コアでは列挙しない。特定の処理系やサンプルが解釈する policy 名は、その処理系側の仕様または ADR に記録する。

### 3.9 assurance — 保証義務

`assurance` は、規範要件を満たしたと判定するための主張、合否基準、必要な検証を宣言する。ここには検証結果を書かない。結果と承認は完了レポートに記録する。

```yaml
assurance:
  ClientTenantIsolation:
    claim: ClientAdmin は他 Tenant の Client を操作できない
    risk: 他 Tenant の認証設定の侵害
    risk_level: critical
    derived_from:
      interfaces: [CreateClient, UpdateClient, DeleteClient]
      permissions: [ClientAdminMayManageOwnTenantClient]
      invariants: [ClientBelongsToExactlyOneTenant]
    acceptance:
      all:
        - evidence: CrossTenantAuthorizationTests
          criterion: create update delete の全操作が Forbidden になる
        - evidence: AuthorizationMutationCheck
          criterion: tenant 比較を除去した変異をテストが検出する
    evidence:
      CrossTenantAuthorizationTests:
        kind: test
        producer: independent
        evaluation: deterministic
        environments: [ci]
        recheck: affected_change
        covers:
          permissions: [ClientAdminMayManageOwnTenantClient]
      AuthorizationMutationCheck:
        kind: mutation_test
        producer: independent
        evaluation: deterministic
        environments: [ci]
        recheck: affected_change
    approval:
      when: [acceptance_not_met, exception_requested, permission_model_changed]
      role: SecurityOwner
```

**保証義務のプロパティ**:

| プロパティ | 型 | 必須 | 説明 |
| ---------- | -- | ---- | ---- |
| `claim` | `string` | ✓ | 真であると保証したい、単一の判定可能な主張 |
| `risk` | `string` | ✓ | 主張が偽だった場合の損失 |
| `risk_level` | `low` \| `medium` \| `high` \| `critical` | ✓ | 検証強度と承認者を決める等級 |
| `derived_from` | `TraceRef` | ✓ | 主張の根拠となる SCL 要素 |
| `acceptance` | `AcceptanceExpression` | ✓ | 検証結果に対する機械判定可能な合否条件 |
| `evidence` | `map[string, EvidenceRequirement]` | ✓ | 必要な検証の宣言 |
| `approval` | `ApprovalRequirement` | – | 人間の承認が必要になる条件 |
| `annotations` | `Annotation` | – | 補助情報 |

`TraceRef` は各 SCL セクション名をキー、要素名の配列を値とするマップである。参照先は存在しなければならない。`assurance` 自身を参照して保証義務を循環定義してはならない。

`AcceptanceExpression` は `all`、`any`、`not` と、次のリーフから成る。

| プロパティ | 型 | 必須 | 説明 |
| ---------- | -- | ---- | ---- |
| `evidence` | `string` | ✓ | 同じ保証義務内の検証名 |
| `criterion` | `string` | ✓ | 成功を判定する具体的条件。単なる「テストが通る」は不可 |

**EvidenceRequirement**:

| プロパティ | 型 | 必須 | 説明 |
| ---------- | -- | ---- | ---- |
| `kind` | `test` \| `property_test` \| `model_check` \| `contract_test` \| `mutation_test` \| `static_analysis` \| `scan` \| `runtime_observation` \| `manual_inspection` | ✓ | 検証手法 |
| `producer` | `generator` \| `independent` | ✓ | 検証結果の生成主体。高・重大リスクで `generator` のみは不可 |
| `evaluation` | `deterministic` \| `heuristic` \| `human` | ✓ | 結果の判定方法 |
| `environments` | `string[]` | ✓ | 検証を実行する環境 |
| `recheck` | `once` \| `affected_change` \| `every_change` \| `release` \| `continuous` | ✓ | 再検証が必要になる条件 |
| `covers` | `TraceRef` | ✓ | この検証が直接確認する要素 |
| `procedure` | `string` | – | 再実行可能な手順または処理系非依存の記述 |
| `oracle` | `string` | – | 期待値を実装と独立に決める合否判定基準 |

`producer: independent` は、実装生成時の会話や自己申告をそのまま合格根拠にせず、隔離されたコンテキスト、別実装、別手法のいずれかで結果を得ることを意味する。`evaluation: deterministic` は、型検査、スキーマ検査、署名検証など、同じ入力から同じ判定を得る検査を意味する。`evaluation: heuristic` となる AI レビューは欠陥探索には使えるが、それだけで高・重大リスクの合否判定基準にしてはならない。

**ApprovalRequirement（人間の承認条件）**:

| プロパティ | 型 | 必須 | 説明 |
| ---------- | -- | ---- | ---- |
| `when` | `string[]` | ✓ | 人間の承認を要求する条件 |
| `role` | `string` | ✓ | 判断責任を持つ役割 |
| `decision_record` | `bool` | – | ADR または例外承認の記録を必須にするか。既定 `true` |

処理系は少なくとも、合否基準の未充足を表す `acceptance_not_met`、例外承認の要求を表す `exception_requested`、リスク上昇を表す `risk_increased`、規範仕様の変更を表す `normative_spec_changed` を `when` の標準条件として解釈する。

保証義務は、合格を表す `passed`、失敗を表す `failed`、未実施を表す `not_run`、例外承認済みを表す `exception_approved` のいずれかとして評価する。`not_run` は `passed` ではない。`exception_approved` には承認者、理由、対象範囲、補償策、期限または失効条件が必要であり、無期限の例外承認は許可しない。

### 3.10 複数コンテキスト

機能数が増え、変更の主軸が機能側に移ったシステムは、境界づけられたコンテキストに縦割りできる。各コンテキストは §2 冒頭の9セクション構造をそのまま持つ独立した SCL ドキュメントであり、コンテキスト間の関係は1つのコンテキストマップが宣言する。コンテキストが1つだけのシステムにはマップは不要。

**マップキー**: コンテキスト名。各コンテキストの SCL ドキュメントの `system` と対応する。

| プロパティ              | 型                                                                                                       | 必須 | 説明                                                                        |
| ----------------------- | -------------------------------------------------------------------------------------------------------- | ---- | --------------------------------------------------------------------------- |
| `publishes`             | `string[]`                                                                                               | -    | 他コンテキストが `Context.Name` で参照してよい名前。既定は空＝全面非公開    |
| `depends_on`            | `map[string, Dependency]`                                                                                | -    | 依存する上流コンテキスト。キーは上流コンテキスト名                          |
| `depends_on.<ctx>.uses` | `string[]`                                                                                               | ✓    | 実際に参照する名前。各要素は上流の `publishes` に含まれていなければならない |
| `depends_on.<ctx>.via`  | `shared_kernel` \| `published_language` \| `customer_supplier` \| `conformist` \| `anticorruption_layer` | -    | 統合パターン（助言的）                                                      |

## 4 型システム

### 4.1 組み込み型

| 型          | 説明                         |
| ----------- | ---------------------------- |
| `String`    | 文字列                       |
| `Integer`   | 整数                         |
| `Float`     | 浮動小数点数                 |
| `Boolean`   | 真偽                         |
| `UUID`      | UUID v4                      |
| `Date`      | 日付 (ISO 8601)              |
| `Timestamp` | 時刻 (RFC 3339, UTC)         |
| `Duration`  | 期間 (例: `30d`, `5m`, `7y`) |
| `JSON`      | 任意の JSON 値               |
| `Bytes`     | バイト列                     |

### 4.2 パラメトリック型

文字列形式で書く。

| 表記        | 説明                      |
| ----------- | ------------------------- |
| `T[]`       | `T` の順序付きリスト      |
| `Set<T>`    | `T` の集合                |
| `Map<K, V>` | キー `K`・値 `V` のマップ |

ユーザ定義型（`models` 内のキー）も `type:` の値として直接書ける。

### 4.3 制約 (Constraint)

`FieldDef.constraints` に書ける制約。短いものは文字列、パラメータを取るものはマップで書く。

| 制約                 | 適用型                 | 説明                                      |
| -------------------- | ---------------------- | ----------------------------------------- |
| `non_empty`          | String, List, Set, Map | 長さ 1 以上                               |
| `{ max_length: N }`  | String, List, Set      | 最大長                                    |
| `{ min_length: N }`  | String, List, Set      | 最小長                                    |
| `{ min: N }`         | Integer, Float         | 最小値                                    |
| `{ max: N }`         | Integer, Float         | 最大値                                    |
| `{ pattern: regex }` | String                 | 正規表現マッチ                            |
| `{ format: name }`   | String                 | 名前付き形式（`email`, `url`, `e164` 等） |
| `unique`             | List, Set              | 要素重複なし                              |

## 5 式 (Expression) の文法

`invariants.assuming` / `invariants.always` / `invariants.never` / `invariants.eventually`、`states.transitions.guard`、`permissions.allow_when` / `permissions.deny_when` に書ける式。式には**文字列式**と**構造化形式**があり、構造化形式の `satisfies` など式を引数に取る位置に文字列式を書ける（リーフ位置でのみ混在可）。文字列式の中に構造化形式は埋め込まない。

### 5.1 文字列式: CEL の限定サブセット

文字列式は [Common Expression Language (CEL)](https://github.com/google/cel-spec) の以下のサブセットに従う。型規則・演算子優先順位・null 意味論・評価順は CEL 仕様が規定する。SCL はその上に変数バインディング（§5.3）を追加するのみである。

**許可される構成要素**:

| カテゴリ | 構成                                                                      |
| -------- | ------------------------------------------------------------------------- |
| リテラル | `int`、`double`、`string`、`bool`、`null`、リスト `[...]`、マップ `{...}` |
| 算術     | `+`、`-`、`*`、`/`、`%`（単項 `-` を含む）                                |
| 比較     | `==`、`!=`、`<`、`<=`、`>`、`>=`                                          |
| 論理     | `&&`、`\|\|`、`!`                                                         |
| 属性参照 | `<var>.<field>`、`<var>[<key>]`                                           |
| 関数     | `size(x)`（コレクション・文字列の長さ）、`has(x.y)`（属性存在）           |

**許可されない構成要素**:

- マクロ（`all`、`exists`、`exists_one`、`filter`、`map`）— 量化は構造化述語（§5.2）を使う
- ユーザ定義関数の呼び出し

**`and` / `or` / `not` の別表記**: 文字列式中の `and`、`or`、`not` は `&&`、`||`、`!` のエイリアスとして受け入れる。

**型対応**: SCL の組み込み型（§4.1）は CEL 型に次のとおり対応する。

| SCL 型      | CEL 型      |
| ----------- | ----------- |
| `String`    | `string`    |
| `Integer`   | `int`       |
| `Float`     | `double`    |
| `Boolean`   | `bool`      |
| `UUID`      | `string`    |
| `Date`      | `string`    |
| `Timestamp` | `timestamp` |
| `Duration`  | `duration`  |
| `JSON`      | `dyn`       |
| `Bytes`     | `bytes`     |

### 5.2 構造化形式

| 形式                                              | 説明                                                               |
| ------------------------------------------------- | ------------------------------------------------------------------ |
| `and: [expr, ...]`                                | 論理積                                                             |
| `or: [expr, ...]`                                 | 論理和                                                             |
| `not: expr`                                       | 否定                                                               |
| `equals: [a, b]`                                  | 等価                                                               |
| `in: [value, collection]`                         | 集合包含                                                           |
| `not_in: [value, collection]`                     | 集合非包含                                                         |
| `exists: <field>`                                 | フィールドが値を持つ（`null`/未設定でない）                        |
| `not_exists: <field>`                             | フィールドが値を持たない                                           |
| `forall: { in: <collection>, satisfies: <expr> }` | 集合の全要素について真。各要素は `satisfies` 内で `x` として参照可 |
| `exists: { in: <collection>, satisfies: <expr> }` | 集合のいずれかの要素について真。要素は `x` で参照可                |
| `count: <collection>`                             | 要素数（数値式。比較やしきい値に使う）                             |
| `len: <collection-or-string>`                     | 長さ（コレクションまたは文字列）                                   |

`exists` は単独で文字列を取ればフィールド存在チェック、`{ in, satisfies }` を取れば集合の存在量化子になる（引数の形で曖昧性なく解釈される）。`<collection>` にはフィールド参照（`audience`）、モデル名（`Consents`）、リテラル（`[1, 2, 3]`）のいずれも書ける。

### 5.3 変数とスコープ

文字列式・構造化形式に共通の変数バインディングは、現れる位置によって決まる。

**`invariants` 内**:

| 変数              | 説明                                |
| ----------------- | ----------------------------------- |
| `<field>`         | `target` モデルの現在のフィールド値 |
| `prev.<field>`    | イベント適用前の値                  |
| `next.<field>`    | イベント適用後の値                  |
| `event`           | 現在のイベント名                    |
| `<Model>.values`  | enum モデルの値の集合               |
| `<Model>.<field>` | 他モデルへの参照                    |

**`states.guard` 内**:

| 変数            | 説明                          |
| --------------- | ----------------------------- |
| `<field>`       | `target` モデルのフィールド値 |
| `input.<field>` | 遷移を引き起こした入力        |

**`permissions` 内**:

| 変数               | 説明                            |
| ------------------ | ------------------------------- |
| `actor.<field>`    | アクター（呼び出し主体）の属性  |
| `resource.<field>` | リソースの属性                  |
| `context.<field>`  | 呼び出し時の文脈（時刻・IP 等） |

**`forall` / `exists` の `satisfies` 内** （上記の各スコープに追加で）:

| 変数 | 説明           |
| ---- | -------------- |
| `x`  | 集合の現在要素 |

## 6 派生関係

SCL はシステム振る舞いの単一の規範的上流ソースである。下流は次の生成チェーンを成す。

1. **SCL（規範仕様として保存）** — 上記 9 セクション
2. **インタフェース・スキーマ・ポリシー・ルール（生成物）** — OpenAPI / JSON Schema / Protobuf / AsyncAPI / Cedar / OPA Rego / OpenSLO 監視ルール / Mermaid 状態機械図 / シーケンス図
3. **言語バインディング・実装・テスト（生成物）** — TypeScript の Zod、Python の Pydantic、Go の構造体、プロパティテスト、行動テスト

開発時には、この生成チェーンとは別にワークアイテムと完了レポートを扱う。ワークアイテムは SCL だけでなく、ADR、障害報告、依存関係の更新要求なども根拠にできる。完了レポートは過去時点の実行結果と判断を含むため再生成物ではなく、対象の SCL 版、ワークアイテム、ソース版、生成物の要約値に結びつけ、改変を検知できる形で保持する。

双方向の対応関係では、すべての変更された規範要件が実装・テスト・検証結果へ到達し、すべての意味を持つ実装変更が SCL、ワークアイテム、ADR のいずれかへ逆参照できなければならない。フォーマット変更など意味を持たない差分はその旨を分類する。

## 7 記法と保存形式

SCL の本質はその抽象構造であり、シリアライズ形式は次の要件を満たすものを選ぶ。

- 構造化（list / map / 型付き値）
- スキーマ検証可能
- バージョン管理可能（テキスト diff が取れる）
- 長期保存可能（ベンダ独自形式を採らない）

現時点での実装形式として **YAML** を推奨する。代替として JSON・CUE も可。重要なのは選んだ形式自体ではなく、SCL の抽象構造を逸脱しないことである。

## 8 変更管理とデータ連続性

SCL の変更は本質的にビジネスルールの変更である。各変更は第2層 ADR と対で進める。SCL は「現時点の何を保存するか」、ADR は「なぜそう保存することにしたか」を保持し、両者は一体である。

SCL は現時点の定義のみを保持し、バージョン間の変遷履歴は持たない。変更の意図・後方互換性・段階展開の方針は ADR に記録される。実際のデータマイグレーションは、変更の意図をADR（第2層）が記録し、新旧 SCL の差分から第3層・第4層が変換コードを導出・実行する。

SCL の変更時には、参照グラフから影響する保証義務とワークアイテムを特定し、必要に応じて更新または追加する。影響を受けた検証結果は `recheck` に従って失効させ、再実行するまで過去の `passed` を流用しない。

保証義務を削除・弱化する変更、`risk_level` を下げる変更、必要な検証の `producer` を `independent` から `generator` に変える変更はリスク受容であり、ADR と人間の承認を必須とする。完了レポートは SCL の履歴ではなく承認履歴なので、SCL 更新後も削除しない。
