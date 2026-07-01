# Agent Drift Debug

Compare two saved control-plane snapshots to explain why local routing changed:

```sh
inferctl diff \
  --before fixtures/last-good.snapshot.json \
  --after fixtures/today.snapshot.json
```

The committed default pair shows `llamacpp_large/qwen-coder-32b.gguf` becoming
unreachable, causing `code` to fall back to `ollama_small/qwen3:8b`. A second
pair, `fixtures/config-change-before.snapshot.json` and
`fixtures/config-change-after.snapshot.json`, shows a route/config change while
both backends remain reachable.

Your agent did not necessarily get worse. Its local inference route changed.
This demo is about inferctl routing state, not answer quality or model
intelligence.

Current output for the default fixture pair:

```text
Local inference drift detected
summary: 8 change(s), 3 high

Route changed:
- before: qwen-coder-32b.gguf on llamacpp_large
- after:  qwen3:8b on ollama_small
- reason: selected route changed from qwen-coder-32b.gguf on llamacpp_large to qwen3:8b on ollama_small
- fallback: fallback introduced

Backend reachability changed:
- llamacpp_large: reachable -> unreachable:backend_unreachable (backend reachability changed (backend_unreachable))

Readiness and inventory changed:
- installed_model_count installed_models: 2 -> 1 (installed model count changed)
- loaded_model_count loaded_models: 2 -> 1 (loaded model count changed)

Diagnostics changed:
- warning_codes W_BACKEND_UNREACHABLE:  -> present (warning code set changed)
- warning_codes W_FALLBACK_USED:  -> present (warning code set changed)
- recommended_action code: inferctl model qwen-coder-32b.gguf --json -> inferctl backends --filter llamacpp_large --json (recommended action changed)
```

This grouped output is current behavior and is covered by
`goldens/reachability-drift.txt`.

## Workflow

The deterministic baseline is file-to-file `diff` over committed or captured
snapshot artifacts. Snapshots are redacted control-plane artifacts: they contain
task identity, prompt metadata, route decisions, candidate reasons, backend
reachability, model inventory summaries, warnings, errors, and recommended
actions. They do not contain prompt text.

Optional live capture uses the same snapshot/diff contract:

```sh
inferctl snapshot --task code --prompt-file task.txt --output last-good.snapshot.json
inferctl snapshot --task code --prompt-file task.txt --output today.snapshot.json
inferctl diff --before last-good.snapshot.json --after today.snapshot.json
```

Stored or relative-date flows are only convenience wrappers around the same
captured snapshot comparison:

```sh
inferctl snapshot --task code --prompt-file task.txt --store
inferctl diff --task code --since 24h
```

Use `route --explain` for the current route decision, `preflight` when
automation needs pass/fail readiness, and `diff` when comparing two captured
control-plane states.

Non-goals:

- no inference
- no model install, pull, load, or warmup
- no daemon management
- no config mutation
- no prompt-output comparison

Run the fixture checks:

```sh
./test.sh
```

## Snapshot Fixture Contract

Fixture snapshots in `fixtures/*.snapshot.json` must be generated from real
product output:

```sh
inferctl snapshot --task code --prompt-file task.txt --output fixtures/<name>.snapshot.json
```

After capture, normalize only values that make committed fixtures unstable or
private:

- `captured_at_iso`: replace with a stable RFC3339 timestamp for the scenario.
- `backend_reachability[].base_url`: replace fixture-local/private endpoints
  with stable non-private `.invalid` URLs, such as
  `http://fixture.invalid/llamacpp_large`.
- `backend_reachability[].error`: replace raw transport text with a stable
  class when present, such as `backend_unreachable`.
- Model timestamps or sizes: keep real product fields, but normalize values that
  are host-specific or nondeterministic.

Do not invent fields, delete required fields, or wrap snapshots in command
envelopes. `inferctl diff` must be able to parse every committed fixture with
the same snapshot parser used for live artifacts.

Every snapshot fixture must retain this comparable control-plane state:

- `snapshot_schema_version`, `contract_version`, `inferctl_version`,
  `captured_at_iso`, and `task`
- prompt metadata only: source kind, basename/filename when applicable,
  character/token counts, and content hash when emitted
- selected route, route candidates, candidate rejection reasons, and fallback
  status
- backend reachability, installed model summaries, loaded model summaries,
  warnings, errors, and recommended action

Prompt text, credentials, headers, private hostnames, and local absolute paths
must not appear in committed fixtures.

## Validation

Run the fixture validator after adding or changing snapshots:

```sh
./validate-fixtures.sh \
  fixtures/last-good.snapshot.json \
  fixtures/today.snapshot.json
```

For before/after scenarios, also run the actual diff command used by the README:

```sh
inferctl diff \
  --before fixtures/last-good.snapshot.json \
  --after fixtures/today.snapshot.json \
  --json
```
