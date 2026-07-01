# ADR-074: Define HTTP request cancellation policy

## ステータス
採用。`spec/contexts/system.yaml` の `objectives.CancellationConsistency` / `objectives.ClientAbortLogClassification` と `internal/shared/adapters/http/support` に反映。

## コンテキスト
Go の HTTP request context は client disconnect、HTTP/2 cancel、handler return などで cancel される。read-only endpoint では速やかな中断として望ましいが、IdP の mutation では authorization code redemption、refresh token rotation、consent accept、admin mutation、key rotation、audit/outbox emit が中途半端に止まる危険がある。

また `context.Canceled` は利用者やネットワーク都合の client abort でも発生するため、そのまま server error log や 5xx error rate に混ぜると運用上の誤検知になる。

## 決定
1. Discovery、JWKS、UserInfo、Introspect、admin list/get/export など read-only endpoint は request context をそのまま使い、client abort で中断してよい。
2. Mutation endpoint は `operation context` を使う。基本は `context.WithTimeout(context.WithoutCancel(requestCtx), operationTimeout)` とし、tenant、issuer、trace、actor など request context value は保持する。shutdown や無期限実行を避けるため timeout は必須とする。
3. client abort で rollback すべき endpoint は request context を使い続ける。ただしその場合も ADR または handler comment で rollback policy を明示する。
4. `/token` の authorization_code、refresh_token、client_credentials、device_code、token-exchange、`/par`、`/device_authorization`、browser login/consent/totp/change-password/logout、password reset、admin clients/users/groups/tenants/keys/consents/settings は mutation として扱う。代表実装から operation context を適用し、残りは同じ helper へ寄せる。
5. commit 後に必須の audit/outbox/security cleanup は request cancel から切り離した detached context で完了させる。detached completion には短い timeout を設定し、失敗は `operation_detached_completion_failures_total` 相当で観測する。
6. `context.Canceled` かつ request context が client abort で閉じた場合は `client_aborted` と分類し、server error として扱わない。`context.DeadlineExceeded` は request context の deadline なら `server_timeout`、別の upstream/DB 操作由来なら `upstream_timeout` とする。
7. abort metrics は `http_request_aborts_total{kind=client_aborted|server_timeout|upstream_timeout}` の label で増やす。client abort は access log では 499 相当として扱えるが、OAuth/browser response body は既存 contract を優先する。

## 却下した代替案
- 全 handler で request context を使い続ける: read-only には自然だが、mutation の commit 後処理が client abort に巻き込まれる。
- 全 mutation を `context.Background()` に置き換える: tenant / issuer / trace / actor など request context value を失い、timeout なしの処理を作りやすい。
- client abort を 5xx として記録する: server fault ではない事象が availability alert に混ざり、実障害の検知精度を落とす。
- endpoint ごとの個別 helper を作る: policy が散り、将来の handler 追加時に分類と metrics がずれやすい。

## 影響
- SCL に cancellation consistency と client abort log classification の security policy objective を追加する。
- HTTP support layer に operation context、detached context、cancel classification、abort metrics hook を追加する。
- Mutation handler は request context を直接渡す代わりに policy helper で operation context を作る。
- Read-only handler は request context のままとし、client disconnect による中断を server error として扱わない。
