# config.md — Config and Identity Are Introspectable

Configuration files are an unindexed agent black hole unless the tool exposes them as a structured surface. An agent that can ask "what's my effective config?" and "where did each value come from?" can debug; one that has to parse TOML cannot.

This file covers the config verb surface (`schema`, `validate`, `show`, `get`, `set`, `patch`), per-key provenance, profiles, and precedence rules.

## The config verb family

Every tool with config ships these:

| Verb | Purpose |
|---|---|
| `config schema --json` | Returns the JSON Schema for the config file |
| `config validate [<path>] --json` | Validates a config file; returns structured errors with `did_you_mean` |
| `config show --json` | Returns the *effective* config with `_provenance` per key |
| `config get <key>` | Returns one value (for use in shell scripts) |
| `config set <key> <value> [--profile=p]` | Sets one value; preserves comments |
| `config patch --json` | Applies a JSON patch to the config |
| `config edit` | Opens `$EDITOR` (only when stdin is a TTY) |

The agent can read, validate, and modify config without ever parsing the file format.

## `config schema --json`

The JSON Schema for the config file. Versioned independently of `contract_version` (config schema evolves differently from CLI contract).

```bash
$ <tool> config schema --json
```

```json
{
  "$schema":          "http://json-schema.org/draft-07/schema#",
  "_schema_version":  "3",
  "type":             "object",
  "properties": {
    "endpoint":     {"type": "string", "format": "uri", "default": "http://localhost:11434"},
    "log_level":    {"type": "string", "enum": ["debug", "info", "warn", "error"], "default": "info"},
    "profiles":     {"type": "object", "additionalProperties": {"$ref": "#/definitions/profile"}},
    "feedback": {
      "type": "object",
      "properties": {
        "upstream_url":   {"type": "string", "format": "uri"},
        "upstream_token": {"type": "string"}
      }
    }
  },
  "definitions": {
    "profile": { /* ... */ }
  },
  "additionalProperties": false
}
```

Agents validate proposed config locally before writing; tooling generates types from this schema. The `additionalProperties: false` is load-bearing — it's how typos in keys get caught.

## `config validate` with did-you-mean

```bash
$ <tool> config validate --json
```

```json
{
  "ok": false,
  "data": null,
  "errors": [
    {
      "code":         "INVALID_CONFIG",
      "message":      "unknown key 'endpoit' at line 4",
      "path":         "$.endpoit",
      "did_you_mean": "endpoint",
      "remediation":  "<tool> config set endpoint http://localhost:11434",
      "exit_code":    1
    },
    {
      "code":         "INVALID_CONFIG",
      "message":      "log_level must be one of: debug, info, warn, error (got: 'verbose')",
      "path":         "$.log_level",
      "did_you_mean": "debug",
      "remediation":  "<tool> config set log_level debug",
      "exit_code":    1
    }
  ]
}
```

Every error is structured, names the path, suggests a fix, and provides a paste-ready command. This is the same discipline as runtime errors (see `errors.md`).

## `config show --json` with `_provenance`

The killer feature. Returns the *resolved* config (all defaults filled in, all sources merged) AND for every key, where the value came from.

```bash
$ <tool> config show --json
```

```json
{
  "ok": true,
  "data": {
    "config": {
      "endpoint":  "https://prod.example.com",
      "log_level": "debug",
      "profiles": {
        "prod": {"endpoint": "https://prod.example.com"}
      }
    },
    "_provenance": {
      "endpoint":  {"source": "profile",  "profile": "prod"},
      "log_level": {"source": "env",      "env_var": "TOOL_LOG_LEVEL"},
      "profiles":  {"source": "config",   "path": "/Users/me/.config/<tool>/config.toml", "line": 7}
    }
  },
  "meta": {...}
}
```

The agent debugging a confused-config situation reads `_provenance` and immediately knows: "log_level is 'debug' because of `TOOL_LOG_LEVEL`, not because the config file said so."

Without `_provenance`, the agent's only recourse is to grep the file and the environment by hand, hoping nothing got merged in unexpected ways.

### Provenance source values

| `source` | Meaning |
|---|---|
| `default` | Built-in default |
| `config` | Read from a config file (also: `path`, `line`) |
| `profile` | Read from an active profile (also: `profile`) |
| `env` | Read from environment (also: `env_var`) |
| `flag` | Read from a command-line flag (also: `flag_name`) |

## `config get / set / patch`

For programmatic config modification without parsing the file:

```bash
$ <tool> config get endpoint
https://prod.example.com

$ <tool> config get endpoint --json
{"ok": true, "data": {"key": "endpoint", "value": "https://prod.example.com"}, ...}

$ <tool> config set endpoint https://staging.example.com --json
{"ok": true, "data": {"key": "endpoint", "old_value": "...", "new_value": "https://staging.example.com"}, ...}

$ <tool> config patch --json
{"endpoint": "https://staging.example.com", "log_level": "info"}
# (reads patch from stdin; applies atomically)
```

### Comment preservation

`set` and `patch` MUST preserve comments and whitespace in the config file. The agent edits one key; everything else stays as the human wrote it.

This requires the tool to read-parse-modify-write the *AST* of the config format, not re-emit it from a parsed dict. For TOML, use a comment-preserving parser (e.g. `tomlkit` in Python, `go-toml` v2 with formatting preservation). For YAML, similar — `ruamel.yaml` round-trip mode.

Test the round-trip:

```bash
$ cp config.toml /tmp/before
$ <tool> config set log_level debug
$ diff /tmp/before config.toml
< log_level = "info"
---
> log_level = "debug"
# Only the one line changed. Comments preserved.
```

This passes or fails by test. Ship the test.

## Profiles

Profiles bundle reusable identity:

```toml
# config.toml
[profiles.prod]
endpoint = "https://prod.example.com"
api_token_env = "TOOL_PROD_TOKEN"
log_level = "warn"

[profiles.dev]
endpoint = "http://localhost:11434"
log_level = "debug"
```

Selection by flag, env, or default:

```bash
$ <tool> list --profile=prod --json
$ TOOL_PROFILE=prod <tool> list --json
$ <tool> list --json    # uses [default] or built-in default
```

Discoverable via capabilities:

```bash
$ <tool> capabilities --json | jq '.config.profiles'
{
  "available":      ["prod", "dev", "staging"],
  "active":         "prod",
  "active_source":  {"source": "env", "env_var": "TOOL_PROFILE"}
}
```

The agent can list profiles, see which is active, and switch — all programmatically.

### Profile-scoped sensitive values

Secrets in config files are a security smell. Profiles reference env vars instead:

```toml
[profiles.prod]
endpoint       = "https://prod.example.com"
api_token_env  = "TOOL_PROD_TOKEN"     # tool reads from this env var at runtime
```

The config file is safe to commit; the secret lives in the environment. Document this pattern; reject literal `api_token = "..."` in `config validate` with a `SECRET_IN_CONFIG` warning.

## Precedence

Documented, fixed, identical across every flag:

```
flag > env > profile > config file > default
```

Higher precedence wins. Lower precedence fills in gaps.

Example:

| Source | `log_level` value |
|---|---|
| default | `info` |
| config file | `warn` |
| profile (`prod`) | not set |
| env `TOOL_LOG_LEVEL` | `debug` |
| flag `--log-level=trace` | `trace` |
| **effective** | `trace` |

`_provenance.log_level` reports `{"source": "flag", "flag_name": "--log-level"}`.

Document precedence in `--help` AND in `capabilities`:

```json
"config": {
  "precedence": ["flag", "env", "profile", "file", "default"]
}
```

## Config file search path

XDG-conformant default:

```
$XDG_CONFIG_HOME/<tool>/config.toml          # if set
$HOME/.config/<tool>/config.toml             # default fallback
```

Plus override:

```bash
$ <tool> --config=/path/to/alt.toml list
$ TOOL_CONFIG=/path/to/alt.toml <tool> list
```

Surface the path in `_provenance` and in `config show`:

```json
"_provenance": {
  "_config_file": "/Users/me/.config/<tool>/config.toml"
}
```

If no config file exists, that's not an error — the tool runs with defaults. `config show` returns the effective config with `_provenance.*.source` mostly `"default"`.

## Anti-patterns

- **No `config show`.** Agent has to `cat config.toml && env | grep TOOL_`. Doesn't account for defaults or profile merging.
- **`config show` without `_provenance`.** Agent sees the effective value but can't tell *why* it has that value.
- **`config set` rewrites the whole file.** Comments destroyed; whitespace mangled; human contributors annoyed.
- **Strict-mode config (any unknown key is fatal) without `did_you_mean`.** Agent typos a key; gets `unknown key 'endpoit'`; has no path forward.
- **Permissive config (unknown keys silently ignored).** Agent typos a key; tool says nothing; wonders why behavior didn't change. Use `additionalProperties: false` + `did_you_mean`.
- **Secrets in config file.** Use env var references.
- **Precedence order varies per flag.** "Flag wins for `endpoint` but env wins for `log_level`" — agent can't reason about effective state.
- **No `config schema`.** Agent has to read documentation or source to know what keys are valid.
- **Profiles without discoverability.** `capabilities` doesn't list available profiles; agent has to grep config file.
- **`config edit` runs `$EDITOR` even when stdin is not a TTY.** Hang on agent invocations. Detect TTY; refuse if not.
- **Patch operation isn't atomic.** Mid-patch crash leaves config in inconsistent state.

## Cross-references

- `envelope.md` — `_provenance` follows the envelope's pattern of attaching metadata to data.
- `errors.md` — `INVALID_CONFIG`, `SECRET_IN_CONFIG` codes; `did_you_mean` discipline.
- `safety.md` — `config set` on shared config requires `--yes` if it changes destructive defaults.
- `schema-evolution.md` — `_schema_version` for the config schema, evolved independently.
- `conventions.md` — `TOOL_PROFILE`, `TOOL_CONFIG`, XDG var conventions.
