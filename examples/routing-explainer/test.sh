#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EXAMPLE_DIR="$ROOT/examples/routing-explainer"
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

PORT_A=$((28000 + $$ % 1000))
PORT_B=$((PORT_A + 1))
PORT_C=$((PORT_A + 2))
PORT_DEAD=$((PORT_A + 9))

go run ./cmd/infer-testserver -addr "127.0.0.1:${PORT_A}" -kind ollama -models "qwen3:8b" -loaded "qwen3:8b" >"$TMP/primary.log" 2>&1 &
PIDS+=("$!")
go run ./cmd/infer-testserver -addr "127.0.0.1:${PORT_B}" -kind ollama -models "qwen3:8b" -loaded "qwen3:8b" >"$TMP/fallback.log" 2>&1 &
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

cat >"$TMP/task.txt" <<'EOF'
Review this change for correctness risks and missing tests.
EOF
printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' >"$TMP/near.txt"
cat >"$TMP/prompt-redaction.txt" <<'EOF'
UNIQUE_SECRET_ROUTE_PROMPT_TEXT should never appear in routing output.
EOF

write_config() {
  local path="$1"
  local profile_tokens="$2"
  local backends="$3"
  local route="$4"
  cat >"$path" <<EOF
[meta]
schema_version = "0.1"

[profile]
name = "routing_explainer"
max_context_tokens = $profile_tokens
max_concurrent_models = 1
allow_premium = false
mode = "warn"

$backends

$route
EOF
}

write_config "$TMP/primary.toml" 8192 \
"[backends.ollama]
kind = \"ollama\"
base_url = \"http://127.0.0.1:${PORT_A}\"
default = true" \
"[routing.code]
model = \"qwen3:8b\"
backend = \"ollama\"
fallback = []"

write_config "$TMP/backend-unreachable.toml" 8192 \
"[backends.llamacpp_large]
kind = \"ollama\"
base_url = \"http://127.0.0.1:${PORT_DEAD}\"
default = true

[backends.ollama_small]
kind = \"ollama\"
base_url = \"http://127.0.0.1:${PORT_B}\"
default = false" \
"[routing.code]
model = \"qwen-coder-32b.gguf\"
backend = \"llamacpp_large\"
fallback = [\"qwen3:8b\"]"

write_config "$TMP/model-unavailable.toml" 8192 \
"[backends.ollama]
kind = \"ollama\"
base_url = \"http://127.0.0.1:${PORT_A}\"
default = true" \
"[routing.code]
model = \"primary:70b\"
backend = \"ollama\"
fallback = [\"qwen3:8b\"]"

write_config "$TMP/no-route.toml" 8192 \
"[backends.ollama]
kind = \"ollama\"
base_url = \"http://127.0.0.1:${PORT_C}\"
default = true" \
"[routing.code]
model = \"primary:70b\"
backend = \"ollama\"
fallback = [\"fallback:8b\"]"

write_config "$TMP/near-context.toml" 10 \
"[backends.ollama]
kind = \"ollama\"
base_url = \"http://127.0.0.1:${PORT_A}\"
default = true" \
"[routing.code]
model = \"qwen3:8b\"
backend = \"ollama\"
fallback = []"

run_success_case() {
  local name="$1"
  local config="$2"
  local prompt="$3"
  INFERCTL_CONFIG="$config" inferctl route code --prompt-file "$prompt" --explain >"$TMP/${name}.txt"
  diff -u "$EXAMPLE_DIR/goldens/${name}.txt" "$TMP/${name}.txt"
  INFERCTL_CONFIG="$config" inferctl route code --prompt-file "$prompt" --json >"$TMP/${name}.json"
}

run_success_case primary "$TMP/primary.toml" "$TMP/task.txt"
jq -e '.data.decision.selected_model == "qwen3:8b"' "$TMP/primary.json" >/dev/null
jq -e '.data.decision.is_fallback == false' "$TMP/primary.json" >/dev/null
jq -e '.data.candidates | length == 1' "$TMP/primary.json" >/dev/null

run_success_case backend-unreachable "$TMP/backend-unreachable.toml" "$TMP/task.txt"
jq -e '.data.decision.selected_backend == "ollama_small"' "$TMP/backend-unreachable.json" >/dev/null
jq -e '.data.decision.is_fallback == true' "$TMP/backend-unreachable.json" >/dev/null
jq -e '.warnings[] | select(.code == "W_BACKEND_UNREACHABLE")' "$TMP/backend-unreachable.json" >/dev/null

run_success_case model-unavailable "$TMP/model-unavailable.toml" "$TMP/task.txt"
jq -e '.data.decision.selected_model == "qwen3:8b"' "$TMP/model-unavailable.json" >/dev/null
jq -e '.data.candidates[] | select(.role == "primary" and .unavailability_reason == "not_installed")' "$TMP/model-unavailable.json" >/dev/null

run_success_case near-context "$TMP/near-context.toml" "$TMP/near.txt"
jq -e '.data.constraints.context_pct == 100' "$TMP/near-context.json" >/dev/null
jq -e '.warnings[] | select(.code == "W_CONTEXT_NEAR_LIMIT")' "$TMP/near-context.json" >/dev/null

if INFERCTL_CONFIG="$TMP/no-route.toml" inferctl route code --prompt-file "$TMP/task.txt" --explain >"$TMP/no-route.txt" 2>"$TMP/no-route.err"; then
  echo "expected no route to exit non-zero" >&2
  exit 1
fi
diff -u "$EXAMPLE_DIR/goldens/no-route.txt" "$TMP/no-route.txt"
grep -F "exit: 4" "$TMP/no-route.err" >/dev/null
if INFERCTL_CONFIG="$TMP/no-route.toml" inferctl route code --prompt-file "$TMP/task.txt" --json >"$TMP/no-route.json" 2>"$TMP/no-route-json.err"; then
  echo "expected JSON no route to exit non-zero" >&2
  exit 1
fi
jq -e '.errors[0].code == "E_NO_ROUTE_AVAILABLE"' "$TMP/no-route.json" >/dev/null
jq -e '.errors[0].exit_code == 4' "$TMP/no-route.json" >/dev/null
jq -e '.errors[0].did_you_mean == "inferctl doctor"' "$TMP/no-route.json" >/dev/null

INFERCTL_CONFIG="$TMP/primary.toml" inferctl route code --prompt-file "$TMP/prompt-redaction.txt" --explain >"$TMP/prompt-redaction.txt.out"
INFERCTL_CONFIG="$TMP/primary.toml" inferctl route code --prompt-file "$TMP/prompt-redaction.txt" --json >"$TMP/prompt-redaction.json"
HUMAN_OUTPUTS=("$TMP/primary.txt" "$TMP/backend-unreachable.txt" "$TMP/model-unavailable.txt" "$TMP/near-context.txt" "$TMP/no-route.txt" "$TMP/prompt-redaction.txt.out")
JSON_OUTPUTS=("$TMP/primary.json" "$TMP/backend-unreachable.json" "$TMP/model-unavailable.json" "$TMP/near-context.json" "$TMP/no-route.json" "$TMP/prompt-redaction.json")
if grep -F "UNIQUE_SECRET_ROUTE_PROMPT_TEXT" "${HUMAN_OUTPUTS[@]}" "${JSON_OUTPUTS[@]}" >/dev/null; then
  echo "route output leaked prompt content" >&2
  exit 1
fi
if grep -F "$TMP/" "${HUMAN_OUTPUTS[@]}" >/dev/null; then
  echo "route human output leaked local temp path" >&2
  exit 1
fi
if grep -E 'http://127\.0\.0\.1|Authorization|Bearer|api[_-]?key|auth[_-]?header|secret' "${HUMAN_OUTPUTS[@]}" "${JSON_OUTPUTS[@]}" >/dev/null; then
  echo "route output leaked private endpoint or credential-looking text" >&2
  exit 1
fi

echo "routing explainer goldens ok"
