# ADR-030: メールによるパスワードリセット (forgot-password)

## ステータス

採用。`spec/scl.yaml` の `objectives.PasswordResetTokenLifetime` /
`events.PasswordResetRequested` / `interfaces.RequestPasswordReset` /
`interfaces.ResetPasswordWithToken` と
`src/authentication/usecases/request-password-reset.ts` /
`src/authentication/usecases/reset-password-with-token.ts` の双子に反映。

## コンテキスト

これまでのパスワード周りの ADR は **既知の現パスワード** から始まる経路
（change-password / ADR-026 / ADR-027 / ADR-028）と、**生きているセッション**
での再保護（ADR-029）に閉じていた。「パスワードを忘れたユーザを救う経路」が
無く、初回パスワード入力ミスやデバイス紛失で永久にアカウントが失われる。

リセット経路は password-policy / password-history / breached-checker / sentinel
verify と同じ部品を再利用しつつ、認証されていない攻撃者の入口にもなるため、
**列挙対策**・**シングルユース トークン**・**TTL**・**email チャネルの抽象** を
ここで一括して決める。

将来は email verification / magic link / breach 通知 / step-up via email を
全て同じ `EmailSender` port に乗せる。本 ADR が port 設計の最初の使用点。

## 決定

1. **トークン**: 32 バイト乱数 → base64url 文字列。
   - URL / email にだけ生 token を載せる。
   - DB / store には SHA-256 hash で保管。流出時に再現できない。
   - 1 件 1 sub。同 sub に未消費トークンが残っていれば古いものを失効させる。

2. **TTL = 30 分**。OWASP "Forgot Password Cheat Sheet" / NIST SP 800-63B-4
   の "short-lived" 条件と整合する。30 分は届いた email を確認して操作する
   現実的な時間と、流出時の窓を狭める要求の中間。

3. **シングルユース**: `consume(token)` は原子的に削除して record を返す。
   再利用は無効。失敗時 (policy 違反など) は token も失効済みになるため、
   ユーザは「新しい reset リンクを送る」操作からやり直す。

4. **anti-enumeration**:
   - `POST /api/auth/forgot_password` は **常に 204** で返す。email が
     未登録 / `email_verified=false` / typo どれでも区別がつかない応答。
   - `POST /api/auth/reset_password` は token の有効性のみ判定し、
     username / email を露呈しない。
   - email 送信は **best effort**: 送信失敗をユーザに直接返さない
     (ConsoleEmailSender でも本物の SMTP でも同じ抽象)。送信は audit log
     にだけ残す。

5. **`email_verified=true` を要求**:
   - 検証済み email を持つ user にだけリセットリンクを送る。
   - self-registration のない現状でも、admin 経由 / seed の demo user は
     `email_verified=true` で配布される。
   - email verification flow が後で入った時点で同じガードが効く。

6. **新パスワード適用パイプライン**:
   - reset 時の new_password は change-password と同じ
     `validatePasswordAsync` + `PasswordHistoryRepository.recent` +
     `BreachedPasswordChecker` を通す。
   - 成功時 `PasswordChanged` を emit (既存イベント再利用)。
   - history への追加もまったく同じ。change-password との違いは
     「current_password verify がなく token consume に置き換わる」点だけ。

7. **EmailSender port**:
   - `sendEmail({ to, subject, text, html? }): Promise<void>`
   - 既定 adapter: `ConsoleEmailSender` (stdout + `EmailSent` event 発火で
     demo / dev で email 内容を確認できる)。
   - 失敗は use case に伝播させない (fail-open)。送信失敗は audit / metric
     で監視。リセット token は失効まで再送可能。
   - SMTP / HTTP プロバイダ adapter は別 ADR。本 ADR では port と Console
     adapter のみ。

8. **per-email rate limit**:
   - 同一 email への過剰な reset 要求は spam・SIEM ノイズになる。
   - 本 ADR では **入れない**。ADR-029 の `LoginAttemptThrottle` 再利用が
     筋なので、独立の Phase で `password-reset` kind を足す。

9. **トークン保管テーブル**:
   - `password_reset_tokens(sub, token_hash, created_at, expires_at,
     consumed_at)`。
   - `ON DELETE CASCADE` で users(sub) に紐付け。
   - 古い consumed/expired 行は GC ジョブで掃く (本 ADR では cron なし、
     INSERT 時に同 sub の未消費を失効させる軽い掃除のみ)。

## 影響

- 新 port: `EmailSender`, `PasswordResetTokenStore`。
- 新 adapter: `ConsoleEmailSender`, `NoopEmailSender`,
  `InMemoryPasswordResetTokenStore`, `PostgresPasswordResetTokenStore`。
- 新 use case: `requestPasswordReset`, `resetPasswordWithToken`。
- 新 HTTP endpoint: `GET /forgot_password`, `POST /api/auth/forgot_password`,
  `GET /reset_password`, `POST /api/auth/reset_password`。
- 新 SPA page: `ForgotPasswordPage`, `ResetPasswordPage`。i18n キー追加 (ja/en)。
- 新 SCL: `PasswordResetTokenRecord` model, `PasswordResetRequested` event,
  `EmailSent` event, `RequestPasswordReset` / `ResetPasswordWithToken`
  interfaces, `objectives.PasswordResetTokenLifetime`。
- 新マイグレーション: `password_reset_tokens` テーブル。

## 却下した代替案

- **TTL を 24 時間に伸ばす**: ユーザビリティ向上は限定的で、流出時の窓が
  広がる。OWASP は分単位の短さを推奨。

- **email を本文に書く / sub を URL に載せる**: 列挙経路を作る。URL は
  token だけにし、サーバ側で sub を解決する。

- **fail-closed (送信失敗時に エラーを画面に返す)**: 列挙の手がかりを
  与え、SMTP 障害でフロー全体が停止する。リセットは「届かなければ再送」
  でユーザが回復できる経路にする。

- **password reset で email_verified を要求しない**: self-registration が
  実装された時、攻撃者が他人の email で登録して victim の password を奪える
  経路が開く。検証済みのみへ送る方針で先回りする。

- **token を DB に平文で保管**: 流出時に攻撃者がそのまま reset を実行できる。
  ハッシュ化は cost 0 で耐性を一段上げる。
