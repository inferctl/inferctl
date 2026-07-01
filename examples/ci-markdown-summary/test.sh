#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EXAMPLE_DIR="$ROOT/examples/ci-markdown-summary"
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

PORT_A=$((31000 + $$ % 1000))
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

cat >"$TMP/prompt.txt" <<'EOF'
UNIQUE_SECRET_CI_PROMPT should never appear in preflight Markdown or JSON.
EOF

write_config() {
  local cfg_path="$1"
  local backends="$2"
  local route="$3"
  cat >"$cfg_path" <<EOF
[meta]
schema_version = "0.1"

[profile]
name = "ci_markdown_summary"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

$backends

$route
EOF
}

check_golden() {
  local name="$1"
  local actual="$2"
  mkdir -p "$EXAMPLE_DIR/goldens"
  if [[ "${UPDATE_GOLDENS:-0}" == "1" ]]; then
    cp "$actual" "$EXAMPLE_DIR/goldens/$name"
  else
    diff -u "$EXAMPLE_DIR/goldens/$name" "$actual"
  fi
}

write_config "$TMP/primary.toml" \
"[backends.ollama]
kind = \"ollama\"
base_url = \"http://127.0.0.1:${PORT_A}\"
default = true" \
"[routing.code_review]
model = \"qwen3:8b\"
backend = \"ollama\"
fallback = []"

write_config "$TMP/fallback.toml" \
"[backends.llamacpp_large]
kind = \"ollama\"
base_url = \"http://127.0.0.1:${PORT_DEAD}\"
default = true

[backends.ollama_small]
kind = \"ollama\"
base_url = \"http://127.0.0.1:${PORT_B}\"
default = false" \
"[routing.code_review]
model = \"qwen-coder-32b.gguf\"
backend = \"llamacpp_large\"
fallback = [\"qwen3:8b\"]"

write_config "$TMP/no-route.toml" \
"[backends.ollama]
kind = \"ollama\"
base_url = \"http://127.0.0.1:${PORT_C}\"
default = true" \
"[routing.code_review]
model = \"qwen-coder-32b.gguf\"
backend = \"ollama\"
fallback = [\"qwen3:8b\"]"

cat >"$TMP/invalid.toml" <<'EOF'
[meta]
schema_version = 1
EOF

INFERCTL_CONFIG="$TMP/primary.toml" inferctl preflight code_review --prompt-file "$TMP/prompt.txt" --format markdown >"$TMP/primary-ready.md"
check_golden primary-ready.md "$TMP/primary-ready.md"

INFERCTL_CONFIG="$TMP/fallback.toml" inferctl preflight code_review --prompt-file "$TMP/prompt.txt" --allow-fallback --format markdown >"$TMP/fallback-allowed.md"
check_golden fallback-allowed.md "$TMP/fallback-allowed.md"

if INFERCTL_CONFIG="$TMP/fallback.toml" inferctl preflight code_review --prompt-file "$TMP/prompt.txt" --format markdown >"$TMP/fallback-blocked.md" 2>"$TMP/fallback-blocked.err"; then
  echo "expected fallback blocked preflight to fail" >&2
  exit 1
fi
check_golden fallback-blocked.md "$TMP/fallback-blocked.md"
grep -F "exit: 5" "$TMP/fallback-blocked.err" >/dev/null

if INFERCTL_CONFIG="$TMP/no-route.toml" inferctl preflight code_review --prompt-file "$TMP/prompt.txt" --allow-fallback --format markdown >"$TMP/no-route.md" 2>"$TMP/no-route.err"; then
  echo "expected no route preflight to fail" >&2
  exit 1
fi
check_golden no-route.md "$TMP/no-route.md"
grep -F "exit: 5" "$TMP/no-route.err" >/dev/null

if INFERCTL_CONFIG="$TMP/invalid.toml" inferctl preflight code_review --prompt-file "$TMP/prompt.txt" --format markdown >"$TMP/invalid-config.md" 2>"$TMP/invalid-config.err"; then
  echo "expected invalid config preflight to fail" >&2
  exit 1
fi
check_golden invalid-config.md "$TMP/invalid-config.md"
grep -F "exit: 3" "$TMP/invalid-config.err" >/dev/null

if grep -E 'UNIQUE_SECRET_CI_PROMPT|Authorization|Bearer|api[_-]?key|auth[_-]?header|http://(localhost|127\.|10\.|172\.(1[6-9]|2[0-9]|3[0-1])|192\.168\.)|https://(localhost|127\.|10\.|172\.(1[6-9]|2[0-9]|3[0-1])|192\.168\.)|/Users/|/home/|/private/var/|/tmp/' \
  "$TMP"/*.md "$EXAMPLE_DIR"/goldens/*.md >/dev/null; then
  echo "preflight Markdown leaked prompt, credential-looking text, unsafe endpoint, or local path" >&2
  exit 1
fi

grep -F 'inferctl preflight "$TASK"' "$EXAMPLE_DIR/run-preflight-markdown.sh" >/dev/null
grep -F -- '--format markdown' "$EXAMPLE_DIR/run-preflight-markdown.sh" >/dev/null
if rg -n '## inferctl preflight|Runnability:|Selected route:' "$EXAMPLE_DIR/run-preflight-markdown.sh" "$EXAMPLE_DIR/buildkite-command.sh" >/dev/null; then
  echo "example contains local Markdown formatter text outside checked goldens" >&2
  exit 1
fi

echo "ci markdown summary goldens ok"
