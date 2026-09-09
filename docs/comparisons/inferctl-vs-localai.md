---
title: inferctl and LocalAI
description: Compare inferctl route planning with LocalAI model serving, routing, and runtime operations.
bucket: concepts
order: 50
---

# inferctl and LocalAI

**Research date:** 2026-09-08
**inferctl source baseline:** `main`, commit [`86ef4c2`](https://github.com/inferctl/inferctl/tree/86ef4c2f8eb8204bb00fe207a989bb3639952c93)
**LocalAI source baseline:** v4.9.0, commit [`f7ad3f7`](https://github.com/mudler/LocalAI/tree/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e), released 2026-08-20

## Conclusion

Use **LocalAI** to run models, receive inference requests, route a request to
one of its configured models, and operate its installed model and backend
components. LocalAI is in the inference request path.

Use **inferctl** to inspect configured local backends, collect evidence,
select a named task route, and test readiness without a model prompt. inferctl
is outside the inference request path. It does not start, load, unload, proxy,
or retry a LocalAI request.

The tools can compose. LocalAI can be one configured OpenAI-compatible backend
for inferctl. The calling application uses inferctl's route result, then sends
its request directly to LocalAI. This article does not claim that either tool
replaces the other.

## Scope and architecture

LocalAI registers OpenAI-compatible inference routes, including
`POST /v1/chat/completions`, and applies request middleware before the handler
serves the request. A configured LocalAI router can classify that request and
rewrite its model to a candidate or configured fallback. This is request-time
work inside the LocalAI server. See the pinned [OpenAI route registration](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/core/http/routes/openai.go#L36-L94), [router configuration](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/core/config/model_config.go#L290-L345), and [router middleware](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/core/http/middleware/route_model.go#L151-L235).

inferctl reports a selection before a caller sends a request. The caller owns
the request, authentication, retries, streaming, timeouts, and any lifecycle
action. The [preparation contract](https://github.com/inferctl/inferctl/blob/86ef4c2f8eb8204bb00fe207a989bb3639952c93/docs/preparation-contract.md) defines that boundary.

## Capability comparison

| Area | LocalAI v4.9.0 | inferctl at `86ef4c2` |
| --- | --- | --- |
| Primary job | Model server, request router, and runtime operations surface. | Out-of-band local backend inspection, route planning, and readiness. |
| Inference traffic | Receives and serves requests through OpenAI-compatible and other APIs. | Does not receive, proxy, or execute inference requests. |
| Backend and model scope | Runs models through LocalAI's configured backends, models, galleries, and optional distributed workers. | Inspects configured Ollama, llama.cpp, LM Studio, MLX, and OpenAI-compatible backends. |
| Model lifecycle | Can install or remove models and backends. Its admin APIs can load or shut down a model backend. | Does not install, start, load, unload, shut down, or remove a model. |
| Health and model evidence | `GET /healthz`, `GET /readyz`, `/v1/models`, `/v1/models/capabilities`, and backend-monitor APIs expose server, model, capability, and loaded-state evidence. | `doctor`, `backends`, `models`, `route`, and `preflight` report configured-backend and selected-route evidence where an adapter exposes it. |
| Request routing and fallback | A LocalAI router selects a model while it handles a request. Its configured fallback applies to a classification error or unmatched labels, not to configuration errors. | Selects a configured named-task route and fallback chain before the caller sends a request. It does not alter a live request. |
| Readiness without a prompt | Process readiness and model-state queries can run without an inference prompt. They do not provide an inferctl named-task readiness contract. | `preflight` is a bounded no-prompt check for a named route. |
| History, metrics, and audit data | Prometheus metrics, API and backend traces, backend logs, and model/backend operation history. | Snapshots, diffs, status frames, and route and readiness reports. |
| Machine interface | HTTP APIs, OpenAPI fragments, a well-known discovery document, and CLI model commands. | CLI commands with JSON envelopes, stable error codes, schemas, and capability metadata. |

LocalAI documents `GET /healthz` as a liveness probe and `GET /readyz` as a
startup readiness probe. These checks do not prove that a selected model can
complete an application request. LocalAI also documents
`GET /v1/models/capabilities` and backend monitoring without a model prompt.
See the pinned [health route source](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/core/http/routes/health.go), [API discovery guide](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/docs/content/features/api-discovery.md), and [backend monitor guide](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/docs/content/operations/backend-monitor.md).

## What LocalAI does well

LocalAI has the functions that a model-serving system needs and inferctl does
not provide:

- It serves OpenAI-compatible chat, completion, embedding, audio, image, and
  other configured inference APIs.
- It can classify and route a live request to a configured candidate model.
- It can install and remove model and backend artifacts, pre-load models, and
  shut down a model backend.
- It exposes a detailed operations surface. The activity API records running,
  queued, failed, and finished install or removal work; LocalAI documents that
  standalone history is in memory and distributed history is stored in
  PostgreSQL.
- It provides metrics, traces, and backend logs for its own server and
  backends.

The LocalAI source registers the lifecycle, trace, and metrics routes in one
server surface. See [route registration](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/core/http/routes/localai.go#L222-L254) and [operations history](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/core/http/routes/ui_api.go#L126-L405).

## What inferctl does differently

inferctl is useful when the decision must happen before a request enters a
server. It can compare the configured local backend fleet, explain a named
task route, apply declared route requirements, and return a redacted execution
handoff. It does not need to become the request gateway to provide that
evidence.

inferctl also keeps a control-plane record through snapshots, diffs, and status
output. LocalAI provides server metrics, traces, and operation history instead.
Those LocalAI surfaces answer different questions and do not replace a
cross-backend inferctl snapshot or route explanation.

## Composition

Configure a LocalAI OpenAI-compatible endpoint as an inferctl backend. The
exact model names and authentication policy remain LocalAI configuration.

```toml
[backends.localai]
kind = "openai_compat"
base_url = "http://127.0.0.1:8080/v1"

[routing.code]
backend = "localai"
model = "<LocalAI model name>"
```

Then use this sequence:

1. Run `inferctl preflight code --json` or `inferctl route code --json`.
2. Stop or choose a permitted fallback when the control-plane result is not
   acceptable.
3. Read the redacted handoff result.
4. Send the request directly to LocalAI's configured endpoint.

The caller must not assume that an inferctl result controls LocalAI's internal
router. The reviewed sources show no automatic integration where LocalAI
consumes an inferctl route result, and no automatic integration where inferctl
calls LocalAI lifecycle APIs.

## Limits and claims not to make

- Do not call inferctl a LocalAI proxy, gateway, lifecycle manager, or runtime.
- Do not call LocalAI a read-only external control plane. Its server receives
  inference traffic and its admin APIs can change model and backend state.
- Do not treat `healthz`, `readyz`, model listings, or backend monitoring as a
  proof that an application prompt will succeed.
- Do not treat LocalAI operation history as an inferctl-compatible snapshot or
  diff format. The LocalAI history has its own retention and storage behavior.
- Do not claim that inferctl supports every LocalAI API or that LocalAI exposes
  every inferctl adapter evidence field. The integration depends on the
  configured endpoint and adapter.

## Sources

- [LocalAI v4.9.0 release](https://github.com/mudler/LocalAI/releases/tag/v4.9.0), 2026-08-20
- [LocalAI OpenAI request route source](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/core/http/routes/openai.go)
- [LocalAI router configuration source](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/core/config/model_config.go)
- [LocalAI health route source](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/core/http/routes/health.go)
- [LocalAI API discovery guide](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/docs/content/features/api-discovery.md)
- [LocalAI backend monitor guide](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/docs/content/operations/backend-monitor.md)
- [LocalAI activity and operation-history guide](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/docs/content/operations/activity.md)
- [inferctl preparation contract](https://github.com/inferctl/inferctl/blob/86ef4c2f8eb8204bb00fe207a989bb3639952c93/docs/preparation-contract.md)
- [inferctl agent guide](https://github.com/inferctl/inferctl/blob/86ef4c2f8eb8204bb00fe207a989bb3639952c93/docs/agent-guide.md)
