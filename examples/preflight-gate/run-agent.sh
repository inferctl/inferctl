#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROMPT_FILE="${PROMPT_FILE:-$DIR/sample-task.txt}"
AGENT_CMD=("$@")
if [[ "${AGENT_CMD[0]:-}" == "--" ]]; then
  AGENT_CMD=("${AGENT_CMD[@]:1}")
fi
if [[ "${#AGENT_CMD[@]}" -eq 0 ]]; then
  AGENT_CMD=(python agent.py)
fi

inferctl preflight code --prompt-file "$PROMPT_FILE" --allow-fallback
exec "${AGENT_CMD[@]}"

