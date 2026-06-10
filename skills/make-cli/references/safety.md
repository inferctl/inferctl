# safety.md — Idempotent, Gated, Reversible Mutations

Agents fail. Networks drop. Processes die mid-call. The mutation surface has to assume retries are normal and dangerous operations are common — and make both safe by construction.

This file covers idempotency, gating, safe alternatives, and multi-agent coordination via advisory reservations.

## The three properties

Every mutating verb should be:

1. **Idempotent** — retrying the same call produces the same end state, no duplication.
2. **Gated** — destructive operations require explicit consent (`--yes` or `--force`).
3. **Reversible (where possible)** — the error or dry-run output names the recovery path.

Read verbs need none of this. Mutate verbs need all of it.

## Idempotency

The pattern: every create takes either a client-supplied idempotency token OR uses a natural key that uniquely identifies the resource.

### Idempotency tokens

```bash
$ <tool> create-item --name="foo" --idempotency-key="agent-run-42" --json
{
  "ok": true,
  "data": {"id": "X-001", "created": true},
  "meta": {...}
}

# Same token, same args → returns the existing resource
$ <tool> create-item --name="foo" --idempotency-key="agent-run-42" --json
{
  "ok": true,
  "data": {"id": "X-001", "created": false, "existing": true},
  "meta": {...}
}

# Same token, different args → conflict
$ <tool> create-item --name="bar" --idempotency-key="agent-run-42" --json
{
  "ok": false,
  "errors": [{
    "code":    "IDEMPOTENCY_CONFLICT",
    "message": "key 'agent-run-42' was used previously with different args",
    "exit_code": 5
  }]
}
```

The token lives in a server-side or local-state store keyed by `(verb, token)`. Same token + same args = return the existing result. Same token + different args = explicit conflict (exit 5).

Token TTL: 24 hours is reasonable for most tools. Document in `capabilities`.

### Natural keys

When the resource has a natural unique key (name, path, hash), use it:

```bash
$ <tool> model pull llama-3-8b --json
{
  "ok": true,
  "data": {"name": "llama-3-8b", "created": true, "size_bytes": 4831233024}
}

$ <tool> model pull llama-3-8b --json     # already present
{
  "ok": true,
  "data": {"name": "llama-3-8b", "created": false, "existing": true}
}
```

Surface the `created` boolean so the agent can branch. Both calls succeed; the agent knows from `created` whether the operation did work or was a no-op.

### Idempotency without tokens or natural keys

When neither fits (e.g. logging an event), document that the verb is *not* idempotent and require an explicit ack:

```bash
$ <tool> log-event --type=signup
error: --not-idempotent: this verb creates a fresh row each call
       pass --not-idempotent to acknowledge, or use '<tool> ingest-event' for upsert semantics
```

Better: design the verb to be idempotent. The above is the fallback.

## Gating

Destructive operations require either `--yes` (skip confirmation) or `--force` (override conflict detection). Bare invocation of a destructive verb refuses:

```bash
$ <tool> delete-model llama-3-8b
error: 'delete-model' is destructive; pass --yes to confirm
  safer: <tool> stop-model llama-3-8b        # stop without delete
  force: <tool> delete-model llama-3-8b --yes
```

The conventional split:

| Flag | Semantics |
|---|---|
| `--yes`, `-y` | Skip confirmation prompts; agent already decided |
| `--force` | Override a detected conflict or safety check |
| `--dry-run` | Print what would happen; do nothing |

`--yes` for "I know this is destructive; proceed." `--force` for "I see the conflict; do it anyway." Both might apply: `--yes --force`.

### What counts as destructive

Any operation that:

- Removes data (delete, drop, purge, clear)
- Overwrites data without recovery (push --force, hard reset)
- Affects other users or processes (kill, restart, deploy)
- Triggers irreversible external side effects (send email, charge card)

Not destructive (no gating needed):

- Read operations
- Append-only writes that don't break consumers
- Internal state changes the user explicitly requested via verb name (`pause`, `resume`)

When in doubt, gate. The cost of an extra `--yes` is tiny; the cost of an accidental delete is large.

## Safe alternatives

When a destructive operation errors out, the error names the safe alternative *and* the override:

```bash
$ <tool> reset --hard HEAD~3
error: --hard reset would discard 7 uncommitted files
  safer: <tool> reset --soft HEAD~3              # keeps working tree
  safer: <tool> stash; <tool> reset --mixed HEAD~3 # preserves changes
  force: <tool> reset --hard HEAD~3 --yes        # if you really meant it
```

Three offers, in order: cheap-and-safe, slightly-more-effort-and-safe, the override.

The safe alternative is *not* `--dry-run`. Dry-run shows what the destructive call would do; it doesn't offer an alternative. Both are useful; they're different surfaces.

### Dry-run output

```bash
$ <tool> delete-model llama-3-8b --dry-run --json
{
  "ok": true,
  "data": {
    "would_delete": {
      "model_name":  "llama-3-8b",
      "size_bytes":  4831233024,
      "in_use_by":   ["session-abc", "session-def"]
    },
    "actual": false
  },
  "meta": {...},
  "warnings": [{"code": "IN_USE", "message": "model is in use by 2 sessions"}]
}
```

The agent reads `would_delete.in_use_by` and decides whether to proceed. The dry-run *is* the safety check; the agent runs it first when in doubt.

## Reversibility

For operations that can be undone, the success response includes the undo path:

```json
{
  "ok": true,
  "data": {
    "id":     "X-001",
    "action": "deleted",
    "undo":   {"command": "<tool> restore X-001 --from-trash", "valid_until_iso": "2026-05-15T14:32:11Z"}
  }
}
```

A 7-day soft-delete window is conventional for resources that aren't perf-sensitive. After the window, the resource is hard-deleted and `undo` no longer works.

When an operation is irreversible (hard delete, external API call), say so explicitly:

```json
{
  "ok": true,
  "data": {
    "id":          "X-001",
    "action":      "deleted",
    "irreversible": true
  }
}
```

Agents can surface this to the user before proceeding next time.

## Multi-agent coordination: advisory reservations

When two agents try to operate on the same resource, the second agent must learn quickly and politely. The pattern is advisory reservations with a TTL — softer than a lock, sufficient for cooperative agents.

```bash
$ <tool> reserve X-001 --ttl=300 --json
{
  "ok": true,
  "data": {
    "resource":   "X-001",
    "reserved":   true,
    "by":         "agent-run-42",
    "expires_iso": "2026-05-08T15:00:00Z",
    "reservation_token": "rsv_abc"
  }
}

$ <tool> work-on X-001                  # auto-checks reservation
$ <tool> release X-001 --token=rsv_abc  # explicit release
```

If a second agent tries to reserve:

```bash
$ <tool> reserve X-001 --ttl=300 --json
{
  "ok": false,
  "errors": [{
    "code":    "LOCKED",
    "message": "X-001 reserved by agent-run-42 until 2026-05-08T15:00:00Z",
    "exit_code": 4
  }],
  "data": {
    "recommended_action": {
      "command":   "<tool> reserve X-001 --wait=300s",
      "rationale": "reservation expires in 298 seconds; --wait will block then claim"
    }
  }
}
```

Exit 4 (transient-failure, retry-safe) signals the agent to retry — not exit 5 (conflict), which would suggest the situation can't resolve itself. A held reservation will expire; the agent should wait, not give up.

### Why advisory, not strict locks

Strict locks require lease renewal, lock recovery on agent crash, and distributed-systems machinery most CLIs shouldn't carry. Advisory reservations with TTL collapse those concerns: if the holding agent dies, the reservation expires naturally in `ttl` seconds. The cost: an agent that ignores the reservation can still proceed. That's acceptable for cooperative agents; if the threat model includes uncooperative agents, advisory reservations are the wrong tool.

### TTL discipline

- Short TTL (30s–5min) for fast operations.
- Long TTL (1hr+) only when the operation actually takes that long.
- Automatic renewal during `--wait` blocking; tool extends the reservation while it works.
- Surface remaining TTL in any verb that operates on the resource.

## Every mutation returns the identifier

The success envelope's `data` always returns the identifier the agent will need to refer to the resource next:

```json
{"ok": true, "data": {"id": "X-001", "version": 3, "etag": "abc"}}
```

The agent's next call uses `id` (or `etag` for optimistic concurrency). If the tool doesn't return the identifier, the agent has to re-list to find it — wasted round-trip.

This applies to creates (return the new ID), updates (return the new version/etag), and deletes (return the freed identifier so the agent can confirm).

## Anti-patterns

- **`--force` makes destructive ops succeed without `--yes`.** Conflating "override safety check" with "skip confirmation" gives one flag too much power. Keep them split.
- **No `--dry-run` on a destructive verb.** Agent has no way to inspect before acting.
- **Implicit retry on idempotency-token mismatch.** Same token + different args = error, not silent dedupe.
- **Strict locks without expiration.** Crashed agent leaves resource locked forever.
- **Reservation-busy returns exit 5.** Exit 5 means "conflict that won't resolve"; reservations resolve naturally. Use exit 4 (transient-failure).
- **Mutation returns no identifier.** Agent re-lists. Wasted call.
- **Soft-delete with no `restore` verb.** "Reversible" needs the reverse verb to exist.
- **Hardcoded `--yes` skips all checks.** `--yes` skips confirmation only; safety checks (clobber, conflict) still gate behind `--force`.
- **Destructive op success message says "deleted"; doesn't say `irreversible: true`.** Agent thinks it can undo.
- **`feedback clear` doesn't require `--yes`.** It deletes; it's destructive; gate it.

## Cross-references

- `errors.md` — `IDEMPOTENCY_CONFLICT`, `LOCKED`, `CONFLICT` error codes; exit code dictionary.
- `envelope.md` — `recommended_action` shape for lock-busy responses.
- `async.md` — `--wait` blocking; reservation auto-renewal during long ops.
- `mega-commands.md` — DIAGNOSE shape returns `recommended_action` for stuck mutations.
- `conventions.md` — `--yes` / `--force` / `--dry-run` flag conventions.
