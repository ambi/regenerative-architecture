# ADR-029: ログイン試行のスロットリングとユーザー名列挙対策

## ステータス

採用。`spec/scl.yaml` `objectives.LoginThrottlePolicy` と
`src/authentication/ports/login-attempt-throttle.ts` の双子に反映。

## コンテキスト

ロードマップの「ブルートフォース防御」項目を埋める。bundled common-password 辞書
(ADR-026) と外部漏洩データベース検査 (ADR-028) は **新規 / 変更** のパスワードに
作用し、**既存アカウントへのオンライン パスワード推測 / credential stuffing** には
何も効かない。NIST SP 800-63B-4 §5.2.2 は verifier に「consecutive failed
authentication attempts on a single account」のレート制限を要求しており、
本 ADR はその要件と現場慣行 (OWASP ASVS v4.0.3 §2.2.1) を満たす最小スライスを定義する。

SCL には `rate_limit_per_minute` policy kind と `ClientAuthFailureRateLimit`
(client_secret 失敗) / `AuthorizationCodeRedemptionFailureRateLimit` (code 交換失敗)
の語彙が既に予約されているが、**ログインエンドポイントへの enforcement は未実装**。
本 ADR でログイン経路の port と adapter、SCL annotation、ADR を同時に整える。

## 決定

1. **二軸スロットリング**: per-account と per-IP の両方を独立にカウントする。
   - per-account: 同一 username（小文字正規化）の失敗回数。攻撃者が単一アカウントに
     対して辞書攻撃する経路を塞ぐ。
   - per-IP: 同一クライアント IP からの失敗回数。credential stuffing（多数の
     username × 漏洩 password を 1 IP から試行）を捉える。
   - 両者は別カウンタ / 別 lockout window。どちらかが閾値に達すれば 429 を返す。

2. **しきい値**:
   - per-account: 10 失敗 / 15 分 → 15 分ロック。
   - per-IP: 30 失敗 / 15 分 → 15 分ロック。
   - 数字は NIST §5.2.2 の "limit to no more than 100 failed attempts per 30 days"
     を上限とし、現場慣行 (OWASP ASVS / Auth0 / Okta の既定値) と合わせた範囲で
     設定。テナント別ポリシー（将来）で上書き可能。

3. **ロック表現**: counter + lock の二段。
   - counter (`failures:{kind}:{key}`) は sliding window 内のインクリメント。
     しきい値到達で lock キー (`lock:{kind}:{key}`) に `EX` 付きで書く。
   - tryAcquire は lock キーの存在を見て allowed / retryAfterSeconds を返す。
   - 単純な fixed-window counter で十分（攻撃者目線では window 跨ぎで 2 倍試行が
     できるが、5 分 window だと UI 操作の誤入力を弾くので 15 分にする）。

4. **成功時クリア**: ログイン成功時に該当 username の counter / lock を削除する。
   - per-IP の counter は **クリアしない**: 1 IP に多数の正規ユーザが居る場合
     (NAT / オフィス IP) でも、成功はその IP の正常性を保証しない。ただし
     window 内で自動失効するので運用上のロックは時間で解ける。

5. **ユーザー名列挙対策 (constant-time + 同一カウンタ)**:
   - 現状の `findByUsername → verify` は user 未存在時に verify をスキップして
     timing oracle (Argon2 ~50–200 ms の有無) で存在を漏らす。fix として、
     未存在時に **固定 sentinel ハッシュ** で `passwordHasher.verify` を回す。
   - throttle counter は **username 文字列** (lowercase) で集計し、user 存在
     チェックの前に increment する。未存在 username の試行も同じ枠を消費し、
     429 のタイミング / Retry-After ヘッダから存在有無を漏らさない。

6. **イベント** `LoginThrottled`
   - payload: `occurredAt`, `kind` (`account` | `ip`), `keyHash` (username
     を SHA-256, IP を SHA-256 — 平文を audit log に流さない), `retryAfterSeconds`.
   - `oauth2.authentication.v1` トピックに既存 auth event と同じレーンで流す。
   - SIEM 側で IP 集計 / impossible travel と組み合わせる前提。

7. **クライアント IP 解釈 (trusted proxy)**:
   - デフォルトは直接 peer の IP (Hono `c.req.raw.remote_addr` 相当) を使い、
     `X-Forwarded-For` は **信頼しない**。
   - `TRUSTED_FORWARDED_HOPS=N` (整数) が設定されたとき、`X-Forwarded-For` の
     右から N 番目 (= 末端の信頼境界の次の hop) を採用する。0 / 未設定は無効化。
   - 設定誤りで本物の IP が見えなくなる事故より、攻撃者が `X-Forwarded-For` を
     偽装して per-IP throttle を回避する事故のほうが致命的なので、デフォルトを
     off にする。

8. **適用範囲（本 ADR 時点）**:
   - `/api/auth/login` (SPA JSON API) と `/login` (no-JS form)。
   - TOTP (`/api/auth/totp`) は同 port を再利用するが本スライスでは触らない。
     パスワード突破後の絞り込みであり、一次防御ではない。
   - `/token` の client_secret 失敗 / code 交換失敗は別カウンタ
     (`ClientAuthFailureRateLimit` / `AuthorizationCodeRedemptionFailureRateLimit`)
     として enforcement する Phase は別 ADR。

9. **CAPTCHA / 行動分析を本 ADR で採用しない理由**:
   - 外部サービス依存 (reCAPTCHA / hCaptcha / 行動分析 SaaS) が入り、port を
     切る前に脅威モデルとプライバシ影響評価が要る。レート制限は単体で
     NIST 要件を満たし、CAPTCHA 無しでも価値が出る。`BotChallenge` port は
     後続スライスで追加する。

## 影響

- 新 port `LoginAttemptThrottle` (`tryAcquire` / `recordFailure` / `recordSuccess`)。
- 新 adapter:
  - `InMemoryLoginAttemptThrottle` (memory / テスト)
  - `ValkeyLoginAttemptThrottle` (本番、`INCR` + `EXPIRE` + `SET NX EX` で lock)
- `authentication-routes.ts` に IP 抽出 + throttle 配線 + sentinel verify。
- `LoginThrottled` event を `DomainEvent` 判別ユニオン / SCL `events` /
  `infra/event-routing.yaml` に追加。
- SCL `objectives.LoginThrottlePolicy` にしきい値を記録する。
- `TRUSTED_FORWARDED_HOPS` は実行環境固有の設定として実装側で管理する。
- セッションマネージャやパスワード検証ロジックは触らない（純粋にレイヤを増やす）。

## 却下した代替案

- **per-account のみで per-IP を持たない**: credential stuffing は 1 アカウント
  あたり 1〜数試行で次のアカウントへ移るため per-account では捕まらない。
- **永続ロック (failed attempts >= N で管理者が解除するまで)**: 自身を DoS
  できる脆弱性になる (攻撃者が任意の victim username を継続的に失敗させて
  恒久ロックに追い込む)。NIST も明示的に "do not implement permanent
  lockouts" を推奨。時間で解ける lockout に統一する。
- **`X-Forwarded-For` をデフォルトで信頼**: 偽装で per-IP を回避できる。
  プロキシ段数が運用ごとに違うため安全側に倒し、明示 opt-in にする。
- **カウンタを username ではなく sub で集計**: 未登録 username の試行が
  カウントされず、存在しないユーザの探索を許す。username 文字列で揃える。
- **CAPTCHA を同時導入**: 上記。スコープを切り、まずレート制限で NIST
  要件を満たす。
