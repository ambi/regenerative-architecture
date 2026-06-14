# ADR-028: 漏洩パスワード検査ポートと HIBP k-anonymity 採用

## ステータス

採用（Phase 0 — 認証の土台。`spec/scl.yaml` `objectives.PasswordPolicy` と
`src/authentication/ports/breached-password-checker.ts` の双子に反映）。

## コンテキスト

ADR-026 で「HIBP 等の外部漏洩データベース検査は `password_policy` とは別に
`BreachedPasswordChecker` port を経由する」と明文化したが、実装は ADR-027 と同時に
保留していた。本 ADR は port の存在意義・採用 adapter・失敗時挙動を定める。

外部漏洩データベース検査は、bundled common-password 辞書（offline / 即時）が拾えない
**過去に大規模流出に含まれた具体的なパスワード文字列** を弾くために必要である。
NIST SP 800-63B-4 §3.1.1.2 は「subscriber chooses a password, the verifier SHALL compare
the prospective secrets against a list that contains values known to be commonly-used,
expected, or compromised」と要求する。bundled 辞書は最小限の baseline、
外部知識は port に委ねる。

## 決定

1. **port 形状**: `isBreached(plain: string): Promise<boolean>` の 1 メソッド。
   - 戻り値は二値で十分。検査ヒット数 / 漏洩元の詳細は本 IdP の判定には使わない
     （NIST は count 閾値を要求していない）。将来 count を返す要件が来たら別 port
     を切る。
   - 引数は plain（生パスワード）。adapter 内部で必要な hash 変換を閉じる。
     port 層で SHA-1 にしてしまうと別 adapter（pwnedpasswords 互換以外）の
     導入余地が消える。

2. **デフォルト adapter は Noop**
   - `NoopBreachedPasswordChecker` を memory モードと in-memory テストの既定とする。
   - 「外部依存が無い in-memory 起動でも login / change-password が動く」は ADR-016
     の永続化アダプタ選定方針と同じ可逆性原則。

3. **本番 adapter は HIBP Range API**
   - `https://api.pwnedpasswords.com/range/{SHA1-prefix5}` への GET。
   - k-anonymity: 生パスワードの SHA-1 を取り、先頭 5 文字だけサーバに送る。
     サーバは同一 prefix のすべての suffix を返す。adapter はその中に
     残り 35 文字 suffix と count > 0 で一致するエントリがあるかを比較する。
   - レスポンスフォーマットは `SUFFIX:COUNT\r\n` の繰り返し。count > 0 で
     `isBreached = true`。
   - リクエストヘッダ `Add-Padding: true` を常に付け、レスポンスサイズ
     side-channel を抑える（HIBP の推奨）。

4. **失敗時挙動（fail-open）**
   - HTTP error / timeout / network failure は **breached = false** を返す。
     - 外部サービス停止で IdP の change-password が全停止するリスクを避ける。
       漏洩検査は password-policy のうち bundled 辞書・長さ・履歴と独立に動く
       追加レイヤーであり、片肺で運用継続できる設計にする。
     - log/metric は adapter 内で記録するが、usecase 層には伝播しない。
   - timeout は 2 秒（HIBP は通常 100 ms 以下）。adapter コンストラクタで
     上書き可能。

5. **適用経路（本 ADR 時点）**
   - `change-password` usecase: bundled policy 通過後に checker を呼ぶ。
     違反時は `PasswordPolicyError(['breached'])` で他の policy 違反と同じ
     エラー語彙に乗せる。
   - registration: 現状未実装。実装される時点で同じ port を再利用する。
   - `bootstrap/seed.ts`: 起動経路では呼ばない（デモ用パスワードを将来 HIBP
     が検出した場合に起動失敗するのを避ける）。

6. **将来のテナント別 / opt-out（Phase 4）**
   - テナント別ポリシー導入時、`breached_check_enabled` を `PasswordPolicyResolver`
     の戻り値に追加して checker 呼び出しを skip できるようにする。本 ADR では
     enabled 一択。

## 影響

- 新 port `BreachedPasswordChecker`（`isBreached` 1 メソッド）。
- 新 adapter 2 種:
  - `NoopBreachedPasswordChecker`（既定 / memory）
  - `HibpBreachedPasswordChecker`（外部 API / opt-in）
- `password-policy.ts` に `breached` 違反語彙と `validatePasswordAsync` を追加。
  既存 `validatePassword`（同期）は seed / 純検査用に残す。
- `change-password.ts` の DI に `breachedPasswordChecker` を追加。
- `bootstrap/dependencies.ts` で `BREACHED_PASSWORD_CHECKER=hibp` のとき HIBP adapter、
  それ以外は Noop を返す（未指定が既定）。

## 却下した代替案

- **HIBP 結果の数値 (count) を usecase に持ち込む**: 閾値判断を usecase に
  漏らすと「いくつまでなら許すか」の policy 表現が分散する。port は二値、
  count 閾値判定は adapter 内に閉じる。

- **失敗時に fail-closed（変更を拒否）**: 外部サービス障害で IdP の
  change-password が利用不能になる。漏洩検査は補強レイヤであり、blocking 化
  すると可用性を犠牲にする。可用性側に倒し、検査失敗は監査ログで補完する。

- **bundled 辞書を巨大化して外部 API を不要にする**: ADR-026 で却下済み
  （配布サイズ・更新性で劣る）。HIBP 採用で本 ADR は閉じる。

- **plain password 全体を外部に送る**: k-anonymity プロトコル違反。
  HIBP Range API は prefix 5 文字送信が前提。
