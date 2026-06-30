# Regenerative Architecture

このリポジトリは、**Regenerative Architecture (RA)** というソフトウェア設計・開発手法と、その実例をまとめた作業場です。

RA の中心にある考え方は、実装やランタイムを固定資産として守るのではなく、**仕様・決定・検証可能な変更記録を保存し、外側の実装や環境を再生成できるようにする**ことです。詳しい思想は [REGENERATIVE_ARCHITECTURE.md](REGENERATIVE_ARCHITECTURE.md) にあります。

## このリポジトリにあるもの

```text
REGENERATIVE_ARCHITECTURE.md   RA の考え方と層構造
SPECIFICATION_CORE_LANGUAGE.md 仕様核言語 SCL の定義
CHANGE_RECORD_FORMAT.md        ワークアイテムと ADR の正本フォーマット
ra-idp-go/                     RA に従って構成した IdP 実装
tools/                         SCL/ワークアイテム検証と派生成果物のツール
work-items/                    リポジトリ横断の変更管理レコード
justfile                       ルートから実行するコマンド地図
```

## 最初に読むもの

RA の全体像を知りたい場合は、次の順で読むのが短いです。

1. [REGENERATIVE_ARCHITECTURE.md](REGENERATIVE_ARCHITECTURE.md): 何を保存し、何を再生成可能にするか。
2. [SPECIFICATION_CORE_LANGUAGE.md](SPECIFICATION_CORE_LANGUAGE.md): 仕様核をどう書き、どう検証対象にするか。
3. [ra-idp-go/README.md](ra-idp-go/README.md): 実例の IdP がどのように構成されているか。
4. [tools/README.md](tools/README.md): 仕様・変更記録・派生成果物を扱うツール群。

新しいワークアイテムや ADR を作るときは、既存ファイルから書式を推測せず、[CHANGE_RECORD_FORMAT.md](CHANGE_RECORD_FORMAT.md) を正本として使います。

## 開発の入口

このリポジトリでは、モノレポ内の作業ディレクトリを間違えないように、ルートから `just ...` を実行します。各レシピは目的ベースの名前を持ち、内部で正しいディレクトリへ移動します。

利用可能な入口を見る:

```bash
just
```

依存関係を入れる:

```bash
just setup
```

標準検証を実行する:

```bash
just verify
```

よく使う個別コマンド:

```bash
just verify-go     # ra-idp-go の lint + race test
just verify-ui     # UI の format check + lint + typecheck + build
just verify-tools  # RA tools と YAML/SCL 検証
just yaml-check    # work item / SCL YAML 検証
just scl-render    # SCL 由来の HTML / JSON Schema / OpenAPI を再生成
just dev-api       # Go API 開発サーバー
just dev-ui        # UI 開発サーバー
just dev-compose   # Docker Compose 開発スタック
```

## 変更するときの基本ルール

- 機能や振る舞いを変える場合は、実装より先に SCL を見直します。
- 重要な判断は ADR に残します。決定の理由や却下した代替案も保存対象です。
- 一つの意味変更はワークアイテムとして扱い、根拠・範囲・検証・残リスクを記録します。
- 生成物を更新した場合は、対応する生成コマンドと検証結果を残します。
- 変更後は影響範囲に応じて最小の `just ...` 検証を実行します。迷う場合は `just verify` が標準入口です。

## AI エージェント向けの最小コンテキスト

作業前に [REGENERATIVE_ARCHITECTURE.md](REGENERATIVE_ARCHITECTURE.md) と [SPECIFICATION_CORE_LANGUAGE.md](SPECIFICATION_CORE_LANGUAGE.md) を読み、機能変更では SCL-first を守ってください。新規ワークアイテムと ADR の書式は [CHANGE_RECORD_FORMAT.md](CHANGE_RECORD_FORMAT.md) に従います。

README は入口として小さく保ちます。詳細な仕様・判断・検証手順は、該当する SCL、ADR、ワークアイテム、各サブディレクトリの README に置いてください。
