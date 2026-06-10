# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project intends to follow semantic versioning once public release
gates are cleared.

## [Unreleased]

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
