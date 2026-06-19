# ADR-035: 本番運用可能な EmailSender adapter (SMTP)

## ステータス

採用。`ra-idp-go/internal/adapters/notification/smtp_email_sender.go` と
`internal/bootstrap` の `EMAIL_SENDER` / `SMTP_*` 環境変数で実装。`EmailSender`
port の SCL バインディングは変更しない。

## コンテキスト

ADR-030 で `EmailSender` port と `ConsoleEmailSender` (stdout) /
`NoopEmailSender` (テスト) を導入したが、本番系の adapter は意図的に「別 ADR」
として保留した。結果として `bootstrap/server.go` が `ConsoleEmailSender` を
hardcode しており、ra-idp-go を本番環境にデプロイしても forgot-password の
リセットリンクが実ユーザに届かない (`/api/auth/forgot_password` は 204 を返す
が、メールは サーバ stdout に出るだけ)。後続の email verification / breach
通知 / step-up via email も同じ port に乗る前提なので、production adapter が
無いままだと積み増した瞬間に複数機能が同時に死ぬ。

## 決定

1. **SMTP のみ採用。HTTP-only プロバイダ専用 SDK は採用しない。**
   - SendGrid / Resend / Mailgun / Postmark / AWS SES は全て SMTP relay を
     公式に提供する。SMTP 1 本で主要プロバイダ + 社内 SMTP まで到達できる。
   - HTTP API (SendGrid REST 等) を 1 つ採用するごとに、(i) 依存・SDK の更新
     負担、(ii) 認証情報の形 (API key の置き場所)、(iii) error 形が増える。
     port の抽象 (`SendEmail(...) bool`) に対して便益が見合わない。
   - 将来 HTTP-only な配送経路が必要になったら、本 ADR を上書きする ADR を
     起こす。

2. **TLS 戦略**: 環境変数 `SMTP_TLS` で 3 モードを選ぶ。
   - `starttls` (既定): 平文で接続 → `EHLO` → `STARTTLS` で昇格 → 認証。
     プロバイダ標準ポート 587 を想定。
   - `implicit`: 接続時から TLS。ポート 465 を想定 (SMTPS)。
   - `none`: 平文のまま。**開発用** だけを意図する。dev compose の mailpit
     などローカル SMTP に向けるためのみ存在。

3. **認証ポリシー**: `SMTP_USERNAME` を設定したときだけ AUTH を実行する。
   - `smtp.PlainAuth` を使う。
   - **PLAIN auth は TLS (implicit / starttls) の下でだけ許可する**。
     `SMTP_TLS=none` かつ `SMTP_USERNAME` 指定の組み合わせは起動時には
     reject せず実行時に Send を fail させる。設定ミスを早期に検知させる。
   - CRAM-MD5 / LOGIN / OAUTHBEARER は採用しない。主要プロバイダはすべて
     PLAIN over TLS をサポートし、複数 auth 方式をサポートすると adapter が
     肥大化するため、PLAIN 1 本に絞る。

4. **From / Reply-To の決め方**: `SMTP_FROM` を必須環境変数とし、RFC 5322
   の `From:` ヘッダ・SMTP `MAIL FROM` の両方に同値を用いる。
   - 値は bare address (`noreply@example.com`) を期待する。display name 付き
     (`"ra-idp" <noreply@example.com>`) はサポートしない (parse の余地を持たず
     設定ミスを減らす)。
   - `Reply-To` は提供しない (リセットメールに返信させる経路が無いため)。
     必要になった ADR を改訂する。

5. **送信失敗時の fail-open ポリシー**: ADR-030 §7 と整合させ、SMTP
   エラーは use case に伝えず `SendEmail` の戻り値 `false` で表現する。
   呼び出し側はそれを使い `EmailSent` event の `delivered` フィールドだけを
   切り替える。ユーザ向け HTTP 応答は常に 204 で、送信成否を漏らさない。
   失敗内容はサーバログに出す (構造化ログでも SMTP_PASSWORD は決して出さない)。

6. **timeout / retry 方針**:
   - 接続 + 各コマンドに統一の deadline を当てる。既定 10 秒。
     `SMTP_TIMEOUT_SECONDS` で上書きできる。
   - retry は adapter 内で**行わない**。fail-open の方針上、失敗は use case
     側の「再送リンク要求」UX で吸収する。queue / outbox を adapter 内に
     抱えると複雑化し、メール固有の冪等性 (Message-ID) 設計まで巻き込む。

7. **`Message-ID` / `Date` ヘッダ**: adapter で都度生成する。
   - `Date`: 送信時の UTC 時刻を RFC 1123Z 形式。
   - `Message-ID`: ランダム 16 バイト hex + `@` + `SMTP_FROM` の domain 部。
     receiving MTA で重複扱いされないよう毎送信ユニークにする。

8. **multipart**:
   - `EmailMessage.Text` のみ → `text/plain; charset=utf-8`。
   - `EmailMessage.HTML` のみ → `text/html; charset=utf-8`。
   - 両方 → `multipart/alternative` で plain → html の順に並べる
     (RFC 2046 §5.1.4 で最後の part が推奨表示)。
   - 添付ファイルはサポートしない (リセット / 検証メールに不要)。

9. **件名の文字符号化**: `mime.QEncoding.Encode("utf-8", subject)` を使い
   ASCII 範囲外を RFC 2047 encoded-word に変換する。本文は `8bit`
   Content-Transfer-Encoding で UTF-8 をそのまま送る (主要 MTA は 8BITMIME
   をサポートする前提)。

10. **秘密情報の取り扱い**:
    - `SMTP_PASSWORD` は環境変数だけで受け取る。設定ファイル経路は提供しない。
    - SMTP_PASSWORD を含む config struct は構造化ログに渡さない (`%v` でも
      展開しない設計とし、テストで担保する)。
    - OTel 属性にも乗せない。

11. **メール内容の正規化**:
    - `From` / `To` ヘッダは `net/mail` で parse できる address のみ出力する。
    - `Subject` は CR / LF を空白へ正規化し、ヘッダ注入を許さない。
    - `Text` は CRLF へ改行を正規化し、NUL を除去する。
    - `HTML` は任意 HTML を信頼して配送しない。本文文字列を
      `html.EscapeString` でエスケープし、HTML メール上の表示テキストとして
      扱う。将来、装飾済み HTML テンプレートを必要とする場合は、許可タグ /
      許可属性ベースのサニタイザ導入を別 ADR で判断する。

## 影響

- 新ファイル: `ra-idp-go/internal/adapters/notification/smtp_email_sender.go`
  (+ unit test)。
- `ra-idp-go/internal/bootstrap/email.go` (新) で env → adapter 切替。
  既定は `console`、`smtp` 選択時は `SMTP_HOST` / `SMTP_FROM` を必須にする。
- 既存 `ConsoleEmailSender` / `NoopEmailSender` は dev / test 用として保持。
- 環境変数表 (`ra-idp-go/README.md` の `### 設定`) に `EMAIL_SENDER` /
  `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` / `SMTP_FROM`
  / `SMTP_TLS` / `SMTP_TIMEOUT_SECONDS` を追加。
- SCL: `EmailSender` port のシグネチャは無変更。`events.EmailSent` の
  wire 形式も無変更。component / interface / permission / objective は無変更。

## 却下した代替案

- **SendGrid / Mailgun / Resend / Postmark / AWS SES の REST API SDK を直接
  叩く**: SMTP 1 本で同じ仕事ができるため。`EmailSender` の port は単純な
  片方向送信なので REST 専用 SDK が提供する高度な機能 (テンプレート、変数
  置換、サブスクリプション管理) は使わない。

- **adapter 内に retry / outbox を入れる**: 冪等性キー設計と queue が必要に
  なる。fail-open + use case 側の再送導線で十分。

- **複数の SMTP auth 方式 (CRAM-MD5 / LOGIN / OAUTHBEARER) をサポート**:
  主要プロバイダはすべて PLAIN over TLS をサポートする。複数方式を許すと
  adapter が「どの方式を試すか」を決める必要があり複雑化する。

- **`SMTP_PASSWORD` をファイルから読む経路 (`SMTP_PASSWORD_FILE` 等)**: k8s
  Secret / Docker secret はファイル → env 変換で配布されるのが常で、adapter
  に持ち込む必要は無い。

- **display name 付き `From` をサポートする (`"ra-idp" <…>`)**: parse の余地
  を残すと設定ミスで送信不能になりやすい。bare address で十分。display name
  が必要になったら別 env で受ける ADR を改訂する。
