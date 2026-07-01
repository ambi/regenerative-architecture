---
name: scl-change
description: SCL-first workflow for feature work. Use before implementing any feature or behavior change — update the spec (scl.yaml) first, decide which SCL sections to touch, then regenerate derived artifacts. Use when adding/changing a feature, endpoint, model, state, permission, or non-functional target.
---

# SCL-first 機能変更フロー

**実装より先に SCL を更新する。** コードを書いてから SCL を後追いさせない
（`SPECIFICATION_CORE_LANGUAGE.md §3` 冒頭）。

## 手順

1. **どの SCL セクションを見直すか判定する**（網羅表）。Yes の節を更新し、判定した節は
   ワークアイテムの `scope` に列挙する。

   | 変更のきっかけ | 見直す節 |
   | --- | --- |
   | 新しい用語・別名・翻訳・外部標準語が出る | `glossary` |
   | 集約・エンティティ・値・イベントの形や同一性が変わる | `models` |
   | 外部との契約（入力・出力・エラー・前後条件）が増減する | `interfaces` |
   | ライフサイクルの状態や許可される遷移が変わる | `states` |
   | 常に成り立つべき条件・liveness を足す | `invariants` |
   | Use Case と受け入れ例を足す（原則として常に） | `scenarios` |
   | 誰が何をできるか（認可ルール）が変わる | `permissions` |
   | 非機能目標（TTL・レイテンシ・上限など）を決める | `objectives` |
   | 画面・画面遷移・横断 UX 要件が変わる | `user_experience` |

2. **抜けやすい節を重点的に埋める。**
   - `scenarios`: 機能には必ず受け入れ例。正常系だけでなく**境界・失敗・拒否を最低 1 つずつ**。
   - `invariants`: 「壊れていない」と言える不変条件を 1 つ以上。property-based / model-check 可能な形を優先。
   - `permissions`: 認証・認可を伴う機能は、追加した操作の認可ルールを必ず書く。

3. **scl.yaml に wi/ADR/commit 番号を書かない。** scl.yaml は全層の最内で、純粋な仕様文に保つ。

4. **検証**: `just yaml-check-scl`

5. **派生物を再生成する**（drift を残さない）。`scl-render` Skill 参照、または:
   `just scl-render`

6. 実装はこの後。完了したらワークアイテムに `completion` を追記する（`new-work-item` Skill §1.3）。
