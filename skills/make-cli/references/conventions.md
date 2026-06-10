# conventions.md — Names, Flags, Env Vars, TTY Behavior

Conventions are the unglamorous backbone of first-try inevitability. An agent's prior says `--json` and `--force` and `list`; matching those priors means the first guess works. Inventing names — even better names — costs an agent an error round-trip and breaks pattern-completion across tools.

This file covers verb naming, flag naming, env var naming, exit code conventions, and the TTY/CI/NO_COLOR contract.

## The principle

Agents pattern-match across tools. The pattern that ships with the most CLIs wins, regardless of whether it's the "best" name in isolation. A tool that calls its JSON flag `--format=structured` because it's "more general" is wrong — wrong about the population of agents that will use it, all of whom have been trained on `--json`.

When in doubt, copy the most-used existing pattern.

## Verb conventions

Read verbs:

| Prefer | Avoid |
|---|---|
| `list` | `ls`, `enumerate`, `index` |
| `get` | `fetch`, `retrieve`, `info` |
| `show` | `display`, `view`, `print` |
| `search` | `find`, `query`, `lookup` |
| `status` | `state`, `health`, `info` (for stateful tools) |

Note: `get` returns one resource by ID; `list` returns many; `show` returns a verbose render of one. These three overlap in common usage — pick a discipline (e.g. `get` returns terse, `show` returns verbose) and document it in `--help`.

Mutate verbs:

| Prefer | Avoid |
|---|---|
| `create` | `new`, `add`, `init` (use `init` only for project bootstrap) |
| `update` | `modify`, `edit`, `patch` (reserve `patch` for partial-update semantics) |
| `delete` | `remove`, `rm`, `destroy` (use `destroy` only when truly irreversible) |
| `set` | `assign`, `configure` (single-field write) |

Lifecycle verbs:

| Prefer | Avoid |
|---|---|
| `start` / `stop` | `up` / `down`, `run` / `kill` |
| `pause` / `resume` | `freeze` / `thaw` |
| `restart` | `reload` (use `reload` for config-only) |

Operational verbs:

| Prefer | Avoid |
|---|---|
| `doctor` | `check`, `health`, `validate` (for full diagnosis) |
| `validate` | `verify`, `lint` (for config or input validation) |
| `repair` | `fix`, `recover` |
| `prune` | `cleanup`, `gc` |

When two priors compete (`fetch` vs `get`, `pull` vs `download`), pick the one shared by tools in your archetype. `git pull`, `docker pull`, `ollama pull` → `pull` is the canonical "fetch a remote artifact to local" verb. `kubectl get`, `aws ec2 describe-...` → `get` for indexed retrieval.

## Flag conventions

Output:

| Flag | Meaning | Notes |
|---|---|---|
| `--json` | Emit the universal envelope | Not `--format=json`, not `--output=json` |
| `--no-color` | Suppress ANSI | Honor `NO_COLOR` env var too |
| `--quiet`, `-q` | Suppress non-essential stderr | Errors still emit |
| `--verbose`, `-v` | More detail on stderr | Not on stdout |
| `--fields=a,b,c` | Sparse field selection | Reduces token cost |
| `--limit=N` | Cap result count | Bounded default; see `determinism.md` |

Input:

| Flag | Meaning |
|---|---|
| `--from-stdin` | Read structured input from stdin |
| `-` (positional) | Read named input from stdin |
| `--file=<path>` | Read input from file |
| `--config=<path>` | Use specific config file (overrides default search path) |

Behavior:

| Flag | Meaning |
|---|---|
| `--yes`, `-y` | Skip safety prompts; proceed with the requested action |
| `--force` | Override safety check (destructive or conflict-detected) |
| `--dry-run` | Print what would happen; do nothing |
| `--wait` | Block until async operation completes |
| `--resume` | Continue an interrupted long-running operation |

Selection:

| Flag | Meaning |
|---|---|
| `--all` | Operate on all items (use carefully with mutating verbs) |
| `--filter=expr` | Filter results |
| `--sort=<key>` | Sort key |

### Flag-form discipline

- Use `--long-form=value`, not `--long-form value` (parser-stable; ambiguity-free).
- Short forms only for the most-used flags (`-h`, `-v`, `-y`, `-q`). Don't proliferate.
- Boolean flags take no value: `--force`, not `--force=true`. Inverted form: `--no-force` (rarely needed).
- Repeatable flags accumulate: `--label=a --label=b` produces `["a", "b"]`. Document in `capabilities`.

### Flag-name parity across verbs

`--json` means the same thing on every verb. `--limit` means the same thing on every verb. If `list --limit=10` returns 10 items, `search --limit=10` returns 10 results — not 10 pages, not 10 matches per file. Parity reduces agent surprise across the surface.

When a flag is verb-specific (e.g. `pull --resume` to continue a partial download), the meaning should still match the global flag where overlap exists.

## Env var conventions

Naming: `TOOL_FOO_BAR`. All caps, underscores, tool-name prefix.

```
TOOL_CONFIG=/path/to/config.toml
TOOL_PROFILE=production
TOOL_LOG_LEVEL=debug
TOOL_NO_TELEMETRY=1
TOOL_API_TOKEN=...
```

Honor industry-standard vars without a prefix:

| Var | Behavior |
|---|---|
| `NO_COLOR` | Any non-empty value → suppress ANSI |
| `CI=true` | Suppress interactive prompts; suppress progress; force non-TTY behavior |
| `TERM=dumb` | Suppress ANSI |
| `XDG_CONFIG_HOME` | Config file lookup root (default: `~/.config`) |
| `XDG_CACHE_HOME` | Cache lookup root (default: `~/.cache`) |
| `XDG_DATA_HOME` | Data file lookup root (default: `~/.local/share`) |
| `SOURCE_DATE_EPOCH` | Force deterministic timestamps (see `determinism.md`) |
| `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY` | Network egress (lowercase variants too) |

Document every env var in `capabilities.env_vars`:

```json
"env_vars": {
  "TOOL_CONFIG":  {"description": "Path to config file",  "overrides": "--config flag"},
  "TOOL_PROFILE": {"description": "Active profile name",  "overrides": "config.toml profile"},
  "NO_COLOR":     {"description": "Suppress ANSI output", "industry_standard": true}
}
```

## TTY, CI, and color detection

The rule: emit ANSI only when the user is plausibly looking at it.

Detection sequence (suppress ANSI if any match):

1. `NO_COLOR` set to anything non-empty.
2. `CI=true` (or any of the common CI markers: `GITHUB_ACTIONS`, `GITLAB_CI`, `CIRCLECI`).
3. `TERM=dumb`.
4. stdout is not a TTY (piped, redirected, captured).
5. `--no-color` flag passed.

Override: `--color=always` forces ANSI on. Useful for `<tool> ... | less -R`.

Progress bars and spinners: stderr only, and suppressed under the same rules. A spinner emitted to a CI log produces hundreds of lines of garbage.

```python
# Canonical detection
def color_enabled(stream):
    if os.environ.get("NO_COLOR"):       return False
    if os.environ.get("CI", "").lower() == "true": return False
    if os.environ.get("TERM") == "dumb": return False
    if not stream.isatty():              return False
    return True
```

`--json` mode should never emit ANSI on stdout, regardless of any of the above. Color in JSON breaks parsers.

## Bare invocation

`<tool>` with no arguments does one of:

- Print short usage and exit 0 (most common).
- Run the canonical default verb (only when truly canonical, e.g. `git status`).

Never:

- Launch a TUI. Agents and shell scripts will hang.
- Block on stdin without a flag. Same hang.
- Open a browser, editor, or any GUI. Same hang.

If interactive mode is desirable, gate it explicitly: `<tool> interactive` or `<tool> ui`.

## Exit codes referenced

Full dictionary in `errors.md`. Convention summary:

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | user-input-error |
| 2 | safety-block |
| 3 | tool-environment-error |
| 4 | transient-failure |
| 5 | conflict |

Surface these in `capabilities.exit_codes` and in `--help`. Don't invent tool-specific exit codes inside this range; use 6+ for tool-specific.

## Mechanical enforcement

Naming conventions enforced by code review will drift. Enforce by CI:

```python
# test_naming_conventions.py
def test_no_banned_verb_names():
    caps = json.loads(run(["<tool>", "capabilities", "--json"]))
    banned = {"ls", "rm", "info", "fetch"}
    for verb in caps["commands"]:
        assert verb not in banned, f"verb '{verb}' violates conventions; rename"

def test_json_flag_present():
    caps = json.loads(run(["<tool>", "capabilities", "--json"]))
    for verb, spec in caps["commands"].items():
        if spec.get("json", False):
            flag_names = {f["name"] for f in spec.get("flags", [])}
            assert "--json" in flag_names or spec.get("json") is True, \
                f"verb '{verb}' claims JSON support but no --json flag"
```

Conventions enforced by tests survive contributor turnover. Conventions in a CONTRIBUTING.md don't.

## Anti-patterns

- **Inventing flag names because they're "more accurate".** `--output-format=json` vs `--json`. The agent's prior is `--json`; everything else is friction.
- **`--ls` shorthand for `list`.** Either ship one or the other. Two surfaces means two patterns to learn and two surfaces to drift.
- **`-y` does something different than `--yes`.** Short forms must be exact aliases.
- **Color on stdout in `--json` mode.** ANSI codes break JSON parsers. Strip unconditionally when emitting JSON.
- **Progress bar to stdout.** Pipelines break.
- **Tool-specific env vars without the tool prefix.** `LOG_LEVEL=debug` collides with every other tool reading the same env. Use `TOOL_LOG_LEVEL`.
- **Bare invocation launches TUI.** Agents hang.
- **Inconsistent verb tense.** `create` / `created` / `creating` for similar verbs. Pick imperative across the board.
- **Honoring `NO_COLOR` but not `CI`.** Half the standards is worse than neither — agent behavior depends on which env it's run from.

## Cross-references

- `errors.md` — exit code dictionary; did-you-mean for typo'd flags.
- `envelope.md` — `--json` output shape; stdout/stderr split rules.
- `introspection.md` — `capabilities.commands.<verb>.flags` documents conventions per verb.
- `determinism.md` — `SOURCE_DATE_EPOCH` honored as part of conventions.
- `archetypes.md` — verb-name priors vary by tool category.
