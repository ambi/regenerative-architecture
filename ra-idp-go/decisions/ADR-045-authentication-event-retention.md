# ADR-045: 認証イベントの保持期間と sweep による削除

## ステータス

採用。[[wi-44-authentication-event-store-and-search]]。[[ADR-041]] が定めた認証イベントの
2 系統モデルに対し、**種類別の保持期間と削除メカニズム**を確定する。[[ADR-036]] (user 削除と
匿名化)・[[ADR-046]] (PII ポリシー) と整合させる。

## コンテキスト

認証イベントは単調増加する。partition 化なしの単一テーブルでは、保持期間を定めず溜め続けると
検索 index が効かなくなり、PII を必要以上に長く抱えるコンプライアンス上の問題も生む。一方、
成功ログイン履歴は「いつもと違うログイン」の調査のため一定期間は残す必要があり、失敗詳細や
セッション記録は短期で十分という非対称がある。種類ごとに根拠ある保持期間を決め、確実に削除する。

## 決定

1. **種類別の既定保持期間**。
   - 成功イベント (`success`): **365 日**。過去 1 年の正規ログイン傾向を調査可能にする。
   - 失敗詳細 (`fail` の個別行): **30 日**。攻撃の短期調査に足り、平文粒度の PII を長く持たない。
   - bucket 集約 (`aggregated`): **90 日**。攻撃の発生事実と規模を四半期スパンで残す。
   - セッション記録 (`sessions`): **90 日**。失効済みセッションの調査窓。
   - MFA チャレンジイベント: **90 日**。

2. **tenant override と global cap**。各保持期間は `TenantSettings` で短縮・延長できるが、
   global cap (`max_retention_days`) を上限とし、どの tenant も cap を超えて保持できない。
   cap はコンプライアンス上の「保持しすぎない」上限として運用する。

3. **削除は時間単位 cron の sweep**。`internal/bootstrap` の周期 job が、種類ごとに
   `occurred_at < now - retention` の行を削除する。sweep は idempotent で、1 回の実行で
   全件削除できなくても次回で収束するよう上限件数つきバッチで回す。index (`tenant_id`,
   `occurred_at`) が当たることを前提にする。

4. **impersonation は短縮不可**。`SessionImpersonationStarted` / `SessionImpersonationEnded`
   は「admin が user として操作した事実」であり、本人保護のため tenant override による**短縮の
   対象外**とする ([[ADR-041]] / [[ADR-046]])。global cap までは保持し、それ未満へは縮めない。

5. **partition / cold storage は本 WI では行わない**。declarative partitioning や cold storage
   archive は将来の最適化として方針のみ残し、本 WI は retention sweep + index で運用する。
   そのため admin 検索は**必ず期間絞り込みを要求**し、全期間スキャンを禁じる (検索性能の担保)。

## 影響

- retention sweep が確実に動くこと・index が当たること・admin 検索が期間絞り込みを要求すること
  をテストで確認する (境界 29 / 31 / 91 日)。
- [[ADR-036]] の user 削除 (anonymize cascade) と独立に動く。user 削除は sub に紐づく行を
  匿名化し、retention sweep は時間でまとめて消す。両者の二重適用で不整合が出ないよう、sweep は
  匿名化済み行もそのまま時間条件で削除する。

## 却下した代替案

- **全種類一律の保持期間**: 成功履歴は長く・失敗詳細は短くという非対称を表現できず、PII を
  必要以上に持つか、調査窓が足りなくなる。
- **削除しない (無限保持)**: 検索 index の劣化と PII コンプライアンスの両面で不可。
- **アプリ起動時のみの一括削除**: 長時間稼働で溜まり続ける。時間単位 cron で継続的に削る。
