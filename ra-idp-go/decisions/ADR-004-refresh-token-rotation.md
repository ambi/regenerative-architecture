# ADR-004: リフレッシュトークンを毎回ローテーションし、再利用検出時にファミリー一括失効する

## ステータス

採用

## コンテキスト

リフレッシュトークンは（access_token と比較して）長寿命のため、漏洩した場合の被害が大きい。
RFC 9700 §4.14 は、特に public クライアント（SPA / ネイティブ）について、
ローテーションと再利用検出を強く推奨している:

> The authorization server MUST ... rotate the refresh token on every use.
> ... If a refresh token is presented that was already used, the authorization
> server MUST revoke all refresh tokens that were issued based on the
> originally issued refresh token.

## 決定

すべてのクライアント（public / confidential 問わず）に対してリフレッシュトークンの
ローテーションを必須とする。

具体的な実装:

1. **ローテーション**: `/token` で `grant_type=refresh_token` を成功させたとき、
   提示されたリフレッシュトークンを `rotated=true` とマークし、新しいリフレッシュトークンを
   発行する。新トークンの `parent_id` に旧トークンの ID を保持する。

2. **ファミリー識別子**: すべてのリフレッシュトークンは `family_id` を持つ。
   認可コード交換から派生したトークンチェーンは、同じ `family_id` を共有する。

3. **再利用検出とファミリー失効**: `rotated=true` のトークンが再提示されたら、
   - そのリクエストを `400 invalid_grant` で拒否する
   - 同じ `family_id` のすべてのトークン（祖先・子孫）を `revoked=true` にする
   - `RefreshTokenReuseDetected` 監査イベントを発行する
   - SRE / セキュリティチームに 60 秒以内に通知する（`spec/slo.yaml`）

4. **絶対期限**: `absolute_expires_at` を発行時に設定する（30 日固定）。
   ローテーションしても延長されない。

## 並行リフレッシュへの対応

ネットワーク上、まったく同じリフレッシュトークンを並行使用するクライアントは
正当ケースでも存在する（タブを 2 枚開いている SPA など）。
このため:

- **狭い時間窓（grace period: 5 秒）内の並行使用は片方のみ成功**させ、
  他方は同じ新トークンを返却する設計も選択肢にあるが、
  本アプリでは「常に片方成功・片方失効」とする。
- リプレイ攻撃と並行使用を区別しないことで、設計を単純に保つ。
- 並行クライアントは新トークンを取得し損ねた側がエラー後に再ログインすることで回復する。

## 却下した代替案

- **リフレッシュトークンを長寿命のままローテーションしない**: 漏洩時の被害が指数的に大きい
- **public クライアントだけローテーション**: confidential クライアントも侵入は起こりうる。
  「全クライアント一律」のほうが運用と監査が単純
- **5 秒 grace period**: 実装が複雑化。Valkey のような外部状態ストアが必要。
  本アプリは "single success, family-revoke on reuse" を採用

## 影響

- `RefreshTokenRecord` は `family_id` / `parent_id` / `rotated` / `revoked` フィールドを持つ
- リフレッシュトークン用ストアは「`family_id` での一括失効」を効率的にできる必要がある
- 監査ログは `RefreshTokenRotated` と `RefreshTokenReuseDetected` を区別する
- `requirements.md §5` と `scenarios.feature` にローテーション動作を明記する
