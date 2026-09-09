---
title: inferctl, llm-d Router, and KServe
description: Compare inferctl local route planning with llm-d Router request placement and KServe Kubernetes model deployment.
bucket: concepts
order: 120
---

# inferctl, llm-d Router, and KServe

**Research date:** 2026-09-08
**inferctl source baseline:** `main`, commit [`55f779c`](https://github.com/inferctl/inferctl/tree/55f779c8a6405c7eef6fa41fb865b405f4cc3439)
**llm-d Router source baseline:** v0.10.0, commit [`71f4f09`](https://github.com/llm-d/llm-d-router/tree/71f4f0999f95b96c49a9d0c4afbd18dfdb943c26), released 2026-08-17
**KServe source baseline:** v0.20.0, commit [`1fb7810`](https://github.com/kserve/kserve/tree/1fb781055dd1567164358233e1125142ca6ef1fe), released 2026-08-06

## Conclusion

Use [**llm-d Router**](https://github.com/llm-d/llm-d-router) for
Kubernetes request placement that uses worker load, prefix-cache locality, and
request priority. Use [**KServe**](https://kserve.github.io/website/) to
deploy, scale, and operate model services on Kubernetes. Both operate the
serving or request path.

Use **inferctl** for a read-only local decision before a client submits a
request. inferctl does not create Kubernetes resources, run an Envoy proxy,
select live pods, manage model-service replicas, or change traffic splits.

## Scope and architecture

llm-d Router is an inference traffic entry point. Its Endpoint Picker evaluates
an incoming request against InferencePool state, including KV-cache locality,
current load, and priority, then works with Envoy or another L7 proxy. It can
run in a standalone proxy mode or Kubernetes Gateway mode. See the pinned
[llm-d Router overview](https://github.com/llm-d/llm-d-router/blob/71f4f0999f95b96c49a9d0c4afbd18dfdb943c26/README.md#L11-L64).

KServe is a Kubernetes inference platform. It supports scalable model
deployment, request-based autoscaling, canary rollout, inference pipelines,
and multi-model serving options. See the pinned [KServe overview](https://github.com/kserve/kserve/blob/1fb781055dd1567164358233e1125142ca6ef1fe/README.md#L10-L52).

inferctl makes a decision outside this data plane and does not replace either
system. See the [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md).

## Capability comparison

| Area | llm-d Router and KServe | inferctl at `55f779c` |
| --- | --- | --- |
| Primary job | Kubernetes request placement and model-service deployment. | Out-of-band local backend inspection, route planning, and readiness. |
| Request path | Router and proxy select live inference endpoints. | Does not receive, proxy, or execute inference requests. |
| Deployment lifecycle | KServe creates and operates model-service workloads; llm-d works with active endpoint pools. | Does not deploy, scale, load, unload, or alter backends. |
| Routing | llm-d uses load, cache locality, priority, and objectives for live placement. | Chooses a named-task route before a request. |
| Rollout and scaling | KServe supports autoscaling, canaries, and service deployment modes. | No rollout, traffic-split, or replica control. |
| Health and inventory | Kubernetes service and inference-server health plus endpoint-pool state. | Adapter evidence and bounded no-prompt route preflight. |
| Interface | Kubernetes APIs, Gateway API, CRDs, manifests, and network endpoints. | JSON CLI interfaces and schemas. |

## What llm-d Router and KServe do well

- llm-d Router places a live request on a suitable Kubernetes endpoint.
- It can use KV-cache locality and load information for routing.
- KServe deploys scalable inference services and supports model-serving modes.
- KServe can apply canary traffic control and request-based autoscaling.
- The KServe V2 contract includes live, ready, and model-ready health APIs.

The pinned [KServe V2 API requirements](https://github.com/kserve/kserve/blob/1fb781055dd1567164358233e1125142ca6ef1fe/docs/predict-api/v2/required_api.md#L40-L83) describe the readiness levels. Their meaning is server- and model-service-specific; they are not an inferctl state snapshot.

## What inferctl does differently

inferctl compares configured local backend evidence and returns a redacted
handoff before a caller enters Kubernetes routing or serving. It does not own
the cluster, the model pool, or a live request. A successful inferctl preflight
is not proof of a later Kubernetes generation.

## Composition

A client or automation can run inferctl first, enforce its local policy, then
submit an inference request to the llm-d/KServe gateway endpoint. llm-d
objectives, KServe manifests, autoscaling, traffic policies, credentials, and
worker lifecycle remain under their own Kubernetes configuration. The reviewed
sources show no automatic handoff integration.

## Limits and claims not to make

- Do not call inferctl a Kubernetes gateway, scheduler, or model operator.
- Do not call llm-d Router or KServe a read-only external control plane.
- Do not treat an endpoint-pool or KServe readiness result as proof of every
  future generation.
- Do not treat Kubernetes status as inferctl-compatible snapshot or diff data.

## Sources

- [llm-d Router home](https://github.com/llm-d/llm-d-router)
- [llm-d Router v0.10.0 release](https://github.com/llm-d/llm-d-router/releases/tag/v0.10.0), 2026-08-17
- [llm-d Router architecture source](https://github.com/llm-d/llm-d-router/blob/71f4f0999f95b96c49a9d0c4afbd18dfdb943c26/README.md)
- [KServe home](https://kserve.github.io/website/)
- [KServe v0.20.0 release](https://github.com/kserve/kserve/releases/tag/v0.20.0), 2026-08-06
- [KServe source](https://github.com/kserve/kserve/tree/1fb781055dd1567164358233e1125142ca6ef1fe)
- [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/55f779c8a6405c7eef6fa41fb865b405f4cc3439/docs/preparation-contract.md)
