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

PORT_A=$((25000 + $$ % 1000))
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
name = "team_requirements"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.llamacpp_large]
kind = "llama.cpp"
base_url = "http://127.0.0.1:${PORT_A}"
default = false

[backends.ollama]
kind = "ollama"
base_url = "http://127.0.0.1:${PORT_B}"
default = true

[routing.code]
model = "qwen-coder-32b.gguf"
backend = "llamacpp_large"
fallback = ["qwen3:8b"]
EOF

cat >"$TMP/prompt.txt" <<'EOF'
Review this repository change for correctness risks and missing tests.
EOF

cat >"$TMP/pass.toml" <<EOF
[tasks.code]
prompt_file = "prompt.txt"
selected_model_any_of = ["qwen-coder-32b.gguf", "qwen3:8b"]
allow_fallback = true
require_ready = false

[backends.ollama]
required = true
require_reachable = true
EOF

INFERCTL_CONFIG="$TMP/config.toml" examples/team-requirements/verify-local-llm.py --requirements "$TMP/pass.toml" >"$TMP/pass.out"
grep -F "PASS local inference preflight" "$TMP/pass.out" >/dev/null

cat >"$TMP/fail.toml" <<EOF
[tasks.code]
prompt_file = "prompt.txt"
selected_model_any_of = ["missing.gguf"]
allow_fallback = true
require_ready = false

[backends.missing_backend]
required = true
require_reachable = true
EOF

if INFERCTL_CONFIG="$TMP/config.toml" examples/team-requirements/verify-local-llm.py --requirements "$TMP/fail.toml" >"$TMP/fail.out"; then
  echo "expected requirements mismatch to fail" >&2
  exit 1
fi
grep -F "missing backend: missing_backend" "$TMP/fail.out" >/dev/null
grep -F "none of the allowed models are exposed for task code: missing.gguf" "$TMP/fail.out" >/dev/null
grep -F "task code selected unexpected model: qwen3:8b" "$TMP/fail.out" >/dev/null

cat >"$TMP/no-fallback.toml" <<EOF
[tasks.code]
prompt_file = "prompt.txt"
selected_model_any_of = ["qwen-coder-32b.gguf", "qwen3:8b"]
allow_fallback = false
require_ready = false
EOF

if INFERCTL_CONFIG="$TMP/config.toml" examples/team-requirements/verify-local-llm.py --requirements "$TMP/no-fallback.toml" >"$TMP/no-fallback.out"; then
  echo "expected fallback policy mismatch to fail" >&2
  exit 1
fi
grep -F "task code is not runnable" "$TMP/no-fallback.out" >/dev/null
grep -F "fallback selected but --allow-fallback was not set" "$TMP/no-fallback.out" >/dev/null

echo "team requirements verifier ok"
