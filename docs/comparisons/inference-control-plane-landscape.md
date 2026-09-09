---
title: Inference Control-Plane Landscape
description: Compare local inference runtimes, request gateways, and inferctl as an out-of-band control plane.
bucket: concepts
order: 10
---

# Inference control-plane landscape

**Research date:** 2026-09-08
**inferctl source baseline:** `main`, commit [`86ef4c2`](https://github.com/inferctl/inferctl/tree/86ef4c2f8eb8204bb00fe207a989bb3639952c93)
**External-source review date:** 2026-09-07, unless a row gives another date

## Conclusion

Inference runtimes execute models. Request gateways receive and control live
inference requests. inferctl is different: it is an out-of-band local
inference control plane. It inspects configured backends, gathers evidence,
plans named task routes, and tests readiness without sending an inference
prompt. It does not proxy inference traffic, execute prompts, retry requests,
load models, or change backend state.

This is a living index. It records the comparison research that is complete
and the research that is still in progress. It does not make a performance,
replacement, or universal-backend claim.

## The three tool classes

| Class | Main job | Inference request path | Example role with inferctl |
| --- | --- | --- | --- |
| **Inference runtime** | Loads and executes one or more models. | Owns model execution. | inferctl can inspect the configured runtime before a client sends a request. |
| **Request gateway** | Receives, proxies, routes, retries, or observes live requests. | Is in the request path. | A caller can use inferctl for local route evidence before it calls the gateway. |
| **Out-of-band control plane** | Inspects state and prepares a route without carrying traffic. | Is outside the request path. | inferctl selects a configured route and returns readiness evidence. |

The categories can overlap in one product. For example, a server can provide
health APIs and still be a runtime or gateway when it receives inference
requests. The deciding question here is whether the tool carries the live
request.

## Comparison index

| Candidate | Class | Overlap with inferctl | Main difference | Comparison status |
| --- | --- | --- | --- | --- |
| [OpenClaw Infer](https://docs.openclaw.ai/cli/infer) | Provider inference command | Model selection and request preparation | Sends provider requests; inferctl does not. | [Complete](inferctl-vs-openclaw-infer.md) |
| [InferFlux](https://github.com/anvai-labs/inferflux) | Inference server and gateway | Backend health, model state, and routing | Serves live requests; inferctl can inspect it as a backend. | [Complete](inferctl-vs-inferflux.md) |
| [LocalAI](https://localai.io/docs/features/api-discovery/) | Local runtime with operations features | Discovery, configuration, resource, and worker evidence | Serves requests and can change runtime state; inferctl is read-only across configured backends. | [Complete](inferctl-vs-localai.md) |
| [llama-swap](https://github.com/mostlygeek/llama-swap) | Local lifecycle proxy | Compatible-server health, model routing, and profiles | Proxies requests and manages server lifecycle; inferctl does neither. | [Complete](inferctl-vs-llama-swap.md) |
| [LiteLLM Proxy](https://litellm.ai/) | Multi-provider request gateway | Provider selection, routing, fallback, policy, and visibility | Controls live requests; inferctl makes a route decision before a request. | [Complete](inferctl-vs-litellm-proxy.md) |
| [llama.cpp server router](https://github.com/ggml-org/llama.cpp) | Local server and model router | Model-name routing and load state | Owns a server process and its model subprocesses; inferctl inspects independent backends. | [Complete](inferctl-vs-llamacpp-router.md) |
| [Otari](https://otari.ai/) | Inference gateway with control-plane features | Routing, credentials, budgets, usage, and policy | Includes a gateway request path; inferctl does not carry traffic. | [Complete](inferctl-vs-otari.md) |
| [Bifrost](https://docs.getbifrost.ai/overview) | Multi-provider request gateway | Routing, failover, load balancing, and governance | Provides data-plane gateway control rather than local installed and loaded model evidence. | [Complete](inferctl-vs-bifrost.md) |
| [NVIDIA Dynamo](https://docs.nvidia.com/dynamo/) | Distributed inference platform | Worker health, routing, canaries, and capacity control | Operates distributed serving; inferctl has local, out-of-band scope. | [Complete](inferctl-vs-nvidia-dynamo.md) |
| [llm-d Router and KServe](https://github.com/llm-d/llm-d-router) | Kubernetes routing and serving stack | Model pools, deployment state, and request routing | Operates Kubernetes model servers and request paths; inferctl does not. | [Complete](inferctl-vs-llmd-router-kserve.md) |
| [Ray Serve LLM](https://docs.ray.io/en/latest/serve/llm/index.html) | Distributed LLM serving system | Deployment, health, multi-model serving, and routing | Deploys and serves traffic; inferctl does not deploy or serve models. | [Complete](inferctl-vs-ray-serve-llm.md) |
| [SGLang Model Gateway](https://github.com/sgl-project/sglang) | Model gateway | Model routing, health checks, retry, and circuit control | Handles request-time faults for SGLang deployments; inferctl checks readiness before submission. | [Complete](inferctl-vs-sglang-model-gateway.md) |
| [Ollama API](https://docs.ollama.com/api/ps) | Local inference runtime | Loaded-model list, model metadata, and runtime state | A backend that inferctl can inspect, not a separate control plane. | Integration context |
| [vLLM](https://docs.vllm.ai/en/latest/serving/online_serving/) | Inference runtime | Health, model inventory, loading, and runtime metrics | A backend and possible integration target, not a direct substitute. | Integration context |

The classes, overlaps, and differences in the table are a first-pass research
summary. The sources in the candidate names were reviewed on 2026-09-07. A
completed article pins its own project revision or source-inspection date.

## Product position

inferctl is a local control-plane CLI. It can inspect configured inference
backends, report backend and model evidence where a backend exposes it, select
a named route, and run a bounded readiness check without a model prompt. Its
machine-readable commands support scripts and agents.

inferctl does not send inference or model-lifecycle requests. The client that
uses its result owns authentication, streaming, retries, timeouts, request
execution, and lifecycle actions. A successful readiness result is evidence
for a selected route. It is not proof of model quality or a successful model
response. See the pinned [preparation contract](https://github.com/inferctl/inferctl/blob/86ef4c2f8eb8204bb00fe207a989bb3639952c93/docs/preparation-contract.md) and [agent guide](https://github.com/inferctl/inferctl/blob/86ef4c2f8eb8204bb00fe207a989bb3639952c93/docs/agent-guide.md).

## Composition

inferctl can compose with both runtimes and gateways. A typical workflow is:

1. Configure the runtime or gateway as an inferctl backend.
2. Run `inferctl route <task> --json` or `inferctl preflight <task> --json`.
3. Let the caller stop, choose a permitted fallback, or use the returned
   handoff.
4. Send the inference request directly to the selected runtime or gateway.

The runtime or gateway remains responsible for request execution and any
request-time routing, retries, model lifecycle, policy, or observability.
inferctl remains outside that request path.

## Completed research

- [inferctl compared with OpenClaw Infer](inferctl-vs-openclaw-infer.md)
- [OpenClaw preparation pipeline assessment](openclaw-preparation-pipeline-assessment.md)
- [inferctl compared with InferFlux](inferctl-vs-inferflux.md)
- [inferctl compared with LocalAI](inferctl-vs-localai.md)
- [inferctl compared with llama-swap](inferctl-vs-llama-swap.md)
- [inferctl compared with LiteLLM Proxy](inferctl-vs-litellm-proxy.md)
- [inferctl compared with llama.cpp server router](inferctl-vs-llamacpp-router.md)
- [inferctl compared with Otari](inferctl-vs-otari.md)
- [inferctl compared with Bifrost](inferctl-vs-bifrost.md)
- [inferctl compared with NVIDIA Dynamo](inferctl-vs-nvidia-dynamo.md)
- [inferctl compared with llm-d Router and KServe](inferctl-vs-llmd-router-kserve.md)
- [inferctl compared with Ray Serve LLM](inferctl-vs-ray-serve-llm.md)
- [inferctl compared with SGLang Model Gateway](inferctl-vs-sglang-model-gateway.md)

All candidates in this comparison index have a completed source-based article.
New candidates will be added only after source review.

## Sources and update policy

The table uses the primary project documentation and repositories linked in
each candidate name. Check the named project version, commit, release, or
documentation date again before publication or when this page changes. Update
this index after each completed comparison.
