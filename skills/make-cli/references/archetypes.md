# archetypes.md — Calibrating to Tool Shape

The thirteen principles aren't equally load-bearing for every CLI. A stateless converter doesn't need a job ledger; a stateful control plane lives or dies by its `status` mega-command. Pick the archetype that fits the tool being built; tune emphasis accordingly.

This file sketches five archetypes common to the agent-tool ecosystem and shows which principles weigh heaviest for each. It is intentionally not a full taxonomy — five sketches cover most of what an `inferctl`-class tool needs to reason about.

## Why archetypes matter

The principles apply universally; their *priority* doesn't. A `doctor` mega-command is mandatory for a daemon manager and overkill for a converter. Pagination matters for list-heavy tools and is irrelevant for tools that return one thing. Picking the wrong archetype's defaults wastes design effort on surfaces the agent will never use, and underweights surfaces it will need every call.

Identify the archetype first. Apply principles with its priority weighting second.

## Sketch 1: Control plane (`*ctl`)

Examples: `kubectl`, `systemctl`, `inferctl`.

**Shape.** Read-heavy with a mutate edge. Tool sits between an agent and one or more managed services; agent's most common verb is read-state-of-the-system, second most common is request-state-change, rarely commits anything directly.

**Highest-priority principles.**

- **Mega-commands (DIAGNOSE).** Status with `recommended_action` is the canonical task. Without it, every interaction is multi-turn.
- **Graceful degradation.** Managed services can be partially up; the tool reports what's reachable and what isn't.
- **Idempotent mutations.** Retried `apply` / `restart` / `scale` is the norm; must converge.
- **Async observability.** State transitions take time; `--wait` and job ledger are first-class.

**Lower-priority.**

- **Composable I/O.** Less common; control planes are usually invoked directly, not piped.
- **`--fields` sparse selection.** Useful but not load-bearing; control-plane payloads are usually small.

**Distinctive risk.** Conflating "I asked for X" with "X is now true." A control plane returns the request's acceptance, not the system's convergence. Surface intent vs observed state separately.

## Sketch 2: Stateful daemon manager

Examples: `docker`, `ollama`, `pm2`.

**Shape.** Wraps a long-lived background process or processes. Lifecycle verbs dominate: `start`, `stop`, `restart`, `pull`, `serve`. Each verb may take seconds-to-minutes.

**Highest-priority principles.**

- **Async with `--wait`.** Default verb returns a handle; `--wait` blocks until terminal. Job ledger essential.
- **Idempotent lifecycle.** `start` an already-running daemon: no-op success with `existing: true`, not an error.
- **DIAGNOSE mega-command.** Daemon health is the daily question.
- **Determinism in `list`.** Running processes listed alphabetically or by ID, not by start order.

**Lower-priority.**

- **Heavy config introspection.** Config tends to be per-daemon, not per-tool.
- **PLAN mega-command.** Lifecycle ops rarely have explicit dependency graphs.

**Distinctive risk.** Reporting the daemon as "started" when only the launch command succeeded. Health-check the daemon before claiming success; surface health-check state in the response.

## Sketch 3: Triage / recommender

Examples: `gh` (issue listing with priority), `dependabot` (PR ranking), task-tracker CLIs.

**Shape.** Pure read with ranking. Agent invokes once, picks from a recommended list, acts on the chosen item via separate verbs.

**Highest-priority principles.**

- **Mega-commands (TRIAGE).** The whole tool is essentially one canonical task: "what's next?" The mega-command is the tool.
- **Paste-ready `commands`.** The recommendation without the follow-up command is half-built.
- **`data_hash` for change detection.** Agent skips re-invocation when nothing changed.
- **Bounded output.** Recommendations beyond top-N are noise.

**Lower-priority.**

- **Mutating safety.** Read-only tools don't need most of `safety.md`.
- **Async surfaces.** Recommendations should be cheap; if they aren't, two-phase (see `mega-commands.md` § two-phase) makes them feel cheap.

**Distinctive risk.** Returning a recommendation without explaining *why*. Agents (and humans) can't act on opaque rankings. `reason_components` matters.

## Sketch 4: Stateless converter / filter

Examples: `jq`, `pandoc`, `imagemagick convert`.

**Shape.** Input in, transformed output out. No state across invocations. No ranking, no recommendation, often no `--json` (the output *is* the data).

**Highest-priority principles.**

- **Composable I/O.** stdin/stdout pipelining is the entire UX.
- **Determinism.** Same input bytes → same output bytes. Non-negotiable.
- **Errors teach.** Bad input messages must say what was bad and where in the input.
- **`SOURCE_DATE_EPOCH`** if output includes timestamps (e.g. metadata in transcoded media).

**Lower-priority.**

- **Mega-commands.** No canonical multi-step task to collapse.
- **Async / job ledger.** Synchronous by nature.
- **DIAGNOSE.** Stateless; nothing to diagnose.
- **Config surface.** Many converters live entirely on flags.

**Distinctive risk.** Treating the tool as if it needs the full agent-ergonomics package. Most of this skill's surfaces don't apply. Keep it small; nail the I/O contract and the error messages.

## Sketch 5: Build / task orchestrator

Examples: `bazel`, `nx`, `cargo`, `make` (with imagination).

**Shape.** Work-with-dependencies. Agent describes goals; tool computes the execution plan, runs it, reports per-task status.

**Highest-priority principles.**

- **Mega-commands (PLAN).** The canonical task is "show me what would run, in what order, in parallel where possible."
- **Async + checkpointing.** Long-running; `--wait` and `--resume` essential.
- **Determinism.** Build outputs must be reproducible. Affects every layer.
- **Detailed exit codes.** "Build failed" vs "test failed" vs "lint failed" vs "transient infra" — agents branch differently on each.

**Lower-priority.**

- **`feedback`.** Useful but not load-bearing.
- **TRIAGE.** Not the canonical shape for build tools.

**Distinctive risk.** Returning success when the build *plan* succeeded but the build *itself* hasn't finished. Distinguish "submitted" from "complete" rigorously; this is the async observability surface earning its keep.

## How to use this file

When designing a new verb or a new mega-command, ask:

1. Which archetype is this tool? (Pick the closest single one; tools can be hybrid but tuning to one anchor is cleaner.)
2. Which principles weigh heaviest for that archetype?
3. Which principles can be lighter-touch?
4. What's the distinctive risk?

Then design with that weighting. The full thirteen still apply; their priority shifts.

## Anti-patterns

- **One-size-fits-all.** Applying every principle at full weight produces an overspecified surface most agents won't use. Calibrate.
- **Picking the wrong archetype.** A converter dressed up as a control plane carries job-ledger and DIAGNOSE machinery it doesn't need. A control plane treated as a converter has no `recommended_action` and forces agents into multi-turn debugging.
- **Hybrid tool without an anchor archetype.** Some tools span two archetypes (e.g. a control plane that also converts). Pick a primary; tune to it; document the secondary as a sub-surface.
- **Applying another archetype's *distinctive risk* check to your own.** A converter doesn't need to distinguish "submitted" from "complete"; a control plane very much does.

## Cross-references

- `mega-commands.md` — the four canonical shapes; which archetype calls for which.
- `async.md` — applies most to control planes, daemon managers, build orchestrators.
- `safety.md` — applies most to control planes and stateful daemon managers.
- `composability.md` — applies most to converters and filters.
- `determinism.md` — applies universally but most visibly to converters and build orchestrators.
