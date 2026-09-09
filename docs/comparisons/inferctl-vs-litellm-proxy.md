---
title: inferctl and LiteLLM Proxy
description: Compare inferctl local route planning with LiteLLM Proxy request routing and provider policy.
bucket: concepts
order: 70
---

# inferctl and LiteLLM Proxy

**Research date:** 2026-09-08
**inferctl source baseline:** `main`, commit [`55f779c`](https://github.com/inferctl/inferctl/tree/55f779c8a6405c7eef6fa41fb865b405f4cc3439)
**LiteLLM Proxy source baseline:** v1.100.0, commit [`e4f2526`](https://github.com/BerriAI/litellm/tree/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4), released 2026-09-06

## Conclusion

Use [**LiteLLM**](https://litellm.ai/) Proxy when clients need one LLM API,
live provider routing, retries, fallbacks, authentication, budgets, and
request-time visibility. LiteLLM Proxy is in the request path. It receives a
request, chooses a configured deployment, calls the provider, and records the
result.

Use **inferctl** when an operator or agent needs an out-of-band, read-only
local control plane. It inspects configured backends, gathers evidence,
selects a named task route, and tests readiness without sending a model prompt.
inferctl does not receive prompts, proxy inference traffic, choose a live
fallback, or manage LiteLLM models, keys, users, or teams.

The tools can compose. Configure a LiteLLM endpoint as an inferctl
OpenAI-compatible backend. Use inferctl before the caller sends a request to
LiteLLM. Neither tool automatically consumes the other's route result.

## Scope and architecture

LiteLLM Proxy provides OpenAI-compatible request routes. Its chat-completions
route reads the request body and processes it through the proxy request path.
The router selects a deployment, calls the provider, and records success or
failure. Per-request settings can define fallbacks, retry count, timeout,
routing strategy, and tag filters. See the pinned [chat route](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/proxy/proxy_server.py#L10381-L10470), [router](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/router.py#L4843-L4933), and [request routing settings](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/proxy/route_llm_request.py#L508-L535).

inferctl makes its route decision before an application sends an inference
request. The application remains responsible for request execution,
authentication, streaming, request-time retry, timeouts, and provider policy.
See the pinned [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md).

## Capability comparison

| Area | LiteLLM Proxy v1.100.0 | inferctl at `55f779c` |
| --- | --- | --- |
| Primary job | Multi-provider API gateway with request-time policy and visibility. | Out-of-band local backend inspection, route planning, and readiness. |
| Inference traffic | Receives prompts and forwards provider requests. | Does not receive, proxy, or execute inference requests. |
| Backend scope | Configured provider deployments and compatible API endpoints. | Configured Ollama, llama.cpp, LM Studio, MLX, and OpenAI-compatible backends. |
| Routing and fallback | Chooses a configured deployment for a live request. It can apply configured routing, retries, and fallbacks. | Selects a named-task route and declared fallback chain before a request. It does not alter a live request. |
| Access and policy | API keys, users, teams, budgets, model access, and management policy. | Local configuration and route requirements. It does not manage caller access or budgets. |
| Model inventory | `/v1/models` returns models available to the calling key. This inventory is configuration- and access-scoped. | `models` reports adapter evidence from configured local backends where available. |
| Health and readiness | `/health` can test configured endpoints; `/health/readiness` reports proxy and database readiness. | `preflight` is a bounded no-prompt readiness check for a named route. |
| Metrics and history | Optional Prometheus metrics plus request, health, activity, and spend records when configured. | Snapshots, diffs, status frames, and route and readiness reports. |
| Machine interface | HTTP OpenAI-compatible API, management APIs, SDKs, UI, and configuration. | CLI commands with JSON envelopes, stable error codes, schemas, and capability metadata. |

## What LiteLLM Proxy does well

LiteLLM Proxy has request-path and provider-governance functions that inferctl
does not provide:

- It accepts live client requests through a common API and calls configured
  provider deployments.
- It can route, retry, and fall back while it owns a request.
- It controls access by key, user, and team, and can apply model access and
  budget policy.
- It returns a model list for the calling key and can hide models marked
  unhealthy. This is useful for gateway clients, but it is not an inventory of
  locally installed or loaded model files.
- It provides health history and latest health results. It can expose
  Prometheus metrics when that integration is enabled.
- It stores or reads request activity and spend data when the required storage
  or metrics configuration exists.
- It provides management APIs that change gateway state, including model,
  key, user, and team management.

The pinned [`/v1/models` implementation](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/proxy/proxy_server.py#L10069-L10109) shows the access-scoped model list. The [management route list](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/proxy/_types.py#L641-L721) and [model update endpoint](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/proxy/management_endpoints/model_management_endpoints.py#L616-L655) show that this interface changes gateway state.

## What inferctl does differently

inferctl is useful before a caller enters a gateway or runtime. It compares
the configured local backend fleet, explains why a named task route was
selected, applies declared capability requirements, and returns a redacted
execution handoff. It stays outside the inference request and provider-policy
paths.

This boundary also makes the readiness distinction clear. LiteLLM `/health`
can send the text `test from litellm` to test a configured endpoint. LiteLLM
`/health/readiness` reports proxy and database readiness, not general proof
that every configured model backend is ready. inferctl `preflight` is designed
as a bounded no-prompt route check. See the pinned [LiteLLM health endpoint](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/proxy/health_endpoints/_health_endpoints.py#L996-L1017), [probe call](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/proxy/health_endpoints/_health_endpoints.py#L2060-L2068), and [readiness endpoint](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/proxy/health_endpoints/_health_endpoints.py#L1713-L1745).

LiteLLM records gateway activity and spend. inferctl snapshots and diffs
describe local control-plane state. These interfaces answer different
questions and should not be treated as substitutes.

## Composition

Configure LiteLLM Proxy as an inferctl OpenAI-compatible endpoint. Keep
LiteLLM provider credentials, budgets, keys, deployment names, and live
routing policy in LiteLLM configuration.

```toml
[backends.litellm]
kind = "openai_compat"
base_url = "http://127.0.0.1:4000/v1"

[routing.code]
backend = "litellm"
model = "<LiteLLM model group or deployment name>"
```

Then use this sequence:

1. Run `inferctl preflight code --json` or `inferctl route code --json`.
2. Stop or choose a permitted control-plane fallback when the result is not
   acceptable.
3. Read the redacted handoff result.
4. Send the request to LiteLLM Proxy.

The reviewed sources show no automatic integration where LiteLLM consumes an
inferctl route result. inferctl also does not call LiteLLM management APIs or
change LiteLLM provider policy.

## Limits and claims not to make

- Do not call inferctl a LiteLLM proxy, gateway, budget service, or provider
  policy engine.
- Do not call LiteLLM a read-only external control plane. It receives live
  requests and its management APIs can change gateway state.
- Do not call a LiteLLM `/v1/models` response proof of installed or loaded
  local models. It is configuration- and access-scoped.
- Do not call LiteLLM `/health` a no-prompt readiness check. The reviewed
  source can send a test prompt.
- Do not treat LiteLLM readiness or liveness as proof that every provider
  deployment will complete every request.
- Do not assume metrics, health history, or spend data exist without the
  relevant LiteLLM integration, storage, and configuration.
- Do not treat LiteLLM request and spend records as inferctl-compatible
  snapshots or diffs.

## Sources

- [LiteLLM home](https://litellm.ai/)
- [LiteLLM Proxy documentation](https://docs.litellm.ai/)
- [LiteLLM v1.100.0 release](https://github.com/BerriAI/litellm/releases/tag/v1.100.0), 2026-09-06
- [LiteLLM v1.100.0 source](https://github.com/BerriAI/litellm/tree/v1.100.0)
- [LiteLLM Proxy chat route](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/proxy/proxy_server.py#L10381-L10470)
- [LiteLLM router source](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/router.py#L4843-L4933)
- [LiteLLM health endpoint source](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/proxy/health_endpoints/_health_endpoints.py)
- [LiteLLM metrics source](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/integrations/prometheus.py#L4101-L4127)
- [LiteLLM activity and spend source](https://github.com/BerriAI/litellm/blob/e4f25265704e2b2c6cf6e81be2e4c5cffff896f4/litellm/proxy/spend_tracking/spend_management_endpoints.py)
- [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md)
