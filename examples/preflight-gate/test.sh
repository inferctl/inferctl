#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP="$(mktemp -d)"
PIDS=()
cleanup() {
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" >/dev/null 2>&1 || true
  done
  rm -rf "$TMP"
}
trap cleanup EXIT

cd "$ROOT"
go build -o "$TMP/inferctl" ./cmd/inferctl
PATH="$TMP:$PATH"

PORT_A=$((27000 + $$ % 1000))
PORT_B=$((PORT_A + 1))
PORT_C=$((PORT_A + 2))

go run ./cmd/infer-testserver -addr "127.0.0.1:${PORT_A}" -kind ollama -models "qwen3:8b" -loaded "qwen3:8b" >"$TMP/primary.log" 2>&1 &
PIDS+=("$!")
go run ./cmd/infer-testserver -addr "127.0.0.1:${PORT_B}" -kind ollama -models "fallback:8b" -loaded "fallback:8b" >"$TMP/fallback.log" 2>&1 &
PIDS+=("$!")
go run ./cmd/infer-testserver -addr "127.0.0.1:${PORT_C}" -kind ollama -models "other:8b" >"$TMP/other.log" 2>&1 &
PIDS+=("$!")

for _ in $(seq 1 80); do
  if curl -fsS "http://127.0.0.1:${PORT_A}/api/version" >/dev/null 2>&1 &&
    curl -fsS "http://127.0.0.1:${PORT_B}/api/version" >/dev/null 2>&1 &&
    curl -fsS "http://127.0.0.1:${PORT_C}/api/version" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

cat >"$TMP/prompt.txt" <<'EOF'
UNIQUE_SECRET_PROMPT_TEXT should never appear in preflight JSON or Markdown.
EOF

write_config() {
  local path="$1"
  local backend_url="$2"
  local primary="$3"
  local fallback="${4:-}"
  cat >"$path" <<EOF
[meta]
schema_version = "0.1"

[profile]
name = "preflight_gate"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.fixture]
kind = "ollama"
base_url = "$backend_url"
default = true

[routing.code]
model = "$primary"
backend = "fixture"
fallback = [${fallback}]
EOF
}

write_config "$TMP/primary.toml" "http://127.0.0.1:${PORT_A}" "qwen3:8b"
INFERCTL_CONFIG="$TMP/primary.toml" inferctl preflight code --prompt-file "$TMP/prompt.txt" --json >"$TMP/primary.json"
jq -e '.ok == true' "$TMP/primary.json" >/dev/null
jq -e '.data.runnability.exit_code == 0' "$TMP/primary.json" >/dev/null
jq -e '.data.route_decision.selected_model == "qwen3:8b"' "$TMP/primary.json" >/dev/null
jq -e '.data.route_decision.is_fallback == false' "$TMP/primary.json" >/dev/null
jq -e '.data.recommended_action.command == "inferctl route code --json"' "$TMP/primary.json" >/dev/null

INFERCTL_CONFIG="$TMP/primary.toml" inferctl preflight code --prompt-file "$TMP/prompt.txt" --format markdown >"$TMP/primary.md"
grep -F 'Runnability: `runnable`' "$TMP/primary.md" >/dev/null
grep -F 'Readiness: configured=`true` reachable=`true` ready=`true`' "$TMP/primary.md" >/dev/null
if grep -F "UNIQUE_SECRET_PROMPT_TEXT" "$TMP/primary.json" "$TMP/primary.md" >/dev/null; then
  echo "preflight output leaked prompt content" >&2
  exit 1
fi
if grep -F "$TMP/prompt.txt" "$TMP/primary.json" "$TMP/primary.md" >/dev/null; then
  echo "preflight output leaked local prompt path" >&2
  exit 1
fi

write_config "$TMP/fallback.toml" "http://127.0.0.1:${PORT_B}" "primary:70b" '"fallback:8b"'
INFERCTL_CONFIG="$TMP/fallback.toml" inferctl preflight code --prompt-file "$TMP/prompt.txt" --allow-fallback --json >"$TMP/fallback-allowed.json"
jq -e '.ok == true' "$TMP/fallback-allowed.json" >/dev/null
jq -e '.data.route_decision.selected_model == "fallback:8b"' "$TMP/fallback-allowed.json" >/dev/null
jq -e '.data.route_decision.is_fallback == true' "$TMP/fallback-allowed.json" >/dev/null
jq -e '.warnings[] | select(.code == "W_FALLBACK_USED")' "$TMP/fallback-allowed.json" >/dev/null

if INFERCTL_CONFIG="$TMP/fallback.toml" inferctl preflight code --prompt-file "$TMP/prompt.txt" --json >"$TMP/fallback-blocked.json" 2>"$TMP/fallback-blocked.err"; then
  echo "expected fallback block to exit non-zero" >&2
  exit 1
fi
jq -e '.errors[0].code == "E_PREFLIGHT_POLICY_BLOCKED"' "$TMP/fallback-blocked.json" >/dev/null
jq -e '.errors[0].exit_code == 5' "$TMP/fallback-blocked.json" >/dev/null
jq -e '.data.route_decision.is_fallback == true' "$TMP/fallback-blocked.json" >/dev/null
grep -F 'exit: 5' "$TMP/fallback-blocked.err" >/dev/null

write_config "$TMP/no-route.toml" "http://127.0.0.1:${PORT_C}" "primary:70b" '"fallback:8b"'
if INFERCTL_CONFIG="$TMP/no-route.toml" inferctl preflight code --prompt-file "$TMP/prompt.txt" --json >"$TMP/no-route.json" 2>"$TMP/no-route.err"; then
  echo "expected no route to exit non-zero" >&2
  exit 1
fi
jq -e '.errors[0].code == "E_NO_ROUTE_AVAILABLE"' "$TMP/no-route.json" >/dev/null
jq -e '.errors[0].exit_code == 5' "$TMP/no-route.json" >/dev/null
jq -e '.data.runnability.status == "readiness_blocked"' "$TMP/no-route.json" >/dev/null

cat >"$TMP/invalid.toml" <<'EOF'
[meta]
schema_version = 1
EOF
if INFERCTL_CONFIG="$TMP/invalid.toml" inferctl preflight code --prompt-file "$TMP/prompt.txt" --json >"$TMP/invalid.json" 2>"$TMP/invalid.err"; then
  echo "expected invalid config to exit non-zero" >&2
  exit 1
fi
jq -e '.errors[0].code == "E_CONFIG_INVALID"' "$TMP/invalid.json" >/dev/null
jq -e '.errors[0].exit_code == 3' "$TMP/invalid.json" >/dev/null

echo "preflight gate scenario ok"
