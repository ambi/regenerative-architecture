#!/usr/bin/env bash
# PostToolUse hook (Edit|Write): remind to verify/regenerate when spec or
# work-item files are edited, so scl.yaml <-> derived artifacts stay in sync.
# Reads the hook payload (JSON) on stdin; emits additionalContext for Claude.

set -euo pipefail

payload="$(cat)"
fp="$(printf '%s' "$payload" | jq -r '.tool_input.file_path // .tool_input.path // empty')"

[ -z "$fp" ] && exit 0

msg=""
case "$fp" in
  *scl.yaml|*/spec/contexts/*.yaml)
    msg='SCL を編集しました。`just yaml-check-scl` で検証し、`just scl-render` で派生物（HTML / JSON Schema / OpenAPI）を再生成してください（drift を残さない）。scl.yaml に wi/ADR/commit 番号は書かないこと。'
    ;;
  work-items/*.yaml|*/work-items/*.yaml)
    msg='work-item を編集しました。`just yaml-check-work-items` で検証してください。書式の正本は CHANGE_RECORD_FORMAT.md §1。完了・中止にしたファイルは work-items/done/ へ移すこと。'
    ;;
  decisions/ADR-*.md|*/decisions/ADR-*.md)
    msg='ADR を編集しました。書式の正本は CHANGE_RECORD_FORMAT.md §2。連番は再利用せず、廃止 ADR も削除しないこと。決定が SCL に反映されているなら対応する SCL 要素を相互参照してください。'
    ;;
esac

[ -z "$msg" ] && exit 0

jq -n --arg ctx "$msg" \
  '{hookSpecificOutput: {hookEventName: "PostToolUse", additionalContext: $ctx}}'
