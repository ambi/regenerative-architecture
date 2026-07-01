# ADR-070: technical shared context for cross-context adapters

## ステータス
採用。README と Go import path に反映。SCL の規範振る舞いは変更しない。

## コンテキスト
ra-idp-go は RA §3.6 の「層 × 境界づけられたコンテキスト」の格子に合わせて、
`authentication` / `oauth2` / `tenancy` などの context 配下に `domain` / `ports` /
`usecases` / `adapters` を置いている。

一方で、暗号、永続化、通知、観測性、イベント送信、AuthZEN policy、HTTP 共有基盤は
複数 context から使われるため、ADR-068 では `internal/infrastructure/` に集約していた。
しかし `infrastructure` は RA Layer 5 の Runtime & Infrastructure と語義が近く、
Go 実装上は Layer 4 adapter implementation であることが読み取りにくい。

`internal/adapters/` のようなトップレベル名は、各 context の `adapters/` と二重化して
「どちらが Adapter Layer か」を曖昧にする。`shared/` は DDD の Shared Kernel と誤読される
余地があるため、単なる共有物置き場にはせず、`spec` と `adapters` に層を切った
technical shared context として扱う。

HTTP については Go の import cycle が制約になる。全体 router は各 context の
`adapters/http` を import する一方、各 context の HTTP adapter は依存集約、tenant middleware、
response helper を必要とする。そのため router と共有基盤は同一 package にできない。

## 決定
1. `shared` は business bounded context ではなく technical shared context とする。
   SCL Go binding と、コンテキスト横断の Layer 4 adapter implementation を所有する。
2. 旧 `internal/spec/` を `internal/shared/spec/` へ移す。これは SCL Go binding であり、
   複数 context が参照する共通の内側に属する。
3. 旧 `internal/infrastructure/{crypto,eventsink,notification,observability,persistence,policy}` を
   `internal/shared/adapters/{crypto,eventsink,notification,observability,persistence,policy}` へ移す。
   ただし、2026-07-02 時点で persistence 配下にある context 固有 repository 実装は
   実装上の利便性を優先した暫定配置であり、business bounded context の所有物として
   per-context adapter へ移す余地がある。詳細は ADR-047 の永続化アダプタ補足を参照する。
4. HTTP の横断実装は `internal/shared/adapters/http/` 配下で役割別に分ける。
   `support/` は各 context の HTTP adapter が使う `Deps`、tenant middleware、CSRF、response helper を持つ。
   `server/` は Echo router と health endpoint を持ち、各 context の `RegisterRoutes` を集約する。
   依存方向は `support <- context adapters/http <- server` とする。
5. 旧 `internal/validation` は削除する。`Error(z.ZogIssueList)` は Zog 依存を隠していないため、
   現時点では利用箇所の private 関数に畳む。

## 却下した代替案
- `internal/infrastructure/` を維持する: Clean Architecture では一般的な名前だが、RA Layer 5 と混同しやすい。
- `internal/adapters/` に集約する: 各 context の `adapters/` と二重になり、構造上の主軸が曖昧になる。
- `internal/shared/` 直下に全共有物を平置きする: DDD の Shared Kernel と誤読されやすく、
  責務が「共有」に寄りすぎる。
- `internal/shared/adapters/http` に router と共有基盤を同居させる: Go の package cycle を生む。
- `internal/spec` を維持する: import path は短いが、shared が adapters だけを持つように見え、
  common inner layer と cross-context adapters の対応が読み取りにくい。
- `internal/spec` を `specbinding` に改名する: 正確ではあるが import path が長く、頻出 package として読みにくい。

## 影響
- Go import path は `ra-idp-go/internal/shared/spec` と
  `ra-idp-go/internal/shared/adapters/...` へ変わる。
- `internal/shared/adapters/http/support` と `internal/shared/adapters/http/server` の分離により、
  context-owned HTTP adapter と全体 router の循環依存を避ける。
- `internal/validation` は消滅し、Zog error 整形は利用箇所の private 関数になる。
- 振る舞い、SCL、HTTP route、DB schema、公開 API は変更しない。
