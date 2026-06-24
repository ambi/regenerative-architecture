# ADR-059: Federation bounded context と宣言的 claim 発行エンジンを確定する

## ステータス

採用 (accepted)。[[wi-63-federation-metadata-and-claims-mapping]] の第 1 スライス
(claim 発行エンジン) を実装する意思決定。WS-Federation passive
([[wi-61-ws-federation-passive-requestor-idp]])・WS-Trust active
([[wi-62-ws-trust-active-sts]])・Entra domain federation
([[wi-64-entra-domain-federation-m365-sso]]) と、将来の SAML 2.0 IdP
([[wi-29-saml2-idp]]) が共有する claim 組み立ての土台を、XML 署名や個別プロトコルに
先行して確定する。

## コンテキスト

WS-Federation / WS-Trust / SAML はいずれも「内部の identity 属性から、相手 (RP) 向けの
token に載せる claim を組み立てる」工程を共有する。AD FS は relying party trust ごとの
claim issuance rule で `UPN` や `nameidentifier` を発行し、PingFederate は attribute
contract と token generator で、Okta は Office 365 連携の claim mapping で同等を提供する。

ra-idp-go の既存 bounded context (Tenancy / IdentityManagement / Authentication /
OAuth2) はどれもこの federation protocol 群を所有するのに適さない。OAuth2 は OIDC/OAuth に
閉じており、WS-* / SAML の RP trust・metadata・assertion を混ぜると責務が肥大化する。

claim 発行は XML 署名やトランスポートから独立した純粋な変換である。これを先に切り出すと、
XML 署名ライブラリの選定 (重い意思決定) を待たずに、最小権限・属性最小化・fail-closed という
保証を単体テストで確立できる。

## 決定

1. **新規 bounded context `Federation` を導入する。** WS-Federation / WS-Trust、federation
   metadata、RP/SP trust、assertion 組み立て、claim 発行を所有する。Tenancy (realm 境界)・
   IdentityManagement (claim の source となる identity 属性) に依存する。SAML 2.0 IdP も将来
   ここに同居させる。

2. **claim 発行は宣言的 mapping とする。** AD FS の claim rule language は採らない。
   `ClaimMappingRule` は「出力 claim 型 (URI) ← source (user 属性 / 固定値 / NameID)」の
   宣言で表し、`ClaimMappingPolicy` が RP ごとの規則集合と `NameIdConfiguration` を束ねる。

3. **claim 発行エンジンは protocol-agnostic かつ fail-closed とする。** 入力は解決済みの
   属性マップ (identity 集約から切り離す) と policy、出力は `IssuedClaim[]` (型 + 値群)。
   mapping で明示した claim だけを出力し、未マップ属性は決して漏らさない。必須 source が
   欠けた required rule は発行を拒否する。WS-Fed / WS-Trust / SAML が同じエンジンを再利用する。

4. **XML 署名ライブラリ・metadata 署名・assertion 直列化は本 ADR の範囲外。** wi-61 着手時の
   後続 ADR で「実績ある XML-dsig ライブラリを使い自前実装しない」前提のもと選定する。本スライスは
   claim という構造化中間表現までを確定する。

## 影響

- 属性最小化と同意整合が、プロトコル実装より前に単体テストで検証できる。
- WS-Fed / WS-Trust / SAML の各 WI は claim 組み立てを再実装せず、本エンジンに委譲する。
- `Federation` が OAuth2 と並ぶ第 5 の protocol context となり、SAML / WS-* の今後の拡張点になる。

## 却下した代替案

- **OAuth2 context に同居させる。** 責務が肥大化し、OIDC と WS-*/SAML の語彙が混線する。
- **AD FS claim rule language 互換。** 表現力は高いが実装・検証コストが大きく、宣言的 mapping で
  Entra/M365 連携の要件 (UPN / ImmutableID 等) は満たせる。
- **claim 発行を各プロトコルに内包。** WS-Fed / WS-Trust / SAML で三重実装になり、fail-closed の
  保証点が分散する。
