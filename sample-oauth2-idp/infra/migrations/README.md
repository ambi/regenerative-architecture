# Schema Migrations — Layer 5 Operational Asset

## 位置づけ

このディレクトリは **Layer 5 (Runtime & Infrastructure) の運用補助** である。

SPECIFICATION_CORE_LANGUAGE.md §8 が定める通り、SCL は現時点のモデル定義のみを保持し、
バージョン間の変遷履歴は持たない。データマイグレーションの構成要素は次のように分解される:

- **変更の意図 / 後方互換 / 段階展開の方針** → ADR (第2層)
- **旧スキーマから新スキーマへの変換ロジック** → 第3層 (Application Logic) または第4層 (Adapter)
- **実際の SQL 適用** → 本ディレクトリ (Layer 5 の運用補助)

ここで管理する SQL は特定の DB エンジン (PostgreSQL) に依存した運用補助物であり、
SCL モデルと整合する義務だけが残る。整合性は `infra/scripts/check-spec-coherence.ts` が
SCL から派生した JSON Schema と SQL のカラム名を照合して検証する。

## ファイル命名規則

```
0001_init.sql
0002_<change-summary>.sql
0003_<change-summary>.sql
...
```

各ファイルは 4 桁ゼロパディングの連番。連番は **不変**。

## スキーマ進化のルール

### 1. 加法的変更を原則とする

- カラム追加 → OK (NULL 許容 or DEFAULT 付き)
- インデックス追加 → OK
- 新規テーブル → OK
- カラム削除 → 二段階 (まず deprecated コメント + アプリケーション側で参照停止、
  次のリリースで物理削除)
- カラム型変更 → 原則禁止。新規カラム追加 + バックフィル + 旧カラム deprecation で代替
- カラム名変更 → 原則禁止。view または新規カラム + DEFAULT 同期で代替

### 2. 破壊的変更には ADR を伴う

ADR には以下を必須記載:

- 変更の動機
- 影響を受ける既存データの量と移行手順
- ロールバック計画
- 影響を受ける SCL モデル (`spec/scl.yaml`) の差分
- 影響を受ける retention 関連 objective の値

### 3. SCL モデルとの整合

`spec/scl.yaml` の `models.<Entity>.fields` キーと、SQL の `CREATE TABLE` のカラム名は
**1 対 1 対応** しなければならない。整合性は CI で機械検証する
(`infra/scripts/check-spec-coherence.ts`)。

### 4. PII と retention の注釈

各カラムに以下のコメントを付けることで SCL の `annotations.pii` / `annotations.retention_days` /
`annotations.purge_on_deletion` と整合をとる:

```sql
preferred_username TEXT NOT NULL,
  -- pii: true
  -- retention_days: 2555
```

このコメントから、PII マスキング設定・GDPR の Right to Erasure バッチ・
監査ログの取り扱いを派生させる。

## マイグレーション適用

```bash
DATABASE_URL=postgres://... bun run migrate:up
DATABASE_URL=postgres://... bun run migrate:status
```

実装は `adapters/persistence/postgres/migrate.ts`。
`schema_migrations` テーブルで適用済み一覧を管理し、未適用のみを順次適用する。
