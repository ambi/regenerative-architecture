# ADR-056: Identity Assertion Authorization Grant による企業管理型 Cross-App Access を仲介する

## ステータス

提案 (draft)。[[wi-57-cross-app-access-identity-assertion-grant]] の意思決定を先行して起草する。
wi-57 の実装着手とともに「採用」へ移す。[[ADR-049]] (Token Exchange による委譲・代行)・
[[ADR-007]] (コンセントモデル)・[[ADR-010]] (AuthZEN ポリシー)・[[ADR-008]] (client 認証方式)・
[[ADR-055]] (MCP authorization server) を前提に、企業が一元統制する **app-to-app /
agent-to-MCP アクセスの仲介モデルと安全境界**を確定する。本 ADR は Token Exchange 基盤
([[wi-50-token-exchange-delegation-actor-chain]]) と MCP 認可サーバー
([[wi-56-mcp-authorization-server]]) の上に積む。

## コンテキスト

企業内のエージェントやアプリ (app A / MCP クライアント) が、別アプリや MCP サーバー (app B) の
データへアクセスする要求が一般化している。従来この連携は app B ごとの opaque な OAuth 同意画面
([[ADR-007]]) を通すため、企業から見て「どの app が、どの app に、何のアクセスを得ているか」が
不可視で、中央からの統制・棚卸し・一括失効ができない。エージェントの数だけ同意関係が増えると、
横方向の権限拡大が統制不能になる。

これを IdP が仲介し、信頼する identity assertion を起点に企業が一元的に可視化・統制する標準が
**Identity Assertion Authorization Grant** (IETF `draft-ietf-oauth-identity-assertion-authz-grant`、
WG-adopted、authors: Parecki/Okta、McGuinness、Campbell/Ping) である。Okta はこれを
**Cross-App Access (XAA)** と呼ぶ。IdP が信頼する identity assertion (`id_token` 等) を
Token Exchange ([[ADR-049]]) で app B 向けの audience 限定アクセストークンに交換し、per-app の
再同意を排しつつ仲介する。本仕様は 2026-06-18 に MCP の **"Enterprise-Managed Authorization"**
拡張として取り込まれ、エージェント連携の企業向け認可の中心になりつつある。

ra-idp は既に Token Exchange grant ([[ADR-049]] / [[wi-50]])、Resource Indicators (RFC 8707) による
audience 限定、AuthZEN ポリシー ([[ADR-010]])、MCP 認可サーバー ([[ADR-055]] / [[wi-56]]) の素地を
持つ。本 ADR はこれらを再利用し、identity assertion を起点にした企業管理型のブローカ付与を、
許可ポリシーに合致する場合のみ成功させる **fail-closed の保証義務**として確定する。本仕様は
draft 段階のため、対象改訂を本 ADR で固定し互換性変化に追従する必要がある。

## 決定

1. **Identity Assertion Authorization Grant を支持し、対象 draft 改訂を本 ADR で固定する**。
   対象は `draft-ietf-oauth-identity-assertion-authz-grant` の revision **-04** とする。draft 段階の
   仕様であり、後続改訂の互換性変化 (claim 名・assertion 形式・grant パラメータ) は本 ADR の改訂で
   追従する。pin した改訂以外への暗黙の追従はしない (fail-closed)。

2. **信頼する identity assertion を Token Exchange 経由で受理し、厳格に検証する**。app A が提示する
   identity assertion (`id_token` / `urn:ietf:params:oauth:token-type:jwt`) を、[[ADR-049]] の
   token-exchange grant の `subject_token` として受理する。受理にあたり **issuer / audience /
   有効期限 (expiry) / 署名**を厳格に検証し、信頼する issuer の登録に合致しない assertion は拒否する。
   検証に一つでも不備があれば交換しない側へ倒す (fail-closed)。

3. **app-to-app 許可ポリシーを中央で登録・管理し、許可された交換のみ成功させる**。「どの client /
   app が、どの resource / app へのアクセスを取得してよいか」を表す `CrossAppAccessPolicy` を
   企業管理者が中央登録する。token-exchange 要求は、要求元・対象 resource・assertion の主体が
   登録済みポリシーに合致するときのみ成功する。[[ADR-010]] の `authorize()` を経路に使い、ポリシー
   未登録・不一致の交換は既定で拒否する (fail-closed)。client の正当性は [[ADR-008]] の client 認証方式で
   確かめる。

4. **MCP "Enterprise-Managed Authorization" にマッピングする**。本 grant を MCP 認可サーバー
   ([[ADR-055]] / [[wi-56-mcp-authorization-server]]) の Enterprise-Managed Authorization 拡張へ対応づけ、
   agent-to-MCP のアクセスを同一のブローカ付与経路で扱う。MCP クライアントが提示する identity
   assertion を本 grant で MCP サーバー向けトークンに交換し、MCP AS の work と合成する。

5. **交換後トークンの audience を対象 app / resource に限定する**。Resource Indicators (RFC 8707、
   [[ADR-049]]) を再利用し、結果トークンの `aud` を許可ポリシーが指す単一の app / resource に限定する。
   ある app 向けに発行したトークンを別 app へ流用できないようにし、横展開を断つ。要求 resource が
   ポリシーの許す対象でなければ交換しない (fail-closed)。

6. **管理者が付与・一覧・取消でき、取消は後続アクセスを断つ**。企業管理者は app 間アクセスの付与
   (`AppAccessGrant`)・一覧・取消を行える。取消は以後の token-exchange 交換を拒否させ、後続の
   アクセスを断つ (既発行の短命トークンは [[ADR-049]] の短命 + 再交換方針により TTL 内で自然失効し、
   再交換は通らない)。新規 permission **`AdminCrossAppAccessManage`** で管理操作を保護し、判定は
   [[ADR-010]] 経由とする。付与・拒否・ポリシー変更は監査イベント (`CrossAppAccessGranted` /
   `CrossAppAccessRejected` / `AppAccessPolicyChanged`) に残す。

7. **per-user コンセント ([[ADR-007]]) との境界とガバナンスのトレードオフを明示する**。本 grant は
   **企業管理型 (enterprise-managed)** であり、対象範囲内の app-to-app アクセスについては app B ごとの
   per-user 再同意 ([[ADR-007]]) を **排する**。これは企業が一元的に統制・可視化・失効する代わりに、
   個々のユーザーへの同意画面提示を省く設計判断である。境界は次のとおり: 本 grant は中央登録された
   `CrossAppAccessPolicy` の範囲内に限り per-app 再同意を不要にする。ポリシー外のアクセス、および
   エンドユーザー個別同意フローは引き続き [[ADR-007]] の subject × client × scope 単位のコンセントに従う。
   トレードオフとして、同意の主体が個々のユーザーから企業管理者へ移る。これは企業が信頼境界を
   所有する前提で正当化され、ポリシー登録・付与・取消の責務を管理者に集中させることで成立する。

## 影響

- `/token` の token-exchange 経路 ([[ADR-049]]) に identity assertion 系 `subject_token_type` の受理が
  加わり、issuer / audience / expiry / 署名の検証ゲートが入る。検証は fail-closed で、不備は拒否する。
- 新規モデル `IdentityAssertionGrantRequest` / `AppAccessGrant` / `CrossAppAccessPolicy` を追加し、
  Postgres にポリシー・付与の永続化テーブルを置く。token-exchange と Resource Indicators 基盤を再利用する。
- AuthZEN ポリシー ([[ADR-010]]) に app-to-app 交換可否ルールが追加され、網羅性テストでルール実装漏れを
  検知する。新規 permission `AdminCrossAppAccessManage` が RBAC に加わる。
- MCP 認可サーバー ([[ADR-055]] / [[wi-56]]) と合成し、agent-to-MCP アクセスを同一経路で扱う。
- 管理 UI / API に app 間アクセス許可の付与・一覧・取消 (企業管理者ビュー) が加わる。
- 監査経路 ([[ADR-018]] 系) に `CrossAppAccessGranted` / `CrossAppAccessRejected` /
  `AppAccessPolicyChanged` が現れ、ガバナンス ([[wi-59]]) が付与・拒否・ポリシー変更を読める。
- per-user コンセント ([[ADR-007]]) の適用範囲が、企業管理型ポリシーの範囲だけ縮小する。境界は
  ポリシー登録の有無で決まり、ポリシー外は従来どおり [[ADR-007]] に従う。

## 却下した代替案

- **app ごとの per-app OAuth 同意画面を維持する (status quo)**: 実装は追加不要だが、各 app B への
  アクセスが企業から opaque なままで、中央の統制・可視化・一括失効ができない。エージェント数に比例して
  同意関係が増え、横方向の権限拡大が統制不能になる。これは本 ADR が解こうとしている問題そのものであり、
  企業管理型のブローカ付与で置き換える。

- **仲介を外部 IdP (Okta 等) のみに依存する**: 外部 IdP に XAA を委ねれば自前実装は不要だが、ra-idp が
  自身の信頼境界内で broker として振る舞えず、identity assertion の検証・許可ポリシー・失効の主導権を
  外部に預けることになる。ra-idp はブローカたりうるべきであり、本 grant を自前で実装する。外部 IdP との
  相互運用は別途 ([[wi-57]] の out_of_scope) とする。

- **中央の app-to-app 許可ポリシーを持たない (交換を一律許可)**: ポリシー登録なしに identity assertion の
  検証だけで交換を通すと、検証に通った任意の app が任意の resource へ横移動でき、統制不能な lateral
  access を招く。中央登録された `CrossAppAccessPolicy` に合致する交換のみ成功させ、未登録・不一致は
  拒否する (fail-closed)。

- **独自の app 間付与 claim / 交換プロトコルを定義する**: 標準の Identity Assertion Authorization Grant に
  従わず自前仕様にすると、MCP "Enterprise-Managed Authorization" や外部 resource server / IdP との
  相互運用が崩れる。draft の対象改訂を pin した上で標準 grant に従う。
