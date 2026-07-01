#!/usr/bin/env bash
set -euo pipefail

PROMPT_FILE="${1:-sample-pr-review-prompt.txt}"
TASK="${INFERCTL_PREFLIGHT_TASK:-code_review}"

inferctl preflight "$TASK" \
  --prompt-file "$PROMPT_FILE" \
  --format markdown
