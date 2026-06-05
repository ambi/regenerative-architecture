# ADR-009: 署名鍵を 90 日ごとに回転し、旧鍵を JWKS に最低 7 日間残す

## ステータス

採用

## コンテキスト

JWT 署名鍵が無期限に同じものを使い続けると:

- 鍵漏洩時の被害が時間とともに累積する
- 暗号アルゴリズムの強度が将来低下したときに移行できない
- コンプライアンス要件（PCI / SOC2）に違反する場合がある

一方、回転を急ぎすぎると:

- JWKS キャッシュを持つクライアントが旧鍵で署名された有効トークンを検証できなくなる
- 運用負荷が増える

## 決定

以下の方針を採用する:

1. **アクティブ署名鍵の最大寿命: 90 日**
   `spec/slo.yaml` の `signing_key_max_age_days = 90`

2. **回転スケジュール: 7 日前から新鍵を JWKS に公開**
   `signing_key_min_jwks_overlap_days = 7`

3. **旧鍵の保持期間: 最も長寿命のトークン（ID Token = 3600 秒）の寿命以上**
   実運用上は JWKS から削除せず、別カラム `inactive` で管理し、検証用途にのみ残す

4. **緊急回転（漏洩時）: 即座に新鍵に切り替え、旧鍵を即時 revoked にする**
   旧鍵で署名された全トークンの再検証を要求する（イントロスペクションで `active: false` を返す）

## 鍵 ID (kid) の管理

- 各鍵にユニークな `kid` を発行する（UUID）
- JWT ヘッダーの `kid` を見て JWKS から鍵を選ぶ
- `kid` のないトークンは検証拒否

## 鍵の保管

本サンプルではメモリ内 KeyStore に保持するが、本番では:

- KMS / HSM（AWS KMS, Google Cloud KMS, Azure Key Vault, HashiCorp Vault）
- 秘密鍵は IdP プロセスから取り出せない設計が望ましい（KMS API 経由で署名）

これらの判断は本 ADR を分岐させる新規 ADR（例: ADR-009a "KMS 採用"）で記録する。

## 監査イベント

鍵回転時には `SigningKeyRotated` 監査イベントを発行する。
これは `spec/events.schema.json` に定義済み。

## 却下した代替案

- **30 日ごと回転**: 運用負荷が高い。漏洩を 30 日サイクルで防ぐより、漏洩検出と
  即時回転のインシデント応答に投資するほうが ROI が高い
- **年 1 回**: 漏洩時の被害ウィンドウが大きすぎる
- **回転しない**: アンチパターン

## 影響

- `KeyStore` ポートは `getActiveKey()` / `getKey(kid)` / `getJwks()` を提供する必要がある
- `/jwks` エンドポイントはアクティブ + 旧鍵を返す
- 鍵生成スクリプトは `spec/discovery.json` の
  `id_token_signing_alg_values_supported` と整合する鍵タイプを生成する
- 監査ログ保持期間（`spec/slo.yaml` `signing_key_archive_days = 2555`）は、
  「旧鍵で署名された監査トークンを将来検証できる」要件のための値
