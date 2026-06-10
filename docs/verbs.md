# inferctl v0.1 verbs

Generated from `internal/contract/capabilities.golden.json`. Regenerate with `go generate ./internal/contract`.

## `infer doctor`

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
infer doctor --json
```

## `infer backends`

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
infer backends --json
```

## `infer models`

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
infer models --json
```

## `infer model`

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
infer model qwen3:8b --json
```

## `infer route`

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
infer route code --prompt "summarize this" --json
```

## `infer config`

Namespace for config show, validate, and explain. Not directly invokable.

Namespace only; use one of its subcommands.

## `infer config show`

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
infer config show --json
```

## `infer config validate`

Validate config and return source-position findings.

- JSON data schema: `#/schemas/config_validation`
- Exit codes: `0`, `1`, `3`
- Emits data on failure: `true`

### Flags

- `--strict` type=`bool` default=`false`
- `--json` type=`bool` default=`false`

### Example

```sh
infer config validate --json
```

## `infer config explain`

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
infer config explain --key profile.mode --json
```

## `infer capabilities`

Emit the machine-readable CLI contract.

- Mega-command: `CAPABILITIES`
- JSON data schema: `#/schemas/capabilities_manifest`
- Exit codes: `0`
- Emits data on failure: `false`

### Flags

- `--json` type=`bool` default=`false`

### Example

```sh
infer capabilities --json
```

## `infer version`

Show version, build metadata, and optional update status.

- JSON data schema: `#/schemas/version_info`
- Exit codes: `0`
- Emits data on failure: `false`

### Flags

- `--check` type=`bool` default=`false`
- `--json` type=`bool` default=`false`

### Example

```sh
infer version --json
```

