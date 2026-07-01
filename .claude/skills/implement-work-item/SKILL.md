---
name: implement-work-item
description: Implement a chosen Regenerative Architecture work item end to end — inner layers to outer, tests per layer, then UI, verify green, add completion, move to done, commit. Use when starting to build a selected work item, or when the user asks to implement / build a work item — e.g. "implement wi-NN", "wi-NN を実装して", "wi-NN をやって", "ワークアイテムを実装". Companion to scl-change (spec first).
---

# ワークアイテムの実装フロー

対象ワークアイテムが決まってから、SCL の内層から外層・UI・完了記録・コミットまでを
一定の順序で回す。**手順を毎回考え直さない**——この順序と検証ゲートに従う。

## 0. コンテキスト衛生（最初に決める）

大きなトークン消費と思考の遅さは、たいてい「無駄な全文読み」と「1 セッションに文脈を
積み過ぎ」で起きる。着手前に次を徹底する。

- **メタドキュメントは全文で読まない。** RA / SCL は節単位のリファレンス。必要な節だけを
  `rg '^#{2,3} ' <file>` で探し、その行域だけ読む（節マップは `CLAUDE.md §1`）。
- **feature の現状仕様は該当 `scl.yaml` だけ読む**（例 `ra-idp-go/spec/scl.yaml`）。
  リポジトリ全体を舐めない。
- **コードベース横断の探索はサブエージェント（`Explore`）に委譲し、結論だけ受け取る。**
  検索の生出力で本スレッドの文脈を埋めない。
- **規模が大きければ計画をファイル化する。** ワークアイテムに `plan`（層ごとのチェック
  リスト）を追記するか、`work-items/` 配下にスクラッチのプランを置き、1 層ずつ潰す。
- **層の区切りは自然なコンパクト点。** 1 つの層を実装・検証してグリーンにしたら、
  必要に応じ `/clear` して次の層に入ると、文脈が単調増加しない。

## 1. 内層から外層へ（各層でテストを書く）

RA の 5 層（`REGENERATIVE_ARCHITECTURE.md §3`）を内側から。**先に SCL、後で実装**。

1. **Spec Core (SCL)** — `scl-change` Skill に従い `scl.yaml` を先に更新。触れた節を
   ワークアイテムの `scope` に列挙。`just yaml-check` → `just scl-render` で派生物を再生成。
2. **Decision Record** — 非自明な設計判断があれば `new-adr` Skill で ADR を残す。
3. **Application Logic** — ドメイン／ユースケース実装。単体テストを同時に書く。
4. **Adapter Layer** — HTTP / 永続化などの境界実装 + テスト。
5. **Runtime & Infra** — 設定・配線・デプロイ関連（必要なら）。
6. **UI** — 最後。React + Tailwind + Radix/shadcn（`CLAUDE.md §2` Default Tooling）。

各層を終えたらその層のテストを green にしてから次へ進む（層をまたいで未検証を溜めない）。

## 2. 検証ゲート（全部グリーンで完了）

コマンドは `just`（`justfile` = 人間 / AI 共通のコマンドマップ）を使う。

- SCL / YAML: `just yaml-check`
- Go: `just verify-go`（lint + race テスト）
- UI: `just verify-ui`（format / lint / typecheck / build）
- 一括: `just verify`

## 3. 完了処理（手順 5〜6）

1. ワークアイテムに `completion` を追記し `status: completed` にする。証跡の粒度は
   `new-work-item` Skill §1.3 に従う。
2. ファイルを `work-items/done/`（または該当コンテキストの `.../work-items/done/`）へ移す。
   id は変えない。
3. `commit` Skill でコミット（Conventional Commits・subject / body とも英語）。
   ユーザから指示があるまで push しない。
