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
jq .data "$TMP/reachability-drift.json" >"$TMP/reachability-drift.data.json"
diff -u "$EXAMPLE_DIR/goldens/reachability-drift.json" "$TMP/reachability-drift.data.json"

jq -e '.data.changes[0].type == "selected_route"' "$TMP/reachability-drift.json" >/dev/null
jq -e '.data.changes[1].type == "fallback_status"' "$TMP/reachability-drift.json" >/dev/null
jq -e '.data.changes[] | select(.type == "selected_route" and .before == "llamacpp_large/qwen-coder-32b.gguf" and .after == "ollama_small/qwen3:8b")' "$TMP/reachability-drift.json" >/dev/null
jq -e '.data.changes[] | select(.type == "fallback_status" and .before == false and .after == true)' "$TMP/reachability-drift.json" >/dev/null
jq -e '.data.changes[] | select(.type == "backend_reachability" and .subject == "llamacpp_large" and .before == "reachable" and .after == "unreachable:backend_unreachable")' "$TMP/reachability-drift.json" >/dev/null
jq -e '.data.changes[] | select(.type == "loaded_model_count" and .before == 2 and .after == 1)' "$TMP/reachability-drift.json" >/dev/null
jq -e '.data.changes[] | select(.type == "recommended_action" and .after == "inferctl backends --filter llamacpp_large --json")' "$TMP/reachability-drift.json" >/dev/null

inferctl diff \
  --before "$EXAMPLE_DIR/fixtures/config-change-before.snapshot.json" \
  --after "$EXAMPLE_DIR/fixtures/config-change-after.snapshot.json" \
  --json >"$TMP/config-drift.json"
jq .data "$TMP/config-drift.json" >"$TMP/config-drift.data.json"
diff -u "$EXAMPLE_DIR/goldens/config-change-drift.json" "$TMP/config-drift.data.json"

jq -e '.data.changes[] | select(.type == "selected_route" and .before == "ollama_small/qwen3:8b" and .after == "llamacpp_large/qwen-coder-32b.gguf")' "$TMP/config-drift.json" >/dev/null
if jq -e '.data.changes[] | select(.type == "backend_reachability")' "$TMP/config-drift.json" >/dev/null; then
  echo "config-change fixture should not depend on backend reachability drift" >&2
  exit 1
fi

inferctl diff \
  --before "$EXAMPLE_DIR/fixtures/last-good.snapshot.json" \
  --after "$EXAMPLE_DIR/fixtures/today.snapshot.json" \
  >"$TMP/reachability-drift.txt"
diff -u "$EXAMPLE_DIR/goldens/reachability-drift.txt" "$TMP/reachability-drift.txt"
grep -F "Local inference drift detected" "$TMP/reachability-drift.txt" >/dev/null
grep -F "Route changed:" "$TMP/reachability-drift.txt" >/dev/null
grep -F -- "- before: qwen-coder-32b.gguf on llamacpp_large" "$TMP/reachability-drift.txt" >/dev/null
grep -F -- "- after:  qwen3:8b on ollama_small" "$TMP/reachability-drift.txt" >/dev/null

inferctl diff \
  --before "$EXAMPLE_DIR/fixtures/config-change-before.snapshot.json" \
  --after "$EXAMPLE_DIR/fixtures/config-change-after.snapshot.json" \
  >"$TMP/config-drift.txt"
diff -u "$EXAMPLE_DIR/goldens/config-change-drift.txt" "$TMP/config-drift.txt"

jq '.captured_at_iso = "2026-07-01T16:00:00Z"' "$EXAMPLE_DIR/fixtures/last-good.snapshot.json" >"$TMP/timestamp-only.snapshot.json"
inferctl diff \
  --before "$EXAMPLE_DIR/fixtures/last-good.snapshot.json" \
  --after "$TMP/timestamp-only.snapshot.json" \
  --json >"$TMP/timestamp-only.json"
jq -e '.data.summary.total == 0' "$TMP/timestamp-only.json" >/dev/null

printf '{not json\n' >"$TMP/malformed.snapshot.json"
if inferctl diff --before "$TMP/malformed.snapshot.json" --after "$EXAMPLE_DIR/fixtures/today.snapshot.json" --json >"$TMP/malformed.json" 2>"$TMP/malformed.err"; then
  echo "expected malformed snapshot to fail" >&2
  exit 1
fi
jq -e '.errors[0].code == "E_CONFIG_INVALID"' "$TMP/malformed.json" >/dev/null
jq -e '.errors[0].details.arg == "--before"' "$TMP/malformed.json" >/dev/null

jq '.snapshot_schema_version = "9.9"' "$EXAMPLE_DIR/fixtures/last-good.snapshot.json" >"$TMP/incompatible.snapshot.json"
if inferctl diff --before "$TMP/incompatible.snapshot.json" --after "$EXAMPLE_DIR/fixtures/today.snapshot.json" --json >"$TMP/incompatible.json" 2>"$TMP/incompatible.err"; then
  echo "expected incompatible snapshot to fail" >&2
  exit 1
fi
jq -e '.errors[0].code == "E_INVALID_ARG"' "$TMP/incompatible.json" >/dev/null
jq -e '.errors[0].details.arg == "snapshot_schema_version"' "$TMP/incompatible.json" >/dev/null

if grep -E 'UNIQUE_SECRET|Authorization|Bearer|api[_-]?key|auth[_-]?header|http://(localhost|127\.|10\.|172\.(1[6-9]|2[0-9]|3[0-1])|192\.168\.)|https://(localhost|127\.|10\.|172\.(1[6-9]|2[0-9]|3[0-1])|192\.168\.)|/Users/|/home/|/private/var/|/tmp/' \
  "$EXAMPLE_DIR"/goldens/* "$TMP/reachability-drift.txt" "$TMP/config-drift.txt" "$TMP/reachability-drift.data.json" "$TMP/config-drift.data.json" >/dev/null; then
  echo "diff golden output leaked prompt, credential-looking text, unsafe endpoint, or local path" >&2
  exit 1
fi

echo "agent drift fixtures ok"
