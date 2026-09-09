---
title: inferctl and Ray Serve LLM
description: Compare inferctl local route planning with Ray Serve LLM distributed model deployment and replica routing.
bucket: concepts
order: 130
---

# inferctl and Ray Serve LLM

**Research date:** 2026-09-08
**inferctl source baseline:** `main`, commit [`55f779c`](https://github.com/inferctl/inferctl/tree/55f779c8a6405c7eef6fa41fb865b405f4cc3439)
**Ray source baseline:** Ray 2.58.0, commit [`01b49e4`](https://github.com/ray-project/ray/tree/01b49e4fa77b30bc28c69ed702dfe523fd6421d8), released 2026-08-23

## Conclusion

Use [**Ray Serve LLM**](https://docs.ray.io/en/latest/serve/llm/index.html)
to deploy, scale, and serve distributed LLM engines. It provides an
OpenAI-compatible ingress, model-to-deployment routing, replica selection,
autoscaling, and engine management.

Use **inferctl** for a local, read-only route decision before a request is
sent. inferctl does not deploy Ray actors, create GPU workers, load model
engines, select a live replica, or carry inference traffic.

## Scope and architecture

Ray Serve LLM uses `LLMServer` deployments to manage inference engine
instances. A server creates an engine client, starts a distributed executor,
and creates GPU worker actors. `OpenAiIngress` provides OpenAI-compatible
routes and maps model IDs to deployments. See the pinned [architecture source](https://github.com/ray-project/ray/blob/01b49e4fa77b30bc28c69ed702dfe523fd6421d8/doc/source/serve/llm/architecture/overview.md#L1-L168).

For each live request, ingress performs model routing and a request router
selects a deployment replica. The default is Power of Two Choices; prefix-aware
routing can use cache affinity. This is request-path behavior. See the pinned
[routing guide](https://github.com/ray-project/ray/blob/01b49e4fa77b30bc28c69ed702dfe523fd6421d8/doc/source/serve/llm/architecture/routing-policies.md#L1-L88).

inferctl decides before request execution. See the [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md).

## Capability comparison

| Area | Ray Serve LLM 2.58.0 | inferctl at `55f779c` |
| --- | --- | --- |
| Primary job | Distributed LLM deployment and serving. | Out-of-band local backend inspection, route planning, and readiness. |
| Request path | OpenAI ingress and deployment-handle routing send live requests to replicas. | Does not receive, proxy, or execute inference requests. |
| Lifecycle | Starts and places engine, actor, and GPU-worker resources. | Does not change backend process or model state. |
| Scaling | Supports replica and distributed serving patterns, including prefill/decode disaggregation. | No deployment, replica, or autoscaling control. |
| Routing | Maps model IDs to deployments and live requests to replicas. | Selects a named task route before a request. |
| Observability | Request, token, engine, GPU cache, and latency metrics through Grafana and Prometheus. | Snapshots, diffs, status frames, and route reports. |
| Interface | Python APIs, FastAPI ingress, YAML configuration, and Ray cluster tooling. | JSON CLI interfaces and schemas. |

## What Ray Serve LLM does well

- It operates distributed engine and GPU-worker deployments.
- It offers OpenAI-compatible request ingress and model multiplexing.
- It routes live requests to replicas with configurable policies.
- It supports data parallelism and separate prefill/decode deployment patterns.
- It provides service and engine metrics, including TTFT, TPOT, request rate,
  token data, and GPU cache utilization.

The pinned [observability guide](https://github.com/ray-project/ray/blob/01b49e4fa77b30bc28c69ed702dfe523fd6421d8/doc/source/serve/llm/user-guides/observability.md#L1-L76) documents the metrics scope.

## What inferctl does differently

inferctl compares configured local backend evidence and returns a redacted
handoff before a client calls a serving ingress. It does not substitute for
Ray model deployment, replica routing, autoscaling, or request telemetry.

## Composition

An application can use inferctl to make a local pre-request decision, then
call a Ray Serve LLM OpenAI-compatible endpoint. Ray configuration remains the
authority for models, engine settings, replica count, placement, routing, and
traffic. The reviewed sources show no automatic consumption of an inferctl
handoff.

## Limits and claims not to make

- Do not call inferctl a Ray deployment controller, engine manager, or
  replica router.
- Do not call Ray Serve LLM an out-of-band control plane. It owns deployment
  and request-serving functions.
- Do not treat Ray metrics as inferctl snapshots or proof of a future result.

## Sources

- [Ray Serve LLM home](https://docs.ray.io/en/latest/serve/llm/index.html)
- [Ray repository](https://github.com/ray-project/ray)
- [Ray 2.58.0 release](https://github.com/ray-project/ray/releases/tag/ray-2.58.0), 2026-08-23
- [Ray Serve LLM architecture source](https://github.com/ray-project/ray/blob/01b49e4fa77b30bc28c69ed702dfe523fd6421d8/doc/source/serve/llm/architecture/overview.md)
- [Ray Serve LLM routing source](https://github.com/ray-project/ray/blob/01b49e4fa77b30bc28c69ed702dfe523fd6421d8/doc/source/serve/llm/architecture/routing-policies.md)
- [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md)
