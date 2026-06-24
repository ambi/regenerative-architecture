# ADR-060: XML 署名ライブラリと SAML assertion 署名を確定する

## ステータス

採用 (accepted)。[[wi-63-federation-metadata-and-claims-mapping]] の署名スライスの意思決定で、
[[ADR-059]] (Federation bounded context / claim 発行) を前提に、claim 発行エンジンの出力を
署名済み SAML assertion に変換する基盤を確定する。WS-Federation passive
([[wi-61-ws-federation-passive-requestor-idp]]) と WS-Trust active
([[wi-62-ws-trust-active-sts]]) が、この assertion を RSTR / RST レスポンスに包む。

## コンテキスト

WS-Fed / WS-Trust / SAML は署名済み XML token (SAML assertion) を発行する。XML 署名は
canonicalization (C14N)、enveloped transform、参照ダイジェストなど誤りやすい要素を持ち、
署名ラッピング攻撃の温床でもある。これを自前実装するのは禁忌で、[[wi-61-ws-federation-passive-requestor-idp]]
も「実績あるライブラリを使い自前実装しない」と定めている。

ra-idp-go にはまだ XML 署名の依存が無い。OAuth2 の署名鍵 (`SigningKey` / JWK) は JSON token
向けで、X.509 証明書を前提とする SAML/WS-* の署名には直接使えない。

## 決定

1. **XML 署名ライブラリは `github.com/russellhaering/goxmldsig` を採用する。** gosaml2 / dex
   などで実績がある軽量ライブラリで、enveloped 署名と検証に特化する。XML ツリー構築には
   その依存である `github.com/beevik/etree` を用いる。署名・canonicalization・検証は
   すべてライブラリに委ね、自前実装しない。

2. **署名プロファイルは SAML/WS-Fed の相互運用既定に合わせる。** enveloped signature、
   exclusive canonicalization (`http://www.w3.org/2001/10/xml-exc-c14n#`)、RSA-SHA256、
   ダイジェスト SHA-256。ID 参照属性は SAML の `ID` を用いる。

3. **署名 identity は federation 署名証明書 (X.509 + RSA) とし、OAuth の JWK 署名鍵とは
   分離する** ([[wi-32-kms-hsm-and-per-tenant-signing-keys]] / [[ADR-059]] と整合)。本スライスでは
   証明書・鍵は注入で受け取り、証明書のライフサイクル・ローテーション・metadata への複数証明書
   掲載は後続スライス ([[wi-23-signing-key-rotation-scheduler]] と整合) で扱う。

4. **発行 token は SAML 2.0 assertion から始める。** Entra WS-Fed が既定とする SAML 1.1
   assertion は wi-61 で追加する。本スライスは claim 発行エンジン ([[ADR-059]]) の出力を
   bearer subject confirmation・audience restriction・attribute statement を備えた署名済み
   SAML 2.0 assertion に変換するところまで。

5. **配置は Federation context 所有のアダプタ** (`internal/federation/adapters/samltoken`) とする
   ([[ADR-047]] context-owned adapter layout)。XML ワイヤ形式は platform 共通ではなく
   federation の責務に属する。

## 影響

- 署名安全性 (未署名/改竄拒否) を、HTTP 経路を作る前に round-trip 検証で確立できる。
- wi-61 / wi-62 は assertion 構築・署名を再実装せず本アダプタに委譲する。
- 証明書ライフサイクルという重い関心事を本スライスから切り離し、注入境界で先送りできる。

## 却下した代替案

- **フル SAML フレームワーク (crewjam/saml 等) の採用。** IdP/SP の構造を丸ごと持ち込み重く、
  WS-Fed / WS-Trust / SAML 1.1·2.0 を横断する自由度を制約する。署名は goxmldsig、構造は etree で十分。
- **XML 署名の自前実装。** canonicalization・署名ラッピング耐性の誤実装リスクが高く、禁忌。
- **OAuth の JWK 署名鍵を流用。** SAML/WS-* は X.509 証明書を前提とし、鍵の用途・ローテーション
  境界も異なる。証明書を分離する。
