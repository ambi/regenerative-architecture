# ADR-062: Federation metadata 公開と claim mapping の所有境界を確定する

## ステータス

採用 (accepted)。[[wi-63-federation-metadata-and-claims-mapping]] の metadata 公開スライスの意思決定。
[[ADR-059]] の claim 発行エンジンと [[ADR-060]] の federation 署名証明書を前提に、WS-Federation
passive と WS-Trust active が共有する AD FS 互換 metadata の公開面を確定する。

## コンテキスト

WS-Federation / WS-Trust の RP や Microsoft Entra domain federation は、IdP の issuer、
passive / active endpoint、MEX endpoint、署名証明書を federation metadata から取得して信頼を
確立する。claim mapping は RP trust ごとの設定であり、metadata は realm 単位の公開情報である。

ra-idp-go では WS-Fed relying party が `ClaimMappingPolicy` を持ち、token 発行時は
claim 発行エンジンに委譲する。metadata 公開はその管理面とは別に、現在存在する endpoint と
federation 署名証明書を広告する派生物として扱う。

## 決定

1. **AD FS 互換の `federationmetadata.xml` を realm 配下で公開する。**
   URL は `/{realm}/federationmetadata/2007-06/federationmetadata.xml` とし、default tenant でも
   tenant issuer (`/realms/default`) を entityID として広告する。

2. **metadata は WsFederation context の wire adapter が生成する。**
   `EntityDescriptor` に `SecurityTokenServiceType` と `ApplicationServiceType` の
   `RoleDescriptor` を含め、`PassiveRequestorEndpoint`、`SecurityTokenServiceEndpoint`、
   `MetadataEndpoint`、署名用 `KeyDescriptor` を広告する。

3. **署名証明書は WS-* 用の X.509 署名証明書を広告する。**
   OAuth/OIDC の JWK 形式は metadata へ流用しない。鍵用途・ローテーション・公開重複期間は
   `SigningKeys` の責務として扱い、metadata には WS-* token 署名検証用の X.509 証明書を載せる
   ([[ADR-064]])。

4. **MEX は WS-Trust active endpoint の discovery として公開する。**
   `/{realm}/trust/mex` は `usernamemixed` endpoint と UsernameToken 必須の policy を広告する。
   WS-Trust の RST/RSTR 本体は [[wi-62-ws-trust-active-sts]] が所有する。

5. **claim mapping は宣言的 policy を継続採用する。**
   AD FS claim rule language は採らず、`ClaimMappingPolicy` を WS-Fed / WS-Trust / 将来 SAML で
   共有する。未マップ属性は出力しない。

## 影響

- RP / Entra は realm ごとの metadata URL から issuer、endpoint、署名証明書を取得できる。
- WS-Fed / WS-Trust の token 発行は、claim mapping と metadata 生成を再実装しない。
- 文書署名と複数証明書掲載は未完了であり、鍵ローテーション WI で見直す。

## 却下した代替案

- **OAuth discovery に WS-* metadata を混ぜる。** OIDC と WS-* の trust metadata は形式も消費者も異なる。
- **AD FS claim rule language 互換。** 表現力に対して検証コストが高く、Entra/M365 に必要な claim は宣言的 mapping で満たせる。
- **OAuth JWK を metadata に載せる。** WS-* consumer は X.509 証明書を期待し、鍵用途の境界も異なる。
