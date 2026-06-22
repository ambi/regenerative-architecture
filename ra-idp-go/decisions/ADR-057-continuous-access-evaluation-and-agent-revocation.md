# ADR-057: 継続的アクセス評価 (CAEP/SSF) によるエージェントの近リアルタイム失効

## ステータス

提案 (draft)。[[wi-58-continuous-access-evaluation-agent-revocation]] の意思決定を先行して
起草する。wi-58 の実装着手とともに「採用」へ移す。[[ADR-048]] (エージェント一級プリンシパル
と kill-switch)・[[ADR-049]] (token exchange による委譲・actor チェーン)・[[ADR-004]]
(refresh token rotation / family revocation)・[[ADR-012]] (opaque / JWT access token の
失効性トレードオフ)・[[ADR-003]] (JWT 署名アルゴリズム)・[[ADR-009]] (鍵ローテーション)・
[[ADR-018]] (監査 / アプリログ分離) を前提に、AI エージェントのセッション・トークン・
委譲チェーンを**継続評価して近リアルタイムに失効させる安全境界**を確定する。本 ADR は
ガバナンス ([[wi-59-agent-governance-guardrails-audit-inventory]]) が読む失効の網を、
エージェント ([[wi-49-agent-identity-first-class-principal]]) と委譲
([[wi-50-token-exchange-delegation-actor-chain]]) の上に被せる。

## コンテキスト

エージェントは長時間・高頻度にトークンを使い続けるため、発行時点の判定だけでは不十分で
ある。所有者のオフボード、kill-switch ([[ADR-048]])、異常検知といったリスク変化が起きた
とき、既発行のセッション・トークンを近リアルタイムに失効させる必要がある。しかし
ra-idp の access token は既定で短命 JWT (RFC 9068) であり ([[ADR-012]])、JWT は自己完結ゆえに
TTL 満了まで有効で、IdP 側からの即時失効が構造的に効きにくい。refresh token は opaque で
DB レコードに `revoked=true` を立てれば即時失効でき ([[ADR-004]])、Token Revocation
(RFC 7009) も提供するが、これらはいずれも**当該 IdP 内のローカルな失効**であり、エージェントの
委譲先 ([[ADR-049]]) や外部の resource server / 別 IdP には伝播しない。

この「失効を生態系へ伝える」標準が OpenID の Shared Signals Framework (SSF) と
Continuous Access Evaluation Profile (CAEP) である。Security Event Token (SET, RFC 8417) を
transport にイベントを push / receive し、access の継続評価と即時失効を可能にする。
ra-idp-go の README ロードマップ (Phase 3) は CAEP / SSF を汎用機能として挙げているが、
「所有者オフボードで配下エージェントを一括失効」「kill-switch を全トークン・委譲チェーンへ
伝播」というエージェント固有の観点は未着手である。

ここには 3 つの危険がある。

1. **失効の取りこぼし**: イベントが届かない / 反映されないと、停止したはずのエージェントが
   既発行トークンで動き続ける。短命 TTL に頼るだけでは満了までの窓が残る。
2. **誤失効 / 偽造失効**: 署名検証を欠いたイベントを反映すると、攻撃者が任意のセッションを
   失効させる DoS や、なりすましイベントによる権限操作を許す。
3. **委譲チェーンの取り残し**: エージェント本体を失効しても、token exchange で派生した
   下流トークン ([[ADR-049]]) が生き残ると、代行権限が孤立して残存する。

本 ADR は SSF / CAEP を ra-idp の失効プリミティブに接続するにあたり、上記 3 点を
**保証義務 (fail-closed)** として確定する。判定や検証に迷いがあれば「失効する」側へ倒す。

## 決定

1. **SSF transmitter と receiver の双方向として動作する**。ra-idp は CAEP イベントを
   生成して外部へ push する transmitter であると同時に、外部 (別 IdP・workload・上流の
   ガバナンス基盤) からの検証済みイベントを受理する receiver でもある。生態系の中で
   「自分が起こした失効を配り、他者の失効を受け取る」双方向の参加者とする。

2. **CAEP のイベント種別を実装する**。`session-revoked`・`token-claims-change`・
   `credential-change`・`assurance-level-change` を扱う。`session-revoked` は当該
   プリンシパルのセッション・トークンの即時失効、`token-claims-change` は claim 変更
   (権限縮小・剥奪) の反映、`credential-change` は資格情報 ([[ADR-048]] の束縛 `OAuth2Client`) の
   変更、`assurance-level-change` は保証レベル低下に応じた再評価をトリガーする。

3. **イベント transport は署名付き SET (RFC 8417)・push 配送・署名検証必須とする**。
   イベントは Security Event Token として表現し、署名は ra-idp の JWT 署名鍵管理
   ([[ADR-003]] / [[ADR-009]]) を再利用して付与する。配送は push-based (receiver の
   endpoint へ HTTP POST) を既定とする。**受信側は署名検証を通ったイベントのみ反映し、
   検証に失敗 / 鍵不明 / 改竄を検知した場合は反映せず拒否・監査する (fail-closed)**。

4. **kill-switch と所有者オフボードをイベント化し、下流へ伝播する**。[[ADR-048]] の
   `kill` (一方向の緊急停止) と所有者オフボードは、当該エージェントの `session-revoked`
   イベントを生成する。ra-idp はこれを起点に、エージェントの既発行トークン・セッション、
   および token exchange で派生した下流トークン・委譲チェーン ([[ADR-049]] の `act`
   チェーン) へ失効を波及させる。所有者のオフボードは配下エージェント群を一括失効する。

5. **受信した検証済みイベントはローカル失効として反映する**。inbound の SET を検証後、
   ra-idp は対象セッション・トークン・refresh family ([[ADR-004]]) をローカルに失効させる。
   受信から反映までを近リアルタイムに行い、反映結果を監査に残す。外部発のシグナルでも
   自前で起こした失効と同じ失効経路へ収束させる。

6. **既存の失効プリミティブと役割分担し、短命 TTL + introspection へバイアスする**。
   Token Revocation (RFC 7009) と refresh-family 失効 ([[ADR-004]]) は当該 IdP 内の
   ローカル失効を担い、CAEP/SSF はそれを**生態系へ伝播する層**を担う。JWT access token は
   満了まで有効という失効性の弱点 ([[ADR-012]]) があるため、エージェント発行トークンは
   **短い TTL を既定とし、resource server には introspection (リアルタイム失効状態の確認) を
   推奨する**。TTL 短縮だけに頼らず、TTL とイベント駆動失効を組み合わせて失効を強制可能にする。

7. **管理操作・監査・署名鍵を既存資産に載せる**。SSF stream の管理 (登録・更新・削除・
   購読) は新規 permission `AdminSharedSignalsManage` で保護し、判定は [[ADR-010]] の
   `authorize()` 経由とする。`SsfStreamConfigured` / `SecurityEventTransmitted` /
   `SecurityEventReceived` / `AgentAccessRevoked` を [[ADR-018]] の監査経路へ emit する。
   SET の署名・検証鍵は新設せず [[ADR-009]] の鍵ローテーションと JWKS 配布に相乗りする。

## 影響

- 新規 model `SecurityEventToken` / `SsfStream` / `CaepEvent` / `SsfReceiverConfig` /
  `SsfTransmitterConfig` と、SSF transmitter (push) / receiver の HTTP endpoint・stream
  管理 API が加わる。
- kill-switch ([[ADR-048]]) と所有者オフボードの経路に、配下トークン・委譲チェーン
  ([[ADR-049]]) への失効伝播が接続される。`AgentAccessRevoked` がその痕跡を残す。
- エージェント発行トークンは短命 TTL を既定とし、resource server 向けに introspection
  併用を推奨する ([[ADR-012]] の方針をエージェント向けに強める)。JWT 消費側は無変更でも
  失効性が introspection で補える。
- SET 署名は [[ADR-003]] / [[ADR-009]] の鍵を再利用するため、鍵ローテーション時は SSF の
  検証側 (JWKS) も追随する。鍵管理を二重化しない。
- 監査 ([[ADR-018]]) にイベント送受信と失効反映が現れ、ガバナンス ([[wi-59]]) が失効の
  網羅性と伝播経路を読める。

## 却下した代替案

- **polling introspection のみで失効を反映する**: resource server が定期的に `/introspect`
  を叩いて失効を検知する方式は実装が単純だが、長時間稼働するエージェントに対して polling
  間隔ぶんの遅延が残り、近リアルタイム失効にならない。kill-switch ([[ADR-048]]) の即時性が
  損なわれる。push-based の event 配送を主とし、introspection は補助に留める。

- **SSF を一方向 (transmitter だけ / receiver だけ) に限定する**: 実装は半分で済むが、
  生態系の中で「失効を配るだけ」または「受け取るだけ」になり、双方向の継続評価が成立しない。
  外部発のリスクシグナルを取り込めない、または自前の kill-switch を他者へ伝えられない。
  transmitter と receiver の双方として動作する。

- **署名なしのイベントを受理する**: SET に署名を課さず受理すると、偽造イベントによる
  任意セッションの失効 (DoS) や、なりすましによる誤失効を許す。完全性が崩れる。SET は
  署名必須とし、検証を通ったイベントのみ反映する (fail-closed)。

- **短命 TTL だけに頼り、イベント駆動失効を実装しない**: TTL を十分短くすれば失効ウィンドウは
  縮むという考えもあるが、満了までの窓は必ず残り、その間 kill-switch 済みエージェントが
  動き続ける。TTL 短縮と CAEP/SSF のイベント駆動失効を組み合わせ、窓をイベントで閉じる。

- **独自のイベント形式 / transport を定義する**: RFC 8417 (SET) と SSF/CAEP を使わず自前の
  失効通知を作ると、外部 IdP / resource server / ガバナンス基盤との相互運用が崩れる
  ([[ADR-049]] が RFC 8693 の `act` を使う方針と同じく)。標準の SET / SSF / CAEP に従う。
