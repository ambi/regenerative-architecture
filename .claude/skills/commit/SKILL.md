---
name: commit
description: Commit the current working-tree changes with a proper Conventional Commits message (English subject + multi-line English body). Use when the user asks to commit — e.g. "commit", "コミットして", "これをコミット". Splits into multiple commits when the diff mixes clearly unrelated concerns.
---

# 変更のコミット

現在の作業ツリーの変更を、Conventional Commits に従ってコミットする。
push はしない（指示があるまで）。

## 手順

1. **変更を把握する。** `git status` と `git diff`（staged / unstaged 両方）、未追跡ファイルを確認する。
2. **意味でグルーピングする。**
   - 差分が**意味的に全く別の関心事**を複数含むなら、必要に応じて**複数コミットに分割**する
     （例: 機能追加 + 無関係なツール設定変更）。
   - **多少の関連違い程度なら 1 コミットにまとめてよい。** 過剰に刻まない。
   - 分割はファイル / パス単位でステージする（`git add <paths>`）。hunk 単位の対話的
     分割（`git add -p`）はこの環境では使わない。
3. **各コミットのメッセージを書く**（下記書式）。**subject も body も英語。**
4. `git commit` する。1 コミットにまとめるなら全変更をステージ、分割するならグループごとに
   ステージ→コミットを繰り返す。
5. コミット後、`git log --oneline -n <件数>` で結果を確認して報告する。

## メッセージ書式（Conventional Commits）

**必ず英語。1 行で終わらせず、本文で *why* を複数行の詳細さで書く。**

```
type(scope): summary

<空行>
Why this change is needed / what problem it solves (not a restatement of the diff).
- key change 1
- key change 2
```

- `type`: `feat` `fix` `docs` `refactor` `chore` `test` `perf` `build` `ci` `style`
  から。破壊的変更は `!` を付ける（例 `feat(ra-idp)!: ...`）。
- `scope`: 変更したコンテキスト / モジュール（例 `ra-idp-go`, `tools`, `scl`）。
- **subject ≤ 72 文字**、命令形。
- body は *what* ではなく *why*。何を変えたかは diff が語る。
- **subject を英語にして body を日本語にする間違いを繰り返さない。両方英語。**

## 注意

- **attribution はリポジトリ設定（`.claude/settings.json`）に従う。**
  `Co-authored-by` などのフッターを手動で足さない。
- 現在のブランチにそのままコミットする（このリポジトリは既定ブランチ直コミット運用）。
- 生成物と仕様は同じコミットに含める（spec ↔ 派生物の同期を保つ、`scl-render` Skill 参照）。
