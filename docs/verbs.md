# inferctl v0.1 verbs

Generated from `internal/contract/capabilities.golden.json`. Regenerate with `go generate ./internal/contract`.

## `inferctl doctor`

Health, loaded models, and route resolution across configured backends.

- Mega-command: `DIAGNOSE`
- JSON data schema: `#/schemas/doctor_report`
- Exit codes: `0`, `3`, `4`
- Emits data on failure: `false`

### Flags

- `--json` type=`bool` default=`false`
- `--fast` type=`bool` default=`false`
- `--backend` type=`string` default=`<nil>`

### Example

```sh
inferctl doctor --json
```

## `inferctl backends`

List configured backends with reachability status.

- JSON data schema: `#/schemas/backend_list`
- Exit codes: `0`, `3`
- Emits data on failure: `false`

### Flags

- `--filter` type=`string` default=`<nil>`
- `--kind` type=`string` default=`<nil>`
- `--json` type=`bool` default=`false`
- `--fast` type=`bool` default=`false`

### Example

```sh
inferctl backends --json
```

## `inferctl models`

List installed or loaded models across configured backends.

- JSON data schema: `#/schemas/model_list`
- Exit codes: `0`, `3`
- Emits data on failure: `false`

### Flags

- `--backend` type=`string` default=`<nil>`
- `--loaded` type=`bool` default=`false`
- `--installed` type=`bool` default=`true`
- `--json` type=`bool` default=`false`

### Example

```sh
inferctl models --json
```

## `inferctl model`

Inspect one model's backend placements, capabilities, stats, and routing usage.

- JSON data schema: `#/schemas/model_detail`
- Exit codes: `0`, `1`, `3`
- Emits data on failure: `false`

### Args

- `model_name` required=true

### Flags

- `--json` type=`bool` default=`false`
- `--no-probe` type=`bool` default=`false`

### Example

```sh
inferctl model qwen3:8b --json
```

## `inferctl route`

Compute and explain a route for a configured task.

- Mega-command: `PLAN`
- JSON data schema: `#/schemas/route_explanation`
- Exit codes: `0`, `1`, `3`, `4`
- Emits data on failure: `false`

### Args

- `task` required=true

### Flags

- `--prompt-file` type=`string` default=`<nil>`
- `--prompt` type=`string` default=`<nil>`
- `--from-stdin` type=`bool` default=`false`
- `--prefer` type=`enum` default=`default`
- `--explain` type=`bool` default=`true`
- `--quiet` type=`bool` default=`false`
- `--json` type=`bool` default=`false`

### Example

```sh
inferctl route code --prompt "summarize this" --json
```

## `inferctl config`

Namespace for config show, schema, validate, explain, init, set, and patch. Not directly invokable.

Namespace only; use one of its subcommands.

## `inferctl config show`

Show the effective config with per-key provenance.

- JSON data schema: `#/schemas/config_view`
- Exit codes: `0`, `3`
- Emits data on failure: `false`

### Flags

- `--section` type=`string` default=`<nil>`
- `--key` type=`string` default=`<nil>`
- `--no-provenance` type=`bool` default=`false`
- `--json` type=`bool` default=`false`

### Example

```sh
inferctl config show --json
```

## `inferctl config validate`

Validate config and return source-position findings.

- JSON data schema: `#/schemas/config_validation`
- Exit codes: `0`, `1`, `3`
- Emits data on failure: `true`

### Flags

- `--strict` type=`bool` default=`false`
- `--json` type=`bool` default=`false`

### Example

```sh
inferctl config validate --json
```

## `inferctl config schema`

Export the inferctl TOML config JSON Schema.

- JSON data schema: `#/schemas/config_file`
- Exit codes: `0`
- Emits data on failure: `false`

### Flags

- `--json` type=`bool` default=`false`

### Example

```sh
inferctl config schema --json
```

## `inferctl config explain`

Print annotated default config and machine-readable key definitions.

- JSON data schema: `#/schemas/config_explanation`
- Exit codes: `0`, `1`
- Emits data on failure: `false`

### Flags

- `--key` type=`string` default=`<nil>`
- `--format` type=`enum` default=`toml`
- `--json` type=`bool` default=`false`

### Example

```sh
inferctl config explain --key profile.mode --json
```

## `inferctl config init`

Create or print a starter inferctl config.

- JSON data schema: `#/schemas/config_mutation`
- Exit codes: `0`, `1`, `3`
- Emits data on failure: `false`

### Flags

- `--path` type=`string` default=`<nil>`
- `--force` type=`bool` default=`false`
- `--print` type=`bool` default=`false`
- `--json` type=`bool` default=`false`

### Example

```sh
inferctl config init --json
```

## `inferctl config set`

Set one config key while preserving surrounding TOML comments and ordering.

- JSON data schema: `#/schemas/config_mutation`
- Exit codes: `0`, `1`, `3`
- Emits data on failure: `false`

### Args

- `key` required=true
- `value` required=true

### Flags

- `--path` type=`string` default=`<nil>`
- `--type` type=`enum` default=`<nil>`
- `--dry-run` type=`bool` default=`false`
- `--json` type=`bool` default=`false`

### Example

```sh
inferctl config set --json
```

## `inferctl config patch`

Merge a TOML fragment into config without supporting deletions.

- JSON data schema: `#/schemas/config_mutation`
- Exit codes: `0`, `1`, `3`
- Emits data on failure: `false`

### Args

- `toml-fragment` required=false

### Flags

- `--path` type=`string` default=`<nil>`
- `--from-stdin` type=`bool` default=`false`
- `--dry-run` type=`bool` default=`false`
- `--json` type=`bool` default=`false`

### Example

```sh
inferctl config patch --json
```

## `inferctl discover`

Probe fixed localhost backend ports and report verified backend candidates.

- JSON data schema: `#/schemas/discovery_report`
- Exit codes: `0`, `1`, `3`, `4`
- Emits data on failure: `false`

### Flags

- `--format` type=`enum` default=`text`
- `--kind` type=`enum` default=`<nil>`
- `--timeout-ms` type=`int` default=`750`
- `--deliver` type=`string` default=`<nil>`
- `--json` type=`bool` default=`false`

### Example

```sh
inferctl discover --json
```

## `inferctl triage`

Rank deterministic diagnostic next actions from config validation, doctor output, or prior JSON envelopes.

- Mega-command: `TRIAGE`
- JSON data schema: `#/schemas/triage_report`
- Exit codes: `0`, `1`, `3`
- Emits data on failure: `false`

### Flags

- `--input-file` type=`string` default=`<nil>`
- `--backend` type=`string` default=`<nil>`
- `--severity` type=`enum` default=`<nil>`
- `--limit` type=`int` default=`0`
- `--json` type=`bool` default=`false`

### Example

```sh
inferctl triage --json
```

## `inferctl capabilities`

Emit the machine-readable CLI contract.

- Mega-command: `CAPABILITIES`
- JSON data schema: `#/schemas/capabilities_manifest`
- Exit codes: `0`
- Emits data on failure: `false`

### Flags

- `--json` type=`bool` default=`false`

### Example

```sh
inferctl capabilities --json
```

## `inferctl version`

Show version, build metadata, and optional update status.

- JSON data schema: `#/schemas/version_info`
- Exit codes: `0`
- Emits data on failure: `false`

### Flags

- `--check` type=`bool` default=`false`
- `--json` type=`bool` default=`false`

### Example

```sh
inferctl version --json
```

## `inferctl schema`

Export JSON schemas for envelopes and verb data.

- JSON data schema: `#/schemas/schema_export`
- Exit codes: `0`, `1`
- Emits data on failure: `false`

### Flags

- `--command` type=`string` default=`<nil>`
- `--json` type=`bool` default=`false`

### Example

```sh
inferctl schema --json
```

## `inferctl robot-docs`

Namespace for agent workflow documentation. Not directly invokable.

Namespace only; use one of its subcommands.

## `inferctl robot-docs guide`

Print the embedded agent workflow guide.

- JSON data schema: `#/schemas/robot_docs_guide`
- Exit codes: `0`
- Emits data on failure: `false`

### Flags

- `--json` type=`bool` default=`false`

### Example

```sh
inferctl robot-docs guide --json
```

