# ADR-065: Microsoft Entra domain federation profile を確定する

## ステータス

採用 (accepted)。[[wi-64-entra-domain-federation-m365-sso]] のローカル実装スライスの意思決定。
[[ADR-064]] の `WsFederation` context、[[ADR-059]] の宣言的 claim release、[[ADR-062]] の
federation metadata 公開を前提に、Microsoft Entra ID / Microsoft 365 へ登録する WS-* profile を
確定する。

## コンテキスト

Microsoft Entra domain federation は、ブラウザ sign-in では WS-Federation passive、
rich client / legacy active authentication では WS-Trust active STS を使う。Entra 側には
`IssuerUri`、`PassiveLogOnUri`、`ActiveLogOnUri`、`MetadataExchangeUri`、署名証明書を登録し、
発行 assertion には UPN と ImmutableID が必要になる。

ImmutableID はオンプレ AD の `objectGUID` など不変の sourceAnchor から作る。AD FS / Entra の
慣行では GUID 文字列を .NET `Guid.ToByteArray()` の byte order で base64 化した値を使う。
ここを誤ると同一ユーザーとして結合されず、アカウント重複やサインイン失敗につながる。

Hybrid Azure AD Join の device registration は WS-Trust `windowstransport` と
コンピュータアカウント Kerberos を要求する。cloud STS だけでこれを擬似実装すると、セキュリティ境界と
相互運用性の両方を壊す。

## 決定

1. **Entra profile は `WsFederation` の RP preset として扱う。**
   `EntraFederationProfile` は domain、IssuerUri、sourceAnchor 属性、passive / active / MEX endpoint を
   持つ。設定時に同じ IssuerUri を wtrealm / audience とする `WsFedRelyingParty` を upsert する。

2. **required claims は preset で固定し fail-closed にする。**
   UPN は `http://schemas.xmlsoap.org/claims/UPN` として `preferred_username` から発行する。
   ImmutableID は sourceAnchor を `entra_immutable_id` へ正規化し、persistent NameID と
   `http://schemas.xmlsoap.org/claims/nameidentifier` の両方に載せる。

3. **sourceAnchor は設定時と発行時の両方で検証する。**
   設定時は既存 user の sourceAnchor 欠落・重複・変換不能を拒否する。発行時も、対象 user から
   ImmutableID を作れない場合は claim issuance 前に拒否する。既存値が GUID なら Microsoft byte order
   で base64 化し、既に base64 の値ならそのまま使う。

4. **SAML 1.1 を Entra profile の既定 token type にする。**
   Entra / AD FS 互換の WS-Fed 既定に合わせ、profile が作る RP は SAML 1.1 assertion を発行する。

5. **Hybrid Azure AD Join device registration は未提供として明示する。**
   `windowstransport` + コンピュータアカウント Kerberos は本 WI の範囲外。設定 API / UI は
   managed/PHS または AD FS 併存を回避策として案内する。

## 影響

- Entra 向け RP を手作業の claim JSON ではなく preset として登録できる。
- UPN / ImmutableID / NameID の claim 形状が SCL とテストで追跡可能になる。
- 実テナントの domain federation 切替は破壊的なので、ローカル検証とは別に検証用テナントで行う。
- 複数 verified domain は IssuerUri を domain ごとに分けることで同一 tenant 内に共存できる。

## 却下した代替案

- **通常の WS-Fed RP 画面で claim JSON を手入力する。** 誤設定時の失敗が Entra 側で不透明になり、
  sourceAnchor の安定性・一意性を設定時に保証できない。
- **OAuth2/OIDC client として扱う。** Microsoft 365 domain federation は WS-* の trust contract であり、
  OAuth2/OIDC client metadata とは登録値も token 形状も異なる。
- **Hybrid Join device registration を擬似対応する。** コンピュータアカウント Kerberos を伴わない
  windowstransport 互換は、Okta 同等の cloud STS の境界を越える。
