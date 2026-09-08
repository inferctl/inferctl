---
title: inferctl and InferFlux
description: Compare inferctl local readiness planning with the InferFlux inference server and gateway.
bucket: concepts
order: 40
---

# inferctl compared with InferFlux

**Research date:** 2026-09-07  
**inferctl baseline:** v0.3.0, commit [`c50c21c`](https://github.com/inferctl/inferctl/tree/c50c21cfff8f22e4734431534e10234d135e3b3d)  
**InferFlux baseline:** v0.1.1, commit [`c4a8a7f`](https://github.com/anvai-labs/inferflux/tree/c4a8a7f16a2bde75935aea3fa373972850d31f32)

## Decision

Use **inferctl** to inspect and select across existing inference backends. It
does not serve models or pass inference traffic.

Use **InferFlux** to serve configured model runtimes through OpenAI-compatible
HTTP APIs. Its server binary is `inferfluxd`. It receives and processes
inference requests.

InferFlux can be one backend that inferctl inspects. They are complementary,
not interchangeable.

> **Current inferctl update, 2026-09-08:** This comparison records its detailed
> command research against inferctl v0.3.0. The current `main` baseline is
> [`86ef4c2`](https://github.com/inferctl/inferctl/tree/86ef4c2f8eb8204bb00fe207a989bb3639952c93).
> It adds typed credential references, model aliases and capability evidence,
> requirement-aware selection, configuration fingerprints, and redacted
> execution handoffs. These additions do not change the request-path boundary
> in this article. See the [preparation contract](../preparation-contract.md).

> Important: InferFlux also ships a CLI named `inferctl`. It is not this
> project. Both executables have the same name. Install them in separate paths
> or use absolute paths in scripts.

## Main differences

| Area | inferctl | InferFlux |
| --- | --- | --- |
| Main job | Read-only control plane for configured backends | Inference server and operator surface |
| Inference traffic | Never passes through it | Receives and serves requests |
| Primary executable | `inferctl` | `inferfluxd`; the repository also has an unrelated `inferctl` client CLI |
| Model execution | Does not execute models | Serves configured models through its runtime backends |
| HTTP API | No inference API; emits command output | OpenAI-compatible completions, chat completions, model, and embeddings APIs; also health, metrics, and admin endpoints |
| Runtime scope | Inspects Ollama, llama.cpp, LM Studio, MLX, and generic OpenAI-compatible endpoints | CPU plus optional CUDA, ROCm, MPS, Vulkan, and MLX runtime options |
| Routing | Static task routes and fallback chains from inferctl configuration | A server scheduler and admin routing surface manage live request handling |
| Observability | Doctor reports, status frames, events, snapshots, and diffs | `/livez`, `/readyz`, `/healthz`, `/metrics`, optional UI, and admin APIs |
| Security and policy | Can use configured headers for OpenAI-compatible backends; redacts header values in diagnostics | Own API-key, RBAC, guardrail, audit, rate-limit, and admin surfaces |
| Model lifecycle | Does not load, unload, warm, release, proxy, or retry models | Runs the configured model service and exposes model/cache administration |

## Architecture boundary

inferctl is out of the request path. It checks the configured endpoint, model
inventory, and routing policy, then reports a route. The program that owns the
request connects to the selected backend.

InferFlux is in the request path. Clients call its OpenAI-compatible endpoint;
the InferFlux server schedules the work, uses a selected runtime backend, and
returns the model result. Its published API includes `/v1/completions`,
`/v1/chat/completions`, `/v1/models`, `/v1/models/{id}`, and `/v1/embeddings`.

This difference matters for failures:

- inferctl can report that an InferFlux endpoint is reachable and that a model
  is listed. It cannot prove an inference response is good.
- InferFlux can receive a request, load and execute the configured runtime,
  apply its server policy, and return a response. It is the component that can
  add request latency or fail while serving traffic.

## Best fit

| Need | Choose |
| --- | --- |
| Route one named task across several existing local backends | inferctl |
| Check a local-model job before it starts, without sending a prompt | inferctl |
| Detect control-plane change over time | inferctl |
| Serve a local GGUF model through an OpenAI-compatible API | InferFlux |
| Run concurrent requests through a shared server scheduler | InferFlux |
| Require server-side health endpoints, metrics, API keys, RBAC, guardrails, or rate limits | InferFlux |
| Use InferFlux as one member of a larger local backend fleet | Both |

## Recommended composition

Configure `inferfluxd` as an `openai_compat` backend in inferctl. Then use
inferctl for selection and readiness, while the application sends inference
requests directly to InferFlux.

Example shape:

```toml
[backends.inferflux]
kind = "openai_compat"
base_url = "http://127.0.0.1:<port>/v1"
default = false

[routing.code]
backend = "inferflux"
model = "<model-id>"
```

Run `inferctl doctor --json`, `inferctl models --json`, and
`inferctl route code --json` to inspect that configuration. Your application
then calls the InferFlux OpenAI-compatible API at the configured `base_url`.

For a non-loopback InferFlux endpoint, set `remote_allowed = true` in the
inferctl backend configuration. Add the required authentication header through
inferctl configuration when InferFlux API-key policy requires it. inferctl
redacts configured header values in diagnostic output.

## Command mapping

| Need | inferctl project | InferFlux project |
| --- | --- | --- |
| Start the server | Not supported | InferFlux `inferctl serve --config <path>` starts `inferfluxd` |
| Send a completion | Not supported | InferFlux `inferctl completion ...` or an OpenAI-compatible client |
| Send chat completion | Not supported | InferFlux `inferctl chat ...` or `POST /v1/chat/completions` |
| List configured backend health | `inferctl backends --json` | Server health endpoints report service health, not an inferctl backend fleet |
| List models across backends | `inferctl models --json` | `GET /v1/models` returns InferFlux model service data |
| Select a task route | `inferctl route <task> --json` | InferFlux has server routing administration, but not inferctl task-route explanations |
| Check automation readiness without inference | `inferctl preflight <task> --json` | No equivalent inferctl control-plane preflight command |
| Watch and compare control-plane state | `inferctl status --watch --events`, `snapshot`, and `diff` | `/metrics` and health endpoints provide service observation; they do not replace inferctl snapshots/diffs |
| Set runtime guardrails or rate limits | Not supported | InferFlux admin APIs and its own CLI |
| Download a GGUF model or create starter configuration | Not supported | InferFlux `inferctl pull ...` and `inferctl quickstart ...` |

## Performance and production status

InferFlux publishes a concurrent GGUF benchmark for an RTX 4000 Ada and
Qwen2.5-3B Q4_K_M. Its April 2026 README benchmark reports that
`inferflux_cuda` has higher throughput than Ollama and LM Studio at concurrency
8, but that `llama_cpp_cuda` is faster in that test.

Do not treat the benchmark as a general production claim. That README says
native CUDA numerical precision caused about 60 percent of responses to differ
from the reference in its benchmark, and recommends `llama_cpp_cuda` for
production in that result. Other v0.1.1 native-runtime documentation reports
accuracy parity. The published documents are not fully consistent. Validate
your model, hardware, prompt set, correctness requirements, and concurrency
level before production use.

## Limits and operational notes

- inferctl cannot start `inferfluxd`, load a model, or correct a failed
  InferFlux request. It reports the configured control-plane facts only.
- InferFlux does not replace inferctl when one workflow needs a single route
  explanation across Ollama, llama.cpp, LM Studio, MLX, and other configured
  OpenAI-compatible endpoints.
- The two projects use different configuration and command contracts. Do not
  send an inferctl-project TOML file to InferFlux or use InferFlux CLI commands
  as if they were inferctl-project commands.
- The executable name collision can cause a script to call the wrong tool.
  Verify with `inferctl version --json` and use an explicit binary path in CI.
- inferctl has no packaged releases at this baseline; it is installed with the
  Go toolchain or built from source. InferFlux documents a source build with
  `./scripts/build.sh`.

## Sources

- [inferctl v0.3.0 README](https://github.com/inferctl/inferctl/blob/c50c21cfff8f22e4734431534e10234d135e3b3d/README.md)
- [inferctl Agent Guide](https://github.com/inferctl/inferctl/blob/c50c21cfff8f22e4734431534e10234d135e3b3d/docs/agent-guide.md)
- [inferctl command reference](https://github.com/inferctl/inferctl/blob/c50c21cfff8f22e4734431534e10234d135e3b3d/docs/verbs.md)
- [InferFlux v0.1.1 README](https://github.com/anvai-labs/inferflux/blob/c4a8a7f16a2bde75935aea3fa373972850d31f32/README.md)
- [InferFlux API surface](https://github.com/anvai-labs/inferflux/blob/c4a8a7f16a2bde75935aea3fa373972850d31f32/docs/API_SURFACE.md)
- [InferFlux benchmark notes](https://github.com/anvai-labs/inferflux/blob/c4a8a7f16a2bde75935aea3fa373972850d31f32/docs/benchmarks.md)
- [InferFlux configuration reference](https://github.com/anvai-labs/inferflux/blob/c4a8a7f16a2bde75935aea3fa373972850d31f32/docs/CONFIG_REFERENCE.md)
