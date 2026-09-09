---
title: inferctl and llama.cpp server router
description: Compare inferctl route planning with llama.cpp server router request forwarding and local model lifecycle control.
bucket: concepts
order: 80
---

# inferctl and llama.cpp server router

**Research date:** 2026-09-08
**inferctl source baseline:** `main`, commit [`55f779c`](https://github.com/inferctl/inferctl/tree/55f779c8a6405c7eef6fa41fb865b405f4cc3439)
**llama.cpp source baseline:** v0.4.0, commit [`5266f24`](https://github.com/ggml-org/llama.cpp/tree/5266f24da75dc449bd56cbed7addb9c8e4a6a73e), released 2026-09-04

## Conclusion

Use [**llama.cpp**](https://github.com/ggml-org/llama.cpp) server router mode
when one local server must receive live requests, select the requested model,
and load, unload, or proxy to local model-server child processes. It owns the
request path and model lifecycle.

Use **inferctl** when an operator or agent needs an out-of-band, read-only
decision before it sends a request. inferctl inspects configured backends,
selects a named task route, and tests readiness without a model prompt. It
does not start child processes, allocate model memory, load or unload models,
proxy requests, or retry a llama.cpp request.

## Scope and architecture

In router mode, `llama-server` dynamically loads and unloads models. The main
server reads the request model, makes sure that model is ready, and proxies the
request to the matching child HTTP server. The router also supports model
selection in a GET query. See the pinned [router overview](https://github.com/ggml-org/llama.cpp/blob/5266f24da75dc449bd56cbed7addb9c8e4a6a73e/tools/server/README.md#L1651-L1655), [request model routing](https://github.com/ggml-org/llama.cpp/blob/5266f24da75dc449bd56cbed7addb9c8e4a6a73e/tools/server/README.md#L1773-L1797), and [proxy implementation](https://github.com/ggml-org/llama.cpp/blob/5266f24da75dc449bd56cbed7addb9c8e4a6a73e/tools/server/server-models.cpp#L1900-L1942).

The router obtains models from cache, a models directory, or configured
presets. It starts child servers with assigned ports and can unload the least
recently used model at capacity. These are lifecycle actions. See the pinned
[model source rules](https://github.com/ggml-org/llama.cpp/blob/5266f24da75dc449bd56cbed7addb9c8e4a6a73e/tools/server/README.md#L1661-L1680) and [child-process implementation](https://github.com/ggml-org/llama.cpp/blob/5266f24da75dc449bd56cbed7addb9c8e4a6a73e/tools/server/server-models.cpp#L938-L1047).

inferctl decides before request execution. The caller owns the request,
authentication, streaming, timeout, retry, and lifecycle actions. See the
pinned [preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md).

## Capability comparison

| Area | llama.cpp server router v0.4.0 | inferctl at `55f779c` |
| --- | --- | --- |
| Primary job | Local request router and model-server lifecycle manager. | Out-of-band local backend inspection, route planning, and readiness. |
| Inference traffic | Receives and proxies live inference requests. | Does not receive, proxy, or execute inference requests. |
| Model lifecycle | Starts child servers, loads models, unloads models, and manages model memory state. | Does not start, stop, load, unload, or alter a backend. |
| Model selection | Reads the requested model and routes to a child server; it can autoload a missing configured model. | Selects a named-task route before a request. It does not alter a live request. |
| Model state | `/models` reports configured model metadata and loaded or unloaded state. | `models` reports adapter evidence from configured backends where available. |
| Health and metrics | Server health, model-state APIs, and optional Prometheus metrics. Router health is main-server health. | Bounded no-prompt route preflight plus snapshots, diffs, and status frames. |
| Management interface | HTTP APIs can load, unload, add, or delete models. | JSON CLI interfaces. It does not call lifecycle APIs. |

## What llama.cpp server router does well

- It carries a live request to the selected local model-server child.
- It can autoload a configured model, limit loaded models, and unload a model
  when needed for capacity.
- It exposes model state through `/models` and OpenAI-compatible model data.
- It can expose model-specific Prometheus metrics when metrics are enabled.
- It has management APIs for model add, load, unload, delete, and state events.

The router model list and management APIs are documented in the pinned
[model API section](https://github.com/ggml-org/llama.cpp/blob/5266f24da75dc449bd56cbed7addb9c8e4a6a73e/tools/server/README.md#L1799-L1916). The route registration shows these are active lifecycle APIs, not
read-only reports: [server routes](https://github.com/ggml-org/llama.cpp/blob/5266f24da75dc449bd56cbed7addb9c8e4a6a73e/tools/server/server.cpp#L200-L243).

## What inferctl does differently

inferctl can compare configured local backends, explain a named-task route,
apply declared capability requirements, and return a redacted handoff before
a caller enters the router. It does not alter llama.cpp server state.

The readiness surfaces have different meanings. In normal server operation,
`/health` changes while a model loads. In router mode, the main server keeps
that health route, so it is not proof that an arbitrary selected child model
is ready. inferctl preflight is a bounded no-prompt check for its selected
route; it also is not a proof that a later generation will succeed. See the
pinned [health behavior](https://github.com/ggml-org/llama.cpp/blob/5266f24da75dc449bd56cbed7addb9c8e4a6a73e/tools/server/README.md#L472-L483).

## Composition

Configure router mode as an inferctl OpenAI-compatible backend. Keep model
files, child-server settings, memory limits, router policies, and lifecycle
actions in llama.cpp configuration.

```toml
[backends.llama_router]
kind = "openai_compat"
base_url = "http://127.0.0.1:8080/v1"

[routing.code]
backend = "llama_router"
model = "<router model ID>"
```

Run `inferctl preflight code --json` or `inferctl route code --json`, assess
the returned handoff, then send the request directly to llama.cpp. The
reviewed sources show no automatic integration where the router consumes an
inferctl route result.

## Limits and claims not to make

- Do not call inferctl a llama.cpp request router or model lifecycle manager.
- Do not call router-mode health proof that every child model is ready.
- Do not assume router mode supplies general inference retry or cross-model
  failover. The reviewed evidence covers model routing and lifecycle control.
- Do not treat router model state as an inferctl snapshot or diff interface.

## Sources

- [llama.cpp home](https://github.com/ggml-org/llama.cpp)
- [llama.cpp v0.4.0 release](https://github.com/ggml-org/llama.cpp/releases/tag/b10809), 2026-09-04
- [llama.cpp router documentation](https://github.com/ggml-org/llama.cpp/blob/5266f24da75dc449bd56cbed7addb9c8e4a6a73e/tools/server/README.md)
- [llama.cpp router source](https://github.com/ggml-org/llama.cpp/blob/5266f24da75dc449bd56cbed7addb9c8e4a6a73e/tools/server/server-models.cpp)
- [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md)
