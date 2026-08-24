---
title: Landscape
description: See where inferctl fits in the local inference tooling stack.
bucket: project
order: 10
---

# The local-inference landscape

Most "X vs Y" arguments about local LLM tooling are really layer confusion — someone comparing a runtime to a gateway, or a proxy to a control plane. They aren't rivals; they do different jobs at different levels of the stack. Here's the whole stack, so you can see which tools compete and which compose.

## The layers

| Layer | Job it leads with | Tools that live here | In the request path? |
|---|---|---|---|
| **Runtime** | Load and run a model | Ollama, llama.cpp, LM Studio, MLX | Yes |
| **Process swapper** | Load/unload runtimes on demand | llama-swap | Yes |
| **Gateway / proxy** | One API in front of many backends; keys, budgets, guardrails | LiteLLM, Portkey, Mozilla Otari | Yes |
| **Control plane** | Inspect, decide routes, report — out of band | inferctl | No |

Layers blur. Real tools smear across them — Ollama is a runtime with a built-in server; LiteLLM is a gateway with an SDK. But the job a tool *leads with* tells you where it lives, and which question it answers.

Analogy: a busy restaurant. Runtimes are the stoves — where food actually cooks. The swapper is the line cook rotating pots on and off a burner as tickets demand. The gateway is the pass, where every plate crosses and gets checked, plated, and sent out. The control plane is the expeditor with the ticket board — reading the whole line, calling which station takes the next order, touching no food.

## Runtime — "run this model"

The engine. It holds weights in memory and turns tokens into tokens. **Ollama** is the easy on-ramp; **llama.cpp** is the C/C++ engine under much of the ecosystem; **LM Studio** wraps a runtime in a desktop app; **MLX** runs models on Apple Silicon. Most expose an HTTP server, often OpenAI-compatible. Everything else in the stack exists to point traffic *at* a runtime.

## Process swapper — "load the right one on demand"

You have more models than you have RAM or VRAM to hold at once. A swapper sits in front of a runtime and loads/unloads the underlying process based on which model a request asks for. **llama-swap** is the clearest example: keyed by model name, it brings the right backend up, tears the last one down, and forwards the call. Its job is *lifecycle*, and it lives in the request path because it has to catch the request to know what to load.

## Gateway / proxy — "one API in front of many backends"

The unification layer. A gateway gives callers a single OpenAI-compatible endpoint and fans out to many backends behind it — frontier and open-weights alike — plus the policy that belongs at a chokepoint: virtual keys, per-team budgets, rate limits, caching, guardrails, request logging. **LiteLLM** is the widely-used open one; **Portkey** is a full AI gateway with observability; **Mozilla Otari** is a recent entrant spanning 40+ providers with budgets, guardrails, and usage tracking. All of them sit *in* the path — every request and response passes through, which is exactly what lets them meter and filter.

## Control plane — "what's running where, and where should this go?"

Everything above runs *in-line*: traffic flows through it. A control plane runs *out-of-band*. It inspects backends, decides which one a request should target, and reports on the whole fleet — but the inference call itself flows straight from caller to backend, never through the control plane. **inferctl** is this square: a single Go binary that answers "what's live across my Ollama, llama.cpp, LM Studio, and MLX backends, and which one should this request go to," without ever becoming a hop in the request.

A naming note: "control plane" has become a popular label — Mozilla's Otari launched calling itself exactly that. But most tools claiming the term are gateways: they centralize control, yet inference still flows through them. The test that cuts through the marketing is architectural, not linguistic — *does inference flow through the tool?* If yes, it's data plane whatever the README says. That's not a criticism of gateways; centralizing at the chokepoint is a legitimate, useful design. It's just a different architecture from a tool that nothing passes through.

## The line that matters: data plane vs control plane

The single meaningful cut through this landscape isn't runtime-vs-gateway. It's **in the request path or beside it.**

Runtimes, swappers, and gateways are the **data plane** — inference traffic physically passes through them, which is what lets them run, swap, meter, and filter it. A control plane is the data plane's counterpart: it governs and reports without touching the payload. Borrowed from networking, where the data plane forwards packets and the control plane decides where packets should go.

That's why "why isn't inferctl just a gateway?" has a clean answer: a gateway *is* the path. inferctl is deliberately not — no prompt ever crosses it, nothing to log, nothing to leak, nothing to fall over mid-request. Different job, different failure modes, different square.

## Zoom in

For a head-to-head of inferctl against the two tools it's most often confused with — Ollama CLI and llama-swap — see [Comparison](/docs/comparison/).
