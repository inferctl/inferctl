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

PORT_A=$((23000 + $$ % 1000))
PORT_B=$((PORT_A + 1))

go run ./cmd/infer-testserver -addr "127.0.0.1:${PORT_A}" -kind llama.cpp -models "primary.gguf" >"$TMP/primary.log" 2>&1 &
PIDS+=("$!")
go run ./cmd/infer-testserver -addr "127.0.0.1:${PORT_B}" -kind ollama -models "fallback:8b" >"$TMP/fallback.log" 2>&1 &
PIDS+=("$!")

for _ in $(seq 1 80); do
  if curl -fsS "http://127.0.0.1:${PORT_A}/v1/models" >/dev/null 2>&1 &&
    curl -fsS "http://127.0.0.1:${PORT_B}/api/version" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

cat >"$TMP/primary.toml" <<EOF
[meta]
schema_version = "0.1"

[profile]
name = "editor_config_primary"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.llamacpp]
kind = "llama.cpp"
base_url = "http://127.0.0.1:${PORT_A}"
default = true

[routing.code]
model = "primary.gguf"
backend = "llamacpp"
fallback = []
EOF

cat >"$TMP/fallback.toml" <<EOF
[meta]
schema_version = "0.1"

[profile]
name = "editor_config_fallback"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.llamacpp]
kind = "llama.cpp"
base_url = "http://127.0.0.1:$((PORT_A + 100))"
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

INFERCTL_CONFIG="$TMP/primary.toml" examples/editor-configs/generate.py --target aider >"$TMP/aider.yml" 2>"$TMP/aider.err"
grep -F 'model: "openai/primary.gguf"' "$TMP/aider.yml" >/dev/null
grep -F "openai-api-base: \"http://127.0.0.1:${PORT_A}/v1\"" "$TMP/aider.yml" >/dev/null
test ! -s "$TMP/aider.err"

INFERCTL_CONFIG="$TMP/primary.toml" examples/editor-configs/generate.py --target cline >"$TMP/cline.yml" 2>"$TMP/cline.err"
grep -F 'api_provider: OpenAI Compatible' "$TMP/cline.yml" >/dev/null
grep -F 'model_id: "primary.gguf"' "$TMP/cline.yml" >/dev/null
grep -F "base_url: \"http://127.0.0.1:${PORT_A}/v1\"" "$TMP/cline.yml" >/dev/null
test ! -s "$TMP/cline.err"

if INFERCTL_CONFIG="$TMP/fallback.toml" examples/editor-configs/generate.py --target aider >"$TMP/fallback-not-ready.yml" 2>"$TMP/fallback-not-ready.err"; then
  echo "expected not-ready fallback generation to fail without --allow-not-ready" >&2
  exit 1
fi
grep -F "pass --allow-not-ready" "$TMP/fallback-not-ready.err" >/dev/null

INFERCTL_CONFIG="$TMP/fallback.toml" examples/editor-configs/generate.py --target aider --allow-not-ready >"$TMP/fallback.yml" 2>"$TMP/fallback.err"
grep -F 'model: "openai/fallback:8b"' "$TMP/fallback.yml" >/dev/null
grep -F "openai-api-base: \"http://127.0.0.1:${PORT_B}/v1\"" "$TMP/fallback.yml" >/dev/null
grep -F "warning: generated from fallback route" "$TMP/fallback.err" >/dev/null
grep -F "warning: selected route 'fallback:8b' is not ready" "$TMP/fallback.err" >/dev/null

if examples/editor-configs/generate.py --target unknown >"$TMP/unknown.out" 2>"$TMP/unknown.err"; then
  echo "expected unknown target to fail" >&2
  exit 1
fi
grep -F "invalid choice" "$TMP/unknown.err" >/dev/null

INFERCTL_CONFIG="$TMP/primary.toml" examples/editor-configs/generate.py --target aider --output "$TMP/out.yml"
if INFERCTL_CONFIG="$TMP/primary.toml" examples/editor-configs/generate.py --target aider --output "$TMP/out.yml" >"$TMP/overwrite.out" 2>"$TMP/overwrite.err"; then
  echo "expected existing output to require --overwrite" >&2
  exit 1
fi
grep -F "pass --overwrite" "$TMP/overwrite.err" >/dev/null

echo "editor config generator ok"
