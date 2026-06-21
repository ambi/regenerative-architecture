# ワークアイテムの文脈取得

ワークアイテム YAML は正本だが、AI はまず次だけを読む。

1. `motivation`
2. `scope`
3. `out_of_scope`
4. `initial_context`
5. `affected_guarantees`
6. `verification`
7. `risk`

大きな `completion` や証跡は、監査履歴や過去の検証結果が必要なときだけ辿る。
`initial_context` では長いファイル列挙より `features` と feature ディレクトリを
優先し、ファイルパスは feature に同居できていない例外的な入口に限る。
