# envelope.md — The Universal JSON Envelope

Every `--json` response from every verb uses the same outer shape. One envelope, every verb, every error, every success. Agents learn to parse it once.

This file covers the envelope schema, the `meta` object, error placement, graceful-degradation provenance, and the stdout/stderr split.

## Why one envelope

If every verb invents its own JSON shape, agents pay a per-verb learning cost and accumulate per-verb parsing code. A universal envelope amortizes that cost to zero: once an agent can parse `<tool> list --json`, it can parse `<tool> status --json`, `<tool> doctor --json`, and every future verb the same way.

The envelope is the wrapper. `data` is the per-verb payload. Schemas vary inside `data`; the wrapper does not.

## The envelope schema

```json
{
  "ok":           true,
  "tool_version": "0.4.1",
  "data":         { /* verb-specific payload, or null on error */ },
  "meta":         { /* always populated; see below */ },
  "warnings":     [ /* non-fatal issues */ ],
  "commands":     [ /* paste-ready follow-ups; see mega-commands.md */ ],
  "errors":       [ /* present and non-empty iff ok=false; see errors.md */ ]
}
```

Field rules:

| Field | Required | When |
|---|---|---|
| `ok` | always | `true` on success, `false` on any failure |
| `tool_version` | always | Tool's own semver |
| `data` | always present | Verb payload, or `null` on error |
| `meta` | always populated | See § meta below |
| `warnings` | always present | Empty array if none |
| `commands` | always present | Empty array if none |
| `errors` | always present | Empty array on success, non-empty on failure |

"Always present" matters: an agent that does `result.warnings.length` should never throw on missing keys. Empty arrays, not omitted fields.

## The `meta` object

`meta` carries everything about the *call* — never about the payload. Payload concerns belong in `data`.

```json
"meta": {
  "request_id":       "req_01HK3MY...",
  "ts_iso":           "2026-05-08T14:32:11.482Z",
  "elapsed_ms":       42,
  "contract_version": "1",
  "schema_version":   "2",
  "data_hash":        "sha256:8f3c...",
  "tool_version":     "0.4.1"
}
```

Required keys: `request_id`, `ts_iso`, `elapsed_ms`, `contract_version`.

Common optional keys:

| Key | Purpose |
|---|---|
| `data_hash` | Deterministic hash of canonicalized `data`; agents diff across calls |
| `schema_version` | Per-verb output schema version (evolves independently of `contract_version`) |
| `pagination` | `{cursor, has_more, total}` for list-style verbs |
| `search_mode` | Actual mode used (`hybrid` vs `lexical` etc.) |
| `fallback_tier` | Which tier ran (`primary`, `secondary`, `cold`) |
| `fallback_reason` | Human-readable cause of degradation |
| `truncated` | `{by_limit: true, omitted: 47}` when a bound clipped results |
| `cached` | `{hit: true, age_seconds: 30}` if served from cache |

The pattern: any provenance fact the agent might want to branch on goes in `meta`. Anything the agent will consume as primary content goes in `data`.

### Why timestamps live in `meta`, not `data`

Putting `ts_iso` in `data` poisons determinism — same input, different bytes on every call (see `determinism.md`). Putting it in `meta` keeps the contract: `data` is reproducible; `meta` carries the call-specific facts.

Agents that need the timestamp read it from `meta`. Agents diffing `data` across calls don't trip on it.

## Errors inside the envelope

Failure populates `errors`, sets `ok: false`, and sets `data: null` (or a partial-data payload if useful).

```json
{
  "ok":           false,
  "tool_version": "0.4.1",
  "data":         null,
  "meta":         {"request_id": "req_abc", "ts_iso": "...", "elapsed_ms": 8, "contract_version": "1"},
  "warnings":     [],
  "commands":     [],
  "errors": [
    {
      "code":         "INVALID_INPUT",
      "message":      "field 'visibility' must be one of: public, private, unlisted",
      "path":         "$.flags.visibility",
      "remediation":  "use one of: public | private | unlisted",
      "did_you_mean": "public",
      "exit_code":    1
    }
  ]
}
```

The error shape, taxonomy, and exit-code dictionary live in `errors.md`. This file just nails the placement: errors are *inside* the envelope, not on stderr only, not at the top level. Agents parse the same shape for success and failure.

Mirror `errors[0].message` to stderr for shell-script consumers. JSON-parsing agents and grep-stderr agents both work.

## Warnings vs errors

| Use `warnings` when | Use `errors` when |
|---|---|
| Operation succeeded; something noteworthy | Operation failed |
| Deprecation notice ("--skip-confirmations renamed to --force") | Invalid input |
| Partial result ("3 of 5 items fetched; 2 timed out, see details") | Resource not found |
| Degradation occurred but didn't block | Hard refusal |

A warning never causes a non-zero exit. An error always does.

```json
{
  "ok": true,
  "data": { /* the 3 fetched items */ },
  "warnings": [
    {"code": "PARTIAL_RESULT", "message": "2 items timed out", "details": {"timed_out": ["X-009", "X-011"]}}
  ],
  "errors": []
}
```

## Graceful degradation with provenance

When the tool can't run its primary path — index rebuilding, network down, cache cold, model unloaded — it returns best-effort results AND tells the agent which mode actually ran.

```json
{
  "ok": true,
  "data": {
    "results": [ /* lexical-only matches */ ]
  },
  "meta": {
    "search_mode":     "lexical",
    "search_mode_requested": "hybrid",
    "fallback_tier":   "secondary",
    "fallback_reason": "semantic index rebuilding; eta_seconds=120",
    "next_check_after_seconds": 60
  },
  "warnings": [
    {
      "code":    "DEGRADED_TIER",
      "message": "ran in lexical-only mode; semantic results unavailable"
    }
  ]
}
```

The agent learns three things:

1. Results exist — proceed.
2. They're degraded — discount confidence accordingly.
3. Check back in 60s if the gap matters.

Without this provenance, agents either retry blindly (wasteful), assume the results are full-fidelity (wrong), or refuse to use them (defeats the degradation). With it, agents make informed decisions.

### The `recommended_action` block

For diagnose / status / doctor verbs (see `mega-commands.md` § DIAGNOSE), pair degradation with a concrete next step:

```json
"data": {
  "recommended_action": {
    "command":        "<tool> reindex --semantic",
    "rationale":      "semantic index has been rebuilding >5 min",
    "is_destructive": false,
    "alternatives": [
      {"command": "<tool> status --component=index --verbose", "purpose": "investigate first"}
    ]
  }
}
```

The tool names the next step. Agents don't have to derive it from observed state.

## Stdout/stderr split

The discipline:

| Surface | Contents |
|---|---|
| stdout | The envelope JSON (and only that, when `--json`) |
| stderr | Diagnostic text mirroring `errors[0].message`, log lines, progress |

Why split: pipelines compose `stdout` → next-command. ANSI, progress bars, and log lines on stdout poison pipes. Diagnostics on stderr survive even when stdout is captured to a file.

Behavior matrix:

| Surface | Interactive TTY | Piped/redirected |
|---|---|---|
| stdout color | optional | never |
| stderr color | optional | never (NO_COLOR / not-TTY) |
| stdout progress | never (always stderr) | never |
| stderr progress | OK | suppress if non-TTY or `CI=true` |

See `conventions.md` for the TTY/CI/NO_COLOR detection rules.

### Never silent-fail

A successful exit (0) with empty stdout and no `errors` should mean "operation ran, result is empty" — and the envelope should still emit, with `data: []` or equivalent. Returning *nothing* leaves the agent unable to tell success from a crashed pipe.

```json
# Empty result done right
{"ok": true, "data": [], "meta": {...}, "warnings": [], "commands": [], "errors": []}

# Empty result done wrong
# (zero bytes on stdout)
```

The first is parseable. The second is indistinguishable from a tool that crashed before writing.

## Worked example: a full success

```bash
$ <tool> list --status=ready --limit=2 --json
```

```json
{
  "ok": true,
  "tool_version": "0.4.1",
  "data": {
    "items": [
      {"id": "X-001", "status": "ready", "score": 0.92},
      {"id": "X-007", "status": "ready", "score": 0.81}
    ]
  },
  "meta": {
    "request_id":       "req_01HK3...",
    "ts_iso":           "2026-05-08T14:32:11.482Z",
    "elapsed_ms":       18,
    "contract_version": "1",
    "schema_version":   "2",
    "data_hash":        "sha256:7c91...",
    "pagination":       {"cursor": "X-007", "has_more": true, "total": 12},
    "truncated":        {"by_limit": true, "omitted": 10}
  },
  "warnings": [],
  "commands": [],
  "errors":   []
}
```

The agent reads `data.items` for content, `meta.pagination.cursor` to fetch more, `meta.truncated` to surface the bound to the user.

## Anti-patterns

- **Per-verb wrapper.** `{"list_result": [...]}` for `list`, `{"status_info": {}}` for `status`. Each verb invents a shape; agents pay per-verb. Use the universal envelope.
- **Errors on stderr only.** JSON-parsing agents miss the failure cause. Mirror to both: `errors[]` in the envelope AND stderr.
- **Missing `meta`.** Agent can't get `contract_version`, `request_id`, or `data_hash`. Always populate.
- **Timestamps inside `data`.** Breaks determinism. Lift to `meta.ts_iso`.
- **Omitting empty arrays.** `result.warnings === undefined` requires agents to write defensive `(result.warnings || []).length`. Always emit `[]`.
- **Bare arrays at the top level.** `[{...}, {...}]` instead of `{ok, data: [...], meta, ...}`. Can't carry meta, can't carry warnings, can't carry errors.
- **`ok: true` with non-empty `errors`.** Contradictory. `errors` must be empty when `ok` is true.
- **Mixing log lines into stdout.** Progress, info, debug all go to stderr. stdout is the envelope.
- **No degradation provenance.** Tool falls back silently; agent assumes full fidelity. Surface in `meta`.

## Cross-references

- `errors.md` — error object shape, code taxonomy, exit code dictionary.
- `mega-commands.md` — the `commands` field; degradation in DIAGNOSE shape.
- `determinism.md` — why timestamps live in `meta`; `data_hash` construction.
- `schema-evolution.md` — `contract_version` and `schema_version` in `meta`.
- `conventions.md` — TTY/CI/NO_COLOR rules governing the stdout/stderr split.
