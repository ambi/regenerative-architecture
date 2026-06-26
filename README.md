# Regenerative Architecture — リポジトリ概要

このリポジトリは **REGENERATIVE_ARCHITECTURE.md** に記述されている「Regenerative Architecture」というソフトウェア設計・開発手法を説明・デモするためのリポジトリです。

## 構成

```text
REGENERATIVE_ARCHITECTURE.md   ← 手法の説明文書（日本語）
SPECIFICATION_CORE_LANGUAGE.md ← 仕様核言語の説明文書（日本語）
ra-idp-go/                     ← RA で構成した IdP アプリケーション
tools/                         ← RA のツール
```

## 基本コマンド

このリポジトリでは、モノレポ内の正しい作業ディレクトリを間違えないように、
ルートから `just ...` を実行する。`justfile` は人間と AI エージェントのための
コマンド地図であり、各レシピは意図ベースの名前と固定された作業ディレクトリを持つ。

まず利用可能な入口を見る:

```bash
just
```

よく使うコマンド:

```bash
just setup         # Bun 依存関係をインストール
just verify        # 標準検証一式
just verify-go     # ra-idp-go の lint + race test
just verify-ui     # UI の lint + typecheck + build
just verify-tools  # RA tools と YAML/SCL 検証
just yaml-check    # work item / SCL YAML 検証
just scl-render    # SCL HTML 成果物を再生成
just dev-api       # Go API 開発サーバー
just dev-ui        # UI 開発サーバー
```

AI エージェントは、変更前に `REGENERATIVE_ARCHITECTURE.md` と
`SPECIFICATION_CORE_LANGUAGE.md` を読み、変更後は影響範囲に応じて最小の
`just ...` 検証を実行する。迷う場合の標準入口は `just verify`。
