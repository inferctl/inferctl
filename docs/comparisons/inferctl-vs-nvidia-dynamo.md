---
title: inferctl and NVIDIA Dynamo
description: Compare inferctl local route planning with NVIDIA Dynamo distributed inference serving and KV-aware routing.
bucket: concepts
order: 110
---

# inferctl and NVIDIA Dynamo

**Research date:** 2026-09-08
**inferctl source baseline:** `main`, commit [`55f779c`](https://github.com/inferctl/inferctl/tree/55f779c8a6405c7eef6fa41fb865b405f4cc3439)
**Dynamo source baseline:** v1.4.2, commit [`2ecbdfd`](https://github.com/ai-dynamo/dynamo/tree/2ecbdfdf192c69c02c6d21e931d20d3b4a0bb64a), released 2026-08-29

## Conclusion

Use [**NVIDIA Dynamo**](https://docs.nvidia.com/dynamo/) to deploy and operate
distributed inference workers, route live traffic by worker load and KV-cache
overlap, and scale prefill and decode separately. Dynamo owns a distributed
serving and request-routing system.

Use **inferctl** for a local, read-only decision before a caller submits a
request. inferctl does not deploy Kubernetes resources, create worker pools,
route live traffic, manage KV cache, scale workers, or migrate requests.

## Scope and architecture

Dynamo has two request paths: `client → Frontend → Router → workers`, or a
Kubernetes Gateway path through an Endpoint Picker Plugin. Both expose an
OpenAI-compatible API. The router selects workers using load and KV-cache
information. See the pinned [architecture overview](https://github.com/ai-dynamo/dynamo/blob/2ecbdfdf192c69c02c6d21e931d20d3b4a0bb64a/README.md#L99-L130).

It provides worker lifecycle and deployment functions for Kubernetes,
disaggregated prefill/decode serving, model deployment, fault tolerance, and
observability. This is fundamentally a serving platform, not an external
inspection CLI. inferctl makes its own decision before request execution; see
the [preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md).

## Capability comparison

| Area | NVIDIA Dynamo v1.4.2 | inferctl at `55f779c` |
| --- | --- | --- |
| Primary job | Distributed inference deployment, routing, and worker operation. | Out-of-band local backend inspection, route planning, and readiness. |
| Request path | Frontend, router or Gateway API, then inference workers. | Outside the inference request path. |
| Scheduling | KV-aware worker selection, load-aware routing, and prefill/decode topology. | Named-task route selection before a request. |
| Lifecycle | Deploys and operates workers and Kubernetes resources. | Does not change backend process or deployment state. |
| Health and fault tolerance | Worker health, canary checks, request migration, and cluster operations. | Bounded no-prompt route preflight and local state reports. |
| Observability | Platform and worker telemetry. | Snapshots, diffs, status frames, and route reports. |
| Interface | OpenAI-compatible frontend, OpenAPI, CLI, manifests, and Kubernetes APIs. | JSON CLI interfaces and schemas. |

## What Dynamo does well

- It deploys and operates multi-worker inference systems.
- It routes live requests by worker state and KV-cache locality.
- It supports independent prefill and decode scaling.
- It integrates with Kubernetes Gateway API and native Kubernetes discovery.
- It provides fault-tolerance and observability functions for serving fleets.

Its pinned README documents OpenAPI at `/openapi.json`, Kubernetes-native
discovery, and both traffic topologies: [interfaces and deployment](https://github.com/ai-dynamo/dynamo/blob/2ecbdfdf192c69c02c6d21e931d20d3b4a0bb64a/README.md#L119-L130) and [OpenAPI and discovery](https://github.com/ai-dynamo/dynamo/blob/2ecbdfdf192c69c02c6d21e931d20d3b4a0bb64a/README.md#L247-L264).

## What inferctl does differently

inferctl inspects configured local backends and returns evidence before the
application enters a serving platform. It does not substitute for worker
scheduling, model deployment, or live request fault handling.

## Composition

An application can run inferctl as a local pre-request policy check, then send
its request to a Dynamo frontend. Dynamo topology, worker resources, model
deployment, and request routing remain Dynamo configuration. The reviewed
source shows no automatic Dynamo consumption of an inferctl handoff.

## Limits and claims not to make

- Do not call inferctl a Dynamo scheduler, Kubernetes operator, or worker
  manager.
- Do not call Dynamo an out-of-band control plane. It operates the request and
  serving paths.
- Do not treat Dynamo worker health as an inferctl local snapshot or a proof
  that every later generation will succeed.

## Sources

- [NVIDIA Dynamo home](https://docs.nvidia.com/dynamo/)
- [Dynamo repository](https://github.com/ai-dynamo/dynamo)
- [Dynamo v1.4.2 release](https://github.com/ai-dynamo/dynamo/releases/tag/v1.4.2), 2026-08-29
- [Dynamo architecture source](https://github.com/ai-dynamo/dynamo/blob/2ecbdfdf192c69c02c6d21e931d20d3b4a0bb64a/README.md)
- [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md)
