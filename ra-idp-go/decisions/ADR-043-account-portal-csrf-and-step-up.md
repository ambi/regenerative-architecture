# ADR-043: account portal の高 sensitivity 操作に step-up 再認証を要求する

## ステータス

採用。[[wi-43-account-portal-step-up-auth]]。[[ADR-042]] が account portal の trust
boundary (self/admin) を定め、self mutation を CSRF + same-origin で保護したのを受け、
**高 sensitivity な self-service 操作**に横断的な step-up 再認証ゲートを追加する。

## コンテキスト

[[wi-21-end-user-account-portal]] の self mutation は CSRF + same-origin と、操作ごとの
所持証明 (TOTP 解除時の有効コード等) で守られている。しかしセッション cookie が乗っ取られた
場合、攻撃者はパスワード変更・MFA factor の解除・primary email 変更・全セッション失効
といった「アカウントの支配権を奪う操作」をそのまま実行できてしまう。

Google ("確認のためもう一度ログイン")、Okta の re-authentication、Keycloak の `max_age`
相当のように、実運用 IdP は高 sensitivity 操作の直前に**直近の再認証**を要求する。本 ADR は
その横断ゲートを account portal に持ち込み、対象操作の表と recency 条件を確定する。

## 決定

1. **対象表 (step-up を必須とする操作)**。以下の self-service mutation は step-up を
   要求する。SCL では対象 interface に `annotations: { step_up: required }` を付け、
   実ハンドラとの一致を機械照合する (テスト `TestStepUpAnnotatedInterfacesMatchGatedHandlers`)。
   - `ChangePassword` (`POST /api/auth/change_password`)
   - `RemoveTotpFactor` (`POST /api/account/mfa/totp/remove`)
   - `RequestEmailChange` (`POST /api/account/email/change_request`)
   - `RevokeMyOtherSessions` (`POST /api/account/sessions/revoke_others`)

   個別セッションの失効 (`RevokeMySession`)・TOTP の登録 (enroll) は本表に含めない
   (相対的に低 sensitivity / 登録は所持証明で完結)。将来 [[wi-26-webauthn-passkey-and-recovery-codes]]
   の WebAuthn credential 解除など sensitive 操作を足す場合は本表に追記する。

2. **recency 条件**。step-up は「直近 `StepUpRecencySeconds` (= 5 分) 以内に password
   または MFA で (再)認証済み」を満たすときに通過する。判定の時刻ソースは
   `max(session.auth_time, session.step_up_at)`。**新規ログイン直後はそのまま step-up
   済み**として扱う (Google 同様、ログイン直後に再入力を求めない)。

3. **未通過時の応答**。step-up 未通過は 401 ではなく **403 + `step_up_required`** で返す
   (認証はされているが、この操作には再認証が要る、というセマンティクス)。UI はこれを受けて
   再認証 modal を出し、成功後に元の操作を再試行する。

4. **step-up の入口**。
   - `POST /api/account/step_up/start` (`StartStepUpAuthentication`): 利用可能な factor
     (`password` 常時 / `totp` は enrolled 時) を返す。`StepUpRequested` を emit。
     **この interface 自体には step-up を要求しない** (再認証の入口のため)。
   - `POST /api/account/step_up/complete` (`CompleteStepUpAuthentication`): 提示された
     factor (password / totp コード) を検証し、成立すれば `session.step_up_at` を現在時刻で
     刻む。`StepUpCompleted` (method 付き) を emit。検証材料は audit / log に残さない。

5. **状態の置き場所**。recency は `LoginSession.step_up_at` (Unix 秒) に永続化し、
   `AuthenticationContext.StepUpAt` で handler 層に運ぶ。session に紐づくため、cookie が
   別端末へ移っても step_up_at はその session に閉じる。

## 影響

- step-up は actor 本人の再認証であり、対象 sub を変えない (self RBAC を逸脱しない)。
- gate は handler 共通ヘルパ (`requireStepUpSub` / `requireStepUpSession`) として
  sensitive ハンドラに差し込む。CSRF + same-origin ([[ADR-042]]) は引き続き全 mutation に
  掛かり、step-up はその上乗せである。
- `StepUpRequested` / `StepUpCompleted` は監査イベントに残るため、再認証の試行と成立を
  後から追跡できる。検証材料 (パスワード / コード) は payload に含めない。
- 後続: 横断ゲートに依存する sensitive 操作 (WebAuthn credential 解除等) は対象表に
  追記して一貫させる。admin 経路の step-up は本 ADR の範囲外 (end-user self-service に限定)。

## 参照

- [[wi-43-account-portal-step-up-auth]] — 本 ADR を導く WI。
- [[ADR-042]] — account portal の trust boundary と CSRF + same-origin の土台。
- [[wi-21-end-user-account-portal]] — step-up を載せる self-service 操作群。
- [[wi-26-webauthn-passkey-and-recovery-codes]] — 横断ゲートを将来利用する sensitive 操作。
