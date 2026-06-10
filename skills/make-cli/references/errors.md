# errors.md — Errors That Teach, Errors That Infer

Errors are the highest-signal context an agent gets, because they fire exactly when the agent doesn't know what to do next. Every error should leave the agent strictly more capable than before it fired.

This file covers error message structure, did-you-mean intent inference, the JSON-envelope error shape, the exit code dictionary, and cross-agent exit code contracts.

## Error message structure

Every error has three required parts and one conditional fourth:

1. **What failed.** Short, specific. "invalid value for --visibility" not "command failed".
2. **Where (when applicable).** File:line for source-defined; argument position for runtime.
3. **The corrected command — copy-pasteable.** Not a hint. The actual command.
4. **A safe alternative** when the failed operation is destructive.

```
# Bad
error: invalid visibility

# Better
error: --visibility must be one of: public, private, unlisted (got: "secret")

# Best
error: --visibility must be one of: public, private, unlisted (got: "secret")
  try:   <tool> post create --visibility=public --content="hi"
  see:   <tool> capabilities --json | jq '.commands.post.subcommands.create.flags."--visibility"'
```

For destructive operations, also name the safe alternative:

```
error: --hard reset would discard 3 uncommitted files
  safer: <tool> reset --soft HEAD~1        # keeps working tree
  safer: <tool> stash; <tool> reset        # preserves changes
  force: <tool> reset --hard HEAD~1 --yes  # if you really meant it
```

The safe alternative is not `--dry-run`. It is a *different command* the agent should consider first.

## Did-you-mean: recovering from legible-but-wrong invocations

Common typos, deprecated spellings, and mis-orderings either succeed-with-warning or produce a "did you mean" hint with the exact corrected command.

```
$ <tool> post create --jsno
error: unknown flag '--jsno'
  did you mean: --json
  corrected:    <tool> post create --json
```

Implementation: Levenshtein distance ≤ 1 against the published flag list. Keep the suggestion list in sync with source — extract from the same definition the parser uses, not a hand-maintained file.

For verb typos:

```
$ <tool> psot list
error: unknown command 'psot'
  did you mean: post
  corrected:    <tool> post list
```

For deprecated spellings — succeed with warning, don't fail:

```
$ <tool> --skip-confirmations
warning: --skip-confirmations is deprecated; use --force
  (proceeding; will be removed in v0.6.0)
```

For enum values, "did you mean" surfaces the closest valid option:

```
error: --visibility must be one of: public, private, unlisted (got: "publik")
  did you mean: public
  corrected:    <tool> post create --visibility=public
```

An agent that mistypes once and gets a surgical correction learns the spelling permanently. An agent that gets `error: unknown flag` learns nothing.

## Errors in JSON output

When invoked with `--json`, errors emit structured into the envelope (per `envelope.md`):

```json
{
  "ok":           false,
  "tool_version": "0.4.1",
  "data":         null,
  "meta":         {"request_id": "req_abc", "elapsed_ms": 12, "contract_version": "1"},
  "errors": [
    {
      "code":         "INVALID_INPUT",
      "message":      "field 'status' must be one of: open, closed, blocked",
      "path":         "$.data.items[0].status",
      "remediation":  "use one of: open | closed | blocked",
      "did_you_mean": "open",
      "exit_code":    1
    }
  ]
}
```

Mirror `errors[0].message` to **stderr**. Agents reading stderr see the error; agents parsing JSON also see it. Both audiences served.

## Error code taxonomy

Machine-readable identifiers in `error.code`:

| Code | When |
|---|---|
| `INVALID_INPUT` | Argument or flag value invalid |
| `UNKNOWN_FLAG` | Flag not recognized; `did_you_mean` should be populated |
| `UNKNOWN_COMMAND` | Verb not recognized; `did_you_mean` should be populated |
| `MISSING_REQUIRED` | Required argument or flag not provided |
| `NOT_FOUND` | Referenced resource does not exist |
| `CONFLICT` | Operation would conflict with existing state |
| `LOCKED` | Resource held by another process; advisory reservation active |
| `DEGRADED_TIER` | Requested mode unavailable; see `envelope.md` § Degradation |
| `UPSTREAM_FAILURE` | A backing service returned an error |
| `TIMEOUT` | Operation exceeded budget |
| `INTERNAL` | Bug in the tool; report via `<tool> feedback` |

Surface the full list in `capabilities --json` under `error_codes`. Agents read it once and branch deterministically.

## Exit code dictionary

Every exit code has a published meaning and a `retryable` boolean. The dictionary is surfaced in `capabilities --json` and referenced from `--help`.

Conventional set:

| Code | Meaning | Retryable |
|---|---|---|
| 0 | success | n/a |
| 1 | user-input-error | false |
| 2 | safety-block (refused on policy grounds) | false |
| 3 | tool-environment-error (missing dep, broken config) | sometimes |
| 4 | transient-failure (network, lock-busy, rate-limit) | true |
| 5 | conflict (would clobber, version mismatch) | false |
| 6+ | tool-specific; document each |

Never overload exit 1 to mean "ran fine, no results" — that's exit 0 with `data: []`. The empty-result-as-error pattern is a recurring source of agent fragility.

In `capabilities --json`:

```json
"exit_codes": {
  "0": {"meaning": "success",                "retryable": false},
  "1": {"meaning": "user-input-error",       "retryable": false},
  "2": {"meaning": "safety-block",           "retryable": false},
  "3": {"meaning": "tool-environment-error", "retryable": null},
  "4": {"meaning": "transient-failure",      "retryable": true},
  "5": {"meaning": "conflict",               "retryable": false}
}
```

Agents branch:

```bash
case $? in
  0) ;;
  1) echo "fix input" ;;
  2) echo "refused; check policy" ;;
  3) <tool> doctor --json ;;
  4) sleep 5; retry ;;
  5) echo "resolve conflict" ;;
esac
```

This works only when the dictionary is stable across versions (see `schema-evolution.md`).

## Cross-agent exit code contracts

Some agent frameworks have their own exit code expectations:

- **Codex CLI:** exit 2 on stderr signals denial. If the tool is intended for Codex use, ensure exit 2 is reserved for safety-block, not overloaded.
- **Claude Code:** denials surface via stdout JSON with `ok: false` + `errors[0].code`. Exit code is informational rather than dispositive.
- **Shell-script consumers:** treat any non-zero as failure; expect descriptive stderr.

The conventional 0–5 dictionary above is compatible with all of them. If targeting Codex specifically, document the exit 2 = safety-block contract prominently.

## Anti-patterns

- **"See --help" alone.** Failure. The error should name the corrected command, not delegate to help.
- **Stack traces on stderr.** Agents can't act on `RuntimeError at line 942 in foo.py`. Catch and convert to structured error.
- **Silent fail.** Command exits 0 with empty stdout but the operation didn't happen. Worst case for agents — they can't detect the failure to retry.
- **Catch-all exit 1.** Different failure classes (input vs environment vs upstream) collapsed into one exit code. Agents can't branch.
- **Free-form prose error messages.** "Something went wrong, please check your input and try again" — agents can't extract `path`, `code`, or `remediation`.
- **No did-you-mean for known-typo'd flags.** The flag list is in source; extracting it for Levenshtein matching is mechanical.
- **Pointing at `--help` when `capabilities` is richer.** Prefer `see: <tool> capabilities --json | jq '...'`.

## Cross-references

- `envelope.md` — error object shape inside the universal envelope.
- `conventions.md` — flag/verb naming consistency that makes did-you-mean tractable.
- `safety.md` — naming safe alternatives for destructive operations.
- `schema-evolution.md` — keeping the error code taxonomy stable across versions.
