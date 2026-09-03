---
title: Roadmap
description: Track delivered work, current work, and planned inferctl changes.
bucket: project
order: 50
---

# Roadmap

Public-facing snapshot of where inferctl is headed. Not exhaustive — the working plan lives in the repo issue tracker.

## Shipped

- Core verbs: inspect, route, doctor
- Backend support: Ollama, llama.cpp, LM Studio, MLX, OpenAI-compatible
- `github.com/inferctl/inferctl` public, Apache 2.0
- Astro documentation site at [inferctl.dev](https://inferctl.dev)
- `preflight` — readiness checks before an agent run
- `snapshot` and `diff` — point-in-time capture and structural comparison of
  control-plane state
- `status` and `dashboard` — machine status frames and a human status view
- Agent and CI examples for routing, readiness, drift checks, and status
- v0.3.0 provider matrix verification with real Ollama, llama.cpp, LM Studio,
  MLX, and OpenAI-compatible endpoints
- Clean `go install` validation for the v0.3.0 public source tag

## Next

- Use inferctl in evalctl, inferctl, and spoolctl workflows. Record user
  feedback from those workflows before selecting the next feature.
- Validate each new public source tag with a clean `go install` workflow.

## Later

- Homebrew formula (demand-triggered post-launch)
- Python SDK, thin subprocess wrapper (Tier 1)

## Explicitly out of scope

- inferctl does not proxy, retry, log, or otherwise touch inference traffic. It reports and routes at the control-plane level only — see the [Agent Guide](/docs/agent-guide/) for the boundary.

## Feedback

The maintainers use inferctl in real workflows to guide priorities. Open an
issue or discussion on [GitHub](https://github.com/inferctl/inferctl) if a need
should move up.
