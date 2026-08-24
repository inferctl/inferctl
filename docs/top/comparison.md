# inferctl vs Ollama CLI vs llama-swap

!!! info "inferctl is not a proxy"
    Inference traffic never passes through inferctl. It runs out-of-band — inspecting backends, deciding routes, and reporting — while calls flow directly from your app to the backend. Tools like LiteLLM or Mozilla's Otari sit *in* the request path (data plane); inferctl sits *beside* it (control plane). For the full stack and where each layer fits, see [Landscape](landscape.md).

Three tools, three jobs. Picking the wrong one means fighting the tool.

## tl;dr

- **Ollama CLI** — runs models. It's a runtime.
- **llama-swap** — proxies and swaps backend processes. It's a reverse proxy with a memory.
- **inferctl** — inspects and routes across whatever's already running. It's a control plane; it never touches inference traffic.

Analogy: if `ollama` is a single kitchen and `llama-swap` is the shift manager swapping cooks in and out of that one kitchen, `inferctl` is the restaurant group's ops dashboard — it looks across every kitchen you own (Ollama, llama.cpp, LM Studio, MLX) and tells you which one to send an order to. It doesn't cook.

## Table

| | inferctl | Ollama CLI | llama-swap |
|---|---|---|---|
| **What it is** | Control-plane CLI: inspect, route, doctor | Model runtime | Reverse proxy + process swapper |
| **Touches inference traffic** | No — never proxies, retries, or logs prompts/responses | Yes — is the server | Yes — is the proxy in front of the server |
| **Backend scope** | Multi-backend: Ollama, llama.cpp, LM Studio, MLX, OpenAI-compatible | Single-backend (itself) | Any OpenAI/Anthropic-compatible server (llama.cpp, vllm, etc.) |
| **Model swapping** | No — reports what's loaded, doesn't load/unload | Yes, built in | Yes — its core feature |
| **Config** | None required; reads backend state | `Modelfile` per model | Single YAML file |
| **Install** | Single Go binary | Single binary + daemon | Single Go binary |
| **Primary question it answers** | "What's running where, and which backend should this request go to?" | "Run this model." | "Route this request to the right already-configured backend process." |

## Where they compose

Not mutually exclusive. A common stack: `llama-swap` fronting several `llama.cpp` configs, `ollama` running separately for quick pulls, and `inferctl` sitting above both — reporting live state across the two and giving agents one place to ask "what's available" instead of hardcoding endpoints for each.

## Where inferctl doesn't compete

llama-swap solves a real problem inferctl deliberately avoids: it's fine with inferctl not swapping models, because swapping is data-plane-adjacent lifecycle control, not inspection. If what you need is hot-swapping, use llama-swap. If what you need is a single source of truth across tools you're already running, that's inferctl.
