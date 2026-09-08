---
title: Preparation Contract
description: Prepare a local model route, then make the request in your own client.
bucket: guides
order: 25
---

# Preparation contract

Use inferctl before a local model request. inferctl checks configuration and model state. It selects a route. Your application then sends the inference request. inferctl is not in the request path.

## Complete flow

```sh
inferctl preflight code --require-capability tools --json
```

When `ok` and `data.runnable` are true, read `data.handoff`. The v1 handoff has `backend`, `base_url`, `model`, `num_ctx`, `capabilities`, `contract_version`, and `configuration_fingerprint`. Send your request to the handoff endpoint and model. Your client owns authentication, retries, streaming, timeout policy, and lifecycle actions.

The handoff has no credential value, prompt text, or private backend diagnostic data.

## Capability evidence

Each capability has a `status` and `source`. Status is `supported`, `unsupported`, or `unknown`. Source is `declared` or `observed`. A required capability must have `supported` evidence. Unknown and unsupported evidence do not satisfy a requirement. inferctl does not guess capabilities from a model name.

```toml
[models.code_small]
backend = "ollama"
model = "qwen3:8b"

[models.code_small.capabilities.tools]
status = "supported"
source = "declared"
```

## Decision results

| Result | Meaning | Action |
|---|---|---|
| `ok: true`, `runnable: true` | A route and handoff are ready. | Make the request in your client. |
| `E_ROUTE_REQUIREMENTS_UNSATISFIED` | No candidate has required supported evidence. | Add valid evidence or change the requirement. |
| fingerprint mismatch | Configuration changed after your saved result. | Run preflight again. |
| `E_CONFIG_VALIDATION_FAILED` | The local configuration is invalid. | Run `inferctl config validate --json`. |

Use `inferctl config fingerprint --json` when you save a handoff. Compare it before a later request. Use `commands` and `errors.did_you_mean` for safe next actions. See [the error catalog](/docs/errors/) for code details.

`inferctl schema --json` and `inferctl config schema --json` are the machine references for this contract.
