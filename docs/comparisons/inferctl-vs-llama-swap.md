---
title: inferctl and llama-swap
description: Compare inferctl route planning with llama-swap request proxying and model-process lifecycle control.
bucket: concepts
order: 60
---

# inferctl and llama-swap

**Research date:** 2026-09-08
**inferctl source baseline:** `main`, commit [`86ef4c2`](https://github.com/inferctl/inferctl/tree/86ef4c2f8eb8204bb00fe207a989bb3639952c93)
**llama-swap source baseline:** v255, commit [`7761aa1`](https://github.com/mostlygeek/llama-swap/tree/7761aa13360ea379cb89366d07c2d08aa9f1ed10), released 2026-09-06

## Conclusion

Use [**llama-swap**](https://github.com/mostlygeek/llama-swap) when one server
must receive live OpenAI- or Anthropic-compatible requests, start configured
upstream processes on demand, and proxy those requests to the selected process.
llama-swap is in the request path and changes process state.

Use **inferctl** to inspect configured local backends, gather evidence, select
a named task route, and test readiness without sending a model prompt. inferctl
is outside the request path. It does not start, stop, load, unload, proxy, or
retry a llama-swap request.

The tools can compose. inferctl can inspect a configured llama-swap endpoint as
an OpenAI-compatible backend. The caller can then send its request directly to
llama-swap. Neither tool replaces the other.

## Scope and architecture

llama-swap owns an HTTP server with model-dispatched request routes, including
chat completions, completions, responses, embeddings, messages, reranking,
audio, and image routes. It selects a local or peer model, ensures the selected
process is ready, and forwards the request upstream. Its process interface says
that `EnsureReady` starts a stopped process and waits for it to serve traffic;
`ServeHTTP` forwards the request to that process. See the pinned [server route
list](https://github.com/mostlygeek/llama-swap/blob/7761aa13360ea379cb89366d07c2d08aa9f1ed10/internal/server/server.go#L100-L168) and [process interface](https://github.com/mostlygeek/llama-swap/blob/7761aa13360ea379cb89366d07c2d08aa9f1ed10/internal/process/process.go#L13-L67).

inferctl makes a route decision before the application sends a request. The
application owns request execution, retry policy, authentication, streaming,
timeouts, and lifecycle actions. See the pinned [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/86ef4c2f8eb8204bb00fe207a989bb3639952c93/docs/preparation-contract.md).

## Capability comparison

| Area | llama-swap v255 | inferctl at `86ef4c2` |
| --- | --- | --- |
| Primary job | Request proxy and process/model swapper for configured upstream servers. | Out-of-band local backend inspection, route planning, and readiness. |
| Inference traffic | Receives and forwards live inference requests. | Does not receive, proxy, or execute inference requests. |
| Backend scope | Configured compatible upstream processes and peer endpoints. | Configured Ollama, llama.cpp, LM Studio, MLX, and OpenAI-compatible backends. |
| Model lifecycle | Starts configured commands, waits for health, stops processes, and can unload models by TTL or explicit action. | Does not start, stop, load, unload, or alter a backend. |
| Health and model evidence | Configurable upstream health endpoint; `/v1/models` reports configured models and loaded or unloaded status. | `doctor`, `backends`, `models`, `route`, and `preflight` report adapter evidence for configured backends. |
| Request routing and fallback | Selects a configured model or peer for a live request. Group, matrix, selector, profile, and peer configuration can change the target. | Selects a configured named-task route and fallback chain before a request. It does not alter a live request. |
| Readiness without a prompt | Health checks and model-status listing do not need a prompt, but a request can trigger loading. | `preflight` is a bounded no-prompt readiness check for a named route. |
| History, metrics, and audit data | Stores activity metrics; optional in-memory request and response captures are bounded by configuration. | Snapshots, diffs, status frames, and route and readiness reports. |
| Machine interface | HTTP OpenAI-compatible API, `/v1/models` JSON, management APIs, Web UI, and CLI flags. | CLI commands with JSON envelopes, stable error codes, schemas, and capability metadata. |

## What llama-swap does well

llama-swap has the data-plane and lifecycle functions that inferctl does not
provide:

- It proxies live API calls to a selected upstream server.
- It starts a stopped configured process, waits for its configured health
  endpoint, and forwards the waiting request after the process is ready.
- It stops managed processes and supports automatic unload with a configured
  TTL.
- It offers grouped and matrix model-swap policies, configured aliases,
  profiles, peers, and request filters.
- It exposes model state in the OpenAI-compatible `/v1/models` response. The
  source renders local models as `loaded` or `unloaded` from the running
  process set.
- It records request activity. Its optional request and response captures are
  in a size-bounded in-memory cache, so they are not a general durable audit
  contract.

The [configuration guide](https://github.com/mostlygeek/llama-swap/blob/7761aa13360ea379cb89366d07c2d08aa9f1ed10/docs/configuration.md) and [example configuration](https://github.com/mostlygeek/llama-swap/blob/7761aa13360ea379cb89366d07c2d08aa9f1ed10/config.example.yaml) document `cmd`, `proxy`, `checkEndpoint`, `ttl`, groups, and model settings. The [model-list implementation](https://github.com/mostlygeek/llama-swap/blob/7761aa13360ea379cb89366d07c2d08aa9f1ed10/internal/server/api.go#L13-L190) provides the loaded-state evidence.

## What inferctl does differently

inferctl is useful before a caller enters a proxy or runtime. It can compare
the configured local backend fleet, explain why a named task route was
selected, apply declared capability requirements, and return a redacted
execution handoff. It stays outside the request and process-lifecycle paths.

inferctl snapshots and diffs describe its configured control-plane state.
llama-swap activity records describe requests that passed through its server.
They answer different questions.

## Composition

Configure llama-swap as an inferctl OpenAI-compatible endpoint. The exact
model IDs, process commands, upstream endpoints, and lifecycle policy stay in
llama-swap configuration.

```toml
[backends.llama_swap]
kind = "openai_compat"
base_url = "http://127.0.0.1:8080/v1"

[routing.code]
backend = "llama_swap"
model = "<llama-swap model ID>"
```

Then use this sequence:

1. Run `inferctl preflight code --json` or `inferctl route code --json`.
2. Stop or choose a permitted fallback when the control-plane result is not
   acceptable.
3. Read the redacted handoff result.
4. Send the request directly to llama-swap.

The reviewed sources show no automatic integration where llama-swap consumes
an inferctl route result. inferctl also does not call llama-swap management or
lifecycle APIs.

## Limits and claims not to make

- Do not call inferctl a llama-swap proxy, process supervisor, or model
  lifecycle manager.
- Do not call llama-swap a read-only external control plane. It receives live
  requests and starts or stops configured processes.
- Do not treat a successful llama-swap health check as a proof that every
  model prompt will complete successfully.
- Do not assume llama-swap retries a failed model generation on another model.
  The reviewed evidence covers request routing and upstream process readiness,
  not a general post-execution inference retry guarantee.
- Do not treat llama-swap activity history or optional captures as an
  inferctl-compatible snapshot or diff interface.

## Sources

- [llama-swap v255 release](https://github.com/mostlygeek/llama-swap/releases/tag/v255), 2026-09-06
- [llama-swap repository and product overview](https://github.com/mostlygeek/llama-swap)
- [llama-swap server route source](https://github.com/mostlygeek/llama-swap/blob/7761aa13360ea379cb89366d07c2d08aa9f1ed10/internal/server/server.go)
- [llama-swap process lifecycle interface](https://github.com/mostlygeek/llama-swap/blob/7761aa13360ea379cb89366d07c2d08aa9f1ed10/internal/process/process.go)
- [llama-swap model-list API source](https://github.com/mostlygeek/llama-swap/blob/7761aa13360ea379cb89366d07c2d08aa9f1ed10/internal/server/api.go)
- [llama-swap metrics and capture source](https://github.com/mostlygeek/llama-swap/blob/7761aa13360ea379cb89366d07c2d08aa9f1ed10/internal/server/metrics.go)
- [llama-swap configuration guide](https://github.com/mostlygeek/llama-swap/blob/7761aa13360ea379cb89366d07c2d08aa9f1ed10/docs/configuration.md)
- [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/86ef4c2f8eb8204bb00fe207a989bb3639952c93/docs/preparation-contract.md)
