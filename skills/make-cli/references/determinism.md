# determinism.md — Same Input, Same Output Bytes

A deterministic CLI produces the same output bytes for the same input every time. Agents diff outputs to detect change; pipelines hash outputs to cache work; regression tests pin outputs to catch drift. Each of these breaks if the same call produces gratuitously different bytes.

This file covers the sources of non-determinism, the patterns that suppress them, bounded output, the `data_hash` field, and golden-file regression tests.

## Why determinism

Three reasons agents depend on it:

1. **Diffing.** `<tool> list --json` before vs after a mutation should differ *only* in the rows affected. If 50 timestamps changed, the agent can't see the real diff.
2. **Caching.** Pipelines cache outputs by hash. If the hash changes every call despite identical logical state, the cache never hits.
3. **Regression testing.** Pinning output to a golden file is how schema drift gets caught in CI. Non-deterministic output makes pinning impossible.

The cost of determinism is small — sorting keys, factoring out timestamps. The cost of non-determinism is paid forever by every consumer.

## Sources of non-determinism

| Source | Fix |
|---|---|
| Timestamps in `data` | Move to `meta.ts_iso`; honor `SOURCE_DATE_EPOCH` |
| Hash-map iteration order | Sort keys before serialization |
| Goroutine/thread scheduling | Sort results after collection |
| Random IDs in payload | Content-addressed IDs where possible |
| Wall-clock-dependent computation | Make explicit; document |
| Memory addresses in error traces | Strip before emitting |
| Hostnames / process IDs | Move to `meta` or strip |
| Locale-dependent sorting | Force `LC_ALL=C` internally for serialization |

The principle: anything call-specific goes in `meta`. Anything that varies between runs without semantic cause goes nowhere — strip it.

## Sort before serializing

```python
# Wrong — dict iteration order varies
output = {"items": list(items_dict.values())}
print(json.dumps(output))

# Right — sort by stable key
output = {"items": sorted(items_dict.values(), key=lambda x: x["id"])}
print(json.dumps(output, sort_keys=True))
```

Both moves matter:

- `sort_keys=True` on the JSON serializer fixes key ordering at every level.
- Sorting the list itself fixes element ordering.

For verbs with naturally ordered output (e.g. log lines, timeline events), sort by the natural order (timestamp, sequence). For unordered collections, sort by ID.

## Timestamps in `meta`, not `data`

Already covered in `envelope.md`. The shortest summary: `ts_iso` lives in `meta`, never in `data`. Agents diffing `data` don't trip on it; agents needing the timestamp read `meta`.

For data that *requires* timestamps (event logs, history records), the timestamps come from the *recorded* events, not the *current* time. A history record's `created_at` is fine inside `data` — it doesn't change between calls. Wall-clock `now()` does, and doesn't belong.

## `SOURCE_DATE_EPOCH`

The reproducible-builds standard. If `SOURCE_DATE_EPOCH` is set (a Unix timestamp), every `meta.ts_iso` derives from it:

```python
import os
from datetime import datetime, timezone

def now_iso():
    sde = os.environ.get("SOURCE_DATE_EPOCH")
    if sde:
        return datetime.fromtimestamp(int(sde), timezone.utc).isoformat()
    return datetime.now(timezone.utc).isoformat()
```

Use cases: CI rebuilds, regression-test fixtures, deterministic artifact production. Always honored; never overridden.

## Content-addressed IDs

When a resource has a natural content hash (model file, build artifact, document), prefer that hash as the ID over a random UUID:

```json
# Random UUID (bad)
{"id": "550e8400-e29b-41d4-a716-446655440000", "name": "llama-3-8b"}

# Content-addressed (good)
{"id": "sha256:8f3c...", "name": "llama-3-8b"}
```

Two identical model pulls produce the same ID. Agents dedupe. Regression tests pin IDs without needing to mock UUID generation.

Where the resource isn't content-addressable (events, user-created records), random IDs are fine — but document that they're random.

## Bounded output

Unbounded lists are a denial-of-service against agent context windows. Every list-style verb defaults to a bound:

```bash
$ <tool> list --json
# returns at most 50 items by default

$ <tool> list --limit=200 --json
# explicit override
```

Capabilities documents the default and the maximum:

```json
"commands": {
  "list": {
    "flags": [
      {"name": "--limit", "type": "int", "default": 50, "maximum": 1000}
    ]
  }
}
```

When the bound clips results, surface in `meta.truncated`:

```json
"meta": {
  "truncated":  {"by_limit": true, "omitted": 47},
  "pagination": {"cursor": "X-050", "has_more": true}
}
```

The agent sees that 47 items were omitted and decides whether to paginate, narrow with a filter, or stop.

### Pagination cursor

Cursor-based, not offset-based:

```bash
$ <tool> list --json
# returns 50 items + cursor

$ <tool> list --cursor=X-050 --json
# returns next 50 items + cursor
```

Why cursor over offset: offset-based pagination shifts under concurrent mutations (item inserted at position 5 makes offset 50 skip a row, or duplicate one). Cursor-based is stable.

Cursors are opaque strings; agents pass them back verbatim. Document the cursor's encoding semantics in `capabilities` (or don't, if they're truly opaque).

### `--fields` for sparse selection

Token-cost-aware agents want only the fields they need:

```bash
$ <tool> list --fields=id,status --json
{
  "data": {
    "items": [
      {"id": "X-001", "status": "ready"},
      {"id": "X-007", "status": "ready"}
    ]
  }
}
```

`--fields` accepts comma-separated paths. Document the supported paths in `capabilities`. Validate; reject unknown fields with `INVALID_INPUT` + `did_you_mean`.

## `data_hash` construction

Every mega-command and every list-style output includes `meta.data_hash`. The pattern:

```python
import hashlib, json

def canonicalize(obj):
    return json.dumps(obj, sort_keys=True, separators=(',', ':'))

def data_hash(data):
    canon = canonicalize(data)
    return "sha256:" + hashlib.sha256(canon.encode("utf-8")).hexdigest()
```

The hash:

- **Covers `data`, not `meta`.** Meta carries the hash itself plus other call-specific facts; including them would make the hash useless.
- **Is canonical.** Keys sorted, whitespace stripped. Cosmetic changes don't perturb it.
- **Is exported in `meta`.** Agents read it, store it, compare it.
- **Is documented.** Capabilities specifies the algorithm (`sha256`) and the canonicalization (`sort_keys=True, separators=(',', ':')`).

Agents diff using the hash:

```bash
prev=$(cat .cache/triage_hash 2>/dev/null || echo "")
result=$(<tool> --robot-triage)
new=$(echo "$result" | jq -r '.meta.data_hash')
[ "$new" = "$prev" ] && echo "no change since last call"
```

## Golden-file regression tests

Every output schema is pinned by a regression test. The test:

1. Runs the verb.
2. Strips volatile fields (`request_id`, `ts_iso`, `elapsed_ms`).
3. Diffs against the golden file.
4. Fails the build on drift.

```bash
# regression_tests/list_golden.json — checked into source
{
  "ok": true,
  "data": {
    "items": [{"id": "FIXTURE-001", "status": "ready", "score": 0.92}]
  },
  "warnings": [],
  "commands": [],
  "errors": []
}

# regression_tests/list.test.sh
set -e

got=$(<tool> list --json --fixture=fixture-001 | \
      jq 'del(.meta.request_id, .meta.ts_iso, .meta.elapsed_ms, .meta.data_hash, .tool_version)')

want=$(cat regression_tests/list_golden.json | \
       jq 'del(.meta.request_id, .meta.ts_iso, .meta.elapsed_ms, .meta.data_hash, .tool_version)')

diff <(echo "$got" | jq -S .) <(echo "$want" | jq -S .) || {
    echo "REGRESSION: list output drifted; re-pin or bump contract_version" >&2
    exit 1
}
```

The fixture mode (`--fixture=...`) is the trick that makes regression tests reliable — they don't depend on live state.

When the drift is intentional:

1. Bump `contract_version` (or `schema_version`) if breaking; see `schema-evolution.md`.
2. Re-pin: `<tool> list --json --fixture=fixture-001 | jq 'del(...)' > regression_tests/list_golden.json`.
3. Test passes again.

This is the discipline that catches accidental drift while permitting intentional evolution.

## Verifying determinism

A simple test catches most non-determinism:

```bash
# determinism.test.sh
a=$(<tool> --robot-triage --json | jq 'del(.meta.request_id, .meta.ts_iso, .meta.elapsed_ms)')
b=$(<tool> --robot-triage --json | jq 'del(.meta.request_id, .meta.ts_iso, .meta.elapsed_ms)')
diff <(echo "$a") <(echo "$b") || { echo "non-deterministic output"; exit 1; }
```

Run twice; diff. Any difference (beyond the stripped fields) is a bug. Run in CI on every PR.

For tools with internal randomness (search rankings with stochastic tiebreakers, sampling-based metrics), document the seed strategy and accept a `--seed` flag:

```bash
$ <tool> search "query" --seed=42 --json
```

With `--seed`, output is deterministic. Without it, document the non-determinism in capabilities.

## Anti-patterns

- **`ts_iso` inside `data`.** Breaks diffing. Lift to `meta`.
- **Random UUIDs where content hashes would do.** Same model, two pulls, two IDs. Wasted dedup.
- **Unbounded list output.** "List returned 47,000 items" exhausts the agent's context. Bounded default + cursor.
- **Offset-based pagination.** Shifts under concurrent mutations. Use cursors.
- **Hash includes `meta`.** Hash changes every call regardless of semantic state. Hash `data` only.
- **No `data_hash` exported.** Agents can't detect drift without re-comparing the whole payload.
- **Golden files include `request_id` / `ts_iso`.** Strip before comparison; otherwise every test run is a "drift".
- **No regression test for output shape.** Schema drifts silently. The test exists to catch you.
- **Non-deterministic ranking with no `--seed`.** Agents can't reproduce a previous run.
- **Locale-dependent sort.** `LC_ALL=C` internally, always, for serialization comparisons.
- **`SOURCE_DATE_EPOCH` ignored.** Reproducible-builds infrastructure breaks; CI golden files churn.

## Cross-references

- `envelope.md` — `meta.ts_iso`, `meta.data_hash`, `meta.pagination`, `meta.truncated`.
- `schema-evolution.md` — golden-file pinning of `capabilities`; intentional re-pin discipline.
- `mega-commands.md` — `data_hash` discipline applies most visibly to mega-commands.
- `errors.md` — `INVALID_INPUT` for unknown `--fields` paths.
- `conventions.md` — `SOURCE_DATE_EPOCH` honored as part of conventions.
