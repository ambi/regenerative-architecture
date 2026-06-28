# Change Record Format

Regenerative Architecture が要求する**変更管理レコード**——ワークアイテムと決定記録
（ADR）——の正本フォーマットを定義する。ここに書かれた書式がマスターであり、機械検証用の
スキーマ（既定では JSON Schema）はこの文書からの派生物として扱う。

`REGENERATIVE_ARCHITECTURE.md` はこれらのレコードを概念として規定し（決定記録は §3.2、
ワークアイテムは §4.2 / §4.2.1）、本文書はその**記法**を定める。SCL に対する
`SPECIFICATION_CORE_LANGUAGE.md` と同じ役割を、変更管理レコードに対して担う。別プロジェクトで
別の書式を採るときは、本文書だけを差し替える。

新規にレコードを書くときは、**既存ファイルを開いて書式を確認しない**。本文書の書式に従う。
既存ファイルは「似た題材の中身」を参照したいときだけ開く。

見出し名は記法上の固定要素である。本文書は日本語の見出しを正本とし、他言語プロジェクトは
構造を保ったまま翻訳してよい。

---

## 1 ワークアイテム

一つの意味変更として説明・実装・検証できる作業単位。正本は、課題管理サービスに依存しない
スキーマ検証可能なテキスト形式（既定では YAML）で、対象となる境界づけられたコンテキストの
近くに保存する。

### 1.1 配置と命名

- ファイル名は `work-items/<id>.yaml`。`<id>` は `wi-<連番>-<kebab-title>`。
- 特定コンテキストの作業はそのコンテキスト配下の `work-items/` に置き、リポジトリ全体の規約・
  横断ツール・複数コンテキストにまたがる作業だけをルートの `work-items/` に置く。

### 1.2 フィールド

| フィールド | 必須 | 内容 |
| --- | --- | --- |
| `id` | ✓ | ファイル名と一致する安定識別子。`^[a-z0-9][a-z0-9-]*$` |
| `title` | ✓ | 一文で表す意味変更 |
| `created_at` | ✓ | 起票日。`YYYY-MM-DD` または RFC3339 |
| `authors` | ✓ | 1 名以上 |
| `status` | ✓ | `pending` \| `in_progress` \| `completed` \| `cancelled` |
| `motivation` | ✓ | なぜこの変更が必要か（What ではなく Why） |
| `scope` | ✓ | 対象範囲。object / array / string 可。機能変更なら触れる SCL セクションを含める |
| `out_of_scope` | ✓ | 明示的にやらないこと |
| `affected_guarantees` | ✓ | 影響する保証義務（SCL assurance の文脈） |
| `verification` | ✓ | 予定する検証コマンド / 手動手順 |
| `risk` | ✓ | `low` \| `medium` \| `high` \| `critical` |
| `risk_notes` | ✓ | リスクの根拠と軽減 |
| `initial_context` | – | AI が作業開始時に読む最小文脈（`features` / `scl` / `decisions` / `tests` / `stop_before_reading`）。ファイル列挙より feature 名と feature ディレクトリを優先 |
| `target_state` | – | Refactor 系で「整合後の状態」を宣言する場合に使う |
| `completion` | △ | `status` を `completed` / `cancelled` にする時点で必須。下記 1.3 |

### 1.3 完了記録 `completion`

完了または中止の判断記録。`status` が `completed` / `cancelled` になった時点で、同じファイルに
追記する。少なくとも `completed_at`・`summary`・`verification` と、保証義務の状態
（`affected_guarantees_state`）を持つ。証跡は手順・実行環境・実行主体・対象ソース版・結果・
保存先・要約値を `completion.evidence` に記録し、大容量ログ・バイナリ・機密は埋め込まず
`evidence[].artifacts` に保管先とハッシュだけを残す。

### 1.4 スケルトン

```yaml
id: wi-NN-kebab-title       # ファイル名と一致する安定識別子
title: 一文で表す意味変更
created_at: 2026-01-01      # YYYY-MM-DD
authors: [name]
status: pending             # pending | in_progress | completed | cancelled
motivation: |               # なぜこの変更が必要か（What ではなく Why）
  ...
scope: { ... }              # 対象範囲。機能変更なら触れる SCL セクションを含める
out_of_scope: [ ... ]       # 明示的にやらないこと
initial_context: { ... }    # 任意。AI が作業開始時に読む最小文脈
affected_guarantees: [ ... ] # 影響する保証義務
verification: [ ... ]       # 予定する検証コマンド / 手動手順
risk: low                   # low | medium | high | critical
risk_notes: |
  ...
# status を completed / cancelled にする時点で completion を追記する
```

---

## 2 決定記録（ADR）

重要な決定とその理由を残すレコード。SCL から派生し SCL に反映される。配置はワークアイテムと
同じく対象コンテキストの近くの `decisions/` に置く。

### 2.1 配置と採番

- ファイル名は `decisions/ADR-NNN-kebab-title.md`。`NNN` は 3 桁ゼロ詰め連番。
- 連番は再利用しない。廃止した ADR も**削除せず**残す（過去の決定経緯は再生成の文脈になる）。

### 2.2 章構成

次の見出しをこの順で持つ。

1. **ステータス** — `採用` / `提案` / `廃止`。廃止時は置き換え先 ADR と日付を添える
   （例: `廃止（ADR-015 に置き換えられた、2026-03-01）`）。決定が SCL に反映されているときは、
   ここまたは「影響」で対応する SCL 要素とソースを相互参照する。
2. **コンテキスト** — なぜ今この決定が要るか。制約・力学・先行する決定との関係。
3. **決定** — 何を決めたか。番号付きで複数項目に分けてよい。
4. **却下した代替案** — 各案と、それを採らない理由。
5. **影響** — 変更される SCL 要素・契約・データ・運用など。

「決定」と「却下した代替案」の間に、決定を補足する任意のトピック節を置いてよい
（例: `## 鍵のサイズと曲線`）。トピック節は決定の一部であり、必須章を置き換えない。

### 2.3 スケルトン

```markdown
# ADR-NNN: 一文で表す決定

## ステータス
採用。`scl.yaml` の `interfaces.Xxx` / `models.Yyy` と該当ソースに反映。

## コンテキスト
なぜ今この決定が要るか。

## 決定
何を決めたか。

## 却下した代替案
- 案 A: なぜ採らないか。

## 影響
- 新規 / 変更される SCL 要素、契約、データ、運用。
```

---

機能変更で**どの SCL セクションを見直すか**は、SCL 側の関心であり
`SPECIFICATION_CORE_LANGUAGE.md` §3 冒頭の網羅チェックに従う（SCL-first）。
