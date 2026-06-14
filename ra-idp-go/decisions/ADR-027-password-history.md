# ADR-027: パスワード履歴の再利用禁止

## ステータス

採用（Phase 0 — 認証の土台。`spec/scl.yaml` `objectives.PasswordPolicy.value.history_depth` /
invariant `PasswordHistoryNoReuse` / interface `ChangePassword` / event `PasswordChanged` と
`src/authentication/usecases/change-password.ts` の双子に反映）。

## コンテキスト

ADR-026 で「パスワード履歴の再利用禁止は `PasswordHistoryRepository` port と
change-password エンドポイントが揃った時点で別 ADR と共に追加する」と保留した。
本 ADR は `/api/auth/change_password` エンドポイントの導入に合わせて履歴ポリシーを
定義する。

履歴件数・保存形式・カスケード削除・テナント別カスタマイズの取り扱いは
将来の管理基盤（Phase 4 — RBAC / マルチテナンシー）に影響する設計判断のため、
spec↔impl drift を防ぐためここで明文化する。

## 決定

1. **履歴深さ**: 直近 **5 件** のハッシュと一致する新パスワードを拒否する。
   - SCL `objectives.PasswordPolicy.value.history_depth: 5` を権威源とし、
     `PASSWORD_POLICY.historyDepth` を双子として持つ。
   - 5 は NIST SP 800-63B-4 の禁止項目に該当しない範囲で、現場慣行
     （OWASP ASVS v4.0.3 §2.1.10 が "previously-used password" の検知を要求）と
     整合する最小値。これ以上深くしてもユーザーの不便さに対する効果は逓減する。

2. **保存形式**: `password_hash` と同じ PHC エンコード文字列（Argon2id）を
   そのまま履歴行に積む。
   - 追加の暗号化は行わない。`password_hash` 本体と同じ攻撃耐性を持つため、
     履歴のみを別建てで暗号化しても閾値が下がらない。
   - 各履歴行は `{ sub, encoded, created_at }` の 3 フィールド。順序は
     `created_at DESC` で `depth` 件を取り出し、`PasswordHasher.verify` で逐次照合する。

3. **比較戦略**: ハッシュ照合 × 履歴件数。
   - Argon2id verify はパラメータ次第で 50–200 ms / 回。固定 5 件であれば
     最悪 1 秒の遅延に収まる。change-password は対話的な低頻度操作のため
     許容範囲。
   - 履歴件数を増やすと逐次照合のコストが線形に伸びる。テナント別ポリシーで
     上限を設ける際は、本 ADR の遅延見積もりを再評価する。

4. **カスケード削除**: ユーザー削除時に履歴も削除する。
   - Postgres は `ON DELETE CASCADE` で `users(sub)` を参照。
   - 履歴は単体で価値を持たない PII。残しても再認証材料にならず、漏洩面だけが
     広がる。

5. **タイミング**: change-password と registration の双方で履歴を **書く** が、
   履歴チェックは change-password のみで実施する。
   - 初回登録時の現パスは履歴ゼロ件のため照合不能。
   - 登録時点で `[ encoded ]` を 1 件積むことで、登録直後の change-password で
     「初期パスと同じ」が弾ける。

6. **将来のテナント別ポリシー（Phase 4）**:
   - `PASSWORD_POLICY.historyDepth` を constant ではなくテナント解決ポート
     （`PasswordPolicyResolver.resolve(tenantId)`）の戻り値に置き換える。
   - usecase `change-password.ts` は depth を引数経由で受け取る形に既に
     設計してある（policy オブジェクトを直接読まない）。テナント解決の
     追加は port 1 つ差し込むだけで済む。

## 影響

- 新 port `PasswordHistoryRepository`（`add` / `recent` の 2 メソッド）。
- 新 use case `change-password.ts`（現パス verify → policy 検証 → 履歴照合 →
  hash → 保存 → 履歴追加 → `PasswordChanged` emit）。
- 新 HTTP endpoint `POST /api/auth/change_password`（セッション cookie 必須、
  CSRF 必須、JSON）。
- 新マイグレーション `password_histories(sub, encoded, created_at)` +
  `(sub, created_at DESC)` インデックス + `ON DELETE CASCADE`。
- 新 SPA 画面（change-password）。`/change_password` ルート、ja/en i18n。
- `bootstrap/seed.ts` は初期 hash を history に 1 件積む。

## 却下した代替案

- **深さを 12 / 24 等の大きな値にする**: NIST SP 800-63B-4 §3.1.1.2 は履歴禁止
  そのものを推奨も非推奨もしていないが、深さを増やすほど「直前のパスワードに
  +1 する」等の予測可能な書き換えを誘発する。OWASP ASVS の要件 (「previously-used
  passwords を検知できる」) は深さに下限を置いていない。5 は最小要件を満たす。
- **平文比較や独立ソルトでの再ハッシュ**: 既存 `password_hash` と異なる
  攻撃耐性を持ち込むことになり、二重管理になる。PHC エンコードをそのまま
  積むのが最も surgical。
- **履歴を user 行に JSON 配列として埋め込む**: 行の肥大・並列更新時の競合・
  カスケード削除の表現力に劣る。専用テーブルが妥当。
- **登録時に履歴を積まない**: 登録直後の change-password で初期パスへ戻せて
  しまう。1 件だけ積めば最小コストで埋まる穴。
