---
name: make-cli
description: >-
  Design a CLI whose primary user is an AI agent. Use when designing a new CLI
  from scratch, restructuring an existing one for agent ergonomics, or reviewing
  an agent-facing surface. Covers the universal JSON envelope, mega-commands
  that collapse round-trips, three-layer introspection (--help, capabilities
  --json, robot-docs guide), error pedagogy with did-you-mean, exit code
  dictionaries, schema versioning with contract_version, idempotent mutations,
  async observability, config-as-code, and naming conventions. Do NOT use for
  general CLI questions, individual bug fixes, or human-only CLI design.
---

# make-cli — Designing CLIs for Agents

**Purpose.** Guide an agent or human designing a CLI whose primary user is another agent. Humans benefit; agents are not an afterthought.

**Scope.** This file is the spine. The thirteen principles below are summary-density; each points to a `references/` file for templates, schemas, and worked examples. Read this file once per session. Load references on demand.

---

## Stance

Design for agents first. Every principle here makes the CLI better for humans too. The historical default — humans at a terminal as the primary user, agents tolerated — produces CLIs that are inconsistent, prompt-prone, and stdout-only. Designing for agents first inverts the failure modes.

## The over-arching rationale: first-try inevitability

The first command an agent guesses should work — or fail with a precise, copy-pasteable correction. Naming inevitability (verb and flag names match agent priors), behavior inevitability (defaults match expectation), output inevitability (envelope shape predictable). Every principle below feeds this one. When two principles conflict, the one that improves first-try success wins.

## Design order: machine-first

Build the machine-readable surface first; render the human form on top.

1. Sketch `capabilities --json`: verbs, flags, exit codes, env vars, contract_version.
2. Define per-verb output schemas using the universal envelope.
3. Define the error-code taxonomy.
4. Implement verbs to produce JSON.
5. Render JSON → human-readable when output isn't a terminal.
6. Pin every schema with a regression test.

Tools designed human-first and retrofitted for agents always have parity gaps. Tools designed machine-first don't.

---

## Layer 1 — Surface

What the agent encounters on first contact.

### 1. Non-interactive, conventional, predictable

Bare invocation does something useful or directs to `--help`; never a TUI surprise. Verb and flag names match dominant agent priors: `get` not `info`, `list` not `ls`, `--force` not `--skip-confirmations`, `--json` not `--format=json`, `--yes` not `--no-input`. Honor `NO_COLOR`, `CI`, `TERM=dumb`, and non-TTY detection; suppress ANSI when piped. Enforce naming mechanically in CI, not through review.

See: `references/conventions.md`.

### 2. Structured envelope on stdout, diagnostics on stderr

Every `--json` response uses the universal envelope: `{ok, tool_version, data, meta, warnings, commands, errors}`. The `meta` object carries `request_id`, `ts_iso`, `data_hash`, `contract_version`, `elapsed_ms`, plus mode-provenance when applicable. Errors emit structured into the envelope AND mirror to stderr. Never silent-fail: failure produces stderr output and a non-zero exit, always.

See: `references/envelope.md`.

### 3. Errors teach and infer

Every error names what failed, the valid set when the cause is an enum, and the exact corrected command — copy-pasteable. For destructive operations, the error names the safe alternative (`use git revert` rather than `use --dry-run`). Recover from legible-but-wrong invocations (typos, deprecated spellings) with a "did you mean" hint; an agent that mistypes once and gets a surgical correction learns the spelling permanently.

See: `references/errors.md`.

### 4. Three-layer introspection

Layer 1: `--help` — terse human text; first 30 lines convey the core. Layer 2: `capabilities --json` — versioned machine contract describing verbs, flags, exit codes, env vars, output schemas. Layer 3: `robot-docs guide` — long-form workflow handbook teaching the composition of operations into useful tasks. No undocumented behavior: if it isn't in capabilities, it shouldn't exist.

See: `references/introspection.md`.

### 5. Composable I/O

Read structured input via `--from-stdin` (or `-` argument convention) so agents pipeline without shell loops. Route artifacts via `--deliver=stdout|file:<path>|webhook:<url>`; file sinks are atomic, webhook sinks return HTTP status, unknown schemes return a structured refusal. Accept friction reports via `feedback <text>` written locally by default, optionally POSTed upstream when configured.

See: `references/composability.md`.

---

## Layer 2 — Contract

The machine-readable agreement.

### 6. Mega-commands collapse round-trips

For canonical multi-step tasks, ship one invocation that returns data + `recommendations` + `commands` (paste-ready follow-ups). Four canonical shapes: TRIAGE (ranked recommendations), DIAGNOSE (system state + `recommended_action`), PLAN (parallelizable execution proposal), CAPABILITIES (the contract itself; ship this always). Mega-commands include `data_hash` for change detection and may use two-phase latency — cheap slices synchronous, expensive ones async with per-metric `status`.

See: `references/mega-commands.md`.

### 7. Deterministic, bounded, pinnable output

Same input produces the same output bytes. Sort iteration before serialization; no raw timestamps in stdout (push to `meta.ts_iso`); honor `SOURCE_DATE_EPOCH`; content-addressed IDs where possible. Bounded defaults on list-style commands, with pagination cursor and `--fields` for sparse selection. Every output schema is pinned by a golden-file regression test that fails the build on drift.

See: `references/determinism.md`.

### 8. Documented exit-code dictionary

Each exit code has a published meaning and a `retryable` boolean. The dictionary is surfaced in `capabilities --json` and referenced from `--help`. Conventional set: 0 success; 1 user-input-error; 2 safety-block; 3 tool-environment-error; 4 transient-failure (retry-safe); 5 conflict. Never overload exit 1 to mean "ran fine, no results" — that's exit 0 with `data: []`. An agent should branch deterministically on `$?`.

See: `references/errors.md` § exit codes.

### 9. Graceful degradation with provenance

When a capability tier is unavailable (cache cold, network down, index rebuilding), emit best-effort and tell the agent which mode actually ran via `meta.search_mode` / `meta.fallback_tier` / `meta.fallback_reason`. Doctor and status outputs include `recommended_action: {command, rationale, alternatives}` so the tool itself names the next step. Never pretend nothing changed; agents may build downstream logic on the result.

See: `references/envelope.md` § degradation.

---

## Layer 3 — Lifecycle

State, identity, evolution.

### 10. Mutations are idempotent, gated, reversible

Creates use idempotency tokens or natural keys: a retried create returns the existing resource, not a duplicate. Destructive operations require explicit `--yes` or `--force` AND name a safe alternative in their error or dry-run output. Multi-agent coordination uses advisory reservations with TTL rather than locks; lock-busy returns exit 4 (transient-failure) with a `recommended_action: "wait Ns and retry"`. Every mutation response returns the identifier the agent needs for the next call.

See: `references/safety.md`.

### 11. Async work is observable and resumable

Every command that wraps an async API supports `--wait`, blocking until completion via internal polling with backoff and jitter. State persists to a durable job ledger so a `--wait` invocation killed mid-poll finds the existing job rather than submitting a new one. Expose `jobs list/get/prune`. Long operations checkpoint state and support `--resume`.

See: `references/async.md`.

### 12. Config and identity are introspectable

`config schema --json` returns the JSON Schema; `config validate` produces `did_you_mean` for typo'd keys; `config show --json` returns the effective config with `_provenance` per key (default | file | env | profile | flag). `config get/set/patch` lets agents update without parsing TOML; comments preserve on write. Profiles bundle reusable identity, discoverable via `capabilities`. Precedence is documented and fixed: flag > env > profile > config > default.

See: `references/config.md`.

### 13. The contract has a version

`contract_version` lives in `capabilities` and every response's `meta`. Additive changes ship freely. Renames go through a deprecation cycle (≥1 release with both names; old emits warning). Breaking changes bump the major version AND ship `--contract-version=N` compat for ≥6 months so agents migrate gradually. Capabilities is pinned to a golden file in regression tests; intentional bumps re-pin and CHANGELOG documents the migration.

See: `references/schema-evolution.md`.

---

## When designing X, read Y

| Designing... | Read first |
|---|---|
| Output shape for any verb | `envelope.md`, `determinism.md` |
| A `doctor` / `status` / `triage` / `plan` verb | `mega-commands.md` |
| Error messages | `errors.md` |
| `--help`, `capabilities`, `robot-docs guide` | `introspection.md` |
| Async or long-running operations | `async.md` |
| Config file format and surface | `config.md` |
| Versioning the contract | `schema-evolution.md` |
| Destructive operations, multi-agent coordination | `safety.md` |
| Pipelines, `--deliver`, `feedback` | `composability.md` |
| Naming verbs / flags / env vars, exit code conventions | `conventions.md` |
| Calibrating principles to tool shape | `archetypes.md` |

---

## Checklist

### Pre-flight (before writing a verb)

- [ ] Tool's archetype identified (`references/archetypes.md`).
- [ ] `capabilities --json` schema sketched: verbs, flags, exit codes, env vars, contract_version.
- [ ] Canonical agent task named: the one thing an agent will most often do with this tool.
- [ ] Mega-command candidate chosen if applicable (TRIAGE / DIAGNOSE / PLAN; CAPABILITIES always).

### Per-verb

- [ ] Output is the universal envelope with populated `meta`.
- [ ] `--json` works; stdout is data, stderr is diagnostics; no ANSI when piped.
- [ ] Errors name what failed, the valid set, the corrected command, and a safe alternative on destructive paths.
- [ ] Exit code is from the published dictionary.
- [ ] Bounded default with `--limit` / cursor; truncation message teaches narrowing.
- [ ] If mutating: idempotent on retry, gated by `--yes` / `--force`, returns the identifier.
- [ ] If async-backed: `--wait` supported, job written to ledger.
- [ ] If degradation possible: `meta` carries requested vs actual mode.
- [ ] `--from-stdin` supported where pipelining makes sense.
- [ ] Output schema documented in `capabilities` and pinned by a regression test.

### Before ship

- [ ] `capabilities --json` complete and pinned to golden file.
- [ ] `robot-docs guide` paste-ready handbook present.
- [ ] `--help` first 30 lines convey core; AGENT/AUTOMATION footer present.
- [ ] Determinism verified (`<tool> X; <tool> X; diff`).
- [ ] No silent-fail paths; every failure produces stderr + non-zero exit.
- [ ] `contract_version` set; CHANGELOG documents schema state.
- [ ] Cross-archetype concerns addressed (e.g. Codex stderr+exit-2 contract if multi-agent target).
