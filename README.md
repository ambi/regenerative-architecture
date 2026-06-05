# Regenerative Architecture — リポジトリ概要

このリポジトリは **REGENERATIVE_ARCHITECTURE.md** に記述されている
「Regenerative Architecture」というソフトウェア設計・開発手法を
説明・デモするためのリポジトリです。

## 構成

```text
REGENERATIVE_ARCHITECTURE.md   ← 手法の説明文書（日本語）
sample-task-tracker/           ← 手法を具体的に示すサンプル実装
```

## Regenerative Architecture とは

変化を前提とする時代（AIによる攻撃加速・競争速度加速・依存先の変化加速）において、
「何を保存しておけば、すべてを失っても再生成できるか」を設計の中心に置くアーキテクチャ手法。

Clean Architecture の依存性規則を継承しつつ、**再生成可能性**を一級市民とする。

5層構造：

1. **Specification Core（仕様核）** — 機械検証可能な仕様（状態機械・JSON Schema・不変条件テスト）
2. **Decision Record（決定記録）** — なぜそうしたかのADR
3. **Application Logic（アプリケーション論理）** — フレームワーク非依存のユースケース
4. **Adapter Layer（境界層）** — HTTP・DBなど、積極的に差し替える層
5. **Runtime & Infrastructure（実行環境とデリバリ）** — 土台（OS・コンテナ・クラウド）と本番への届け方（デプロイパイプライン）。ツールはコモディティだが、速く安全に届ける能力は一級の要件
