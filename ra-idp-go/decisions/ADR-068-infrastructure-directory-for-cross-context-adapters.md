# ADR-068: 横断アダプタ実装を internal/infrastructure に置く

## ステータス
採用。Go import path、README、ADR-047 に反映。SCL の規範振る舞いは変更しない。

## コンテキスト
ra-idp-go の `internal/platform/` は、OS や CPU などのプラットフォーム差分ではなく、
DB、暗号、通知、観測性、イベント送信、ポリシー、HTTP router/core といった
コンテキスト横断の Layer 4 アダプタ実装を収容していた。

`platform` は Go や一般的な技術語彙では OS/実行基盤を連想しやすく、Clean Architecture /
DDD / RA の「外側のアダプタ実装」という責務を十分に表さない。加えて、Go の `internal/`
ディレクトリは非公開 import 境界を表す機構であり、ソースルート名の代替ではない。

## 決定
1. `ra-idp-go/internal/platform/` を `ra-idp-go/internal/infrastructure/` にリネームする。
2. `internal/infrastructure/` は Layer 4 のコンテキスト横断アダプタ実装だけを表す。
3. `deploy/` は migrations、Docker Compose、OTel Collector など Layer 5 の実行環境・配布資材を表す。
4. `internal/` は維持する。Go compiler が module 外からの import を拒否する境界として使うためであり、`src/` へ置き換えない。
5. 振る舞い、SCL、HTTP route、DB schema、公開 API は変更しない。

## 却下した代替案
- `internal/platform/` を維持する: 既存 import 変更は不要だが、OS platform との語義衝突が残る。
- `internal/shared/`: 共有物であることしか表さず、Layer 4 アダプタ実装という責務を示さない。
- `internal/sharedinfrastructure/`: 責務は表すが、Go import path として読みづらい。
- `internal/boundary/`: 外部境界という意味は近いが、DB・暗号・通知などの実装置き場として一般性が低い。
- `src/`: Go modules では module root が source root であり、`src` は import 境界を提供しない。
- `pkg/`: 外部 module に公開する Go library API を表しやすく、ra-idp-go の内部実装を公開したい意図と誤読される。

## 影響
- Go import path は `ra-idp-go/internal/infrastructure/...` へ変わる。
- README のディレクトリ構成は `internal/infrastructure/` と `deploy/` の責務差を明記する。
- top-level の `infra/` は `deploy/` へ改名する。
- 既存 work item に残る `internal/platform/...` 参照は、その時点の監査記録として扱い、今回の名称変更では書き換えない。
