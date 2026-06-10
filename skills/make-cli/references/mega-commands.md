# mega-commands.md — Single Calls That Collapse Round-Trips

A mega-command returns multiple useful slices in one invocation: data + recommendations + paste-ready follow-up commands + provenance. It is the highest-leverage agent-ergonomic move available — often the difference between 7 agent turns and 3.

## Why mega-commands matter

A canonical task without a mega-command:

```
$ <tool> status
$ <tool> list --json
$ <tool> list --filter=ready --json
$ <tool> show X-001 --json
$ <tool> plan X-001 --json
$ <tool> claim X-001
$ <tool> close X-001
```

Seven round-trips. Each costs tokens, latency, and an opportunity for the agent to wander.

With `--robot-triage`:

```
$ <tool> --robot-triage
[ output includes recommendations + top 3 + paste-ready follow-up commands ]
$ <paste first follow-up>
$ <paste close command>
```

Three round-trips. Same outcome. ~60% reduction.

The `commands` field is the load-bearing part. Recommendations without `commands` force the agent to construct invocations. Data without recommendations forces the agent to choose. Both moves cost tokens. The mega-command pre-computes both.

## The four canonical shapes

### TRIAGE — ranked recommendations

For tools that recommend work: issue trackers, task graphs, build queues, dependency systems.

```json
{
  "ok": true,
  "tool_version": "0.4.1",
  "data": {
    "quick_ref": {
      "summary": "12 ready / 4 blocked / 23 total",
      "top_3": [
        {"id": "X-001", "score": 0.92, "reason": "highest impact + ready"},
        {"id": "X-007", "score": 0.81, "reason": "unblocks 3 downstream"},
        {"id": "X-013", "score": 0.74, "reason": "quick win"}
      ]
    },
    "recommendations": [
      {
        "id": "X-001",
        "title": "...",
        "score": 0.92,
        "reason_components": {"impact": 0.7, "ready": 1.0, "effort": 0.3},
        "unblocks": ["X-005", "X-009"]
      }
    ],
    "project_health": {"ready_count": 12, "blocked_count": 4}
  },
  "meta": {"data_hash": "sha256:abc", "ts_iso": "...", "contract_version": "1"},
  "commands": [
    "<tool> show X-001 --json",
    "<tool> claim X-001",
    "<tool> plan X-001"
  ],
  "warnings": []
}
```

When to use: ranked recommendations with explanations.
When not: pure data-fetch tools where there's no "what next" choice.

### DIAGNOSE — status + recommended action

For stateful tools with components that can fail independently: daemons, indexes, caches, distributed systems. Most CLIs with a `doctor`, `status`, `health`, or `info` subcommand.

```json
{
  "ok": true,
  "data": {
    "operation_outcome": {
      "kind":           "health-failure",
      "exit_code_kind": "health-failure"
    },
    "components": {
      "runtime":  {"state": "healthy",   "version": "0.5.1"},
      "index":    {"state": "degraded",  "details": "lexical-only fallback active"},
      "model_a":  {"state": "unhealthy", "error": "OOM during last load", "since": "2026-05-06T11:55:00Z"}
    },
    "recommended_action": {
      "command":        "<tool> repair --component=model_a",
      "rationale":      "model_a has been unhealthy for >5 min",
      "is_destructive": false,
      "alternatives": [
        {"command": "<tool> diagnose --component=model_a --verbose", "purpose": "investigate first"}
      ]
    },
    "fallbacks_active": [
      {"component": "index", "active_mode": "lexical", "preferred_mode": "hybrid", "reason": "rebuild-in-progress"}
    ],
    "next_check_after_seconds": 60
  },
  "meta": {"data_hash": "sha256:def", "ts_iso": "...", "contract_version": "1"},
  "commands": [
    "<tool> repair --component=model_a",
    "<tool> reindex --semantic"
  ],
  "warnings": []
}
```

When to use: state across runs, multiple components, recoverable failures.
When not: stateless filters and converters.

### PLAN — parallelizable execution proposal

For tools that track work with dependencies: build systems, test runners, task graphs.

```json
{
  "ok": true,
  "data": {
    "plan": {
      "tracks": [
        {
          "id": "track-1",
          "items": [
            {"id": "X-001", "estimated_effort": "S"},
            {"id": "X-005", "estimated_effort": "M", "depends_on_in_track": ["X-001"]}
          ]
        },
        {"id": "track-2", "items": []}
      ],
      "summary": {
        "total_items":            12,
        "parallel_tracks":        3,
        "longest_chain":          ["X-007", "X-013", "X-021"],
        "estimated_total_effort": "L"
      },
      "blocked_by_external": [
        {"id": "X-099", "external_dep": "design-review-pending"}
      ]
    }
  },
  "meta": {"data_hash": "sha256:ghi", "contract_version": "1"},
  "commands": [
    "<tool> claim X-001 X-007 X-013",
    "<tool> start track-1"
  ]
}
```

When to use: explicit dependencies between work items.
When not: tools where ordering is implicit.

### CAPABILITIES — the contract itself

Every CLI ships this. Non-optional. Full schema in `introspection.md`.

```json
{
  "tool_name":        "<tool>",
  "version":          "0.4.1",
  "contract_version": "1",
  "features":         ["json_output", "did_you_mean", "deterministic_output"],
  "commands":         { /* per-verb description + output_schema */ },
  "exit_codes":       { /* dictionary; see errors.md */ },
  "env_vars":         { /* documented */ },
  "limits":           { /* documented */ },
  "schemas_uri":      "<tool> schema --json",
  "robot_docs_uri":   "<tool> robot-docs guide"
}
```

## Decision tree: which mega-command to add

```
Always:                                    ship CAPABILITIES.
Tool is stateful (state across runs)?      add DIAGNOSE.
Tool produces ranked recommendations?      add TRIAGE.
Tool tracks work-with-dependencies?        add PLAN.
```

If multiple apply: pick the most canonical-task one as the default mega-command (e.g. `--robot-triage`); expose the others as named subcommands (`doctor --json`, `plan --json`). Cross-reference in capabilities.

## The `commands` field — design discipline

Every mega-command's `commands` field contains paste-ready follow-up commands. Pre-built strings the agent copies verbatim, not templates the agent fills in.

Good:
```json
"commands": [
  "<tool> show X-001 --json",
  "<tool> claim X-001"
]
```

Bad (agent has to template):
```json
"recommendations":   [{"id": "X-001"}],
"command_template":  "<tool> show {id} --json"
```

When a recommendation has multiple possible follow-ups (claim vs skip vs defer), use a structured form:

```json
"actions": [
  {"action": "claim", "command": "<tool> claim X-001",                    "destructive": false},
  {"action": "defer", "command": "<tool> defer X-001 --until=tomorrow",   "destructive": false},
  {"action": "skip",  "command": "<tool> skip X-001 --reason='<reason>'", "destructive": false, "requires_input": ["reason"]}
]
```

Agent branches on `action`; sees which require input filling.

## `data_hash` for change detection

Every mega-command includes `meta.data_hash`. Construction: deterministic hash of the canonical-serialized `data` field (sorted keys, no whitespace variance).

Agents skip re-invocation when hash hasn't changed:

```bash
prev_hash=$(cat .cache/triage_hash 2>/dev/null || echo "")
result=$(<tool> --robot-triage)
new_hash=$(echo "$result" | jq -r '.meta.data_hash')
if [ "$new_hash" = "$prev_hash" ]; then
  # use cached parse
fi
```

The hash must be stable across cosmetic changes (key ordering, whitespace) and change only when semantic content changes. See `determinism.md`.

## Two-phase latency pattern

Mega-commands often combine cheap slices (counts, IDs) with expensive ones (graph metrics, semantic scoring). Don't make the cheap path wait.

```json
{
  "meta": {
    "data_hash": "sha256:abc",
    "phase_status": {
      "phase_1": "computed_ms_3",
      "phase_2": {
        "pagerank":    "computed_ms_240",
        "betweenness": "timeout_ms_500",
        "centrality":  "skipped"
      }
    }
  },
  "data": {
    "phase_1": { "ready_count": 12 },
    "phase_2": {
      "pagerank":    { "values": { "X-001": 0.42 } },
      "betweenness": { "values": null, "reason": "exceeded budget" },
      "centrality":  { "values": null, "reason": "user opted out" }
    }
  }
}
```

Agents read partial results immediately, decide whether to wait for more.

Target latency for the canonical mega-call: < 1s cold cache, < 100ms warm.

## Anti-patterns

- **Data without recommendations.** Forces the agent to choose. Defeats the purpose.
- **Recommendations without `commands`.** Forces the agent to template. Same defeat.
- **Mega-command without `data_hash`.** Agent can't detect drift across calls.
- **Mixing data and prose.** No `"explanation": "I see you have 12 items..."`. Prose goes in robot-docs guide.
- **Inconsistent envelope across mega-commands.** All four shapes use the universal envelope (see `envelope.md`).
- **Network-required mega-command without declaration.** If `--robot-triage` requires network, offline agents get stuck. Either work offline OR document the requirement in capabilities.
- **Implicit mega-command name.** `<tool> triage` (no `--robot-*` or `--json` mandatory) returns a TUI by default. Make the agent-targeted form explicit: `<tool> --robot-triage` or `<tool> triage --json`.

## Implementing a mega-command

1. **Pick the shape.** TRIAGE / DIAGNOSE / PLAN / CAPABILITIES.
2. **Sketch the JSON output before writing code.** Field by field.
3. **Build the data layer.** Cache aggressively; many slices derive from shared underlying data.
4. **Wire the verb.** Global flag form (`<tool> --robot-triage`) or subcommand form (`<tool> triage --json`) — both work; pick one as canonical.
5. **Populate `commands`.** For every recommendation, derive the canonical follow-up. The highest-impact part of the implementation.
6. **Pin the schema.** Regression test asserting the output shape.
7. **Document.** Surface in `capabilities --json` and `robot-docs guide`.

## Capabilities entry for a mega-command

```json
{
  "commands": {
    "triage": {
      "description":     "Mega-command: ranked recommendations + commands + project_health.",
      "mutates":         false,
      "json":            true,
      "is_mega_command": true,
      "schema_uri":      "<tool> schema --command=triage --json"
    }
  },
  "global_flags": {
    "--robot-triage": {
      "description": "Shortcut for `<tool> triage --json`",
      "alias_for":   "triage --json"
    }
  }
}
```

If only one form ships, document only that one.

## Cross-references

- `envelope.md` — universal envelope mega-commands live in.
- `introspection.md` — capabilities entry; robot-docs guide cross-link.
- `errors.md` — exit code dictionary for mega-command failures.
- `determinism.md` — `data_hash` deterministic construction.
