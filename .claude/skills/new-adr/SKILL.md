---
name: new-adr
description: Create a new Architecture Decision Record (ADR) following the canonical format. Use when recording an important decision and its rationale, or when the user asks to draft/write an ADR or ADR-NNN.
---

# 新規 ADR（決定記録）の作成

正本書式は `CHANGE_RECORD_FORMAT.md §2`。**既存ファイルを開いて書式を逆算しない**。
ADR は SCL から派生し SCL に反映される決定記録。

## 手順

1. **配置先を決める**（§2.1）
   - 対象コンテキストの近くの `decisions/` に置く（例 `ra-idp-go/decisions/`）。
2. **採番する**
   - 既存を確認: `ls ra-idp-go/decisions decisions 2>/dev/null`
   - ファイル名は `decisions/ADR-NNN-kebab-title.md`。`NNN` は 3 桁ゼロ詰め連番。
   - **連番は再利用しない。廃止した ADR も削除せず残す**（過去の決定経緯は再生成の文脈になる）。
3. 下記スケルトンの **5 つの必須章をこの順で**書く。「決定」と「却下した代替案」の間に
   補足トピック節（例 `## 鍵のサイズと曲線`）を置いてよいが、必須章は置き換えない。
4. 決定が SCL に反映されているときは、**「ステータス」または「影響」で対応する SCL 要素**
   （`interfaces.Xxx` / `models.Yyy` 等）とソースを相互参照する。
5. 廃止時は「ステータス」に置き換え先 ADR と日付を添える（例 `廃止（ADR-015 に置き換えられた、2026-03-01）`）。

## スケルトン

```markdown
# ADR-NNN: 一文で表す決定

## ステータス
採用。`scl.yaml` の `interfaces.Xxx` / `models.Yyy` と該当ソースに反映。

## コンテキスト
なぜ今この決定が要るか。制約・力学・先行する決定との関係。

## 決定
何を決めたか。番号付きで複数項目に分けてよい。

## 却下した代替案
- 案 A: なぜ採らないか。

## 影響
- 新規 / 変更される SCL 要素、契約、データ、運用。
```
