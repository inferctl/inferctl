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

PORT_A=$((24000 + $$ % 1000))
PORT_B=$((PORT_A + 1))

go run ./cmd/infer-testserver -addr "127.0.0.1:${PORT_A}" -kind llama.cpp -models "qwen-coder-32b.gguf" -unreachable >"$TMP/llamacpp.log" 2>&1 &
PIDS+=("$!")
go run ./cmd/infer-testserver -addr "127.0.0.1:${PORT_B}" -kind ollama -models "qwen3:8b" -loaded "qwen3:8b" >"$TMP/ollama.log" 2>&1 &
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
name = "agent_discovery"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.llamacpp_large]
kind = "llama.cpp"
base_url = "http://127.0.0.1:${PORT_A}"
default = false

[backends.ollama_small]
kind = "ollama"
base_url = "http://127.0.0.1:${PORT_B}"
default = true

[routing.code]
model = "qwen-coder-32b.gguf"
backend = "llamacpp_large"
fallback = ["qwen3:8b"]
EOF

PROMPT="$TMP/prompt.txt"
cat >"$PROMPT" <<'EOF'
Summarize this repository change without running any tools.
EOF

PREFLIGHT="$TMP/preflight.json"
INFERCTL_CONFIG="$TMP/config.toml" inferctl preflight code --prompt-file "$PROMPT" --allow-fallback --json >"$PREFLIGHT"
jq -e '.data.route_decision.selected_backend == "ollama_small"' "$PREFLIGHT" >/dev/null
jq -e '.data.route_decision.selected_model == "qwen3:8b"' "$PREFLIGHT" >/dev/null
if grep -F "Summarize this repository change" "$PREFLIGHT" >/dev/null; then
  echo "preflight JSON leaked prompt text" >&2
  exit 1
fi

INFERCTL_CONFIG="$TMP/config.toml" examples/agent-discovery/demo.py --prompt-file "$PROMPT" --allow-fallback --dry-run >"$TMP/demo.out" 2>"$TMP/demo.err"
grep -F "inferctl selected: ollama_small / qwen3:8b" "$TMP/demo.out" >/dev/null
grep -F "data plane: calling http://127.0.0.1:${PORT_B}/v1 directly" "$TMP/demo.out" >/dev/null
grep -F "[dry run: backend call skipped]" "$TMP/demo.out" >/dev/null
test ! -s "$TMP/demo.err"

if INFERCTL_CONFIG="$TMP/config.toml" examples/agent-discovery/demo.py --prompt-file "$PROMPT" --dry-run >"$TMP/blocked.out" 2>"$TMP/blocked.err"; then
  echo "expected fallback route to require --allow-fallback" >&2
  exit 1
fi
grep -F "fallback selected but --allow-fallback was not set" "$TMP/blocked.err" >/dev/null

echo "agent discovery demo ok"
