#!/usr/bin/env bash
set -euo pipefail

PROMPT_FILE="${PROMPT_FILE:-sample-pr-review-prompt.txt}"
TASK="${INFERCTL_PREFLIGHT_TASK:-code_review}"
OUTPUT="${INFERCTL_PREFLIGHT_MARKDOWN:-inferctl-preflight.md}"

inferctl preflight "$TASK" \
  --prompt-file "$PROMPT_FILE" \
  --format markdown >"$OUTPUT"

printf 'wrote inferctl preflight Markdown to %s\n' "$OUTPUT"
