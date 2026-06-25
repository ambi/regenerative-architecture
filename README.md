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
ルートから `mise run ...` を実行する。

```bash
mise run setup         # Bun 依存関係をインストール
mise run verify        # 標準検証一式
mise run verify:go     # ra-idp-go の lint + race test
mise run verify:ui     # UI の lint + typecheck + build
mise run verify:tools  # RA tools と YAML/SCL 検証
mise run yaml-check    # work item / SCL YAML 検証
mise run dev:api       # Go API 開発サーバー
mise run dev:ui        # UI 開発サーバー
```
