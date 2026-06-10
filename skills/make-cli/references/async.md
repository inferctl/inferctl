# async.md — Observable, Resumable, Job Ledger

Most CLIs wrap APIs that complete in milliseconds. The interesting ones wrap APIs that complete in seconds, minutes, or hours: model downloads, indexes rebuilding, builds compiling, jobs running on a queue.

When an operation might exceed an agent's tolerance for blocking, three surfaces matter: `--wait` for synchronous behavior, a job ledger that survives mid-call interruption, and `jobs` verbs for the agent to inspect what's in flight.

This file covers all three plus checkpointing for very-long operations.

## `--wait`: synchronous behavior on async APIs

Default for an async-backed verb: submit, return a job ID, exit. Adding `--wait` makes the verb block until completion.

```bash
# Async default
$ <tool> pull llama-3-8b --json
{
  "ok": true,
  "data": {"job_id": "job_abc", "status": "submitted", "estimated_seconds": 240},
  "meta": {...}
}

# Synchronous via --wait
$ <tool> pull llama-3-8b --json --wait
# (blocks for ~240s, polling internally with backoff and jitter)
{
  "ok": true,
  "data": {"job_id": "job_abc", "status": "complete", "model": {...}},
  "meta": {"elapsed_ms": 247103, ...}
}
```

Implementation: internal polling with exponential backoff and jitter. Don't tight-loop; don't poll at fixed intervals (synchronizes against the server, creates thundering-herd patterns if many agents wait simultaneously).

Reference backoff:

```python
def poll_until_done(job_id, max_wait_s=3600):
    delay = 0.5
    deadline = time.time() + max_wait_s
    while time.time() < deadline:
        status = get_job(job_id)
        if status.terminal:
            return status
        time.sleep(delay + random.uniform(0, delay * 0.2))   # jitter
        delay = min(delay * 1.6, 30)                          # cap at 30s
    raise TimeoutError(...)
```

### `--wait` exit codes

`--wait` blocks; when the underlying job fails, the wrapping invocation exits with a code that matches the failure type:

| Job outcome | Exit code |
|---|---|
| Job completed successfully | 0 |
| Job failed (user-causable) | 1 |
| Job failed (environment-causable) | 3 |
| Job failed transiently (retry-safe) | 4 |
| Job timed out exceeding `--wait-timeout` | 4 |
| Reservation/lock held the whole time | 4 |

The agent branches on `$?` exactly as it would for any other failure.

### `--wait-timeout`

Default `--wait` should block indefinitely (or for a generous default like 1 hour). Agents that need a specific budget pass `--wait-timeout`:

```bash
$ <tool> pull llama-3-8b --wait --wait-timeout=60s --json
```

If the timeout fires:

```json
{
  "ok": false,
  "data": {"job_id": "job_abc", "status": "in_progress"},
  "errors": [{
    "code":        "WAIT_TIMEOUT",
    "message":     "job not complete after 60s; still running",
    "remediation": "<tool> jobs get job_abc --wait",
    "exit_code":   4
  }]
}
```

The job continues running; `--wait` just gave up. The agent gets `job_id` to resume polling later.

## The job ledger

Async jobs persist to a durable local ledger. Reason: an agent that runs `<tool> pull X --wait`, gets SIGKILLed mid-wait, and retries should *find the existing job*, not submit a fresh one.

Ledger location: `$XDG_DATA_HOME/<tool>/jobs.jsonl` or similar. Append-only is ideal (one line per state transition); pruned by a `jobs prune` verb.

Ledger entry:

```json
{
  "job_id":           "job_abc",
  "verb":             "pull",
  "argv":             ["llama-3-8b"],
  "idempotency_key":  "agent-run-42",
  "submitted_iso":    "2026-05-08T14:00:00Z",
  "submitted_by":     "agent-run-42",
  "status":           "in_progress",
  "last_seen_iso":    "2026-05-08T14:02:11Z",
  "estimated_seconds": 240
}
```

### Mid-wait kill resumption

```bash
$ <tool> pull llama-3-8b --wait --idempotency-key="agent-run-42"
# (killed mid-wait)

$ <tool> pull llama-3-8b --wait --idempotency-key="agent-run-42"
# Finds existing job_abc in ledger via idempotency key; resumes polling.
```

The idempotency key is the join: same key + same args → same job. (See `safety.md` § Idempotency.)

Without a key, the tool can still match on `(verb, args, recent submission)` heuristically, but key-based match is the correct path. Document the heuristic if you ship one.

## `jobs` verbs

Once jobs exist as first-class objects, the agent needs to inspect, get, and clean them:

```bash
$ <tool> jobs list --json
$ <tool> jobs list --status=in_progress --json
$ <tool> jobs list --since=24h --json
$ <tool> jobs get job_abc --json
$ <tool> jobs get job_abc --wait --json     # block until terminal
$ <tool> jobs cancel job_abc --json
$ <tool> jobs prune --before=7d --yes --json
```

The shape:

```json
# <tool> jobs list --json
{
  "ok": true,
  "data": {
    "jobs": [
      {"job_id": "job_abc", "verb": "pull", "status": "in_progress", "submitted_iso": "...", "elapsed_seconds": 131},
      {"job_id": "job_def", "verb": "serve","status": "running",     "submitted_iso": "...", "elapsed_seconds": 4823}
    ]
  },
  "meta": {"pagination": {"cursor": "job_def", "has_more": false}}
}
```

Job states (terminal vs non-terminal):

| State | Terminal | Meaning |
|---|---|---|
| `submitted` | no | accepted by tool; not yet started |
| `in_progress` | no | executing |
| `running` | no | long-lived (e.g. a serving model) |
| `complete` | yes | finished successfully |
| `failed` | yes | finished with error |
| `canceled` | yes | user-requested stop |
| `timed_out` | yes | exceeded internal budget |

Document the full set in `capabilities.job_states`. Agents branch deterministically.

### Cancel semantics

`<tool> jobs cancel <id>` is destructive (interrupts work). Gate behind `--yes`:

```bash
$ <tool> jobs cancel job_abc
error: cancel is destructive; pass --yes to confirm
  safer: <tool> jobs get job_abc --wait    # let it finish
  force: <tool> jobs cancel job_abc --yes
```

If the underlying job can't be canceled cleanly, return:

```json
{
  "ok": false,
  "errors": [{
    "code":    "CANCEL_NOT_SUPPORTED",
    "message": "job 'job_abc' (verb=pull) cannot be canceled mid-flight; will run to completion",
    "exit_code": 1
  }]
}
```

## Checkpointing and `--resume`

For *very* long operations (multi-hour builds, large model fine-tunes), submit-and-poll isn't enough. The operation itself should checkpoint state to disk and resume from the last checkpoint when re-invoked.

```bash
$ <tool> train --config=config.toml --json
{
  "ok": true,
  "data": {
    "job_id":      "job_train_abc",
    "checkpoint":  "/var/lib/<tool>/checkpoints/job_train_abc/step_2400.ckpt",
    "step":        2400,
    "status":      "complete"
  }
}

# Interrupted at step 1800; resume:
$ <tool> train --config=config.toml --resume --json
{
  "ok": true,
  "data": {
    "job_id":           "job_train_def",
    "resumed_from":     "/var/lib/<tool>/checkpoints/job_train_abc/step_1800.ckpt",
    "starting_step":    1800,
    "status":           "in_progress"
  }
}
```

Discipline:

- **Checkpoints are content-addressed or step-numbered.** Agent can identify which one to resume from.
- **`--resume` finds the latest checkpoint automatically.** Don't make the agent name the file.
- **Resume is opt-in.** Default behavior is fresh start; `--resume` opts into continuation.
- **Checkpoint metadata in the envelope.** Agent sees what step it's on and when the next checkpoint will write.

## Observability surface

Async operations live longer than the calling agent. Three observability surfaces matter:

1. **`<tool> jobs get <id> --wait`** — block until completion from any future invocation.
2. **`<tool> jobs get <id> --logs`** — stream log lines from the job's stderr stream.
3. **`<tool> jobs get <id> --metrics --json`** — current progress metrics (% complete, throughput, ETA).

```bash
$ <tool> jobs get job_abc --metrics --json
{
  "ok": true,
  "data": {
    "job_id":           "job_abc",
    "progress_percent": 67,
    "throughput":       {"unit": "tokens_per_second", "value": 142},
    "eta_seconds":      78,
    "stage":            "downloading_layer_5_of_8"
  },
  "meta": {...}
}
```

The agent can poll `--metrics` between work units to decide whether to wait or move on.

## Anti-patterns

- **No job ID returned.** Async default with no handle — agent can't find what it submitted.
- **`--wait` does tight-loop polling.** Wastes server resources; collides with other agents waiting simultaneously. Backoff + jitter required.
- **No ledger.** Retried `--wait` resubmits a fresh job. Two of the same operation running.
- **Ledger lives in-memory only.** Tool restart loses all job tracking.
- **Cancel without `--yes`.** Accidental cancellation of in-flight work.
- **`jobs list` paginates the wrong way.** Agent has no cursor or limit; can't browse safely. See `determinism.md` § pagination.
- **No `jobs prune`.** Ledger grows unboundedly.
- **Logs only viewable via tail-on-disk.** Agent has no programmatic path; expose via `jobs get --logs`.
- **`--resume` requires the agent to specify the checkpoint path.** Tool should find it. Manual paths are agent surface area that doesn't need to exist.
- **Async verb has no synchronous escape hatch.** Some agents *want* to block. `--wait` always exists.
- **Synchronous verb with no async escape hatch.** Long-running operation with no way to fire-and-forget. Agent times out and can't recover.

## Cross-references

- `safety.md` — idempotency keys are the join between retried `--wait` calls and existing jobs.
- `errors.md` — `WAIT_TIMEOUT`, `CANCEL_NOT_SUPPORTED` codes; exit 4 for transient.
- `envelope.md` — `meta.elapsed_ms` for total wait time; `pagination` for `jobs list`.
- `mega-commands.md` — DIAGNOSE shape often includes `next_check_after_seconds` for async-aware retry timing.
- `config.md` — ledger location configurable via `[jobs] ledger_path`.
