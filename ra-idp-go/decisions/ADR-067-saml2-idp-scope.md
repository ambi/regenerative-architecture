# ADR-067: SAML 2.0 IdP の対応範囲を確定する

## ステータス

採用 (accepted)。[[wi-29-saml2-idp]] の意思決定で、ra-idp-go を SAML 2.0 IdP として
振る舞わせる初期スコープを定める。[[ADR-059]] (claim 発行) と
[[ADR-060]] (XML 署名と SAML assertion 署名) を前提に、[[ADR-064]] が分離した
`Saml` bounded context にブラウザ経路と SP 管理を実装する。

## コンテキスト

OIDC だけでは B2B / enterprise の最低ラインを満たせない取引が多い。Okta / Entra ID /
Keycloak は SAML 2.0 IdP として SP-initiated / IdP-initiated SSO、metadata 公開、
assertion 署名、attribute mapping、Single Logout を提供する。

ra-idp-go には既に WS-Federation passive ([[wi-61-ws-federation-passive-requestor-idp]]) と
WS-Trust active ([[wi-62-ws-trust-active-sts]]) があり、claim 発行エンジン ([[ADR-059]]) と
署名済み SAML assertion アダプタ ([[ADR-060]]) を備える。SAML 2.0 IdP は新しい token 形式や
claim engine を必要とせず、これらを SAML 2.0 Web Browser SSO Profile のワイヤ形式に
包み直す層を足せばよい。XML 署名は誤りやすく署名ラッピング攻撃の温床なので、自前実装しない
方針 ([[ADR-060]]) を引き継ぐ。

## 決定

1. **初期スコープは SAML 2.0 Web Browser SSO Profile に限る。** HTTP-Redirect (deflate+base64) /
   HTTP-POST (base64) binding、署名済み Response / Assertion、metadata 公開、
   SP-initiated / IdP-initiated SSO、Single Logout を対象とする。SAML ECP、encrypted
   assertion、SAML を SP として外部 IdP に繋ぐ inbound federation は対象外とし、必要なら
   別 WI とする。

2. **claim 発行・assertion 署名は WS-Fed / WS-Trust と共有する protocol-agnostic な基盤を再利用する**
   ([[ADR-059]] / [[ADR-060]])。`internal/wsfederation/adapters/samltoken` の
   assertion ビルダ・署名器は SAML version・bearer subject confirmation・audience restriction を
   既に扱える。SAML 2.0 IdP のために再実装せず、`InResponseTo` の往復のような SP-initiated 固有の
   入力だけを足す。

3. **`Saml` bounded context にブラウザ経路と SP 管理を所有させる** ([[ADR-064]])。
   `internal/saml` に AuthnRequest の復号・解析・検証 (domain)、SAMLResponse / IdP metadata
   アダプタ、`/saml/metadata`・`/saml/sso`・`/saml/slo` の HTTP ハンドラ、SP リポジトリを置く。
   SP 登録は Application Catalog ([[ADR-066]]) の SAML binding として管理 UI から作成・編集する。

4. **fail-closed の相互運用ガードを domain に集約する。** Issuer は登録 SP の entityID と完全一致、
   AssertionConsumerServiceURL は SP ごとの許可集合に対して検証 (open redirect 防止)、
   audience restriction は SP の entityID / Audience に限定する。判定不能・不一致はすべて拒否側へ倒す。

5. **署名は Assertion 署名を既定とし、Response 署名を任意で足す** (Okta / Entra の "Sign Response")。
   `goxmldsig` は enveloped 署名を要素末尾に付与する。enveloped transform は署名位置に依存せず
   検証されるため、署名要素は付与位置のまま残す。署名後に要素を再配置すると名前空間の再描画で
   ダイジェストが変わり検証不能になるため、再配置はしない (assertion 署名と同じ扱い)。

6. **署名 identity は federation 署名証明書 (X.509 + RSA) を流用する** ([[ADR-060]])。IdP metadata は
   この証明書を KeyDescriptor として広告する。鍵ローテーション・per-tenant 鍵・metadata 署名は
   後続スライス ([[wi-32-kms-hsm-and-per-tenant-signing-keys]] / [[wi-23-signing-key-rotation-scheduler]])
   に委ねる。

## 影響

- 署名安全性・audience restriction・open redirect 防止・tenant isolation を、HTTP 経路の前に
  domain / adapter の round-trip 検証で確立できる。
- SP 登録・属性マッピング・NameID format・署名方針は Application Catalog の SAML binding として
  OIDC client / WS-Fed RP と同じ tenant boundary・割当・監査に乗る。
- SAML ECP・encrypted assertion・inbound SAML federation を初期から切り離し、スコープを保てる。

## 却下した代替案

- **フル SAML フレームワーク (crewjam/saml 等) の採用。** IdP/SP の構造と独自の trust / metadata
  モデルを丸ごと持ち込み、既存の claim engine ([[ADR-059]]) と署名アダプタ ([[ADR-060]]) を
  迂回する。署名は goxmldsig、構造は etree、claim は既存エンジンで足り、横断する自由度も保てる。
- **encrypted assertion を初期必須にする。** 鍵交換と SP 側鍵管理の重い関心事を初期に持ち込む。
  署名済み assertion + TLS で初期の機密性は満たせるため、必要時に別 WI とする。
- **SAML 専用の署名鍵・証明書を新設する。** WS-Fed / WS-Trust と同じ federation 署名証明書で
  足り、鍵の用途境界も同じ。証明書ライフサイクルは横断スライスで一括して扱う。
