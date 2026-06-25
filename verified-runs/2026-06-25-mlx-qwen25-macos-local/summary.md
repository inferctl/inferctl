# Verified Run: MLX Qwen on macOS Localhost

Date: 2026-06-25

This artifact pack records an end-to-end inferctl validation against a real local MLX server on Apple Silicon. The run validates inferctl's `mlx` adapter path against `mlx_lm.server` exposing an OpenAI-compatible `/v1/models` endpoint.

## Result

Status: pass

inferctl discovered a localhost MLX provider, patched a clean config, validated the config, inspected backend and model state, selected the configured model for the `code` task, and produced a clean triage report.

## Environment

- inferctl commit: `d8f812a8d0087fe6cb382ab67d3f352c4fa6547b`
- inferctl version: `0.2.2-0.20260625174225-d8f812a8d008`
- target: macOS arm64
- provider: `mlx-lm 0.31.3`
- runtime: `mlx 0.31.2`
- endpoint: `http://127.0.0.1:8081`
- selected route backend: `mlx`
- selected route model: `mlx-community/Qwen2.5-0.5B-Instruct-4bit`
- source model: `mlx-community/Qwen2.5-0.5B-Instruct-4bit`
- quantization: `4bit`

## Commands Captured

- `inferctl version --json`
- `inferctl capabilities --json`
- `inferctl discover --json`
- `inferctl discover --kind mlx --json`
- `inferctl discover --kind mlx --format toml`
- `inferctl config init --path init.scaffold.toml --json`
- `inferctl config patch --from-stdin --path inferctl.toml --json < patch.mlx.toml`
- `INFERCTL_CONFIG=inferctl.toml inferctl config validate --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl doctor --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl backends --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl models --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl model mlx-community/Qwen2.5-0.5B-Instruct-4bit --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl route code --json`
- `INFERCTL_CONFIG=inferctl.toml inferctl triage --json`

## Caveats

- This validates provider workflow behavior and route selection, not model quality.
- `mlx_lm.server` exposes model inventory through `/v1/models`; inferctl's `mlx` adapter treats listed models as loaded for v0.1 readiness checks.
- `config init` is captured as the stock scaffold artifact. The final run config starts from a minimal meta/profile config and applies `patch.mlx.toml` because v0.1 config patching adds and updates keys but does not delete scaffold backends.

## Redaction Review

No raw inferctl JSON fields required redaction. The artifacts use localhost provider URLs and public model identifiers. They do not include private hostnames, home paths, Tailscale addresses, tokens, API key values, or local model cache paths.
