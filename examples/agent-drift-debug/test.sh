#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EXAMPLE_DIR="$ROOT/examples/agent-drift-debug"
TMP="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP"
}
trap cleanup EXIT

cd "$ROOT"
go build -o "$TMP/inferctl" ./cmd/inferctl
PATH="$TMP:$PATH"

"$EXAMPLE_DIR/validate-fixtures.sh" \
  "$EXAMPLE_DIR/fixtures/last-good.snapshot.json" \
  "$EXAMPLE_DIR/fixtures/today.snapshot.json" \
  "$EXAMPLE_DIR/fixtures/config-change-before.snapshot.json" \
  "$EXAMPLE_DIR/fixtures/config-change-after.snapshot.json"

inferctl diff \
  --before "$EXAMPLE_DIR/fixtures/last-good.snapshot.json" \
  --after "$EXAMPLE_DIR/fixtures/today.snapshot.json" \
  --json >"$TMP/reachability-drift.json"

jq -e '.data.changes[] | select(.type == "selected_route" and .before == "llamacpp_large/qwen-coder-32b.gguf" and .after == "ollama_small/qwen3:8b")' "$TMP/reachability-drift.json" >/dev/null
jq -e '.data.changes[] | select(.type == "fallback_status" and .before == false and .after == true)' "$TMP/reachability-drift.json" >/dev/null
jq -e '.data.changes[] | select(.type == "backend_reachability" and .subject == "llamacpp_large" and .before == "reachable" and .after == "unreachable:backend_unreachable")' "$TMP/reachability-drift.json" >/dev/null
jq -e '.data.changes[] | select(.type == "loaded_model_count" and .before == 2 and .after == 1)' "$TMP/reachability-drift.json" >/dev/null
jq -e '.data.changes[] | select(.type == "recommended_action" and .after == "inferctl backends --filter llamacpp_large --json")' "$TMP/reachability-drift.json" >/dev/null

inferctl diff \
  --before "$EXAMPLE_DIR/fixtures/config-change-before.snapshot.json" \
  --after "$EXAMPLE_DIR/fixtures/config-change-after.snapshot.json" \
  --json >"$TMP/config-drift.json"

jq -e '.data.changes[] | select(.type == "selected_route" and .before == "ollama_small/qwen3:8b" and .after == "llamacpp_large/qwen-coder-32b.gguf")' "$TMP/config-drift.json" >/dev/null
if jq -e '.data.changes[] | select(.type == "backend_reachability")' "$TMP/config-drift.json" >/dev/null; then
  echo "config-change fixture should not depend on backend reachability drift" >&2
  exit 1
fi

inferctl diff \
  --before "$EXAMPLE_DIR/fixtures/last-good.snapshot.json" \
  --after "$EXAMPLE_DIR/fixtures/today.snapshot.json" \
  >"$TMP/reachability-drift.txt"
grep -F "Local inference drift detected" "$TMP/reachability-drift.txt" >/dev/null
grep -F "Route changed:" "$TMP/reachability-drift.txt" >/dev/null
grep -F -- "- before: qwen-coder-32b.gguf on llamacpp_large" "$TMP/reachability-drift.txt" >/dev/null
grep -F -- "- after:  qwen3:8b on ollama_small" "$TMP/reachability-drift.txt" >/dev/null

echo "agent drift fixtures ok"
