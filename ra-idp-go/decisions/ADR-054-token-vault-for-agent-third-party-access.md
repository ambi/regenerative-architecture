# ADR-054: エージェントの外部 API 代行のための Token Vault (federated connections) を確定する

## ステータス

提案 (draft)。[[wi-55-token-vault-federated-connections]] の意思決定を先行して起草する。
wi-55 の実装着手とともに「採用」へ移す。[[ADR-048]] (エージェント一級プリンシパル)・
[[ADR-049]] (token exchange による委譲・actor チェーン)・[[ADR-024]] (永続共有署名鍵)・
[[ADR-009]] (鍵ローテーション戦略)・[[ADR-005]] (DPoP を既定の sender constraint)・
[[ADR-018]] (監査) を前提に、エージェントがユーザーを代行して外部 SaaS API を呼ぶときの
**upstream token の保管・更新・仲介・失効の安全境界**を確定する。本 ADR は委譲チェーン
([[wi-50-token-exchange-delegation-actor-chain]]) の上に積み、ログイン用 federation
([[wi-30-inbound-federation-and-identity-broker]]) とは別経路として位置づける。

## コンテキスト

AI エージェントの主要ユースケースは、ユーザーに代わって多数の外部 SaaS API
(Google・GitHub・Slack 等) を呼ぶことである。そのためには各 upstream provider が
発行する third-party token (access / refresh token) が必要になる。これをアプリや
エージェント自身が直接保持すると、次の問題を招く。

1. **漏洩面の拡大**: raw な upstream シークレットがエージェントの実行環境やアプリ DB に
   散らばり、漏洩時の影響範囲が広がる。
2. **集中失効の不在**: ユーザーが連携を切りたくても、各所に散った token を一括で失効
   できず、後続アクセスを確実に断てない。
3. **最小権限の崩れ**: エージェントが provider の full scope token を握り、必要以上の
   権限で外部 API を呼べてしまう。

Auth0 の Token Vault (federated connections) は、ユーザーが各 upstream へ与えた同意と
token を IdP 側で暗号化保管・更新 (refresh)・失効し、エージェントには必要時に最小権限の
アクセスを仲介する。これにより「エージェントが raw シークレットを持たずに外部 API を
代行呼び出しする」が成立する。

ra-idp は inbound federation / identity broker ([[wi-30-inbound-federation-and-identity-broker]])
を持つが、それは**ログイン時**に外部 IdP で認証し FederatedIdentity を結ぶための機構で、
**外部 API 呼び出し用**の upstream token を保管・仲介する機構ではない。両者はトークンの
ライフサイクルと scope の意味論が異なる。また ra-idp は既に永続共有署名鍵 ([[ADR-024]])・
鍵ローテーション ([[ADR-009]]) と KMS / HSM を前提とする鍵管理資産を持つ。本 ADR は、
**鍵管理を新設せず既存資産を流用しつつ**、upstream token の保管・仲介層だけを新設する
境界を確定する。

## 決定

1. **Upstream connection をテナントスコープで定義する**。`FederatedConnection` を新設し、
   provider 識別子・OAuth エンドポイント (authorization / token / revocation)・要求 scope・
   client 資格情報参照を持たせる。connection の定義・参照・操作はすべて tenant-scoped とし
   ([[ADR-048]] の境界に倣う)、cross-tenant な connection 共有は認めない。connection 定義は
   ログイン用 external IdP ([[wi-30-inbound-federation-and-identity-broker]]) とは別集約とする。

2. **Upstream token は既存の鍵管理で暗号化保管する**。`UpstreamToken` (access / refresh token・
   有効期限・scope・所有 user・connection 参照) は、**新たな鍵管理を導入せず**、既存の
   KMS / KeyStore ([[ADR-024]] の永続共有鍵・[[ADR-009]] のローテーション戦略、および
   KMS / HSM work item) を流用して暗号化する。鍵のローテーションは既存戦略 ([[ADR-009]]) に
   従い、復号は KeyStore 経由のみとする。raw token を平文で永続化しない。

3. **refresh とローテーションの責務は vault が負う**。upstream token の期限切れ前 refresh、
   refresh token rotation への追従、失効検知時の再連携誘導は Token Vault の責務とする。
   エージェントやアプリに refresh を委ねない。refresh の成否は監査 ([[ADR-018]]) に
   `UpstreamTokenRefreshed` として記録する。

4. **エージェントへの仲介は「token 返却」を baseline とする**。エージェントへの提供方式は
   (a) upstream token を返却する方式と (b) ra-idp が外部 API 呼び出しを proxy する方式が
   あり得るが、**baseline は token 返却**とする。返却する brokered token は connection scope に
   narrow し、要求元エージェントと委譲チェーン ([[ADR-049]] の `act` / actor チェーン) に
   合致する場合のみ発行する。connection scope・委譲・テナントいずれかの判定に欠けがあれば
   発行しない (**fail-closed**)。要求元が DPoP 束縛されている場合 ([[ADR-005]])、仲介経路でも
   sender constraint を尊重する。

5. **失効の伝播を保証する**。ユーザーまたは admin が connection を解除 / 失効すると、
   保管済み `UpstreamToken` を破棄し、可能なら provider の revocation endpoint を呼び、
   以後の vault token 取得要求を拒否する。解除後にエージェントが後続アクセスを得られない
   ことを境界条件として保証する (fail-closed)。`UpstreamTokenRevoked` を emit する。

6. **ログイン用 federation と明確に区別する**。Token Vault は**外部 API への outbound 呼び出し**の
   ための token 保管・仲介であり、[[wi-30-inbound-federation-and-identity-broker]] の
   **login 時 inbound federation** とは目的・ライフサイクル・scope 意味論が異なる。両者で
   集約・event・interface を分離し、ログイン経路の FederatedIdentity を API token 用に
   流用しない。

7. **認可は新規 permission とセルフサービスで分ける**。connection (provider) の定義・管理は
   新規 permission `AdminFederatedConnectionsManage` で保護し、判定は [[ADR-010]] の
   `authorize()` 経由とする。end-user は account portal から自身の連携済み外部サービスを
   一覧し、連携 / 解除をセルフサービスで行える。

## 影響

- 新規集約 `FederatedConnection` / `UpstreamToken` / `ConnectionGrant` と、エージェント向け
  `VaultTokenRequest` / `VaultTokenResponse` が加わる。いずれも `tenant_id` 必須。
- upstream token の暗号化は既存 KMS / KeyStore ([[ADR-024]] / [[ADR-009]]) を呼ぶのみで、
  新たな鍵管理コンポーネントは追加しない。暗号化サーフェスを二重化しない。
- 新規 event `ConnectionConfigured` / `UpstreamTokenStored` / `UpstreamTokenRefreshed` /
  `UpstreamTokenRevoked` / `VaultTokenIssued` を既存監査・outbox 経路 ([[ADR-018]]) へ emit する。
- 仲介経路は委譲チェーン ([[ADR-049]]) と connection scope による fail-closed 判定を通す。
  判定漏れは「発行しない」側に倒す。
- connection 連携の開始 / コールバック (upstream authorization code フロー) と、エージェント
  向け token 取得エンドポイントが HTTP に加わる。
- 管理 UI に connection (provider) 定義・管理、account portal に連携済みサービスの一覧・
  連携 / 解除が加わる。新規 permission `AdminFederatedConnectionsManage`。
- ra-idp は API gateway 化しない。外部 API 呼び出しそのものはエージェント側に残り、vault は
  token 仲介までに責務を限定する。

## 却下した代替案

- **アプリ / エージェントが raw upstream token を保持する**: 実装は単純だが、シークレットが
  各所へ散り漏洩面が広がる。ユーザーが連携を切っても集中失効ができず、後続アクセスを
  確実に断てない。IdP 側で暗号化保管し、解除を後続アクセスへ伝播する vault に集約する。
- **専用の鍵管理システムを新設する**: vault 用に独自の鍵階層・ローテーションを作ると暗号
  サーフェスが二重化し、運用と監査が分散する。[[ADR-024]] / [[ADR-009]] の既存 KMS /
  KeyStore を流用し、鍵管理を単一資産に保つ。
- **全 upstream 呼び出しを ra-idp で proxy する**: IdP が事実上の API gateway となり、各
  provider 固有 API への追従・スループット・障害境界を抱え込む。scope creep であり、
  baseline は brokered token の返却とし、呼び出しはエージェント側に残す。
- **ログイン用 federation / identity broker 経路を API token に流用する**:
  [[wi-30-inbound-federation-and-identity-broker]] は login 時の認証 federation であり、
  token のライフサイクル (短命 session vs 長命 API token) と scope 意味論 (認証 vs API 権限) が
  異なる。経路を共有すると失効・scope 統制が混線する。集約と経路を分離する。
- **仲介を scope / 委譲で絞らず full-scope token を返す**: エージェントが必要以上の権限を
  握り横展開を招く。connection scope と委譲チェーン ([[ADR-049]]) で narrow し、判定欠落時は
  fail-closed とする。
