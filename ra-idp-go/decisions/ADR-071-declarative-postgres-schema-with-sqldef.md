# ADR-071: PostgreSQL schema は sqldef で宣言的に管理する

## ステータス

採用。`deploy/schema/postgres.sql` を PostgreSQL adapter の現在形 schema とし、
`deploy/migrations/`、`schema_migrations`、アプリ起動時 migration runner は廃止する。

## コンテキスト

`ra-idp-go/deploy/migrations/` は番号付き SQL ファイルを自前 runner
(`internal/shared/adapters/persistence/postgres/migrate.go`) で順に適用している。
この方式は単純で監査しやすいが、schema が成長すると現在の望ましい形が履歴の総和に
埋もれる。空 DB を作るだけでも過去の `ALTER TABLE`、index 作り直し、列削除を
順に実行するため、PostgreSQL adapter の現在形を再生成する成果物として読みづらい。

Regenerative Architecture では外側 adapter は再生成可能であるべきで、PostgreSQL の
物理 schema は SCL や ADR から導かれる派生物である。一方、データ移行の手順や
危険操作の承認条件は機械的に導出できない運用判断を含むため、明示的な記録が必要である。

## 決定

1. PostgreSQL schema の現在形を `deploy/schema/postgres.sql` に置く。
   このファイルは `CREATE TABLE` / `CREATE INDEX` / 制約定義で構成し、過去の
   中間状態を含めない。
2. schema 差分の計画と適用には `psqldef` を使う。通常はデプロイ前ジョブで
   `--dry-run` をレビューし、承認後に `--apply` する。アプリケーション起動時に
   外部 CLI として `psqldef` を呼ばない。
3. 既存の `deploy/migrations/` と Go runner は削除する。`AUTO_MIGRATE` /
   `MIGRATIONS_DIR` は設定としても廃止する。
4. 開発 Docker 環境では compose の一回限りの `schema` サービスが PostgreSQL 起動後に
   `psqldef --apply --file /schema/postgres.sql` を実行し、その完了後に `idp` を起動する。
5. データ移行は宣言的 schema に混ぜない。列分割、値変換、参照データ投入、削除前の
   backfill などは WI に紐づく runbook または目的別 SQL script として保存する。
6. 参照データは schema 正本に含めない。たとえば default tenant は起動時の
   `EnsureDefault` によって収束させる。
7. 構造変更を行う WI は、原則として `deploy/schema/postgres.sql` を先に更新し、
   既存環境に必要なデータ移行がある場合だけ別の runbook / SQL script を追加する。

## 却下した代替案

- Atlas Pro checkpoint:
  現在形 schema と履歴の短絡という目的には合うが、checkpoint は Pro 機能であり、
  この段階で商用機能を運用前提にしない。
- Atlas OSS のみ:
  差分生成・drift 検出には使えるが、checkpoint なしでは空 DB を現在形から直接作る
  問題を単独で解決しない。必要になれば補助ツールとして再検討する。
- Prisma Migrate:
  Go + pgx + 明示 SQL の adapter に対して Prisma schema という別の正本候補を増やす。
  PostgreSQL 固有の index、制約、JSONB 利用も raw SQL へ逃げやすく、本プロジェクトの
  adapter replaceability には過剰である。
- Ent / GORM への寄せ替え:
  ORM を永続化 adapter の中心に据えるなら選択肢になるが、migration 目的だけで導入すると
  repository SQL と ORM model の二重管理になる。
- 現行 runner のまま migration squash を手動管理:
  空 DB の効率は改善できるが、現在形 schema を宣言的に読む体験が弱い。

## 影響

- `deploy/schema/postgres.sql` が PostgreSQL schema の現在形 artifact になる。
- `deploy/migrations/`、`schema_migrations`、`internal/shared/adapters/persistence/postgres/migrate.go`
  は削除する。
- README と ARCHITECTURE は、PostgreSQL 構造変更時に schema 正本を更新する規約へ変わる。
- CI / デプロイでは、将来的に `psqldef --dry-run` による drift check を追加する。
- アプリケーションコードは引き続き pgx と明示 SQL を使い、ORM は導入しない。
