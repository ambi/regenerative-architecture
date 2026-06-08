# ADR-026: NIST SP 800-63B-4 整合のパスワードポリシー

## ステータス

採用（Phase 0 — 認証の土台。`spec/scl.yaml` `annotations.password_policy` と
`src/authentication/usecases/password-policy.ts` の双子に反映）。

## コンテキスト

Phase 0 で IdP に求められる「商用品質のパスワードポリシー」を定義する。
README ロードマップは深化方向として 4 項目（文字種要件・ユーザー識別子との
類似禁止・共通パスワード辞書・パスワード履歴）を挙げていたが、現行 NIST
SP 800-63B-4 §3.1.1.2 はそのうち 2 つ（文字種要件・periodic rotation）を
明示的に **採用しない** よう推奨している。これと衝突する control を実装する
かどうかは設計判断であり、ADR が必要。

仕様核 `spec/scl.yaml` には既に「長さのみを強制し、文字種混在ルールは課さない」
という宣言があり、HIBP 等の漏洩データベース検査は別 port
（`BreachedPasswordChecker`）に切ってある。本 ADR は NIST 整合性を明文化し、
類似禁止と共通パスワード辞書のみを追加する。

## 決定

1. **採用する control**
   - 長さ: `min_length=12` / `max_length=128`（既存）。
   - ユーザー識別子との類似禁止: `preferred_username` / `email` / `email`
     の local-part を case-insensitive に substring 比較。識別子長 4 文字
     未満は誤検知回避のためチェック対象外。
   - 共通パスワード辞書: バンドル小規模リスト
     (`src/authentication/usecases/common-passwords.ts`) を case-insensitive
     に lookup。

2. **採用しない control（理由付き）**
   - **文字種混在 (composition rule)**: NIST §3.1.1.2 が明示的に非推奨。
     ユーザーに `P@ssw0rd!` のような予測しやすいパターンを誘発し、長さや
     エントロピーの実効向上にほぼ寄与しない。組織・規制要件で必要になった
     場合は将来別 ADR で opt-in flag を導入する。
   - **periodic rotation（定期変更強制）**: NIST §3.1.1.2 が同じく非推奨。
   - **パスワード履歴の再利用禁止**: 直近 N 件のハッシュ保管は新規 port
     `PasswordHistoryRepository` が必要で、change-password エンドポイント
     未実装の現状ではテスト経由でしか検証できない。change-password を
     導入する Phase で別 ADR と共に追加する。

3. **外部漏洩データベース検査は別 port**
   - HIBP k-anonymity 等は `BreachedPasswordChecker` port を経由する
     （SCL `password_policy.description` に明記）。本 ADR の bundled
     辞書は offline / 即時に弾けるベースラインであり、外部知識は port に
     委ねる。

4. **適用経路（現状）**
   - `validatePassword(plain, context?)` は demo seed (`bootstrap/seed.ts`)
     から呼ばれる。registration / change-password / admin user-create
     エンドポイントは現時点で未実装であり、それらが追加された時点で同じ
     関数を経由させる（適用経路追加時に SCL `description` を更新）。

## 影響

- `password-policy.ts` のシグネチャに optional `context?: { username?; email? }`
  が増える。既存呼び出し（seed）は context を渡すよう更新。
- 共通パスワード辞書ファイル `common-passwords.ts` を追加。中身は port では
  なく Layer 3 の定数 — `BreachedPasswordChecker` port とは目的が異なる
  （前者は offline baseline、後者は外部知識）。
- `bootstrap/seed.ts` のデフォルト demo password を `alice-password` から
  `demo-password-1234` に変更（前者は新ポリシーで `similar_to_identifier`
  違反となるため）。dev.sh が既に渡している値と一致する。

## 却下した代替案

- **大規模辞書 (rockyou / SecLists) を bundle**: 配布バイナリサイズと
  メモリ消費に対して効果が薄い。HIBP k-anonymity への port で代替するほうが
  カバレッジ・更新性とも優れる。
- **類似閾値に Levenshtein / sequence-matcher を採用**: 実装コストが増える
  割に substring containment より誤検知が減るとは限らない。識別子長下限を
  4 文字に切る単純な containment で実用上の lower-hanging fruit を取る。
- **文字種要件を opt-in flag として今同時に導入**: 「未要求の flexibility」
  に当たる。要件が現れた時に別 ADR で追加する。
