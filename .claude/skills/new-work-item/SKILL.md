---
name: new-work-item
description: Create a new Regenerative Architecture work item under work-items/ following the canonical format. Use when starting a new unit of work, a task, or when the user asks to draft/open a work item or wi-NN.
---

# 新規ワークアイテムの作成

正本書式は `CHANGE_RECORD_FORMAT.md §1`。**既存ファイルを開いて書式を逆算しない**——
本 Skill と §1 の記法に従う。既存ファイルは「似た題材の中身」を見たいときだけ開く。

## 手順

1. **配置先を決める**（§1.1）
   - 特定コンテキストの作業 → そのコンテキスト配下の `work-items/`（例 `ra-idp-go/work-items/`）。
   - リポジトリ全体の規約・横断ツール・複数コンテキスト跨ぎ → ルートの `work-items/`。
2. **id を採番する**
   - 既存の最大連番を確認: `ls work-items work-items/done ra-idp-go/work-items ra-idp-go/work-items/done 2>/dev/null`
   - `<id>` = `wi-<連番>-<kebab-title>`。ファイル名は `work-items/<id>.yaml`。
3. **未着手・進行中は `work-items/` 直下に置く**。完了・中止になったら `done/` サブディレクトリへ移す（id は変えない）。
4. 機能変更なら **触れる SCL セクションを `scope` に列挙する**。判定は SCL-first の網羅表に従う（`scl-change` Skill / `SPECIFICATION_CORE_LANGUAGE.md §3` 冒頭）。
5. 下記スケルトンを埋める。`motivation` は **Why を書く（What ではない）**。
6. **検証**: `just yaml-check-work-items` を通す。

## スケルトン

```yaml
id: wi-NN-kebab-title       # ファイル名と一致する安定識別子。^[a-z0-9][a-z0-9-]*$
title: 一文で表す意味変更
created_at: YYYY-MM-DD      # 起票日（今日の日付）
authors: [name]            # 1 名以上
status: pending             # pending | in_progress | completed | cancelled
motivation: |               # なぜこの変更が必要か（Why）
  ...
scope: { ... }              # 対象範囲。機能変更なら触れる SCL セクションを含める
out_of_scope: [ ... ]       # 明示的にやらないこと
initial_context: { ... }    # 任意。AI が作業開始時に読む最小文脈（feature 名と feature ディレクトリ優先）
affected_guarantees: [ ... ] # 影響する保証義務（SCL assurance の文脈）
verification: [ ... ]       # 予定する検証コマンド / 手動手順
risk: low                   # low | medium | high | critical
risk_notes: |
  ...
# status を completed / cancelled にする時点で completion を追記し、done/ へ移す（§1.3）
```

## 完了時（§1.3）

`status` を `completed` / `cancelled` にする時点で同じファイルに `completion` を追記し、
`work-items/done/` へ移す。`completion` は最低でも `completed_at` / `summary` / `verification` と
`affected_guarantees_state` を持つ。証跡は `completion.evidence` に手順・実行環境・実行主体・
対象ソース版・結果・保存先・要約値を記録し、大容量ログ・バイナリ・機密は埋め込まず
`evidence[].artifacts` に保管先とハッシュだけ残す。
