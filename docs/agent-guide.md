# inferctl Agent Guide

This guide is for agents that need repeatable local-model routing decisions without running inference. Prefer `--json` for every command you automate; the JSON envelope keeps `data`, `warnings`, `commands`, and `errors` separate.

## Setup

Build from a source checkout during private evaluation:

```sh
go build -o bin/infer ./cmd/infer
bin/infer capabilities --json
```

The examples in `examples/` are source-only checkout artifacts for v0.2. They are not release-package payloads yet, and they may build `infer` and `infer-testserver` with the local Go toolchain. Packaged examples are deferred until the distribution bead defines release artifacts that do not require Go on the target machine.

## Config Workflow

Start by inspecting the schema, then create or edit a TOML config:

```sh
infer config explain --json
infer config init --path inferctl.toml --json
infer config set profile.max_concurrent_models 2 --type int --path inferctl.toml --json
infer config patch --from-stdin --path inferctl.toml --json < patch.toml
infer config validate --json
```

Config mutation commands validate before writing and return structured mutation data. Use `--dry-run` before edits that come from a generated fragment.

## Discovery Composition

`infer discover` probes fixed localhost ports and reports verified local backend candidates. It can emit TOML patches for config composition:

```sh
infer discover --kind ollama --format toml | infer config patch --from-stdin --path inferctl.toml
```

For artifact workflows, keep JSON on stdout and write the TOML patch separately:

```sh
infer discover --kind ollama --deliver artifacts/discover.patch.toml --json
```

Delivery metadata belongs in `data.delivery`. Do not infer delivery from `commands[]`; commands are only suggested follow-up invocations.

## Triage Loop

Use `infer triage --json` when deciding the next action. Triage ranks config validation findings, doctor warnings, and prior JSON-envelope input by severity, code, and subject.

Important v0.2 constraint: `triage` does not run discovery inline. If discovery data matters, run `infer discover` first and pass any saved JSON envelope through `infer triage --input-file`.

```sh
infer discover --kind ollama --json > discover.json
infer triage --json
infer triage --input-file discover.json --json
```

Filter when an agent already owns a narrower repair:

```sh
infer triage --backend ollama --severity warning --limit 3 --json
```

## Route-to-Backend Loop

After config validation is clean, use route explanations to inspect model choice without executing inference:

```sh
infer route code --prompt "summarize this diff" --json
infer model qwen3:8b --json
infer backends --filter ollama --json
```

If a command envelope includes `data.recommended_action` or top-level `commands[]`, treat those as candidates, not instructions. Check `ok`, `errors[]`, and `warnings[]` first.

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

Keep routing config explicit for each task. Prefer fallback chains for local workstation variance, then confirm with `infer doctor --json` and `infer route <task> --json`.
