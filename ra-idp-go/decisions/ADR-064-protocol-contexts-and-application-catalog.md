# ADR-064: Protocol bounded contexts と Application Catalog を分離する

## ステータス

採用 (accepted)。[[ADR-059]] の `Federation` bounded context 名と、管理 UI の
「アプリケーション」情報設計を見直す。実装リネームと UI 再編は
[[wi-68-protocol-context-and-application-catalog-realignment]] で段階的に行う。

## コンテキスト

`Federation` という語は広すぎる。OAuth2/OIDC も、SAML も、WS-Federation / WS-Trust も、
いずれも identity federation の一種である。にもかかわらず ra-idp-go には既に `OAuth2`
bounded context が存在し、将来 SAML 2.0 IdP は [[wi-29-saml2-idp]] として独立実装される
見込みである。この状態で WS-Federation / WS-Trust の実装だけを `Federation` と呼ぶと、
文脈名が実際の責務を表さない。

管理 UI でも同じ問題がある。現在の「アプリケーション」は実際には OAuth2/OIDC client の
管理画面であり、WS-Fed RP は別項目「WS-Federation」に置かれている。将来 SAML SP が加わると
さらに別項目になる可能性が高い。しかし運用者から見れば OIDC client / SAML SP / WS-Fed RP は
どれも「接続する業務アプリケーション」であり、表示名、所有者、割当、状態、監査、共通ポリシー
などの設定はプロトコルをまたいで共有される。

## 決定

1. **プロトコル bounded context は protocol family 単位で命名する。**
   既存 `OAuth2` は OAuth2/OIDC を所有する。WS-Federation passive と WS-Trust active は
   `WsFederation` に改名する。SAML 2.0 IdP は将来 `Saml` bounded context として追加する。
   `Federation` という bounded context 名は使わない。

2. **WS-Federation / WS-Trust は同じ `WsFederation` bounded context に置く。**
   Entra / AD FS 互換の federation では passive path が WS-Federation、active path が
   WS-Trust であり、同じ relying party trust、issuer、署名証明書、claim mapping を共有する。
   したがって `WsTrust` を単独 bounded context にせず、`WsFederation` 内の active STS adapter
   として扱う。

3. **WS-* と SAML は近いが、bounded context としては分ける。**
   両者は XML assertion、NameID、AttributeStatement、X.509 署名を共有する一方で、
   trust metadata、protocol state machine、request validation、logout、相互運用対象が異なる。
   WS-* は AD FS / Entra domain federation の passive / active STS として進化し、SAML は
   SP metadata / AuthnRequest / ACS / SLO を中心に進化する。共有部分を切り出し、
   protocol 境界は `WsFederation` と `Saml` に分ける。

4. **Claim release は protocol-neutral capability として `ClaimRelease` に置く。**
   `ClaimMappingPolicy`、NameID 解決、出力許可、属性最小化は WS-Fed / WS-Trust / SAML だけでなく、
   OIDC ID Token / UserInfo の claim 選択にも再利用できる。ただし JSON claim name、
   OIDC scope / consent、SAML AttributeStatement、WS-Fed claim URI への wire projection は
   各 protocol context が所有する。短期的には既存の `ClaimMappingPolicy` 名を残し、
   SCL 上は `ClaimRelease` が所有する。

5. **署名鍵ライフサイクルは `SigningKeys` が所有し、署名 wire adapter は各 protocol が所有する。**
   OAuth2/OIDC の JWK / JWKS と JWT signer、SAML / WS-* の X.509 証明書と XML signer は
   表現形式が違う。しかし tenant isolation、鍵用途、ローテーション、公開重複期間、監査は
   protocol 横断の運用責務である。従って key material のライフサイクルは `SigningKeys` に集約し、
   JWT / XML の署名処理と metadata/discovery への投影は各 protocol context の adapter に置く。

6. **管理 UI の「アプリケーション」は将来 `ApplicationCatalog` の語彙に予約する。**
   ApplicationCatalog は OIDC client / SAML SP / WS-Fed RP を束ねる上位 aggregate を所有し、
   表示名、所有者、ライフサイクル、割当、共通監査、共通ポリシーを扱う。各 protocol context は
   application に紐づく protocol binding を所有する。

7. **ApplicationCatalog が実装されるまで、プロトコル固有画面は正確な名前にする。**
   現在の OAuth2/OIDC client 管理画面を単に「アプリケーション」と呼ぶのは避ける。
   統合 ApplicationCatalog が入った時点で、上位ナビゲーション名を「アプリケーション」に戻し、
   その配下に OIDC / SAML / WS-Fed の binding を表示する。

## 影響

- SCL の bounded context `Federation` は `WsFederation` に置き換える。
- `internal/federation` は `internal/wsfederation` へ移す。ただし protocol-neutral な
  claim release / signing key lifecycle の最終配置は ApplicationCatalog / Saml 導入時に見直す。
- [[ADR-059]] は「WsFederation bounded context 導入」の決定としては置き換え対象になる。
  claim mapping を宣言的にし fail-closed にする決定は維持する。
- OAuth2/OIDC の ID Token / UserInfo claim 選択は、将来 `ClaimRelease` を再利用する候補にする。
  既存 OAuth2/OIDC の scope / consent semantics は `OAuth2` に残す。
- JWK/JWKS 管理 API は `SigningKeys` の責務として SCL 上の所有を移す。既存 HTTP path は互換性の
  ため維持し、実装移動は後続の鍵 lifecycle WI と整合させる。
- [[wi-29-saml2-idp]] の UI scope は「admin clients に SAML client/SP 種別を追加」ではなく、
  ApplicationCatalog または暫定 SAML 専用画面との整合を取る必要がある。

## 却下した代替案

- **`Federation` を WS-* / SAML の総称として残す。** OAuth2/OIDC も federation であり、
  既存 `OAuth2` context と語彙が非対称になる。
- **すべてを `Application` bounded context に統合する。** プロトコル state machine と wire
  contract は OAuth2 / SAML / WS-* で大きく異なり、1 context に集約すると変更単位が肥大化する。
- **管理 UI を protocol 名だけで構成する。** 実装者には正確だが、運用者の「業務アプリを接続する」
  という作業単位とずれる。protocol 名は application の binding 種別として扱う。
