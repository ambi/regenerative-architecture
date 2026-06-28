# ADR-047: Adapter Layer をコンテキスト所有へ（層×コンテキスト格子）

## ステータス

採用。ただし、コンテキスト横断 Layer 4 アダプタ実装のディレクトリ名は
ADR-068 により `internal/platform/` から `internal/infrastructure/` へ置き換えられた
（2026-06-28）。

RA §3.6（境界づけられたコンテキスト）を Layer 4 まで適用し、ディレクトリ構造を
「(層 × コンテキスト) の格子」へ揃える。UI 側で先行した `ui/src/features/` 分割
（commit 3dd65b0）の Go 側対応。後続だった http の per-context 分割は
[[wi-48-http-handler-per-context-split]] で完了済み。

## コンテキスト

ra-idp-go は Layer 3（domain / ports / usecases）こそ `tenancy` / `authentication` /
`oauth2` の3コンテキストに分割済みだったが、Layer 4 のアダプタが技術レイヤ単位で全
コンテキスト混在していた。

- 旧 `internal/adapters/http`: 57ファイル・約9,800行が**単一 `Deps` 構造体のメソッド**として同居
- 旧 `internal/adapters/persistence/{memory,postgres}`: `memory.go`(1045行)・`postgres.go`(776行)に
  全コンテキストのリポジトリが混在
- 旧 `internal/adapters/{crypto,observability,notification,eventsink,policy}`: 横断インフラ

1機能を読むのに http / usecase / port / persistence の複数箇所を辿る必要があり、AI への
文脈投入単位が「機能」に閉じない（shotgun surgery）。RA §3.6 は「トップでコンテキスト分割し、
各コンテキスト内部で5層を繰り返す。境界づけられたコンテキストは再生成と AI への文脈投入の
自然な単位」と規定する。

## 決定

1. **境界づけられたコンテキストを最上位の垂直軸とする**。`tenancy` / `authentication` /
   `oauth2` を `internal/<context>/` 直下に置き、各コンテキストが自身の Layer 3 を所有する
   （既存どおり）。依存方向は `oauth2`(基底) ← `authentication`、`tenancy`(独立) で非循環。

2. **コンテキスト横断の Layer 4 アダプタは `internal/infrastructure/` に集約する**。
   `crypto` / `observability` / `notification` / `eventsink` / `policy` を移設。これらは
   どのコンテキストにも固有でない共有アダプタ実装である。

3. **永続化アダプタは `internal/infrastructure/persistence/` に置き、リソース別ファイルへ分割する**。
   実装は全コンテキストへ bootstrap が一様に配線する共有テストダブル / アダプタ実装であり、
   コンテキスト境界は各 `ports` が担保する。1045行の `memory.go` と776行の `postgres.go` を
   `tenants.go` / `clients.go` / `users.go` / `sessions.go` … のように所有コンテキストを明記した
   リソース別ファイルへ carve し、共有鍵ヘルパ（`tenantKey` / `defaultTenant` / `rowScanner` /
   接続・migrate）のみ `helpers.go` / `base.go` に残す。
   - per-context **パッケージ**分割は採らない。memory/postgres は ~50ファイル（横断テストを
     含む）から参照され、3パッケージに割るとエイリアス import とクロスコンテキストテスト依存が
     増え、純構造変更にしては回帰リスクが高いため。

4. **http アダプタもコンテキスト所有とする**。当初は移行コスト（共有 `Deps` God-struct の
   分解、約60個のヘルパの横断/固有の仕分け、未公開ヘルパを白box呼び出しするテストの移行）から
   `internal/infrastructure/http` の単一パッケージのままとし [[wi-48-http-handler-per-context-split]]
   へ切り出したが、同 WI で完了した。各コンテキストが `internal/<context>/adapters/http` で
   自身のハンドラを所有し、依存集約 `core.Deps`・テナント解決 middleware・横断ヘルパは
   `internal/infrastructure/http/core` が持つ。`internal/infrastructure/http` は各コンテキストの
   `RegisterRoutes` を束ねる router に縮約した。依存方向は core ← 各コンテキスト http ← router。

5. **挙動は不変**。本 ADR の変更は純粋なファイル・ディレクトリ移動であり、SCL・spec
   バインディング・ビジネスロジック・HTTP ルーティングは変更しない。`go build` / `go test` /
   `golangci-lint run` のグリーンを各段階で確認した。

## 結果

- 横断アダプタ実装と永続化が現行の `internal/infrastructure/` 配下に集約された。
- ADR-047 以前の「全コンテキストの HTTP handler と横断アダプタが混在する旧 `internal/adapters/`」は消滅した。
- 各コンテキストの Layer 3 は従来どおり `internal/<context>/` に閉じる。
- http の per-context 局所化は [[wi-48-http-handler-per-context-split]] で完了し、
  1機能のハンドラ・ユースケース・ポートが1コンテキストディレクトリ配下に揃った。
