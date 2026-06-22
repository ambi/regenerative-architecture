# ADR-055: Model Context Protocol (MCP) の認可サーバーとして振る舞う

## ステータス

提案 (draft)。[[wi-56-mcp-authorization-server]] の意思決定を起草する。wi-56 の実装着手とともに
「採用」へ移す。[[ADR-002]] (全 client で PKCE 必須)・[[ADR-005]] (DPoP を既定の sender constraint)・
[[ADR-011]] (Discovery を導出成果物として扱う)・[[ADR-012]] (opaque / JWT access token)・
[[ADR-049]] (Token Exchange と Resource Indicators による audience 限定) を前提に、ra-idp が
MCP リソースサーバー群に対する **Authorization Server** として振る舞うときの境界と保証義務を確定する。

## コンテキスト

Model Context Protocol (MCP) は AI エージェントが外部ツール / データソース (MCP サーバー) へ
接続する事実上の標準になった。MCP の認可仕様は OAuth 2.1 を基盤とし、リモート MCP サーバーは
OAuth Resource Server として扱われる。2025-11 改訂以降、インターネット公開の MCP サーバーには
次が課される。

1. **OAuth 2.1 + PKCE(S256) を必須**とする。
2. **Protected Resource Metadata (RFC 9728)** を配信し、対応する認可サーバーを広告する。
3. MCP クライアントは **Resource Indicators (RFC 8707)** でトークンの audience をリソースに束縛する。
4. discovery は **Authorization Server Metadata (RFC 8414)** と **Dynamic Client Registration
   (RFC 7591)** で行い、クライアントが認可サーバーを自動発見し動的登録する。

ra-idp は既に OAuth 2.1 相当 (PKCE 必須 [[ADR-002]]・DPoP [[ADR-005]]・PAR)、Discovery
([[ADR-011]])、JWKS、DCR (RFC 7591) を備えており、MCP の認可サーバーになる素地が揃っている。
ここで重要な危険は **audience の取り違えと再利用**である。あるツール向けに発行したトークンが
別の MCP リソースで受理されると、エージェント / ツール間で権限が横展開する。MCP は
無数のツールとエージェントが動的に増減するエコシステムであり、ad-hoc な API キーや手動の
client 登録ではスケールしない。本 ADR は ra-idp を MCP の Authorization Server と位置づけ、
audience 限定を fail-closed の保証義務として確定する。

なお MCP 認可仕様は版差が大きいため、本 ADR は**対象改訂を固定**して準拠範囲を明示する。

## 決定

1. **MCP authorization 仕様に準拠し、対象改訂を本 ADR で固定する**。準拠対象は MCP authorization
   仕様の **2025-11-25 改訂** (OAuth 2.1 基盤) とする。仕様は版差が大きいため、改訂が進む場合は
   本 ADR を更新し新たな対象改訂を pin し直す。実装は対象改訂が要求する OAuth 2.1 + PKCE(S256) /
   RFC 9728 / RFC 8707 / RFC 8414 / RFC 7591 のサブセットを満たす。

2. **Protected Resource Metadata (RFC 9728) を配信し、MCP リソースサーバー登録モデルを設ける**。
   `/.well-known/oauth-protected-resource` で、各 MCP リソースの `resource` 識別子・対応する
   authorization server (= ra-idp issuer)・サポート scope を広告する。配信内容は [[ADR-011]] と
   同じ方針で **MCP リソースサーバー登録 (`McpResourceServer`) から導出**し、metadata を手書きで
   独立に保守しない。リソースサーバーの登録・参照・更新・削除は tenant-scoped の管理モデルとする。

3. **Resource Indicators (RFC 8707) による audience 限定を必須とし、fail-closed で強制する**。
   MCP 向けトークン要求は `resource` を必須とし、結果トークンの `aud` を **1 つの MCP リソースに
   限定**する。**1 トークン = 1 MCP リソース**を不変条件とし、要求された resource と異なる
   リソースでトークンが提示された場合は受理しない ([[ADR-049]] の audience 限定方針と一致)。
   resource 未指定・不一致・複数 resource への拡大は、いずれも**「発行しない / 受理しない」側へ
   倒す**。これは MCP ツール間のトークン再利用 (権限越境) を遮断する中核保証である。

4. **MCP 文脈でも PKCE(S256) と DPoP を強制する**。MCP クライアントの authorization code flow では
   PKCE(S256) を必須とする ([[ADR-002]])。sender constraint は DPoP を既定とし ([[ADR-005]])、
   発行トークンを要求元の鍵に `cnf` で束縛する。所有証明を MCP 文脈で外さない。

5. **既存の AS メタデータ (RFC 8414) / Discovery と DCR (RFC 7591) を MCP クライアントの
   自動オンボーディングに再利用する**。RFC 9728 が指す authorization server を ra-idp の
   既存 Discovery ([[ADR-011]]) で発見させ、MCP クライアントは既存 DCR で動的登録する。MCP 専用の
   別経路は設けず、既存資産へ接続する。これにより無数の MCP クライアントが手動登録なしに接続できる。

6. **新規 permission `AdminMcpResourcesManage` を導入する**。MCP リソースサーバーの登録・更新・
   削除・参照は本 permission で保護し、判定は [[ADR-010]] の `authorize()` 経由とする。

トークン形態は [[ADR-012]] の opaque / JWT access token の方針に従い、MCP リソースの resource
identifier を `aud` に厳格に束縛する。委譲・代行を伴う MCP アクセスは
[[wi-50-token-exchange-delegation-actor-chain]] の token exchange を経路に使い、Cross-App Access
([[wi-57-cross-app-access-identity-assertion-grant]]) は本 ADR の audience 限定の上に積む。

## 影響

- 新規エンドポイント `/.well-known/oauth-protected-resource` (RFC 9728) が加わる。配信内容は
  `McpResourceServer` 登録から導出し、[[ADR-011]] と同様に独立編集しない。
- 新規 model `ProtectedResourceMetadata` / `McpResourceServer` / `ResourceIndicator` が追加され、
  `AccessTokenClaims` の `aud` を MCP resource 単位に厳格化する。
- トークン発行・検証経路に **resource 単位の audience ゲート**が入る。要求 resource と異なる
  リソースでの提示は fail-closed で拒否する。
- 新規イベント `ProtectedResourceMetadataServed` / `ResourceScopedTokenIssued` /
  `ResourceAudienceRejected` を既存監査経路 ([[ADR-018]]) へ emit する。
- 既存の DCR / Discovery / DPoP / PKCE をそのまま MCP 文脈へ接続するため、新規暗号要素は増えない。
- 管理 API / UI に MCP リソースサーバー registry (一覧・登録・更新・削除) が `AdminMcpResourcesManage`
  保護で加わる。
- MCP サーバー (ツール側) 実装・MCP transport は対象外で、ra-idp は認可サーバー / metadata 提供側に
  徹する。

## 却下した代替案

- **MCP に API キー / bearer secret を使う (2025 以前の ad-hoc 方式)**: 共有秘密はローテーション・
  失効・最小権限・所有証明を欠き、エージェント / ツール間で容易に漏洩・横展開する。MCP 認可仕様が
  まさにこれを置き換えるために OAuth 2.1 を採った。OAuth 2.1 + PKCE + DPoP に従う。
- **audience 限定を任意 / 緩くする**: resource 束縛を緩めると、あるツール向けトークンが別の
  MCP ツールへ再利用でき、権限が横展開する (token replay)。Resource Indicators (RFC 8707) を
  必須とし 1 トークン 1 リソースを fail-closed で強制する ([[ADR-049]] と一致)。
- **RFC 9728 / 8414 ではなく独自・非標準の metadata を配信する**: MCP クライアントの自動発見
  (RFC 9728 → RFC 8414 → DCR の連鎖) が崩れ、相互運用を失う。標準 metadata に従い [[ADR-011]] の
  方式で導出配信する。
- **DCR を設けず手動 client 登録に留める**: エージェント / ツールが動的に増減する MCP
  エコシステムでは、リソース・クライアントごとの手動登録がスケールしない。既存 DCR (RFC 7591) を
  MCP クライアントの自動オンボーディングに再利用する。
