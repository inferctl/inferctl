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

PORT_A=$((26000 + $$ % 1000))
PORT_B=$((PORT_A + 1))

go run ./cmd/infer-testserver -addr "127.0.0.1:${PORT_B}" -kind ollama -models "fallback:8b" -loaded "fallback:8b" >"$TMP/ollama.log" 2>&1 &
PIDS+=("$!")

for _ in $(seq 1 80); do
  if curl -fsS "http://127.0.0.1:${PORT_B}/api/version" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

cat >"$TMP/config.toml" <<EOF
[meta]
schema_version = "0.1"

[profile]
name = "status_dashboard"
max_context_tokens = 8192
max_concurrent_models = 2
allow_premium = false
mode = "warn"

[backends.llamacpp]
kind = "llama.cpp"
base_url = "http://127.0.0.1:${PORT_A}"
default = false

[backends.ollama]
kind = "ollama"
base_url = "http://127.0.0.1:${PORT_B}"
default = true

[routing.code]
model = "primary.gguf"
backend = "llamacpp"
fallback = ["fallback:8b"]
EOF

INFERCTL_CONFIG="$TMP/config.toml" inferctl status --json >"$TMP/status.json"
jq -e '.data.summary.backends_total == 2' "$TMP/status.json" >/dev/null
jq -e '.data.summary.backends_reachable == 1' "$TMP/status.json" >/dev/null
jq -e '.data.routes[0].decision.is_fallback == true' "$TMP/status.json" >/dev/null
jq -e '.warnings[] | select(.code == "W_FALLBACK_USED")' "$TMP/status.json" >/dev/null

INFERCTL_CONFIG="$TMP/config.toml" inferctl status --json --watch --events --interval 200ms >"$TMP/watch.jsonl" 2>"$TMP/watch.err" &
WATCH_PID=$!
PIDS+=("$WATCH_PID")
sleep 0.5

go run ./cmd/infer-testserver -addr "127.0.0.1:${PORT_A}" -kind llama.cpp -models "primary.gguf" >"$TMP/llamacpp.log" 2>&1 &
PIDS+=("$!")

for _ in $(seq 1 20); do
  if jq -e 'select(.data.event_schema_version? == "0.1")' "$TMP/watch.jsonl" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

kill "$WATCH_PID" >/dev/null 2>&1 || true
wait "$WATCH_PID" >/dev/null 2>&1 || true

jq -e 'select(.data.status_frame_schema_version? == "0.1")' "$TMP/watch.jsonl" >/dev/null
jq -e 'select(.data.event_schema_version? == "0.1") | .data.events[] | select(.kind == "backend_reachability_changed")' "$TMP/watch.jsonl" >/dev/null
jq -e 'select(.data.event_schema_version? == "0.1") | .data.events[] | select(.kind == "selected_route_changed")' "$TMP/watch.jsonl" >/dev/null
jq -e 'select(.data.event_schema_version? == "0.1") | .data.events[] | select(.summary | test("route|backend|fallback"))' "$TMP/watch.jsonl" >/dev/null

if INFERCTL_CONFIG="$TMP/config.toml" inferctl dashboard --json >"$TMP/dashboard-json.out" 2>"$TMP/dashboard-json.err"; then
  echo "expected dashboard --json to refuse machine mode" >&2
  exit 1
fi
jq -e '.errors[0].did_you_mean | contains("status --json --watch")' "$TMP/dashboard-json.out" >/dev/null

echo "status dashboard scenario ok"
