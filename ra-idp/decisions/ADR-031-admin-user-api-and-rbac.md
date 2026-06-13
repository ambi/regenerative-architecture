# ADR-031: 管理ユーザー API と RBAC 基盤

## ステータス

採用。

## コンテキスト

Phase 4 の管理 API、テナント分離、管理 UI は、管理者主体と通常ユーザーを
区別する認可境界を必要とする。OAuth Client の scope は人間の管理権限とは
別概念であり、既存の Client 向け AuthZEN ルールだけでは `/admin/*` を保護
できない。

## 決定

1. `User.roles` に RBAC role 名を保存し、最初の組み込み role として `admin`
   を定義する。
2. `/admin/*` は認証済み browser session の `sub` から User を解決し、
   `admin in roles` かつ `disabled_at == null` の場合だけ許可する。
3. 変更系 API は session 認証に加えて Origin と CSRF token を検証する。
4. `disabled_at` は復活可能なアカウント停止であり、`deleted_at` と区別する。
   無効ユーザーは新規ログイン、既存 session、token 再発行、UserInfo を拒否する。
5. 管理 API のレスポンスは `AdminUserResponse` を使い、`password_hash` を
   契約上含めない。
6. 管理操作は actor sub と target sub を含む domain event を発行する。
7. tenant role や tenant membership は次の増分で独立モデルとして追加する。
   `roles` に tenant ID を埋め込まない。

## 影響

- 最初の管理対象は User lifecycle に限定する。
- client / consent / key / audit-event 管理と管理 UI は同じ認可境界の上に追加する。
- デモ環境では seed user に `admin` role を付与する。本番では明示的な
  bootstrap 手順へ置き換える必要がある。
