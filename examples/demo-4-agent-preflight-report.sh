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

PORT_A=$((22000 + $$ % 1000))
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
name = "agent_workstation"
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

[routing.code_review]
model = "qwen-coder-32b.gguf"
backend = "llamacpp_large"
fallback = ["qwen3:8b"]
EOF

PROMPT="$TMP/prompt.txt"
cat >"$PROMPT" <<'EOF'
Review this pull request for correctness risks and missing tests. Prefer the
large local coding model when available, but do not block the agent loop if that
backend is offline.
EOF

PREFLIGHT="$TMP/preflight.json"

INFERCTL_CONFIG="$TMP/config.toml" inferctl preflight code_review --prompt-file "$PROMPT" --allow-fallback --json >"$PREFLIGHT"

jq -e '.ok == true' "$PREFLIGHT" >/dev/null
jq -e '.data.preflight_schema_version == "0.1"' "$PREFLIGHT" >/dev/null
jq -e '.data.route_decision.selected_model == "qwen3:8b"' "$PREFLIGHT" >/dev/null
jq -e '.data.prompt.filename == "prompt.txt"' "$PREFLIGHT" >/dev/null
jq -e '.commands | length >= 3' "$PREFLIGHT" >/dev/null

INFERCTL_CONFIG="$TMP/config.toml" inferctl preflight code_review --prompt-file "$PROMPT" --allow-fallback --format markdown
