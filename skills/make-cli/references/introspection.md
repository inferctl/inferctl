# introspection.md — The Three Layers of Self-Description

A CLI must teach itself to every agent that picks it up. Three layers, each answering a different question:

| Layer | Surface | Audience | Answers |
|---|---|---|---|
| 1 | `--help` | human at terminal | "what does this command do?" |
| 2 | `capabilities --json` | agent reading the machine contract | "what's the shape of everything?" |
| 3 | `robot-docs guide` | agent learning workflows | "when would I compose these?" |

All three exist, all three are versioned, all three are kept in sync by the same generation step.

## Layer 1: `--help`

Terse human text. Top 30 lines convey the core. AGENT/AUTOMATION footer at the bottom for agent-specific guidance.

```
<tool>  Local LLM inference control plane.

USAGE: <tool> <command> [flags]

COMMANDS:
  list             List local models
  pull             Download a model
  serve            Start a model
  stop             Stop a running model
  status           Show runtime status
  doctor           Diagnose problems
  --robot-status   Mega-command: status + recommendations + commands

GLOBAL FLAGS:
  --json           Structured output (also --robot-*)
  --no-color       Suppress ANSI
  --version        Print version
  --help, -h       Show this help

EXIT CODES: 0 ok; 1 user-input; 2 safety; 3 environment; 4 transient; 5 conflict.
           See `<tool> capabilities --json | jq '.exit_codes'`.

AGENT/AUTOMATION:
  Machine contract:  <tool> capabilities --json
  Workflow guide:    <tool> robot-docs guide
  Mega-command:      <tool> --robot-status
  Schema export:     <tool> schema --json
```

Discipline:

- **30-line limit on the top.** Smaller models stop reading after that.
- **AGENT/AUTOMATION footer required.** Names the other introspection layers.
- **Exit code summary inline.** Even though `capabilities` has the dictionary, agents reading `--help` shouldn't have to leave to branch on `$?`.
- **No prose paragraphs.** Bulleted, dense, scannable.
- **Stable text across versions.** Reduces drift for agents that cache `--help`.

Per-subcommand help follows the same shape, scoped:

```
<tool> pull  Download a model.

USAGE: <tool> pull <model-name> [flags]

FLAGS:
  --json           Structured output
  --dry-run        Print what would be downloaded; don't download
  --force          Re-download even if cached
  --from-stdin     Read model name from stdin

EXAMPLES:
  <tool> pull llama-3-8b --json
  echo "llama-3-8b" | <tool> pull --from-stdin --json

EXIT CODES: 0 ok; 1 invalid model name; 4 network failure (retry-safe).

AGENT: schema at `<tool> schema --command=pull --json`.
```

## Layer 2: `capabilities --json`

The machine contract. Versioned JSON describing every verb, flag, exit code, env var, and output schema. Pinned to a golden file in regression tests (see `schema-evolution.md`).

Top-level shape:

```json
{
  "tool_name":        "<tool>",
  "version":          "0.4.1",
  "contract_version": "1",
  "features":         ["json_output", "did_you_mean", "deterministic_output", "from_stdin"],
  "commands":         { /* per-verb */ },
  "global_flags":     { /* see below */ },
  "exit_codes":       { /* dictionary; see errors.md */ },
  "error_codes":      { /* taxonomy; see errors.md */ },
  "env_vars":         { /* documented */ },
  "limits":           { /* documented */ },
  "config": {
    "format":             "toml",
    "schema_uri":         "<tool> config schema --json",
    "show_uri":           "<tool> config show --json",
    "search_path":        ["$XDG_CONFIG_HOME/<tool>/config.toml"],
    "supports_profiles":  true
  },
  "schemas_uri":      "<tool> schema --json",
  "robot_docs_uri":   "<tool> robot-docs guide"
}
```

### Per-verb entries

```json
"commands": {
  "pull": {
    "description":   "Download a model.",
    "mutates":       true,
    "json":          true,
    "stdin":         "model_name",
    "args": [
      {"name": "model_name", "required": true, "type": "string"}
    ],
    "flags": [
      {"name": "--dry-run",    "type": "bool", "default": false},
      {"name": "--force",      "type": "bool", "default": false},
      {"name": "--from-stdin", "type": "bool", "default": false}
    ],
    "exit_codes":    [0, 1, 4],
    "output_schema": { /* JSON Schema for --json output */ },
    "examples": [
      {"invocation": "<tool> pull llama-3-8b --json", "summary": "download a model"}
    ]
  }
}
```

Every flag mentioned in source is documented here. Every exit code site has a meaning. Undocumented behavior is a defect.

### Schema export endpoint

`output_schema` can be inline (per `commands.<verb>.output_schema`) or referenced via `schema_uri`. The full export:

```bash
$ <tool> schema --json
{
  "tool":        "<tool>",
  "version":     "0.4.1",
  "envelope":    { /* universal envelope schema */ },
  "schemas":     { /* per-verb output schemas */ },
  "definitions": { /* shared types */ }
}

$ <tool> schema --command=pull --json
# returns just the pull verb's output_schema
```

Agents validate output against these schemas — cheap correctness check, also useful for downstream tooling (dashboards, type generators).

## Layer 3: `robot-docs guide`

Long-form workflow handbook. Teaches composition, not commands. Paste-ready handbook the agent reads when it doesn't know how to approach a task.

```
# <tool> — Agent Workflow Guide

## Quick reference

Mega-call:    <tool> --robot-status
Capabilities: <tool> capabilities --json
Schemas:      <tool> schema --json

## Canonical workflows

### "What's running locally?"
$ <tool> --robot-status
# Returns running models, resource usage, recommended_action if degraded.

### "Pull and start a model"
$ <tool> pull llama-3-8b --json --wait
$ <tool> serve llama-3-8b --json

### "Inspect a stuck model"
$ <tool> doctor --component=<model_id> --json
# Returns state, last error, recommended_action.

## Idioms

- All read-side verbs accept `--json`. Stdout is the envelope; stderr is diagnostics.
- All mutating verbs accept `--dry-run`.
- Async-backed verbs accept `--wait` for synchronous behavior.
- `--from-stdin` works on `pull`, `serve`, `stop`.

## Exit codes for branching

case $? in
  0) ;;
  1) echo "fix args" ;;
  3) <tool> doctor --json ;;
  4) sleep 5; retry ;;
esac

## Where to look next

- Output schemas:  <tool> schema --json
- Config:          <tool> config show --json
- Active jobs:     <tool> jobs list --json
```

Discipline:

- **Task-shaped, not command-shaped.** "Pull and start a model", not "the `pull` command".
- **Paste-ready.** Every block is copy-pasteable.
- **Cross-links to capabilities.** Doesn't duplicate the contract; links to it.
- **Idiomatic.** Names patterns the agent can apply across verbs.
- **Short.** A page or two, not a manual.

## No telepathy required

Together, the three layers must cover every agent-relevant behavior:

- Every flag mentioned in source is in `capabilities`.
- Every exit code site has a documented meaning.
- Every undocumented behavior is a defect.

If an agent has to guess, the tool failed to teach itself.

## Anti-patterns

- **`--help` only.** No machine-readable contract means agents parse human text. Fragile.
- **`capabilities` exists but isn't versioned.** Without `contract_version`, agents can't detect breaking changes.
- **`robot-docs guide` drifts from the command surface.** Generate from the same source as `--help` and `capabilities`, or pin in CI.
- **Per-verb help that doesn't reference capabilities.** Agent reads `<tool> pull --help` and doesn't know `<tool> schema --command=pull --json` exists.
- **Aspirational `--help` text.** Behaviors described but not implemented. Worse than missing docs.
- **`--help` longer than 200 lines.** Smaller models give up.
- **`robot-docs guide` that recapitulates `--help`.** Wrong layer; it should teach composition, not list commands.

## Cross-references

- `envelope.md` — JSON shape capabilities and schema-export endpoints emit.
- `errors.md` — exit code dictionary and error code taxonomy surfaced in capabilities.
- `schema-evolution.md` — versioning capabilities; golden-file pinning.
- `mega-commands.md` — mega-command entries in capabilities.
- `config.md` — config schema endpoint referenced from capabilities.
