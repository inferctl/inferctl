# Verified Run: OpenAI-Compatible Qwen on Linux Localhost

Date: 2026-06-25

This artifact pack records an inferctl validation against the generic `openai_compat` adapter path using a real local OpenAI-compatible `/v1/models` server. The server was llama.cpp serving a small Qwen GGUF model, but inferctl was configured only as `kind = "openai_compat"` to exercise the generic adapter rather than the dedicated llama.cpp backend.

## Result

Status: pass with expected v0.1 warnings

inferctl discovered a localhost OpenAI-compatible provider, patched a clean config, validated the config, inspected backend and model state, selected the configured model for the `code` task, and produced deterministic triage output.

## Environment

- inferctl commit: `d8f812a8d0087fe6cb382ab67d3f352c4fa6547b`
- inferctl version: `0.2.2-0.20260625174225-d8f812a8d008`
- target: Linux amd64
- provider adapter under test: `openai_compat`
- provider server: llama.cpp `9789 (b3ce5cedf)` exposing OpenAI-compatible `/v1/models`
- endpoint: `http://127.0.0.1:8090`
- selected route backend: `openai_compat_local`
- selected route model: `qwen2.5-0.5b-instruct-q4_k_m-openai-compat`
- source model: `Qwen/Qwen2.5-0.5B-Instruct-GGUF`
- quantization: `Q4_K_M`

## Commands Captured

- `inferctl version --json`
- `inferctl capabilities --json`
- `inferctl discover --json`
- `inferctl discover --kind openai_compat --json`
- `inferctl discover --kind openai_compat --format toml`
- `inferctl config init --path init.scaffold.toml --json`
- `inferctl config patch --from-stdin --path inferctl.toml --json < patch.openai_compat.toml`
- `INFERCTL_CONFIG=inferctl.toml inferctl config validate --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl doctor --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl backends --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl models --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl model qwen2.5-0.5b-instruct-q4_k_m-openai-compat --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl route code --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl triage --json`

## Caveats

- This validates provider workflow behavior and route selection, not model quality.
- `openai_compat` in v0.1 does not support loaded-model inventory. `doctor` and `triage` therefore report `W_BACKEND_DEGRADED`, and `route code` reports `W_MODEL_NOT_LOADED` even though the provider endpoint is reachable and the model is listed by `/v1/models`.
- The provider server software path overlaps with the separately validated llama.cpp run; the inferctl adapter path under test here is the generic `openai_compat` backend.
- `config init` is captured as the stock scaffold artifact. The final run config starts from a minimal meta/profile config and applies `patch.openai_compat.toml` because v0.1 config patching adds and updates keys but does not delete scaffold backends.

## Redaction Review

No raw inferctl JSON fields required redaction. The artifacts use localhost provider URLs and a neutral model alias. They do not include private hostnames, home paths, Tailscale addresses, tokens, API key values, or local GGUF paths.
