# ADR-041: 認証イベントを通常イベントと bucket 集約の 2 系統で持つ

## ステータス

採用。[[wi-44-authentication-event-store-and-search]]。[[wi-20-authentication-event-history]]
では bucket 切替の挙動だけを in-memory で先に実装し、本 ADR を後追いで正式起草する。
[[ADR-018]] (監査 / アプリログ分離)・[[ADR-029]] (ログイン試行スロットリング) を前提に、
認証イベントの**データモデルと攻撃時の挙動**を確定する。

## コンテキスト

production IdP は「誰が・いつ・どの手段で・どこから認証したか / 失敗したか」を時系列で
保持し、admin が調査できる必要がある (Keycloak の login events、Okta の System Log 相当)。
一方で credential stuffing / brute force 時には失敗イベントが秒間数千件のオーダーで発生し、
1 失敗 = 1 行で素朴に INSERT すると次の 2 つが同時に壊れる:

1. **可用性**: 監査ストア (Postgres) への書き込みが攻撃トラフィックで詰まり、正規の認証
   経路まで巻き添えで遅くなる。
2. **調査性**: 数百万行の同一パターン失敗に埋もれ、admin が異常を読み取れず MTTR が悪化する。

[[ADR-029]] の throttle は試行**そのもの**を遅らせる防御だが、ロック後も試行は届き続ける。
そのため throttle とは別に、**監査イベント側でも爆発を吸収する**仕組みが要る。

## 決定

1. **2 系統モデル**。認証イベントは次の 2 つの系統で表現する。
   - **通常イベント** (`authentication_events`): 1 認証アクション = 1 行。成功・失敗・
     MFA・federation・impersonation・session 開始などの個別事象を時系列で保持する。
     poly kind = `success` | `fail` | `aggregated`。
   - **bucket 集約** (`authentication_event_buckets`): 攻撃時に通常イベントへ落とさず、
     `(tenantId, kind, keyHash, 5 分窓)` 単位の 1 行へ畳み込んだ計数。1 窓 = 1 行で、
     以後の増分は行の `count` に積むだけで個別 INSERT を出さない。

2. **bucket への切替条件**。per-account / per-IP の throttle が [[ADR-029]] の lockout 閾値に
   到達したアクター由来の失敗は、以後 individual な `AuthenticationFailed` を emit せず
   bucket へ集約する。throttle と bucket は同じ `keyHash` (username / IP の SHA-256、tenant
   salt 付き) を共有し、平文は監査に流さない ([[ADR-046]])。

3. **集約の閾値 (既定) と tenant override**。窓は 5 分。既定の集約発火閾値は次のとおりで、
   `TenantSettings` で override できる。
   - per-account: 10 失敗 / 窓
   - per-ip: 50 失敗 / 窓
   - per-tenant: 1000 失敗 / 窓 (テナント全体の floods に対する安全弁)

   閾値の根拠: per-account 10 は [[ADR-029]] の account lockout と揃え、正規ユーザの打ち間違い
   (数回) を individual に観察可能なまま残す。低すぎると正規失敗が bucket に隠れて MTTR が悪化し、
   高すぎると爆発が止まらない。tenant ごとの運用差は override で吸収する。

4. **bucket = 1 admin 監査イベント**。1 つの窓につき `AuthenticationEventAggregated` を
   **1 件だけ** emit し ([[ADR-018]] の監査ストアへ)、以後の増分はイベントを増やさず bucket 行の
   `count` を更新する。admin はこの 1 行から同 key の個別失敗サンプル (10 件) へドリルダウンできる。

5. **impersonation はフィールドのみ**。「admin が user として操作した事実」(`SessionImpersonationStarted`
   / `SessionImpersonationEnded`) は本人通知を前提とする監査事象である。本 WI ではイベント型と
   ストレージ列のみを用意し、**機能本体の有効化は本人通知 WI 後**に行う。impersonation イベントは
   retention 短縮の対象外とする ([[ADR-045]] / [[ADR-046]])。

6. **後方互換**。既存 `UserAuthenticated` / `AuthenticationFailed` の payload は**拡張のみ**で
   破壊しない (sessionId / clientId / acr / ipHash / ipTruncated / uaHash / countryCode /
   deviceFingerprintHash / riskScore はすべて optional)。既存 SIEM connector が落ちないことを
   wire スナップショットで確認する。

## 影響

- bucket モードは in-memory counter + 5 分単位 flush で動き、攻撃時も個別 INSERT を出さない。
  この挙動は in-memory ([[wi-20-authentication-event-history]]) と Postgres 永続 ([[wi-44-authentication-event-store-and-search]])
  の双方で同一に保つ。
- 認証成功・失敗・MFA の結果は共通の `AuthenticationOutcomeBus` を通し、bucket 切替判定を
  bus 層へ集約する (個々の use case が切替ロジックを持たない)。
- 通常イベントと bucket は別テーブルだが、admin 検索 UI では同一タイムラインに混在表示し、
  bucket 行はドリルダウン可能な集約行として描く。

## 却下した代替案

- **1 失敗 = 1 行のみ (bucket なし)**: 実装は単純だが攻撃時にストレージと調査性が破綻する。
- **サンプリング (N 件に 1 件だけ記録)**: 計数の正確さが失われ、攻撃規模の評価ができない。
  bucket は全件を `count` に積むため規模を保持する。
- **throttle ロック後はイベントを一切残さない**: 攻撃の発生事実と規模が監査から消えるため不可。
  bucket により「1 行 + count」で痕跡と規模の両方を残す。
