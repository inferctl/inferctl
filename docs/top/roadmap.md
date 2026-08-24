# Roadmap

Public-facing snapshot of where inferctl is headed. Not exhaustive — the working plan lives in the repo issue tracker.

## Shipped

- Core verbs: inspect, route, doctor
- Backend support: Ollama, llama.cpp, LM Studio, MLX, OpenAI-compatible
- `github.com/inferctl/inferctl` public, Apache 2.0

## In progress

- Backend compatibility verification across the full matrix
- Docs site (this one)

## Planned

- `preflight` — pre-flight checks against a target backend/model before an agent run
- `diff` — structural comparison between two backend states or runs
- `snapshot` — point-in-time capture of backend state, shipped alongside `diff`
- Homebrew formula (demand-triggered post-launch)
- Python SDK, thin subprocess wrapper (Tier 1)

## Explicitly out of scope

- inferctl does not proxy, retry, log, or otherwise touch inference traffic. It reports and routes at the control-plane level only — see [Agent Guide](../agent-guide.md) for the boundary.

## Feedback

Roadmap priorities shift based on real usage. Open an issue or discussion on [GitHub](https://github.com/inferctl/inferctl) if something here should move up.
