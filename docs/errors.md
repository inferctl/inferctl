# inferctl error catalog

Generated from `internal/contract/capabilities.golden.json`. Regenerate with `go generate ./internal/contract`.

## Errors

| Code | Status | Exit | Retryable | Message | Details |
|---|---:|---:|---:|---|---|
| `E_BACKEND_AUTH_FAILED` | v0.2 | 3 | false | backend '<backend>' authentication failed | `#/schemas/error_details/E_BACKEND_AUTH_FAILED` |
| `E_BACKEND_REMOTE_NOT_ALLOWED` | v0.2 | 3 | false | backend '<backend>' remote endpoint requires remote_allowed=true | `#/schemas/error_details/E_BACKEND_REMOTE_NOT_ALLOWED` |
| `E_BACKEND_TIMEOUT` | v0.1 | 4 | true | all configured backends timed out | `#/schemas/error_details/E_BACKEND_TIMEOUT` |
| `E_BINARY_INTERNAL` | v0.1 | 3 | false | internal error: <short_description> | `#/schemas/error_details/E_BINARY_INTERNAL` |
| `E_CONFIG_INVALID` | v0.1 | 3 | false | config file at <path> failed to parse | `#/schemas/error_details/E_CONFIG_INVALID` |
| `E_CONFIG_KEY_UNKNOWN` | v0.2 | 1 | false | config key '<key>' is not recognized | `#/schemas/error_details/E_CONFIG_KEY_UNKNOWN` |
| `E_CONFIG_MISSING` | v0.1 | 3 | false | no config file found | `#/schemas/error_details/E_CONFIG_MISSING` |
| `E_CONFIG_PATCH_DELETE_UNSUPPORTED` | v0.2 | 1 | false | config patch deletion is not supported | `#/schemas/error_details/E_CONFIG_PATCH_DELETE_UNSUPPORTED` |
| `E_CONFIG_UNREADABLE` | v0.1 | 3 | false | config file at <path> could not be read: <reason> | `#/schemas/error_details/E_CONFIG_UNREADABLE` |
| `E_CONFIG_VALIDATION_FAILED` | v0.1 | 1 | false | config validation found <n> error(s) and <m> warning(s) | `#/schemas/error_details/E_CONFIG_VALIDATION_FAILED` |
| `E_CONFIG_WRITE_FAILED` | v0.2 | 3 | false | could not write config to <path>: <reason> | `#/schemas/error_details/E_CONFIG_WRITE_FAILED` |
| `E_INCOMPATIBLE_FLAGS` | v0.1 | 1 | false | flags <flag_a> and <flag_b> cannot be used together | `#/schemas/error_details/E_INCOMPATIBLE_FLAGS` |
| `E_INVALID_ARG` | v0.1 | 1 | false | invalid value for <arg_name>: '<given>' (expected: <description>) | `#/schemas/error_details/E_INVALID_ARG` |
| `E_MISSING_ARG` | v0.1 | 1 | false | verb '<verb>' requires <arg_name> | `#/schemas/error_details/E_MISSING_ARG` |
| `E_NO_BACKENDS_CONFIGURED` | v0.1 | 3 | false | config at <path> defines no backends | `#/schemas/error_details/E_NO_BACKENDS_CONFIGURED` |
| `E_NO_ROUTE_AVAILABLE` | v0.1 | 4 | true | no candidate model for task '<task>' is reachable | `#/schemas/error_details/E_NO_ROUTE_AVAILABLE` |
| `E_PREFLIGHT_POLICY_BLOCKED` | v0.2 | 1 | false | preflight policy blocks task '<task>': <reason> | `#/schemas/error_details/E_PREFLIGHT_POLICY_BLOCKED` |
| `E_UNKNOWN_BACKEND` | v0.1 | 1 | false | no backend named '<backend>' in config | `#/schemas/error_details/E_UNKNOWN_BACKEND` |
| `E_UNKNOWN_FLAG` | v0.1 | 1 | false | unknown flag '<flag>' for verb '<verb>' | `#/schemas/error_details/E_UNKNOWN_FLAG` |
| `E_UNKNOWN_MODEL` | v0.1 | 1 | false | model '<model>' not found on any reachable backend | `#/schemas/error_details/E_UNKNOWN_MODEL` |
| `E_UNKNOWN_TASK` | v0.1 | 1 | false | no routing rule for task '<task>' | `#/schemas/error_details/E_UNKNOWN_TASK` |
| `E_UNKNOWN_VERB` | v0.1 | 1 | false | unknown verb '<verb>' | `#/schemas/error_details/E_UNKNOWN_VERB` |
| `E_VERB_RENAMED` | v0.1 | 1 | false | verb '<old>' has been renamed; use '<new>' | `#/schemas/error_details/E_VERB_RENAMED` |

## Warnings

| Code | Status | Message | Details |
|---|---:|---|---|
| `W_BACKEND_BACKOFF` | v0.1 | backend '<backend>' is in backoff (<seconds>s remaining) | `#/schemas/warning_details/W_BACKEND_BACKOFF` |
| `W_BACKEND_DEGRADED` | v0.1 | backend '<backend>' is responding slowly (<ms>ms; threshold <threshold>ms) | `#/schemas/warning_details/W_BACKEND_DEGRADED` |
| `W_BACKEND_KIND_UNSUPPORTED` | v0.1 | backend '<backend>' uses kind '<kind>' with v0.1-unsupported option(s): <list> | `#/schemas/warning_details/W_BACKEND_KIND_UNSUPPORTED` |
| `W_BACKEND_UNREACHABLE` | v0.1 | backend '<backend>' is unreachable: <reason> | `#/schemas/warning_details/W_BACKEND_UNREACHABLE` |
| `W_CONFIG_KEY_DEPRECATED` | v0.1 | config key '<old_key>' is deprecated; use '<new_key>' instead | `#/schemas/warning_details/W_CONFIG_KEY_DEPRECATED` |
| `W_CONFIG_KEY_UNKNOWN` | v0.1 | config key '<key>' is not recognized | `#/schemas/warning_details/W_CONFIG_KEY_UNKNOWN` |
| `W_CONFIG_SCHEMA_VERSION_MISMATCH` | v0.1 | config schema_version '<got>' does not match expected '<expected>' | `#/schemas/warning_details/W_CONFIG_SCHEMA_VERSION_MISMATCH` |
| `W_CONTEXT_NEAR_LIMIT` | v0.1 | input is <pct>% of profile max_context_tokens (<used>/<limit>) | `#/schemas/warning_details/W_CONTEXT_NEAR_LIMIT` |
| `W_FALLBACK_USED` | v0.1 | routed to fallback '<model>' because primary '<primary>' is <reason> | `#/schemas/warning_details/W_FALLBACK_USED` |
| `W_MODEL_NOT_INSTALLED` | v0.1 | model '<model>' is referenced by config but not present on any reachable backend | `#/schemas/warning_details/W_MODEL_NOT_INSTALLED` |
| `W_MODEL_NOT_LOADED` | v0.1 | model '<model>' is selected but not currently loaded (estimated warmup: <ms>ms) | `#/schemas/warning_details/W_MODEL_NOT_LOADED` |
| `W_PROBE_TIMEOUT` | v0.1 | probe tier '<tier>' timed out; envelope marked degraded | `#/schemas/warning_details/W_PROBE_TIMEOUT` |
| `W_PROFILE_MODE_NOT_ENFORCED` | v0.1 | profile.mode '<mode>' is recognized but v0.1 enforces 'warn' semantics regardless | `#/schemas/warning_details/W_PROFILE_MODE_NOT_ENFORCED` |
| `W_UPDATE_CHECK_FAILED` | v0.1 | failed to check for updates: <reason> | `#/schemas/warning_details/W_UPDATE_CHECK_FAILED` |

## Examples

- `inferctl doctr --json` emits `E_UNKNOWN_VERB` with `did_you_mean: "inferctl doctor"`.
- `inferctl explain code --json` emits `E_VERB_RENAMED` with `did_you_mean: "inferctl route code --json --explain"`.
- `inferctl version --check --json` emits `W_UPDATE_CHECK_FAILED` if the update endpoint cannot be reached and still exits 0.
