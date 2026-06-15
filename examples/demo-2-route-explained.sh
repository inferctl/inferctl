#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
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

PORT_A=$((20000 + $$ % 1000))
PORT_B=$((PORT_A + 1))

go run ./cmd/infer-testserver -addr "127.0.0.1:${PORT_A}" -kind ollama -models "fallback:8b" -loaded "fallback:8b" >"$TMP/ollama.log" 2>&1 &
PIDS+=("$!")
go run ./cmd/infer-testserver -addr "127.0.0.1:${PORT_B}" -kind llama.cpp -models "primary.gguf" -unreachable >"$TMP/llama.log" 2>&1 &
PIDS+=("$!")

for _ in $(seq 1 80); do
  if curl -fsS "http://127.0.0.1:${PORT_A}/api/version" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

cat >"$TMP/config.toml" <<EOF
[meta]
schema_version = "0.1"

[profile]
name = "demo_route_explained"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.llamacpp]
kind = "llama.cpp"
base_url = "http://127.0.0.1:${PORT_B}"
default = false

[backends.ollama]
kind = "ollama"
base_url = "http://127.0.0.1:${PORT_A}"
default = true

[routing.code]
model = "primary.gguf"
backend = "llamacpp"
fallback = ["fallback:8b"]
EOF

OUT="$TMP/route.json"
INFERCTL_CONFIG="$TMP/config.toml" inferctl route code --prompt "explain this fallback" --json >"$OUT"

jq -e '.ok == true' "$OUT" >/dev/null
jq -e '.data.decision.is_fallback == true' "$OUT" >/dev/null
jq -e '.data.decision.selected_model == "fallback:8b"' "$OUT" >/dev/null
jq -e '.data.candidates[0].model == "primary.gguf"' "$OUT" >/dev/null
jq -e '.data.candidates[0].available == false' "$OUT" >/dev/null
jq -e '.data.candidates[0].unavailability_reason == "backend_unreachable"' "$OUT" >/dev/null
jq -e '.warnings[] | select(.code == "W_FALLBACK_USED")' "$OUT" >/dev/null

echo "demo 2 ok"
