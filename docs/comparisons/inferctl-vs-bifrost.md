---
title: inferctl and Bifrost
description: Compare inferctl local route planning with Bifrost provider gateway routing, failover, and observability.
bucket: concepts
order: 100
---

# inferctl and Bifrost

**Research date:** 2026-09-08
**inferctl source baseline:** `main`, commit [`55f779c`](https://github.com/inferctl/inferctl/tree/55f779c8a6405c7eef6fa41fb865b405f4cc3439)
**Bifrost source baseline:** core v1.8.5, commit [`4c6f108`](https://github.com/maximhq/bifrost/tree/4c6f108360ec10d8a3284ca3e7467737ede9a6a4), released 2026-09-08

## Conclusion

Use [**Bifrost**](https://docs.getbifrost.ai/overview) when clients need one
OpenAI-compatible gateway for configured providers, request-time fallback,
load balancing, caching, and gateway observability. Bifrost is in the
inference request path.

Use **inferctl** when an operator or agent needs a read-only, external local
decision before request submission. inferctl does not receive prompts, proxy
provider traffic, retry a live request, enforce Bifrost governance, or change
Bifrost provider configuration.

## Scope and architecture

Bifrost describes itself as an AI gateway that unifies configured providers
behind one OpenAI-compatible API. Its documented request endpoint is
`/v1/chat/completions`; it advertises automatic fallback and load balancing.
The source fallback logic only proceeds when the primary error permits it and
the request declares fallback providers. See the pinned [overview](https://github.com/maximhq/bifrost/blob/4c6f108360ec10d8a3284ca3e7467737ede9a6a4/README.md#L14-L45), [features](https://github.com/maximhq/bifrost/blob/4c6f108360ec10d8a3284ca3e7467737ede9a6a4/README.md#L82-L105), and [fallback decision code](https://github.com/maximhq/bifrost/blob/4c6f108360ec10d8a3284ca3e7467737ede9a6a4/core/bifrost.go#L5053-L5129).

inferctl makes a route decision before the caller enters a gateway. The caller
owns the request, authentication, streaming, timeout, retry, and Bifrost
policy. See the pinned [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md).

## Capability comparison

| Area | Bifrost core v1.8.5 | inferctl at `55f779c` |
| --- | --- | --- |
| Primary job | Multi-provider request gateway. | Out-of-band local backend inspection, route planning, and readiness. |
| Inference traffic | Receives and dispatches live provider requests. | Does not receive, proxy, or execute inference requests. |
| Fallback and balancing | Can apply configured fallback, retry, and load-balancing policy while a request is active. | Selects a named-task route and declared fallback chain before a request. |
| Provider scope | Configured cloud and compatible providers, including local-provider endpoints. | Configured local runtimes and OpenAI-compatible endpoints. |
| Runtime lifecycle | Does not claim ownership of local model-server processes. | Does not start, stop, load, unload, or alter a backend. |
| Health and metrics | Gateway `/health` and Prometheus `/metrics`; observability is gateway-oriented. | Bounded no-prompt preflight plus snapshots, diffs, and status frames. |
| Policy and management | Provider settings, virtual-key and governance features, and gateway configuration. | Local route requirements and JSON CLI reports. |

## What Bifrost does well

- It provides one request API for many configured providers.
- It can apply fallback, retry, load-balancing, cache, and governance policy
  while it owns a live request.
- It exposes gateway health and Prometheus metrics.
- It can work as a drop-in compatible provider endpoint for client SDKs.

The repository documents the health and metrics endpoints in its [performance guide](https://github.com/maximhq/bifrost/blob/4c6f108360ec10d8a3284ca3e7467737ede9a6a4/docs/providers/performance.mdx#L442-L461). Configuration examples show provider retry settings and configured fallback directions: [Helm values](https://github.com/maximhq/bifrost/blob/4c6f108360ec10d8a3284ca3e7467737ede9a6a4/helm-charts/bifrost/values.yaml#L1277-L1279).

## What inferctl does differently

inferctl compares configured local backend evidence and explains its selected
named-task route before the client submits a prompt. It returns a redacted
handoff and does not change Bifrost state.

Bifrost health and metrics describe the gateway process and traffic. They are
not an inferctl-compatible local installed-model inventory, state snapshot, or
route preflight result. A successful check also does not prove that every
configured provider will complete a future request.

## Composition

Configure Bifrost as an inferctl OpenAI-compatible endpoint. Keep provider
credentials, gateway policies, caching, and request-time fallback rules in
Bifrost.

```toml
[backends.bifrost]
kind = "openai_compat"
base_url = "http://127.0.0.1:8080/v1"

[routing.code]
backend = "bifrost"
model = "<Bifrost provider/model ID>"
```

Run `inferctl route code --json` or `inferctl preflight code --json`, assess
the handoff, then send the request to Bifrost. The reviewed sources show no
automatic Bifrost integration that consumes an inferctl route result.

## Limits and claims not to make

- Do not call inferctl a Bifrost proxy, cache, gateway, or governance engine.
- Do not call Bifrost a read-only control plane. It owns live requests and can
  apply configured request-time actions.
- Do not promise a fallback where request configuration or the primary error
  disallows it.
- Do not treat Bifrost health or metrics as proof of local loaded-model state.
- Do not treat gateway telemetry as inferctl snapshots or diffs.

## Sources

- [Bifrost home](https://docs.getbifrost.ai/overview)
- [Bifrost repository](https://github.com/maximhq/bifrost)
- [Bifrost core v1.8.5 release](https://github.com/maximhq/bifrost/releases/tag/core/v1.8.5), 2026-09-08
- [Bifrost overview and API example](https://github.com/maximhq/bifrost/blob/4c6f108360ec10d8a3284ca3e7467737ede9a6a4/README.md)
- [Bifrost fallback source](https://github.com/maximhq/bifrost/blob/4c6f108360ec10d8a3284ca3e7467737ede9a6a4/core/bifrost.go#L5053-L5129)
- [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md)
