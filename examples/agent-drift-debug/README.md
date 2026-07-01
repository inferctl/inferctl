# Agent Drift Debug

Compare two saved control-plane snapshots to explain why local routing changed:

```sh
inferctl diff \
  --before fixtures/last-good.snapshot.json \
  --after fixtures/today.snapshot.json
```

This demo is about inferctl routing state, not model output quality. It does not
run inference, install models, start daemons, warm models, or mutate config.

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
