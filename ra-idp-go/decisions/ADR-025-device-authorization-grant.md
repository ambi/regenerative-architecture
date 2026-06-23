# ADR-025: Device Authorization Grant (RFC 8628) の実装

## ステータス

採用（ADR-001 の device-code 状態機械を実装に落とす）

## コンテキスト

`spec/flows/device-code-flow.json` に device flow の状態機械、`spec/grants/grant-types.json`
に device_code グラント、`spec/discovery.json` に `device_authorization_endpoint` が
宣言済みだった。しかし HTTP / usecase 実装が無く、**Discovery が広告する
`/device_authorization` が 404 を返す**状態だった。入力制約のあるデバイス（TV・CLI・IoT）は
このグラントでしか認可を受けられないため、適合性と実用性の両面で欠落していた。

RA の観点では、これは「仕様核（状態機械・grant matrix・discovery）が既に存在するので、
アダプタと usecase は仕様から再生成できる」ことを実証する好機でもある。

## 決定

1. **エンドポイント**
   - `POST /device_authorization` (§3.1): device_code / user_code を発行。クライアント認証
     (`authenticateClient`) を適用。
   - `GET /device` / `POST /device` (§3.3): verification_uri。ユーザーが user_code を入力し
     承認 / 拒否する。ユーザー認証は本アプリでは X-User-Sub（authorize と同方針）。
   - `/token` の `urn:ietf:params:oauth:grant-type:device_code` 分岐 (§3.4): ポーリング。
2. **コードのエントロピーと保管**
   - `device_code`: 32 バイト乱数。ベアラ秘密なので SHA-256 ハッシュのみ保存。
   - `user_code`: 母音・紛らわしい文字を除いた 20 文字集合 × 8 桁（約 34 bit）。
     `WDJB-MJHT` 形式で表示し、索引キーは正規化して保持（§6.1）。
3. **状態遷移は仕様核に従う** — `spec/flows/device-code-flow.json` の遷移テーブルを
   `transitionDeviceCode` 経由で消費。approve/deny/exchange を勝手に実装しない。
4. **ポーリング動作 (§3.5)** — `authorization_pending` / `slow_down` / `access_denied` /
   `expired_token` を返す。`interval` と slow_down 増分は仕様核 (`polling`) を権威とする。
   interval より速いポーリングは `slow_down` で抑制する。
5. **承認済みからの発行** — access_token + refresh_token + id_token(openid 時) を
   exchange-code-for-token と同じ経路で発行。`approved → exchanged` に進めてから発行し、
   二重発行を防ぐ。
6. **volatile store** — `DeviceCodeStore` ポート + memory / valkey アダプタ。device_code
   ハッシュと user_code の 2 索引を持ち、TTL は device_code 寿命 (600s)。
7. **監査イベント** — `DeviceAuthorizationRequested` / `Approved` / `Denied` を
   events.schema.json → asyncapi → outbox → Zod の全チェーンに追加。user_code は
   推測攻撃の手掛かりにしないため監査ログに残さない。

## 影響

- `DeviceCodeStore` ポート + memory/valkey アダプタ、`device-routes.ts`、
  3 usecase（request / verify / exchange）、`device-authorization.ts` ドメインを追加。
- `token-routes.ts` に device_code 分岐、`OAuthErrorCode` に
  `authorization_pending` / `slow_down` / `expired_token` を追加。
- 仕様核には新規ファイルを足さず、既存の状態機械・grant matrix・discovery を「実装が追いつく」
  形で消費した（RA の再生成可能性の実証）。

## セキュリティ上の注意

- user_code は低エントロピーなので、verification_uri での入力レート制限が production では必須
  （本アプリは枠組みのみ）。`DeviceAuthorizationDenied` の多発を SIEM で総当たり検知できる
  よう監査イベントを設計した。
- device_code はハッシュ保存。`slow_down` でポーリング濫用を抑制。

## 却下した代替案

- **device_code をそのまま保存**: ベアラ秘密の平文保存はアンチパターン。ハッシュのみ保存。
- **user_code に英数字フル集合**: 視認混同（0/O, 1/I）と偶発的な単語生成を避けるため子音集合に限定。
- **承認画面を 2 ステップ (enter_user_code → approve) に分割**: 本アプリの UX を単純化し、
  1 フォームで enter_user_code + approve を連続適用する（状態機械上は両遷移を順に発火）。
