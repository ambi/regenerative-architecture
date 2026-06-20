# ADR-042: エンドユーザー account portal の self-service スコープ

## ステータス

採用。[[wi-21-end-user-account-portal]] の先行 PR (portal shell + アカウント概要 +
個人情報編集)。段階リリースの土台として「self が触れる範囲」と「admin 専用の範囲」の
trust boundary を確定する。後続ステージ (emails / security / activity / applications /
data export / deletion) はこの境界の上に積む。

## コンテキスト

end-user が自分自身に対して操作できる UI は `/account/password` (パスワード変更) と
`/account/profile` (表示名・属性編集、wi-19) しか無く、admin に頼らず完結できる
self-service の中核と、その権限境界が言語化されていなかった。Keycloak Account Console /
Okta End-User Dashboard / Google アカウント相当の "マイページ" を持ち込むにあたり、
**self が変更してよいもの**と**admin だけが変更できるもの**を曖昧にしたまま API/UI を
増やすと、権限昇格や誤編集の温床になる。

## 決定

1. **trust boundary を `self` と `admin` で分ける**。account portal の API は全て
   `/api/account/` プレフィックスで、認証済みセッションの `actor.sub` のみを操作対象に
   する (`requireAuthenticatedSub`)。URL / body / query の sub・tenant_id は信用せず、
   cross-user / cross-tenant 参照は構造的に発生させない。admin shell 用の
   `/api/auth/account` (roles を含む metadata) とは別契約とし、portal の概要は
   `/api/account/summary` (roles を含まない `AccountSummary`) で返す。

2. **self が変更できる範囲 (初期スコープ)**。
   - 表示名 (`name` / `given_name` / `family_name`)。
   - `editable_by_user=true` の属性 ([[ADR-040]] の `UserAttributeDef`)。
   - パスワード変更 (既存 `/api/auth/change_password`)。
   - 後続ステージで追加: secondary / recovery email、TOTP の enroll/remove、
     セッション revoke、consent 取り消し、データエクスポート、アカウント削除リクエスト。

3. **admin だけが変更できる範囲 (self からは不可)**。
   - `roles` と `status` (active / disabled / locked / staged / suspended)。
   - 組織属性 (`department` / `manager` / `employee_number` 等、`editable_by_user=false`)。
   - `editable_by_user=false` の custom 属性全般。
   - `required_actions` の付与/解除 (self は自分の未対応項目を**閲覧**できるのみ。
     パスワード変更による `update_password` の自動解除など、本人の能動操作の結果として
     消えるものを除く — [[wi-19-rich-user-attributes]])。

4. **portal shell は admin shell と分離する**。account portal は専用の `AccountShell`
   を持ち、admin 機能への操作導線 (admin nav) を出さない。admin ロールを持つ user が
   `/account` を開いてもマイページとして振る舞い、管理コンソールへはメニューのリンクで
   明示的に移動する。未認証で `/account/*` を開いた場合は
   [[wi-18-unauthenticated-admin-redirect]] と同じく `/login` へ誘導し戻り先を保持する。

5. **高 sensitivity 操作の step-up とデータエクスポート形式は本 ADR では決めない**。
   step-up 再認証 (ADR-043 予定) とデータエクスポート形式 (ADR-044 予定) は、対象機能を
   実装する後続ステージで別 ADR として定める。本 ADR は scope と trust boundary に閉じる。

## 影響

- self mutation は全て `actor.sub == target.sub` を最低要件とし、admin RBAC とは独立した
  境界になる。SCL では self interface (`GetAccountSummary` / `GetUserProfile` /
  `UpdateUserProfile`) を admin interface と分けて表現する。
- `AccountSummary` は roles を含まないため、portal が誤って admin 権限情報を露出しない。
- 後続ステージの API/UI は本 ADR の self/admin 分類表に従って追加し、admin 専用項目を
  self 経路に出さないことをテストで担保する。

## 参照

- [[wi-21-end-user-account-portal]] — 本 ADR を導く WI。
- [[ADR-040]] — `editable_by_user` を含む属性ポリシー。
- [[wi-19-rich-user-attributes]] — self プロフィール編集と required actions の基盤。
- [[wi-18-unauthenticated-admin-redirect]] — 未認証リダイレクトの pattern。
