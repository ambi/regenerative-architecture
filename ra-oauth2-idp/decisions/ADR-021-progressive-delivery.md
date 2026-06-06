# ADR-021: Progressive delivery — Argo Rollouts canary + SLO-aware 自動ロールバック

## ステータス

採用

## コンテキスト

`spec/slo.yaml availability.token_endpoint.target = 0.9995` を満たすには、
新版デプロイの 100% 即時切替 (`Recreate` / 通常の RollingUpdate) では
**配信中の壊れたコードが全ユーザーに当たる時間**が長すぎる。

Progressive delivery (canary / blue-green / shadow) により、
小規模に新版を晒し、SLO 違反を検出したら自動的に旧版に戻す。

DORA の 4 指標 (REGENERATIVE_ARCHITECTURE.md §6 で参照) のうち
「変更失敗率」と「MTTR」を直接改善する。

## 決定

### 1. Argo Rollouts を採用

理由:
- Kubernetes native (CRD)
- Analysis Template で Prometheus メトリクス (本アプリが Phase 2 で出す)
  を SLO 閾値と比較し自動ロールバック
- 既存の Service / Deployment と互換 (lift-and-shift)

代替案: Flagger (Linkerd ベース、サービスメッシュ前提なので除外)、
Spinnaker (運用負荷が重い)。

### 2. Canary ステップ

```text
deploy v2
  → 5%  for 3 min  + analysis (oauth2_token p99 < SLO, error rate < SLO)
  → 25% for 3 min  + analysis
  → 50% for 5 min  + analysis
  → 100% (auto-promote)
```

analysis 失敗 → 即時 v1 に戻す + RefreshTokenReuseDetected 同等の即時通知。

### 3. analysis に使うメトリクス

`spec/slo.yaml` 由来:

- `oauth2_token_request_duration_seconds:p99 < slo.performance.endpoints.token.p99_latency_ms / 1000`
- `rate(oauth2_token_requests_total{result="error"}[5m]) / rate(oauth2_token_requests_total[5m]) < slo.performance.endpoints.token.error_rate_max`
- `rate(oauth2_refresh_token_reuse_detected_total[1m]) == 0` (絶対 0)

これらは Phase 2.5 で生成される alerts.yaml と同じ式を Analysis Template が使う。

### 4. ロールバック後の手順

- 監査ログに `RolloutAborted` を記録 (新規イベント型を `spec/events.schema.json` に
  追加するかは Phase 3 実装時に判断、当面は OTel 構造化ログでカバー)
- 自動 issue 作成 (GitHub Actions の repository_dispatch)
- インシデント retro を 24 時間以内に書く (本 ADR の外で運用ルール化)

## 却下した代替案

- **Blue-Green**: トラフィックの段階開放ができず、SLO ベース判定の解像度が低い
- **手動 canary**: SLO 違反を発見するのは人間で良いが、ロールバック判断を人に委ねる
  と MTTR が悪化する
- **shadow traffic only**: 副作用 (refresh token 発行など) があるユースケースに不向き

## 影響

- `infra/k8s/base/rollout.yaml` を追加 (Deployment → Rollout)
- `infra/k8s/base/analysistemplate.yaml` を追加
- `prod` overlay でのみ Rollout を有効化、`dev` overlay は Deployment のまま
- `.github/workflows/release.yaml` で kustomize の image tag を更新

## 関連

- ADR-019 (Runtime: Kubernetes)
- `spec/slo.yaml` (availability / performance)
- REGENERATIVE_ARCHITECTURE.md §6 (DORA 指標)
