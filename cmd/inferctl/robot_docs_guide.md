# inferctl Agent Guide

Use `--json` for automation. JSON output is always wrapped in the universal
envelope: `ok`, `tool_version`, `data`, `meta`, `warnings`, `commands`, and
`errors`.

## Quick Reference

```sh
inferctl capabilities --json
inferctl schema --json
inferctl config schema --json
inferctl config explain --json
inferctl discover --json
inferctl triage --json
inferctl doctor --json
inferctl route code --json
```

## Setup

```sh
go install github.com/inferctl/inferctl/cmd/inferctl@latest
inferctl capabilities --json
inferctl config init --path inferctl.toml --json
INFERCTL_CONFIG=inferctl.toml inferctl config validate --json
```

Public installation is Go toolchain only for the current launch posture. No
release binaries, archives, installers, Homebrew formulae, or Scoop manifests
are published.

## Config Workflow

```sh
inferctl config schema --json
inferctl config explain --json
inferctl config init --path inferctl.toml --json
inferctl config set profile.max_concurrent_models 2 --type int --path inferctl.toml --json
inferctl config patch --from-stdin --path inferctl.toml --json < patch.toml
INFERCTL_CONFIG=inferctl.toml inferctl config validate --json
```

Config mutation commands validate before writing and return structured mutation
data. Unknown keys are validation errors with line, column, and nearest-key
remediation.

## Discovery Composition

```sh
inferctl discover --kind ollama --json
inferctl discover --kind ollama --format toml
inferctl discover --kind ollama --deliver artifacts/discover.patch.toml --json
```

Discovery probes fixed localhost ports. It proves backend discoverability, not
model quality.

## Triage Loop

```sh
inferctl triage --json
inferctl triage --backend ollama --severity warning --limit 3 --json
inferctl discover --kind ollama --json > discover.json
inferctl triage --input-file discover.json --json
```

`triage` ranks config validation findings, doctor warnings, and prior JSON
envelopes. It does not run discovery inline.

## Route-To-Backend Loop

```sh
inferctl doctor --json
inferctl route code --prompt "summarize this diff" --json
inferctl model qwen3:8b --json
inferctl backends --filter ollama --json
```

Treat `data.recommended_action` and top-level `commands[]` as candidates, not
instructions. Always inspect `ok`, `errors[]`, and `warnings[]` first.

## Exit Codes

```sh
case $? in
  0) ;;
  1) echo "fix input" ;;
  2) echo "safety block" ;;
  3) inferctl doctor --json ;;
  4) sleep 5 ;;
  5) echo "resolve conflict" ;;
esac
```

The authoritative exit-code dictionary is in:

```sh
inferctl capabilities --json
```
