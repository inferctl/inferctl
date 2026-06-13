# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project intends to follow semantic versioning once public release
gates are cleared.

## [Unreleased]

## [0.2.0] - 2026-06-13

### Added

- Config mutation verbs: `infer config init`, `infer config set`, and
  `infer config patch`, including `--path`, `--force`, `--print`,
  `--dry-run`, `--type`, and `--from-stdin` workflows.
- `infer discover` for fixed localhost backend probes, with `--format`
  `text|json|toml`, `--kind`, `--timeout-ms`, and `--deliver`.
- `infer triage` for deterministic ranking of config validation findings,
  doctor warnings, and prior JSON envelopes, with `--input-file`,
  `--backend`, `--severity`, and `--limit`.
- Backend kind coverage for LM Studio and MLX alongside Ollama, llama.cpp,
  and OpenAI-compatible endpoints.
- Authenticated and remote `openai_compat` configuration through
  `remote_allowed`, `auth_header_name`, and redacted `auth_header_value`.
- Agent-facing documentation in `docs/agent-guide.md`, generated verb and
  error catalogs, and installation guidance in `docs/install.md`.
- GoReleaser packaging for Linux, macOS, and Windows archives, plus Scoop
  manifest generation and a Windows Scoop smoke test.

### Changed

- `infer capabilities --json` now advertises install docs, source-only
  example packaging status, v0.2 backend kinds, new verbs, and generated
  schemas for config mutation, discovery, and triage.
- Release archives now include README, changelog, license notice, install
  docs, agent guide, verb docs, error docs, and contract README while
  intentionally excluding source-only `examples/` scripts.
- CI now validates Windows line endings, snapshot archive contents, and the
  Windows release install path through a local Scoop manifest smoke test.
- Config writes are validated before replacement and use atomic temp-file
  replacement for non-dry-run mutations.

### Error Codes

- Added `E_CONFIG_WRITE_FAILED` for failed atomic config writes.
- Added `E_CONFIG_PATCH_DELETE_UNSUPPORTED` for unsupported TOML deletion
  patches.
- Added `E_BACKEND_AUTH_FAILED` for missing or rejected backend credentials.
- Added `E_BACKEND_REMOTE_NOT_ALLOWED` for remote OpenAI-compatible endpoints
  without `remote_allowed = true`.

### Compatibility

- v0.1 JSON envelope structure and existing verb shapes remain compatible.
- New v0.2 verb outputs are golden-pinned under `testdata/contract/`.
- `triage` rankings are deterministic and do not run discovery inline.
- Examples remain source-checkout artifacts for v0.2 rather than packaged
  release payloads.

## [0.1.0] - 2026-06-10

### Added

- Agent-oriented JSON envelope rendering with deterministic test mode.
- Read-only backend adapters for Ollama, llama.cpp, and OpenAI-compatible
  `/v1/models` endpoints.
- `infer capabilities`, `doctor`, `backends`, `models`, `model`, `route`,
  `config show`, `config validate`, `config explain`, and `version`.
- Stable v0.1 error and warning catalog with structured `did_you_mean`,
  retryability, and exit-code metadata.
- Deterministic fixture server and three repo-local demo scripts.
- Contract goldens, CI workflow, and GoReleaser snapshot configuration.

### Known Limits

- v0.1 does not run inference, warm models, release models, or collect latency
  observations; route latency fields remain null/zero by design.
- Authenticated and remote OpenAI-compatible backends are deferred.
- Human output is intentionally readable but not byte-for-byte stable.
- License remains pending; private evaluation only.

### Deferred

- v0.2: config mutation helpers and authenticated remote backend support.
- v0.5: warmup, release, wait, locks, stats collection, and richer routing
  policy controls.
