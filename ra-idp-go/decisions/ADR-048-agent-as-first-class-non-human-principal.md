# ADR-048: AI エージェントを第一級の非人間プリンシパルとして導入する

## ステータス

採用。[[wi-49-agent-identity-first-class-principal]] の意思決定を確定する。[[ADR-032]] (テナント一級集約)・[[ADR-031]]
(admin user API / RBAC)・[[ADR-008]] (client 認証方式)・[[ADR-010]] (AuthZEN ポリシー)・
[[ADR-018]] (監査 / アプリログ分離) を前提に、エージェント (非人間 ID) を扱う**プリンシパル
モデルと統制の最小核**を確定する。本 ADR は後続の委譲 ([[wi-50-token-exchange-delegation-actor-chain]])・
即時失効 ([[wi-58-continuous-access-evaluation-agent-revocation]])・ガバナンス
([[wi-59-agent-governance-guardrails-audit-inventory]]) の土台となる。

## コンテキスト

LLM ベースの自律・半自律な AI エージェントが、ユーザーに代わって API を呼び、データを
取得し、行動する利用が広がっている。現代の IdP はエージェントを「人間でもなければ従来の
サービスアカウントでもない」第一級のプリンシパル種別 (Non-Human Identity, NHI) として
扱い始めた。Microsoft は Entra Agent ID (2026-04 GA) でエージェントにディレクトリ上の ID を
自動付与し、Okta / Auth0・Google・Ping Identity も NHI としてのエージェント管理を提供する。

ra-idp は現状 `User` と `OAuth2Client` (machine 用の client_credentials を含む) しか持たない。
このためエージェント固有の次の関心事を一級概念として表現できない。

1. **所有者との結びつき**: エージェントは必ず人間または組織に帰属し、孤立した NHI を作らない。
2. **目的と関与レベル**: 用途の宣言と、自律 (autonomous) か監督下 (supervised) かの区別。
3. **ライフサイクルと即時停止**: 無効化と、緊急停止 (kill-switch) の一方向操作。
4. **来歴の主体**: 委譲チェーン ([[wi-50-token-exchange-delegation-actor-chain]]) や監査
   ([[wi-59-agent-governance-guardrails-audit-inventory]]) で「誰が行ったか」を表す actor。

これらを `OAuth2Client` のフィールドに後付けすると、汎用 M2M client と区別できず、監査・
ポリシー・失効が機械的に混ざる。一方で新たな資格情報・暗号要素を二重に持つと攻撃面が広がる。
そこで「資格情報は既存資産を流用し、所有・統制・来歴の層だけを新設する」境界を決める必要がある。

## 決定

1. **`Agent` 集約を新規導入する**。`User` / `OAuth2Client` とは別の第一級プリンシパル種別とする。
   フィールドは `(id, tenant_id, display_name, kind, status, owner, purpose, created_at,
   updated_at, disabled_at?, killed_at?)`。`id` は URL-safe slug。`kind` は
   `autonomous` | `supervised`。

2. **資格情報は新設せず既存の `OAuth2Client` を流用する**。`Agent` は資格情報プリミティブを
   持たず、1 つ以上の既存 `OAuth2Client` 登録に束縛する (`AgentCredentialBinding`)。
   自律ワークロードとしての発行は `client_credentials` ([[ADR-008]]) を、ユーザー代行は
   token exchange の subject / actor ([[wi-50-token-exchange-delegation-actor-chain]]) を経路に使う。
   `Agent` は所有・統制・来歴の層に限定する。

3. **所有者を必須にする**。すべての `Agent` は所有者 (人間 `User` または owning `Group` / 組織) に
   紐づく。所有者のオフボードは配下エージェントの失効へ伝播する
   ([[wi-58-continuous-access-evaluation-agent-revocation]])。孤立した NHI を作らせない。

4. **トークンにプリンシパル種別を持たせる**。`AccessTokenClaims` に発行先が `Agent` であることを
   示す principal type marker を **optional** で追加する。resource server / AuthZEN ([[ADR-010]]) が
   エージェント発行トークンを判別でき、委譲時は actor チェーン ([[wi-50]]) がエージェントを actor として担う。
   既存トークン消費側を壊さないため拡張は optional のみとする ([[ADR-012]])。

5. **ライフサイクルと kill-switch を fail-closed に強制する**。`status` は
   `active` | `disabled` | `killed`。トークン発行経路でエージェント status を確認し、
   `disabled` / `killed` には新規トークンを発行しない。`disable` は可逆な運用停止、`kill` は
   緊急停止の一方向操作で、既発行トークンの失効 ([[wi-58]]) を伴う。判定漏れがあっても
   「発行しない」側へ倒す。

6. **テナントスコープに従う**。`Agent` の登録・参照・操作・束縛は tenant-scoped とする
   ([[ADR-032]] / [[ADR-034]])。cross-tenant なエージェント所有は認めない。

7. **ライフサイクルイベントを監査・outbox に流す**。`AgentRegistered` / `AgentUpdated` /
   `AgentDisabled` / `AgentEnabled` / `AgentDeleted` / `AgentOwnerChanged` を既存経路 ([[ADR-018]]) へ emit する。

8. **認可は AuthZEN とロールに従う**。エージェント CRUD と kill-switch は新規 permission
   `AdminAgentsManage` で保護し、判定は [[ADR-010]] の `authorize()` 経由とする。

## 影響

- 新規集約 `Agent` と Postgres `agents` テーブル (owner / client への外部キー、`tenant_id` 必須、
  [[ADR-034]]) を追加する。`AgentCredentialBinding` で既存 `OAuth2Client` と関連付ける。
- トークン発行経路にエージェント status ゲートが入る。`client_credentials` と token exchange の
  双方で同一の fail-closed 判定を通す。
- `AccessTokenClaims` は principal type を optional 追加するのみで後方互換を保つ。既存の
  introspection / userinfo / SIEM connector は無変更で通る。
- `Agent` は統制・来歴の層、`OAuth2Client` は資格情報・認証のプリミティブ、という責務分離が確立する。
  以後の wi-50〜wi-59 はこの層分けの上に積む。
- 管理 UI / API にエージェント registry (一覧・登録・所有者表示・無効化・kill-switch) が加わる。

## 却下した代替案

- **`OAuth2Client` のみで表現する (新集約なし)**: 実装は最小だが、所有者・関与レベル・kill-switch を
  一級で持てず、汎用 M2M client と監査・ポリシー上区別できない。失効 ([[wi-58]]) とガバナンス
  ([[wi-59]]) が retrofit になる。
- **`User` に `type=service/agent` フラグを足す**: パスワード / MFA / プロフィール / 認証主体性など
  人間前提の不変条件がエージェントに適用され、`User` モデルが濁る。エージェントは認証の主体ではなく
  行為の主体である。
- **独自資格情報を持つ完全独立の `Agent`**: 認証・暗号要素を二重化し攻撃面を広げる。wi-49 の
  リスク方針 (鍵・資格情報を二重化しない) に反する。資格情報は既存 `OAuth2Client` に集約する。
- **`ServiceAccount` と命名する**: レガシーな M2M の含意が強く、ユーザー代行 (on-behalf-of) や
  監督下 (human-in-the-loop) の次元を取りこぼす。SCL vocabulary では `Agent` を canonical とし、
  `ServiceAccount` は必要なら alias 扱いとする ([[ADR-032]] の `Realm`/`Tenant` と同じ方針)。
