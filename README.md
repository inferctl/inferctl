# inferctl

**Explain your local LLM stack.**

No license granted yet; private evaluation only.

Local inference control plane. Introspection, capability detection, readiness, routing, fallback, and VRAM hygiene across multiple local backends (Ollama, llama.cpp, LM Studio, MLX, vLLM, and friends).

*kubectl for your local LLM stack.*

## Status

Pre-implementation. Planning complete; no code yet. Private repo.

## What it is

- A control plane for local inference. Knows what's running, what it can do, where new work should route.
- A single static Go binary.
- Agent-first by design (see `skills/make-cli/`).

## What it is not

- Not an OpenAI-compatible API server. (Ollama, LiteLLM, LocalAI, vLLM already cover that.)
- Not a unified LLM gateway.
- Not an agent framework.

## Repo layout

- `skills/make-cli/` — design discipline for agent-facing CLIs. Reference material for every verb shipped here.

Planning documents and historical handoffs live one directory up, outside the repo.

## License

Pending. No public distribution before resolution.
