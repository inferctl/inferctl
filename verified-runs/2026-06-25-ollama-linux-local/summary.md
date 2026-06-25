# Verified Run: Ollama on Linux Localhost

Date: 2026-06-25

This artifact pack records an end-to-end inferctl validation against a real Ollama provider on a Linux x86_64 machine. The public run name intentionally omits the private hostname.

## Result

Status: pass with caveats

inferctl discovered a localhost Ollama provider, generated a TOML config patch, validated the resulting config, inspected backend and model state, selected a configured route for the `code` task, and produced a clean triage report.

## Environment

- inferctl commit: `860bf6cc07fce70d9f750968d8f2034755110f1d`
- inferctl version: `0.2.2-0.20260625151604-860bf6cc07fc`
- target: Linux amd64
- provider: Ollama `0.20.5`
- provider endpoint: `http://127.0.0.1:11434`
- installed models reported by inferctl: 10
- route task validated: `code`
- selected model: `qwen3:8b`

## Commands Captured

- `inferctl version --json`
- `inferctl capabilities --json`
- `inferctl discover --json`
- `inferctl discover --kind ollama --json`
- `inferctl discover --kind ollama --format toml`
- `inferctl config init --path inferctl.toml --json`
- `inferctl discover --kind ollama --format toml | inferctl config patch --from-stdin --path inferctl.toml --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl config validate --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl doctor --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl backends --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl models --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl model qwen3:8b --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl route code --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl triage --json`

## Caveats

- llama.cpp was not validated; no llama.cpp server process or expected listener was present during the run.
- `route code` emitted `W_MODEL_NOT_LOADED` because `qwen3:8b` was installed but not loaded at route time.
- This validates provider discovery, inspection, diagnosis, and routing behavior. It is not a model quality benchmark.

## Redaction Review

No raw inferctl JSON fields required redaction. The artifacts use localhost provider URLs and do not include private hostnames, home paths, Tailscale addresses, tokens, or API key values. The artifact directory name was neutralized instead of using the private machine hostname.
