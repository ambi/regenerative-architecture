# ADR-051: CIBA による非同期の人間承認をエージェント行動に導入する

## ステータス

提案 (draft)。[[wi-52-ciba-async-human-approval]] の意思決定を先行して起草する。
wi-52 の実装着手とともに「採用」へ移す。[[ADR-048]] (エージェント一級プリンシパル)・
[[ADR-007]] (consent モデル)・[[ADR-043]] (account portal step-up / CSRF)・
[[ADR-029]] (login throttling)・[[ADR-018]] (監査 / アプリログ分離) を前提に、
自律エージェントが高リスク行動の前に**帯域外 (out-of-band) で人間承認を得るための
非同期フローと、その fail-closed な安全境界**を確定する。本 ADR は RAR
([[wi-51-rich-authorization-requests-agent-scopes]]) の構造化された権限提示と
ガバナンス ([[wi-59-agent-governance-guardrails-audit-inventory]]) を承認画面・監査の
両面で再利用する。

## コンテキスト

自律エージェント ([[ADR-048]]) は、ユーザーに代わって API を呼び行動する。その中には
送金・データ削除・外部公開のように、誤れば取り返しのつかない高リスク行動が含まれる。
こうした行動の直前に「人間が明示的に承認する」関門を置きたいが、エージェントを呼び出す
consumption device (バックエンドのワークロード) と、人間が承認操作を行う authentication
device (スマートフォン等) は分離していることが多い。同期的な画面遷移を前提とした既存の
認可・同意・step-up では、この分離を表現できない。

OpenID Client-Initiated Backchannel Authentication (CIBA) はこの「非同期・decoupled な
承認」を標準化する。consumption device が `/bc-authorize` で承認要求を起票し `auth_req_id`
を得て、ユーザーの authentication device に通知を送り、人間の承認/拒否が成立するまで token を
発行しない。Auth0 の "Async Authorization"・Okta 等もエージェント向けに採用しており、
human-in-the-loop なエージェント統制の事実上の標準経路となっている。

ra-idp-go は対話的な認可・consent ([[ADR-007]])・account portal の step-up ([[ADR-043]]) を
持つが、呼び出し元とユーザーが分離した非同期承認フローを持たない。ここには 3 つの危険がある。

1. **未承認のままの token 発行**: 状態遷移 (pending / approved / denied / expired) や
   ポーリング制御が緩いと、人間が承認していないのに token が出てしまう。
2. **取り違え承認 (blind approval)**: 何を承認しているのかが人間に明示されないと、別要求を
   そのまま承認してしまう request-confusion が起きる。
3. **ポーリングの濫用**: `auth_req_id` に対する `/token` ポーリングが無制限だと、リソース
   枯渇や承認状態の総当たりに悪用される。

本 ADR は CIBA Core (poll を既定とする token delivery) を実装するにあたり、上記 3 点を
**保証義務 (fail-closed)** として確定する。

## 決定

1. **`/bc-authorize` と CIBA grant を実装する**。backchannel authentication endpoint
   `/bc-authorize` を新設し、要求を受理して `auth_req_id` を発行する。`/token` には
   `grant_type=urn:openid:params:grant-type:ciba` を追加し、`auth_req_id` を必須パラメータと
   する。要求には承認対象を識別する `scope` / `authorization_details`
   ([[wi-51-rich-authorization-requests-agent-scopes]]) と、後述の `binding_message` を含める。
   discovery に backchannel メタデータ
   (`backchannel_token_delivery_modes_supported` / `backchannel_authentication_endpoint` 等) を反映する。

2. **token delivery mode は `poll` を既定とする**。consumption device が `/token` を反復呼び出して
   承認成立を待つ `poll` を唯一のサポートモードとして実装する。サーバー起点で通知する `ping` /
   `push` は将来拡張とし、本 ADR の範囲外とする (out-of-scope)。discovery では当面
   `poll` のみを advertise する。

3. **`/token` は承認状態に応じて fail-closed に応答する**。`auth_req_id` の状態に従い、
   pending では `authorization_pending`、ポーリングが過密なら `slow_down`、承認成立後は
   token (access / id / 必要に応じ refresh) を返し、有効期限切れ・拒否・未知の `auth_req_id` では
   `expired_token` / `access_denied` 等のエラーを返す。承認要求には有効期限 (`expires_in`、既定値を
   本 ADR で定め `TenantSettings` で override 可) と最小ポーリング間隔 (`interval`、既定 5 秒) を課す。
   **明示的な承認が成立するまで token は一切発行しない**。状態判定に漏れ・曖昧さがあれば、必ず
   「発行しない」側へ倒す。期限切れ・拒否は終端状態とし、その `auth_req_id` で以後 token を出さない。

4. **承認画面で `binding_message` と要求権限を提示する**。`/bc-authorize` 要求は
   `binding_message` を伴い、ユーザーの承認 UI に consumption device 側で表示する文字列と
   一致する形で提示する。あわせて要求 `scope` と `authorization_details`
   ([[wi-51-rich-authorization-requests-agent-scopes]]) を構造的に並べ、「どのエージェントが・何を・
   どの対象に」行おうとしているかを人間が読めるようにする。これにより別要求を取り違えて承認する
   blind approval / request-confusion を防ぐ。`binding_message` の不一致・欠落時は承認させない。

5. **通知チャネルはまず email とする**。authentication device への承認依頼通知は、既存の
   SMTP email sender アダプタ ([[ADR-035]]) を流用して送る。文面・送信経路は password reset
   メール ([[ADR-030]]) と同じ基盤を再利用し、新たな通知インフラを増やさない。モバイル push
   (FCM / APNs) は将来拡張とし、本 ADR では構築しない。

6. **CIBA と step-up / consent の責務を分離する**。CIBA は「呼び出し元とユーザーが分離した
   非同期の行動承認」を担い、対話セッション内で本人性を引き上げる step-up ([[ADR-043]]) や、
   client への権限付与の同意 ([[ADR-007]]) とは別経路とする。CIBA の承認画面は、対象ユーザーの
   認証済みセッション上で行い、必要なら step-up を前段に挟む。consent が「この client にこの scope を
   許すか」を一度問うのに対し、CIBA は「今この具体的な行動を行ってよいか」を都度問う。両者を
   混ぜず、CIBA 承認の成立を consent の代替にしない。

7. **ポーリング濫用を抑止し、監査する**。`/token` の CIBA ポーリングには `interval` 強制に加え、
   違反 (間隔より短い再試行) で `slow_down` を返し、login throttling ([[ADR-029]]) と同系の
   レート制御・バックオフを `auth_req_id` / client 単位で適用する。CIBA grant の実行は新規
   permission `TokenGrantCiba` で保護する。起票・承認・拒否・期限切れ・ポーリング濫用検知を
   `BackchannelAuthRequested` / `BackchannelAuthApproved` / `BackchannelAuthDenied` /
   `BackchannelAuthExpired` として既存の監査経路 ([[ADR-018]]) へ emit し、ガバナンス
   ([[wi-59-agent-governance-guardrails-audit-inventory]]) が人間承認の来歴を読めるようにする。

## 影響

- `/bc-authorize` エンドポイントと CIBA grant が `/token` に加わる。新規 model
  `BackchannelAuthRequest` / `BackchannelAuthResponse` / `AuthReqId` / `BackchannelAuthState` /
  `TokenDeliveryMode` と、`auth_req_id` の状態 (pending / approved / denied / expired) を保持する
  ストアが必要になる。状態は終端まで fail-closed に遷移する。
- discovery に backchannel メタデータ (delivery mode = poll、backchannel authentication endpoint) が
  現れる。`ping` / `push` を出さないことで将来拡張との互換余地を残す。
- 承認 UI に「保留中の承認要求の一覧」と「承認 / 拒否画面」が加わる。画面は `binding_message` と
  要求 `scope` / `authorization_details` を提示する。
- email sender ([[ADR-035]]) と password reset メール ([[ADR-030]]) の基盤を通知に再利用し、
  通知インフラを増やさない。push は将来分離。
- `/token` のレート制御に CIBA ポーリング経路が加わり、login throttling ([[ADR-029]]) と同系の
  抑止を共有する。新規 permission `TokenGrantCiba` が RBAC / AuthZEN に加わる。
- step-up ([[ADR-043]]) / consent ([[ADR-007]]) と CIBA の責務境界が明文化され、以後の
  human-in-the-loop なエージェント統制はこの分担の上に積む。

## 却下した代替案

- **同期的な step-up のみで対応する**: 既存の step-up ([[ADR-043]]) を流用すれば実装は最小だが、
  consumption device (エージェントのワークロード) と authentication device (人間の端末) を
  分離できない。自律エージェントは人間と同じセッション・同じ画面遷移上にいないため、同期前提の
  step-up では承認の関門を表現できない。decoupled な CIBA を採用する。
- **push delivery を最初から既定にする (push-first)**: サーバー起点で authentication device へ
  通知を押し込む `push` は UX が良いが、モバイル push 基盤 (FCM / APNs)・トークン管理・到達保証など
  新たな通知インフラの負担が大きく、本 WI の核 (人間承認の関門そのもの) より先行させる必然がない。
  まず `poll` + email を既定とし、`ping` / `push` は将来拡張に回す。
- **`binding_message` を省く**: 要求識別子だけで承認させれば画面は単純になるが、人間は「何を承認して
  いるか」を確認できず、別要求を取り違えて承認する request-confusion / blind approval を招く。高リスク
  行動の関門としては致命的なので、`binding_message` と要求権限の提示を必須とし、不一致・欠落時は
  承認させない (fail-closed)。
- **承認成立まで待たず楽観的に token を出す**: ポーリングを省いて即時に token を発行し事後で取り消す
  設計は、未承認のまま高リスク行動が実行され得る。token は明示承認の成立まで必ず保留し、
  `authorization_pending` / `slow_down` / 期限切れを厳密に扱う。
