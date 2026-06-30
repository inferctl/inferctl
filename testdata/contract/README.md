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

## Status Snapshot Schema

`status.golden.json` defines the committed aggregate feed for
`inferctl status --json` and `inferctl status --json --watch`.

Top-level fields:

- `status_schema_version`: schema version for the status feed.
- `contract_version`: inferctl machine-contract version.
- `captured_at_iso`: snapshot capture time.
- `summary`: counts for backends, reachable backends, exposed models, loaded
  models, route outcomes, ready routes, and warnings.
- `backends`: backend reachability rows with name, kind, base URL,
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
