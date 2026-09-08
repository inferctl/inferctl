---
title: OpenClaw Preparation Pipeline Assessment
description: Review the OpenClaw request-preparation pipeline and its boundary with inferctl.
bucket: concepts
order: 30
---

# OpenClaw preparation pipeline assessment

**Purpose:** Identify the OpenClaw preparation work that inferctl does not
provide, and define a possible long-term integration direction.  
**Research date:** 2026-09-07  
**inferctl baseline:** v0.3.0, commit [`c50c21c`](https://github.com/inferctl/inferctl/tree/c50c21cfff8f22e4734431534e10234d135e3b3d)  
**OpenClaw source inspected:** local `main` checkout, commit [`c47fa27`](https://github.com/openclaw/openclaw/tree/c47fa27ac39c1b2e2486c05a038220ac01e50025), dated 2026-09-07

## Summary

OpenClaw has a structured preparation pipeline before it sends a request for
`openclaw infer model run`. The pipeline validates the command, resolves the
effective configuration and secrets, selects an agent and model, resolves
authentication and routes, loads provider runtime plugins, and selects a
transport.

inferctl already provides a broader no-prompt view of local backend state:
reachability, installed and loaded models where the backend exposes them,
explicit task routes, fallback policy, snapshots, diffs, and readiness reports.

The tools have different strengths. OpenClaw is stronger at preparing one
provider request for execution. inferctl is stronger at inspecting a local
inference fleet before execution.

> **Status update, 2026-09-08:** This assessment identified a local
> preparation contract for inferctl. The typed credential references, model
> aliases and capability evidence, requirement-aware selection, configuration
> fingerprints, and redacted execution handoff described below are now
> delivered on inferctl `main` at
> [`86ef4c2`](https://github.com/inferctl/inferctl/tree/86ef4c2f8eb8204bb00fe207a989bb3639952c93).
> The possible OpenClaw integration remains an architectural hypothesis, not a
> committed product plan. See the [preparation contract](../preparation-contract.md).

## OpenClaw model-run preparation

For the local path, `openclaw infer model run` performs the following sequence.

1. It validates the prompt, thinking level, transport flags, agent identifier,
   `provider/model` override format, and attached image files.
2. It resolves command-scoped configuration and secret references. This step
   can auto-enable required plugins and pins the resulting configuration
   snapshot.
3. It resolves the agent, configured or requested model, aliases, static model
   catalog, plugin metadata, and provider runtime.
4. It resolves provider authentication and, when relevant, selects an OpenAI
   physical route.
5. It binds the model transport and sends a one-message completion request.

The source separates model preparation from completion execution. The CLI
calls `prepareSimpleCompletionModelForAgent`, then calls
`completeWithPreparedSimpleCompletionModel` only after preparation succeeds.

```text
CLI input
  -> effective config and secrets
  -> agent and model selection
  -> auth, route, plugins, and transport
  -> provider completion request
```

This is an execution-admission system. It is more structured than a command
that only reads provider configuration and then sends a request.

## What OpenClaw checks before execution

| Check | OpenClaw behavior | Value to the caller |
| --- | --- | --- |
| Command input | Rejects blank prompts, invalid thinking levels, conflicting transport flags, malformed model overrides, and unsupported input files | Stops clear local errors before a provider call |
| Agent selection | Resolves an explicit agent or the configured system/sole agent, and rejects an ambiguous selection | Uses the right model and credential owner |
| Effective configuration | Resolves secret references, applies auto-enabled plugins, and pins the effective configuration | Uses one coherent configuration during preparation |
| Model identity | Resolves defaults, aliases, profiles, static catalog records, and explicit provider/model references | Finds the intended provider and model before execution |
| Authentication | Resolves provider credentials and returns a missing-auth failure before execution | Avoids an avoidable provider request |
| Provider route | Selects OpenAI routes after it evaluates the required authentication path | Selects the applicable endpoint and transport setup |
| Runtime integration | Loads provider runtime plugins and attaches the selected completion transport | Lets provider-specific code adjust the request path |
| Gateway admission | On `--gateway`, validates authority and acquires run admission and a prepared runtime before execution | Protects Gateway-owned execution state |

## What OpenClaw does not establish during preparation

OpenClaw preparation does not prove that a provider request will work. The
normal local `infer model run` path uses a static catalog and passes
`skipAgentDiscovery: true` to model preparation.

Before it sends the completion request, it does not establish all of these
facts:

- The provider endpoint is reachable.
- A model exists in the selected local runtime.
- A model is installed or loaded.
- A model has capacity for the request.
- A text-only model accepts an image input.
- The provider accepts the request and returns text.

These facts become known during the actual completion request. A provider error
or an empty text response causes `openclaw infer model run` to fail after it
has sent the prompt.

`openclaw infer model providers` is also not a live health command. It derives
provider configuration from stored settings, auth profiles, plugin settings,
or environment variables. Its catalog entries start as available. Treat its
result as catalog and configuration inventory, not as endpoint evidence.

## Separate OpenClaw checks

OpenClaw has two related checks outside the normal local model-run path.

### Isolated cron endpoint preflight

The isolated cron runner has a narrow local-endpoint preflight. It supports
configured Ollama and OpenAI-completions endpoints. It sends `GET /api/tags` or
`GET /models`, uses a 2.5-second timeout, caches results for five minutes, and
can select a reachable fallback candidate.

An HTTP response is enough for that preflight to consider an endpoint alive.
It does not prove valid authentication, model availability, model load state,
or successful inference. The check is cron-specific and is not used by the
normal `openclaw infer model run` command.

### Model status probe

`openclaw models status --probe` sends a synthetic model request. It is an
execution probe, not a no-prompt readiness check. It can establish more than
endpoint reachability, but it has the cost and data-handling effects of an
inference request.

## Features delivered after the assessment

The following items were candidates for the local preparation contract. They
are now delivered. The table records the boundary that guided the work.

| Candidate | Why OpenClaw has it | Fit for inferctl | Scope boundary |
| --- | --- | --- | --- |
| Typed secret references | OpenClaw resolves credentials before it prepares a request | Strong fit. inferctl should let a backend refer to an environment variable, OS secret store, or approved secret provider without storing the raw header value in TOML | Keep inference credentials opaque. Do not copy OpenClaw's full agent-profile and OAuth system |
| Execution handoff object | OpenClaw produces a prepared provider/model/auth/transport combination | Strong fit. `route --json` could include a redacted execution contract: endpoint, API family, selected model, context limit, and credential-source identity | Do not include credential material or send a request |
| Declared route requirements | OpenClaw resolves provider/model/runtime compatibility before it executes | Possible fit. A route could state requirements such as text, vision, embeddings, or an API family | Report declared and observed facts separately. Do not claim a generic endpoint proves a capability it cannot expose |
| Model identity normalization | OpenClaw resolves defaults, aliases, and catalog identities | Possible fit for configured aliases and case normalization | Preserve explicit inferctl task routes. Do not add agent-specific model selection behavior |
| Configuration fingerprint | OpenClaw pins one effective configuration during preparation | Useful fit. inferctl snapshots and route reports could include a redacted configuration fingerprint | Do not expose secrets or turn inferctl into a long-lived agent runtime |

These features improve safe integration without changing inferctl's
no-inference boundary.

## Features that should remain in OpenClaw

The following OpenClaw functions are part of agent execution. They should not
move into inferctl merely to reproduce the OpenClaw preparation pipeline.

- Agent identity, default-agent selection, and per-agent workspaces.
- OAuth profiles, provider login flows, and credential ownership.
- Plugin activation, plugin lifecycle, and provider runtime hooks.
- Gateway scopes, session ownership, leases, cancellation, and execution
  admission.
- Input conversion, prompt assembly, media handling, retries, and completion
  execution.
- Synthetic inference probes.

Keeping these boundaries avoids turning inferctl into a second agent runtime
or a request-path proxy.

## Possible long-term integration end state

A possible end state is for inferctl to become OpenClaw's portable local
inference preparation layer. This is a design hypothesis. It is not a claim
that inferctl can replace OpenClaw's existing pipeline today.

In that design, inferctl would produce a stable, no-prompt preparation result
for a requested task or capability. OpenClaw would consume the result before
it builds and sends a provider request.

```text
OpenClaw command and agent context
  -> inferctl preparation and route decision
  -> OpenClaw credential, plugin, and Gateway admission
  -> OpenClaw provider transport and request
```

The result could include these facts:

- Selected backend and model.
- Backend reachability and model inventory evidence.
- Loaded-model state when the backend supports it.
- Route and fallback decision with policy reason.
- Declared task or modality requirements.
- Context-budget result and configuration fingerprint.
- A redacted execution handoff object.

OpenClaw would still own agent-specific and provider-specific execution work.
This division would let inferctl replace the local-backend readiness and route
selection part of the pipeline, not the entire preparation system.

To become a credible internal replacement for that part, inferctl would need a
stable machine contract, secure credential references, explicit capability
requirements, and a way for OpenClaw to supply or map its provider/model
configuration. It would also need integration tests against the OpenClaw local
and Gateway execution paths.

## Comparison at the proposed boundary

| Responsibility | OpenClaw today | Possible inferctl role | OpenClaw after integration |
| --- | --- | --- | --- |
| Local endpoint reachability | Normal model run learns this during execution; cron preflight has a narrow endpoint probe | Inspect and report it without a prompt | Consume inferctl evidence before execution |
| Installed and loaded model state | Not a normal Infer preparation result | Inspect where each backend exposes the state | Use it in readiness policy |
| Task route and fallback policy | Provider/model selection is tied to agent configuration and execution | Produce an explicit named-task route and policy decision | Map the result to the selected OpenClaw provider/model |
| Secrets and credentials | Resolves profiles, secret references, and provider credentials | Refer to secrets without exposing them | Keep ownership, login, and credential materialization |
| Provider request transport | Builds provider-specific request transport | Describe the required API family and endpoint | Build and run the actual transport |
| Gateway authority and sessions | Owns admission and runtime lifecycle | Out of scope | Continue to own it |

## Sources

- [OpenClaw Infer CLI command](https://github.com/openclaw/openclaw/blob/c47fa27ac39c1b2e2486c05a038220ac01e50025/src/cli/capability-cli/model.ts)
- [OpenClaw capability helpers](https://github.com/openclaw/openclaw/blob/c47fa27ac39c1b2e2486c05a038220ac01e50025/src/cli/capability-cli/shared.ts)
- [OpenClaw simple completion preparation](https://github.com/openclaw/openclaw/blob/c47fa27ac39c1b2e2486c05a038220ac01e50025/src/agents/simple-completion-runtime.ts)
- [OpenClaw simple completion execution](https://github.com/openclaw/openclaw/blob/c47fa27ac39c1b2e2486c05a038220ac01e50025/src/agents/simple-completion-execution.ts)
- [OpenClaw isolated cron preflight](https://github.com/openclaw/openclaw/blob/c47fa27ac39c1b2e2486c05a038220ac01e50025/src/cron/isolated-agent/model-preflight.runtime.ts)
- [inferctl Agent Guide](https://github.com/inferctl/inferctl/blob/c50c21cfff8f22e4734431534e10234d135e3b3d/docs/agent-guide.md)
- [inferctl route implementation](https://github.com/inferctl/inferctl/blob/c50c21cfff8f22e4734431534e10234d135e3b3d/cmd/inferctl/route.go)
- [inferctl OpenAI-compatible adapter](https://github.com/inferctl/inferctl/blob/c50c21cfff8f22e4734431534e10234d135e3b3d/internal/backends/openaicompat/openaicompat.go)
