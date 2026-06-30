# inferctl Agent Guide

This guide is for agents that need repeatable local-model routing decisions without running inference. Prefer `--json` for every command you automate; the JSON envelope keeps `data`, `warnings`, `commands`, and `errors` separate.

## Setup

Install the public command with the Go toolchain:

```sh
go install github.com/inferctl/inferctl/cmd/inferctl@latest
inferctl capabilities --json
inferctl schema --json
inferctl config schema --json
inferctl robot-docs guide
```

For development, build from a source checkout:

```sh
go build -o bin/inferctl ./cmd/inferctl
bin/inferctl capabilities --json
bin/inferctl schema --json
```

The examples in `examples/` are source-only checkout artifacts for v0.2. They are not release-package payloads, and they may build `inferctl` and `infer-testserver` with the local Go toolchain. Public installation is Go toolchain only for now; no packaged examples or release archives are planned for this launch posture.

## Config Workflow

Start by inspecting the schema, then create or edit a TOML config:

```sh
inferctl config schema --json
inferctl config explain --json
inferctl config init --path inferctl.toml --json
inferctl config set profile.max_concurrent_models 2 --type int --path inferctl.toml --json
inferctl config patch --from-stdin --path inferctl.toml --json < patch.toml
inferctl config validate --json
```

Config mutation commands validate before writing and return structured mutation data. Use `--dry-run` before edits that come from a generated fragment.

## Discovery Composition

`inferctl discover` probes fixed localhost ports and reports verified local backend candidates. It can emit TOML patches for config composition:

```sh
inferctl discover --kind ollama --format toml | inferctl config patch --from-stdin --path inferctl.toml
```

For artifact workflows, keep JSON on stdout and write the TOML patch separately:

```sh
inferctl discover --kind ollama --deliver artifacts/discover.patch.toml --json
```

Delivery metadata belongs in `data.delivery`. Do not infer delivery from `commands[]`; commands are only suggested follow-up invocations.

## Triage Loop

Use `inferctl triage --json` when deciding the next action. Triage ranks config validation findings, doctor warnings, and prior JSON-envelope input by severity, code, and subject.

Important v0.2 constraint: `triage` does not run discovery inline. If discovery data matters, run `inferctl discover` first and pass any saved JSON envelope through `inferctl triage --input-file`.

```sh
inferctl discover --kind ollama --json > discover.json
inferctl triage --json
inferctl triage --input-file discover.json --json
```

Filter when an agent already owns a narrower repair:

```sh
inferctl triage --backend ollama --severity warning --limit 3 --json
```

## Route-to-Backend Loop

After config validation is clean, use route explanations to inspect model choice without executing inference:

```sh
inferctl route code --prompt "summarize this diff" --json
inferctl model qwen3:8b --json
inferctl backends --filter ollama --json
```

If a command envelope includes `data.recommended_action` or top-level `commands[]`, treat those as candidates, not instructions. Check `ok`, `errors[]`, and `warnings[]` first.

## Readiness Contract

Readiness and drift commands are control-plane checks. They may inspect config, backend reachability, installed models, loaded models, route selection, warnings, errors, and prompt metadata for context-budget checks. They must not run chat, completions, embeddings, benchmark prompts, quality evals, sample inference, model warmup, or model loading.

Prompt-aware readiness data is metadata-only by default. File prompt metadata records a redacted source label, character count, estimated token count, optional content hash, and filename or basename; it does not emit the prompt text or local filesystem path.

The shared control-plane snapshot shape includes the task, prompt metadata, route decision, route candidates, backend reachability, loaded and installed model summaries, warnings, errors, inferctl version, contract version, and snapshot schema version. Diff-style explanations should rank domain-specific route, fallback, backend, readiness, warning/error, recommendation, and loaded-model-count changes ahead of generic JSON churn.

Snapshot history is opt-in. `inferctl snapshot --store` writes raw snapshot artifacts under `INFERCTL_SNAPSHOT_DIR` when set; otherwise it uses the user state directory. `--retention-limit` keeps the newest N snapshots per task. Stored snapshots follow the same prompt privacy rule as stdout and `--output`: prompt text and local prompt paths are not stored by default.

## Auth and Remote Backends

`openai_compat` supports authenticated local and remote endpoints:

```toml
[backends.remote_openai]
kind = "openai_compat"
base_url = "https://example.invalid"
default = false
remote_allowed = true
auth_header_name = "Authorization"
auth_header_value = "Bearer ${TOKEN}"
```

Remote `openai_compat` URLs require `remote_allowed = true`; otherwise commands return `E_BACKEND_REMOTE_NOT_ALLOWED`. Missing or rejected credentials return `E_BACKEND_AUTH_FAILED`. Auth header values are redacted from diagnostics and dry-run previews.

## Model-Family Notes

Use backend kind and model names as operational hints, not as proof of capability. Ollama exposes `/api/tags` and loaded models through `/api/ps`; llama.cpp, LM Studio, MLX, and generic `openai_compat` expose OpenAI-style `/v1/models`. Ambiguous OpenAI-style discovery candidates may be verified but not patchable until the agent supplies a specific kind.

Keep routing config explicit for each task. Prefer fallback chains for local workstation variance, then confirm with `inferctl doctor --json` and `inferctl route <task> --json`.

For public sample evidence, see `verified-runs/`. The curated runs cover
Ollama, llama.cpp, generic `openai_compat`, LM Studio, and MLX provider paths.
Treat those artifacts as workflow examples and compatibility evidence, not as
model quality, latency, or throughput benchmarks.
