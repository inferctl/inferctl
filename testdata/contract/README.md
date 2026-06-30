# Contract Goldens

This directory stores reviewable JSON examples for every v0.1 verb data shape.
The capabilities data golden is enforced byte-for-byte against
`inferctl capabilities --json | jq .data` by `scripts/check-contract-goldens.sh`.

To refresh generated goldens:

```sh
INFERCTL_UPDATE_GOLDEN=1 scripts/update-contract-goldens.sh
scripts/check-contract-goldens.sh
git diff -- internal/contract/capabilities.golden.json testdata/contract
```

Review the diff before committing. Human-rendered output is intentionally not
goldened; only JSON contract data belongs here.

## Status Frame Schema

`status.golden.json` defines the committed aggregate feed for
`inferctl status --json` and `inferctl status --json --watch`.

Top-level fields:

- `status_frame_schema_version`: schema version for the status feed.
- `contract_version`: inferctl machine-contract version.
- `summary`: counts for backends, reachable backends, exposed models, loaded
  models, route outcomes, ready routes, and warnings.
- `backends`: backend reachability rows with name, kind, endpoint identifier,
  reachability, default flag, model counts, and optional error.
- `models.exposed`: model rows visible across configured backends, including
  backend, `loaded`, and `available` state.
- `models.loaded`: loaded-model rows with backend and runtime metadata where a
  backend exposes it.
- `routes`: latest route outcome per known task class, including decision,
  candidates, and route-scoped warnings.
- `warnings`: aggregate warnings for the snapshot.
- `recommended_action`: the highest-value follow-up command and alternatives.

The status feed is read-only. It aggregates the same non-inference probes used
by `doctor`, `models`, and `route`; it must not run inference, warm models, load
models, or create a hidden data-plane request path.

Status frame data is an allowlist. It may include backend name, kind, default
flag, reachability, endpoint identifier, aggregate model counts, exposed model
names, route decisions, warning codes/messages/details, and recommended
commands. It must not include prompt text, auth headers, tokens, API keys,
secret config values, full prompt file paths, arbitrary local filesystem paths,
or raw config file content. Capture time lives in the envelope metadata, not in
status frame `data`, so `meta.data_hash` tracks stable live state.

## Status Change Event Schema

`inferctl status --json --watch --events` emits the normal newline-delimited
status snapshot envelopes. After each changed snapshot, it also emits a JSON
envelope whose `data` matches `#/definitions/status_event_batch`.

Event batches are derived by diffing consecutive status snapshots. They do not
run a separate probe path.

Top-level event-batch fields:

- `event_schema_version`: schema version for status change events.
- `contract_version`: inferctl machine-contract version.
- `captured_at_iso`: the current snapshot time.
- `since_captured_at_iso`: the previous snapshot time used as the diff base.
- `events`: ordered change records, each with `sequence`, `kind`, `subject`,
  `severity`, `summary`, `before`, and `after`.

Committed event kinds:

- `backend_reachability_changed`: backend reachability changed.
- `selected_route_changed`: selected backend/model changed for a task.
- `fallback_status_changed`: fallback use changed for a task.
- `selected_model_readiness_changed`: selected model readiness changed.
- `warning_codes_changed`: warning-code set changed.
- `error_codes_changed`: error-code set changed.
- `recommended_action_changed`: recommended command changed.
- `loaded_model_count_changed`: loaded model count changed.

Event severity, ranking, summaries, and before/after values are derived from
the same control-plane change classifier used by `inferctl diff`.
