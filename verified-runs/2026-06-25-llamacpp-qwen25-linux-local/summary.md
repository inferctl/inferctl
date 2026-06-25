# Verified Run: llama.cpp and Ollama on Linux Localhost

Date: 2026-06-25

This artifact pack records an end-to-end inferctl validation against two real local providers in one config: llama.cpp and Ollama. The run validates inferctl's single-control-plane workflow across providers, with `route code` selecting a llama.cpp-hosted model.

## Result

Status: pass

inferctl discovered a localhost llama.cpp provider, created and patched config, validated a two-provider setup, inspected backend and model state, selected a loaded llama.cpp model for the `code` task, and produced a clean triage report.

## Environment

- inferctl commit: `971300d3af49c3857e398bba258465daee076145`
- inferctl version: `0.2.2-0.20260625171324-971300d3af49`
- target: Linux amd64
- provider 1: llama.cpp `9789 (b3ce5cedf)`
- provider 1 endpoint: `http://127.0.0.1:8080`
- provider 2: Ollama `0.20.5`
- provider 2 endpoint: `http://127.0.0.1:11434`
- selected route backend: `llamacpp`
- selected route model: `qwen2.5-0.5b-instruct-q4_k_m`
- source model: `Qwen/Qwen2.5-0.5B-Instruct-GGUF`
- quantization: `Q4_K_M`

## Commands Captured

- `inferctl version --json`
- `inferctl capabilities --json`
- `inferctl discover --json`
- `inferctl discover --kind llama.cpp --json`
- `inferctl discover --kind llama.cpp --format toml`
- `inferctl config init --path inferctl.toml --json`
- `inferctl config patch --from-stdin --path inferctl.toml --json < patch.llamacpp.toml`
- `INFERCTL_CONFIG=inferctl.toml inferctl config validate --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl doctor --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl backends --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl models --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl model qwen2.5-0.5b-instruct-q4_k_m --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl route code --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl triage --json`

## Caveats

- This validates provider discovery, inspection, diagnosis, routing, and triage. It is not a model quality benchmark.
- The llama.cpp server does not expose its server version through `/v1/models`; the version is captured separately in `provider.llamacpp.version.txt`.
- The llama.cpp model was served with a neutral alias so public artifacts do not expose the local GGUF path.

## Redaction Review

No raw inferctl JSON fields required redaction. The artifacts use localhost provider URLs and neutral model aliases. They do not include private hostnames, home paths, Tailscale addresses, tokens, API key values, or local GGUF paths.
