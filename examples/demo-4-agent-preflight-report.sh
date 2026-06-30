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

DOCTOR="$TMP/doctor.json"
ROUTE="$TMP/route.json"
TRIAGE="$TMP/triage.json"

INFERCTL_CONFIG="$TMP/config.toml" inferctl doctor --json >"$DOCTOR"
INFERCTL_CONFIG="$TMP/config.toml" inferctl route code_review --prompt-file "$PROMPT" --json >"$ROUTE"
INFERCTL_CONFIG="$TMP/config.toml" inferctl triage --input-file "$DOCTOR" --json >"$TRIAGE"

jq -e '.ok == true' "$DOCTOR" >/dev/null
jq -e '.ok == true' "$ROUTE" >/dev/null
jq -e '.data.decision.is_fallback == true' "$ROUTE" >/dev/null
jq -e '.data.decision.selected_model == "qwen3:8b"' "$ROUTE" >/dev/null
jq -e '.data.summary.warnings_total >= 1' "$DOCTOR" >/dev/null
jq -e '.data.summary.warnings >= 1' "$TRIAGE" >/dev/null

cat <<EOF
# Agent Preflight Report

Use case: before a coding agent starts a local model job, check whether the
configured primary model is actually available and pick the usable fallback
without running inference.

## Local Stack

- Backends reachable: $(jq -r '.data.summary.backends_reachable' "$DOCTOR") / $(jq -r '.data.summary.backends_total' "$DOCTOR")
- Installed models visible: $(jq -r '.data.summary.models_installed_total' "$DOCTOR")
- Loaded models ready: $(jq -r '.data.summary.models_loaded_total' "$DOCTOR")
- Top warning: $(jq -r '.warnings[0].code + " - " + .warnings[0].message' "$DOCTOR")

## Route Decision

- Task: $(jq -r '.data.task' "$ROUTE")
- Selected backend: $(jq -r '.data.decision.selected_backend' "$ROUTE")
- Selected model: $(jq -r '.data.decision.selected_model' "$ROUTE")
- Fallback used: $(jq -r '.data.decision.is_fallback' "$ROUTE")
- Reason: $(jq -r '.data.decision.reason' "$ROUTE")
- Prompt estimate: $(jq -r '.data.input.estimated_tokens' "$ROUTE") tokens

## Next Action For The Agent

\`\`\`sh
$(jq -r '.data.recommended_action.command' "$DOCTOR")
\`\`\`

## Why inferctl matters here

- It turns local backend drift into a structured preflight check.
- It gives agent code stable JSON instead of fragile terminal scraping.
- It explains the fallback decision before the agent spends time on the job.
EOF
