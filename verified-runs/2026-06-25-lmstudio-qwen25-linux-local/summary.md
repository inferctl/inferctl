# Verified Run: LM Studio Qwen on Linux Localhost

Date: 2026-06-25

This artifact pack records an end-to-end inferctl validation against LM Studio running headlessly on Linux. The run validates inferctl's `lmstudio` adapter path against LM Studio's local OpenAI-compatible `/v1/models` server.

## Result

Status: pass

inferctl discovered a localhost LM Studio provider, patched a clean config, validated the config, inspected backend and model state, selected the configured Qwen model for the `code` task, and produced a clean triage report.

## Environment

- inferctl commit: `d8f812a8d0087fe6cb382ab67d3f352c4fa6547b`
- inferctl version: `0.2.2-0.20260625174225-d8f812a8d008`
- target: Linux amd64
- provider: LM Studio headless daemon `llmster v0.0.15+2`
- CLI commit: `efce996`
- endpoint: `http://127.0.0.1:1234`
- selected route backend: `lmstudio`
- selected route model: `qwen2.5-0.5b-instruct-q8_0-lmstudio`
- source model: `Qwen/Qwen2.5-0.5B-Instruct-GGUF`
- quantization: `Q8_0`

## Commands Captured

- `inferctl version --json`
- `inferctl capabilities --json`
- `inferctl discover --json`
- `inferctl discover --kind lmstudio --json`
- `inferctl discover --kind lmstudio --format toml`
- `inferctl config init --path init.scaffold.toml --json`
- `inferctl config patch --from-stdin --path inferctl.toml --json < patch.lmstudio.toml`
- `INFERCTL_CONFIG=inferctl.toml inferctl config validate --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl doctor --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl backends --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl models --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl model qwen2.5-0.5b-instruct-q8_0-lmstudio --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl route code --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl triage --json`

## Caveats

- This validates provider workflow behavior and route selection, not model quality.
- LM Studio's `/v1/models` response included three model IDs: the loaded Qwen alias, the base Qwen key, and an existing embedding model. The configured route selected the loaded Qwen alias.
- The Qwen model was loaded CPU-only with a neutral API identifier so public artifacts do not expose local cache paths.
- `config init` is captured as the stock scaffold artifact. The final run config starts from a minimal meta/profile config and applies `patch.lmstudio.toml` because v0.1 config patching adds and updates keys but does not delete scaffold backends.

## Redaction Review

No raw inferctl JSON fields required redaction. The artifacts use localhost provider URLs, public model provenance, and a neutral loaded-model alias. They do not include private hostnames, home paths, Tailscale addresses, tokens, API key values, or local model cache paths.
