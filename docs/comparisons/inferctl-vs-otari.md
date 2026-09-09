---
title: inferctl and Otari
description: Compare inferctl local route planning with Otari gateway routing, budget enforcement, and provider policy.
bucket: concepts
order: 90
---

# inferctl and Otari

**Research date:** 2026-09-08
**inferctl source baseline:** `main`, commit [`55f779c`](https://github.com/inferctl/inferctl/tree/55f779c8a6405c7eef6fa41fb865b405f4cc3439)
**Otari source baseline:** v0.5.1, commit [`7c256b6`](https://github.com/mozilla-ai/otari/tree/7c256b6b63c21af3d5576dfd26cde17b2ae49e34), released 2026-09-08

## Conclusion

Use [**Otari**](https://otari.ai/) when applications need an
OpenAI-compatible gateway that authenticates live requests, resolves provider
credentials, enforces budgets, selects request candidates, and records usage.
Otari is in the inference request path.

Use **inferctl** when an operator or agent needs an out-of-band, read-only
local control plane. It inspects configured backends, selects a named task
route, and tests readiness without sending a model prompt. inferctl does not
receive prompts, issue keys, hold provider credentials, enforce budgets, or
choose a live retry or fallback.

## Scope and architecture

Otari describes itself as an OpenAI-compatible LLM gateway between an
application and model providers. Its request flow authenticates the caller,
resolves credentials, reserves budget before dispatch, and records usage after
the provider call. See the pinned [product overview](https://github.com/mozilla-ai/otari/blob/7c256b6b63c21af3d5576dfd26cde17b2ae49e34/README.md#L28-L43) and [chat-completions route](https://github.com/mozilla-ai/otari/blob/7c256b6b63c21af3d5576dfd26cde17b2ae49e34/src/gateway/api/routes/chat.py#L335-L420).

Otari compiles route candidates from the selected router and failure policy.
It walks a multi-candidate plan after retryable provider failures. For
streaming, fallback can occur only before the first byte is sent; later errors
pass to the caller. See the pinned [routing compiler](https://github.com/mozilla-ai/otari/blob/7c256b6b63c21af3d5576dfd26cde17b2ae49e34/src/gateway/services/routing/compiler.py#L255-L365), [attempt rules](https://github.com/mozilla-ai/otari/blob/7c256b6b63c21af3d5576dfd26cde17b2ae49e34/src/gateway/api/routes/_attempts.py#L88-L245), and [streaming limit](https://github.com/mozilla-ai/otari/blob/7c256b6b63c21af3d5576dfd26cde17b2ae49e34/src/gateway/api/routes/chat.py#L436-L495).

inferctl makes its decision before request execution. The caller owns request
execution, authentication, provider policy, streaming, and request-time
retry. See the pinned [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md).

## Capability comparison

| Area | Otari v0.5.1 | inferctl at `55f779c` |
| --- | --- | --- |
| Primary job | Provider gateway with request-time routing, credential control, and budget policy. | Out-of-band local backend inspection, route planning, and readiness. |
| Inference traffic | Receives and sends live provider requests. | Does not receive, proxy, or execute inference requests. |
| Routing and fallback | Compiles provider candidates and can advance after defined retryable failures. | Selects a named-task route and declared fallback chain before a request. |
| Access and budgets | Validates keys, resolves user and workspace, rate-limits, and reserves budget before dispatch. | Does not manage users, keys, rate limits, credentials, or budgets. |
| Provider lifecycle | The reviewed source dispatches provider calls through any-llm. It does not show local model-server lifecycle ownership. | Does not start, stop, load, unload, or alter a backend. |
| Model and health evidence | Authorized provider catalog and provider discovery health. | Adapter evidence from configured local backends plus bounded route preflight. |
| Observability | Gateway usage, cost, token, latency, policy, and attempt records. | Snapshots, diffs, status frames, and route and readiness reports. |
| Machine interface | OpenAI-compatible API, management API, Swagger, and OpenAPI. | JSON CLI interfaces, schemas, and capability metadata. |

## What Otari does well

- It evaluates live provider candidates and failure policy while it owns a
  request.
- It controls virtual keys, provider credentials, users, workspaces, and
  budgets at the gateway boundary.
- It has management APIs for routing policies, budgets, providers, and
  credentials.
- It exposes an authorized model catalog and a provider discovery-health view.
- It records gateway-observed request, cost, token, latency, policy, and
  attempt data.

Otari routing-policy management is documented in the pinned [routing API](https://github.com/mozilla-ai/otari/blob/7c256b6b63c21af3d5576dfd26cde17b2ae49e34/src/gateway/api/routes/routing.py#L382-L643). Its budget routes are state-changing APIs: [budget API](https://github.com/mozilla-ai/otari/blob/7c256b6b63c21af3d5576dfd26cde17b2ae49e34/src/gateway/api/routes/budgets.py#L200-L449).

## What inferctl does differently

inferctl provides a local, external decision before an application calls a
gateway. It compares configured backend evidence, explains a named-task route,
applies declared requirements, and returns a redacted handoff. It does not
enter the prompt, credential, budget, or provider request paths.

Otari `/health/readiness` checks the database or platform reachability. Its
provider-health view can use model discovery and may be cached; a provider
that does not support listing can be degraded without being unreachable. Its
`/v1/models` response is an authorized provider catalog, not a local loaded
model inventory. See the pinned [health endpoints](https://github.com/mozilla-ai/otari/blob/7c256b6b63c21af3d5576dfd26cde17b2ae49e34/src/gateway/api/routes/health.py#L80-L143), [provider health](https://github.com/mozilla-ai/otari/blob/7c256b6b63c21af3d5576dfd26cde17b2ae49e34/src/gateway/api/routes/providers.py#L204-L289), and [model catalog](https://github.com/mozilla-ai/otari/blob/7c256b6b63c21af3d5576dfd26cde17b2ae49e34/src/gateway/api/routes/models.py#L525-L607).

## Composition

Run inferctl before a client calls Otari when local backend evidence is part of
the caller's policy. Keep Otari keys, provider credentials, budgets, route
policies, and live fallback rules in Otari.

```toml
[backends.otari]
kind = "openai_compat"
base_url = "http://127.0.0.1:8080/v1"

[routing.code]
backend = "otari"
model = "<Otari model or policy name>"
```

The caller can run `inferctl preflight code --json`, assess the result, then
send the request to Otari. The reviewed sources show no automatic Otari
integration that consumes an inferctl handoff.

## Limits and claims not to make

- Do not call inferctl an Otari gateway, credential store, or budget service.
- Do not call Otari an out-of-band read-only control plane. It receives live
  requests and management APIs change gateway state.
- Do not claim transparent fallback after streaming has started.
- Do not treat Otari provider discovery or model catalog as local installed or
  loaded model evidence.
- Do not treat Otari usage records as inferctl snapshots or diffs.

## Sources

- [Otari home](https://otari.ai/)
- [Otari repository](https://github.com/mozilla-ai/otari)
- [Otari v0.5.1 release](https://github.com/mozilla-ai/otari/releases/tag/v0.5.1), 2026-09-08
- [Otari request pipeline source](https://github.com/mozilla-ai/otari/blob/7c256b6b63c21af3d5576dfd26cde17b2ae49e34/src/gateway/api/routes/_pipeline.py)
- [Otari routing source](https://github.com/mozilla-ai/otari/blob/7c256b6b63c21af3d5576dfd26cde17b2ae49e34/src/gateway/services/routing/compiler.py)
- [Otari usage source](https://github.com/mozilla-ai/otari/blob/7c256b6b63c21af3d5576dfd26cde17b2ae49e34/src/gateway/api/routes/usage.py)
- [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md)
