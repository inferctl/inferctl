## Ship inferctl v0.1

### ID
epic-v01

### Type
epic

### Priority
P1

### Labels
v0-1, implementation, repo-deliverable

### Description
Build the private Go implementation of `inferctl` v0.1 inside the repository. The product is the `infer` binary, Go library surface, tests, generated docs, demos, CI, release configuration, and repo documentation needed for a private v0.1 release candidate. Non-repo responsibilities such as Guideboard outreach, lawyer review, and external name availability checks are intentionally excluded.

### Design
Implement from the finalized v0.1 specification set and make the machine contract executable before filling in human rendering. Use a single parent epic so cross-cutting implementation rules can be inherited by all tasks. Child tasks are sequenced roughly by the week-by-week plan: repo hygiene and contract foundation, backend adapters and testserver, verbs, error coverage, demos, CI/release prep, docs, and final release-candidate readiness.

### Acceptance
- `go test ./...`, `go vet ./...`, and `go build ./...` pass.
- `infer capabilities --json | jq .data` byte-matches `internal/contract/capabilities.golden.json`; envelope shape is tested separately.
- All ten invokable command forms emit envelope-conformant JSON: `doctor`, `backends`, `models`, `model`, `route`, `config show`, `config validate`, `config explain`, `capabilities`, and `version`.
- All v0.1-status error and warning codes have test paths; reserved codes are listed but not emitted.
- Demo scripts in `examples/` run against `internal/testserver` fixtures, not local developer services.
- README, CHANGELOG, generated docs, CI, and release dry-run artifacts are current enough for a private v0.1.0 release candidate.

## Prepare repo hygiene

### ID
repo-hygiene

### Parent
epic-v01

### Type
task

### Priority
P1

### Labels
v0-1, repo, docs

### Description
The repository needs its initial private-release metadata in place before implementation work starts. This work is limited to files inside the repo and must preserve any existing user changes because the local repo may already contain untracked README, `.gitignore`, and `skills/` content.

### Design
Inspect `git status`, `git log --oneline`, and remotes first. Add `CHANGELOG.md` with a Keep-a-Changelog `[Unreleased]` section, `LICENSE_PENDING.md` explaining that no license is granted yet, a README line near the top saying `No license granted yet; private evaluation only`, and a `.github/workflows/` directory placeholder if a tracked placeholder is needed before CI exists.

### Acceptance
- Existing user work is preserved and no unrelated untracked files are discarded.
- `CHANGELOG.md` exists with an `[Unreleased]` section.
- `LICENSE_PENDING.md` exists instead of a real `LICENSE`.
- README contains `No license granted yet; private evaluation only` near the top.
- The repo has a place for GitHub workflows without adding real CI yet.

## Pin capabilities golden

### ID
capabilities-golden

### Parent
epic-v01

### Dependencies
- repo-hygiene

### Type
task

### Priority
P1

### Labels
v0-1, contract, golden

### Description
The v0.1 machine surface must be pinned before verb implementation so code is built toward a stable contract rather than reverse-engineering a contract from implementation.

### Design
Create `internal/contract/capabilities.golden.json` by hand. The file pins only `envelope.data`, not dynamic envelope metadata. Include `tool`, `binary`, `contract_version`, ten invokable command-form entries plus the `config` namespace entry, `exit_codes`, `env_vars`, `envelope_version`, placeholder schema refs, `error_codes`, `warning_codes`, and `shared_types`. Mark code statuses as `v0.1`, `reserved`, or `future`; `E_CONFIG_WRITE_FAILED` is reserved and not emitted in v0.1.

### Acceptance
- `internal/contract/capabilities.golden.json` exists and a reviewer can reconstruct the CLI machine surface from it.
- `config` is represented as namespace-only and is not counted as an invokable command form.
- Only `config validate` has `emits_data_on_failure: true`.
- `schemas` placeholders have stable outer names for every v0.1 verb and shared type.
- The golden includes 17 active v0.1 errors, 14 active v0.1 warnings, and reserved code status where applicable.

## Build Go foundation

### ID
go-foundation

### Parent
epic-v01

### Dependencies
- capabilities-golden

### Type
task

### Priority
P1

### Labels
v0-1, go, config, foundation

### Description
The CLI needs a compiling Go module, stable package layout, shared types, envelope primitives, and a minimal config loader before backend or verb work can be built safely.

### Design
Initialize module `github.com/Ozhiaki/inferctl` and binary `cmd/infer`. Establish packages for contract, envelope, render, verbs, config, errors, testserver, store, and exported `pkg/inferctl` types. Transcribe shared schema types into Go structs with explicit JSON tags. Verify TOML parser key-position support before committing to validation design; if `github.com/pelletier/go-toml/v2` cannot provide useful key positions, choose a parser that can. Implement config resolution order, defaults, env overlays, flag-overlay hook, provenance map, `INFERCTL_DEFAULT_BACKEND` mutation, and position metadata.

### Acceptance
- `go.mod` exists and `go build ./...`, `go vet ./...`, and `go test ./...` pass.
- The binary can be built as `infer` and bare invocation returns a placeholder no-verb error.
- Shared Go structs exist for documented shared types such as `BackendInfo`, `BackendStatus`, `ModelInfo`, `LoadedModelInfo`, `Capabilities`, `LatencyStats`, `RouteCandidate`, `RouteDecision`, `RouteConstraints`, `Finding`, `ConfigKeyDef`, and `RecommendedAction`.
- Config tests cover the worked example, malformed TOML with line/column, `INFERCTL_DEFAULT_BACKEND` mutation provenance, and key-position smoke behavior.
- `config explain` remains possible without a config file because loader use is bypassable for that verb.

## Implement envelope rendering

### ID
envelope-rendering

### Parent
epic-v01

### Dependencies
- go-foundation

### Type
task

### Priority
P1

### Labels
v0-1, envelope, rendering

### Description
Every JSON-mode command must emit the same agent-friendly envelope, and human output must render from typed data without changing machine semantics.

### Design
Implement `Envelope[T]`, warning/command/error shapes, metadata construction, deterministic test mode, canonical JSON data hashing, JSON renderer, human-renderer dispatch, and ANSI suppression. JSON is emitted only with `--json` or `INFERCTL_FORMAT=json`; non-TTY output remains human text. `INFERCTL_TEST_DETERMINISTIC=1` fixes `request_id`, `ts_iso`, and `elapsed_ms` only; `data_hash` still hashes real data.

### Acceptance
- All JSON envelopes include `ok`, `tool_version`, `data`, `meta`, `warnings`, `commands`, and `errors`.
- `meta.contract_version` is `"0.1"` and `data_hash` is computed from canonical JSON serialization of `data`.
- Human output remains the default for pipes and redirects unless JSON mode is explicitly selected.
- ANSI is suppressed for `NO_COLOR`, `CI`, `TERM=dumb`, and non-TTY output.
- Envelope conformance tests cover required keys, types, and deterministic mode.

## Implement backend adapters

### ID
backend-adapters

### Parent
epic-v01

### Dependencies
- go-foundation

### Type
task

### Priority
P1

### Labels
v0-1, backends

### Description
The read-only verbs depend on deterministic backend abstractions for Ollama, llama.cpp, and the narrowed v0.1 `openai_compat` surface.

### Design
Define the exported `Backend` interface in `pkg/inferctl/backend.go` and implement internal adapters for Ollama, llama.cpp, and `openai_compat`. Ollama uses `/api/version`, `/api/tags`, and `/api/ps`. llama.cpp uses `/v1/models` and treats a reachable server as hosting one live model when applicable. `openai_compat` is local-only, unauthenticated, `remote_allowed=false`, and `/v1/models` only; unsupported auth or remote options produce `W_BACKEND_KIND_UNSUPPORTED` rather than fatal errors. Use context-aware HTTP clients with timeouts and no retry loop in adapters.

### Acceptance
- Adapter tests use `httptest.Server` and cover happy path, timeout, and malformed response behavior.
- Ollama reports reachability, installed models, and loaded models.
- llama.cpp reports models via `/v1/models` and handles the lack of a native loaded-model concept.
- `openai_compat` works for unauthenticated local `/v1/models` and gracefully reports unsupported v0.1 options as warnings.
- Backend failures do not require outbound calls to real local services in tests.

## Build testserver fixtures

### ID
testserver-fixtures

### Parent
epic-v01

### Dependencies
- backend-adapters

### Type
task

### Priority
P1

### Labels
v0-1, testing, backends

### Description
Tests and demos need deterministic mock backends so v0.1 acceptance does not depend on the developer's real Ollama, llama.cpp, or OpenAI-compatible servers.

### Design
Create `internal/testserver` fixtures for Ollama, llama.cpp, and `openai_compat`. Each fixture should take configured models, loaded state, latency, backoff, and failure behavior, then serve the corresponding HTTP endpoints used by adapters. Make fixture startup usable from unit tests and example scripts.

### Acceptance
- Deterministic mock servers exist for all three v0.1 backend kinds.
- Fixtures can simulate reachable, unreachable, timeout, malformed response, loaded models, installed models, latency, and backoff states.
- Demo scripts can launch fixture state without depending on real local services.
- Subsequent verb tests can reuse fixtures without duplicating HTTP stubs.

## Wire capabilities verb

### ID
capabilities-verb

### Parent
epic-v01

### Dependencies
- capabilities-golden
- envelope-rendering

### Type
task

### Priority
P1

### Labels
v0-1, contract, verb

### Description
`infer capabilities` is the executable contract for agents and the first end-to-end verb that proves the envelope, renderer, and golden-file discipline work together.

### Design
Register the `capabilities` command in Cobra, embed `internal/contract/capabilities.golden.json`, and return its JSON content as `envelope.data` in JSON mode. Add the data-only golden diff test and an envelope conformance test that can be reused by other verbs.

### Acceptance
- `infer capabilities --json | jq .data` matches `internal/contract/capabilities.golden.json`.
- The full envelope contains dynamic metadata but is not byte-diffed against the golden.
- Human-mode `infer capabilities` renders a useful summary without becoming the contract.
- Golden and envelope tests pass under `go test ./...`.

## Implement version verb

### ID
version-verb

### Parent
epic-v01

### Dependencies
- envelope-rendering
- capabilities-verb

### Type
task

### Priority
P2

### Labels
v0-1, verb

### Description
The version command exposes tool version, build metadata, dependency versions, contract version, schema version, and an opt-in update check while preserving the no-network-by-default rule.

### Design
Implement `infer version` with build metadata injected by ldflags and `--check` as the only v0.1 outbound network path. Without `--check`, `update.checked=false` and related update fields are `null`. Failed update checks produce `W_UPDATE_CHECK_FAILED` and exit 0.

### Acceptance
- Human and JSON outputs match the version schema.
- Build metadata includes commit, date, Go version, OS, and architecture.
- Dependency versions include core CLI/config libraries actually used by the build.
- `infer version` makes no network calls unless `--check` is set.
- `--check` failure emits `W_UPDATE_CHECK_FAILED` and still exits 0.

## Implement list verbs

### ID
list-verbs

### Parent
epic-v01

### Dependencies
- backend-adapters
- testserver-fixtures
- envelope-rendering

### Type
task

### Priority
P1

### Labels
v0-1, verbs, backends

### Description
The read-only list and inspect commands expose configured backend and model state to humans and agents using the resolved config and backend adapters.

### Design
Implement `infer backends`, `infer models`, and `infer model <name>`. `backends` filters by `--filter` and `--kind`; `models` filters by `--backend`, `--loaded`, and `--installed`; `model` reports all backend placements for a model, capability flags, always-present v0.1-null latency stats, and routing relationships. Same model names on multiple backends produce multiple entries.

### Acceptance
- `infer backends --json` emits `BackendStatus[]`, `total_count`, and `reachable_count`.
- `infer models --json` emits one entry per `(model, backend)` pair and supports loaded-only views.
- `infer model <name> --json` emits model details including `capabilities`, `latency_stats`, and routing usage.
- Unknown backend and unknown model cases emit the documented errors with runnable `did_you_mean` where available.
- Per-verb goldens and fixture-backed tests pass.

## Implement config show

### ID
config-show

### Parent
epic-v01

### Dependencies
- go-foundation
- envelope-rendering

### Type
task

### Priority
P1

### Labels
v0-1, config, verb

### Description
Users and agents need to inspect the effective configuration after defaults, a single selected file, environment overrides, and flag overrides are applied.

### Design
Implement `infer config show` over the loader. Return `source_paths`, `effective_config`, and flat dotted-path provenance in JSON. Honor `--section`, `--key`, and `--no-provenance`; `--key` returns the narrowed shape with `key`, `value`, `provenance`, and `type`. There is no multi-file merge in v0.1.

### Acceptance
- Worked-example config produces the expected effective config and provenance map.
- `source_paths.selected`, `searched`, and `selected_by` reflect the actual resolution path.
- Env and flag overlays appear in provenance as `env` and `flag`, not as source files.
- `--section`, `--key`, and `--no-provenance` work and invalid selectors emit `E_INVALID_ARG`.
- Missing, unreadable, and invalid config errors use the documented codes.

## Implement config validation

### ID
config-validation

### Parent
epic-v01

### Dependencies
- go-foundation
- envelope-rendering

### Type
task

### Priority
P1

### Labels
v0-1, config, verb

### Description
`infer config validate` needs to lint config files with source-position findings and the special v0.1 failure semantics where diagnostic data is still returned on failure.

### Design
Run structural, semantic error, warning, and strict-mode checks from the config schema against the parsed AST and position map. Findings use severity `error`, `warning`, or `info`, dotted keys, line/column where attributable, and details objects for derived constraints. Parse failures are `E_CONFIG_INVALID`; semantic validation failures are `E_CONFIG_VALIDATION_FAILED` with `data.findings[]` still present.

### Acceptance
- Clean config exits 0 with `passed=true`.
- Warnings-only config exits 0 unless `--strict` is set.
- Error findings exit 1 with `ok=false`, `errors[0].code=E_CONFIG_VALIDATION_FAILED`, and populated `data.findings[]`.
- Missing `meta.schema_version` is structural error; present-but-different schema version emits `W_CONFIG_SCHEMA_VERSION_MISMATCH`.
- Findings include line/column for direct key errors and `null` line/column for derived cross-key findings.

## Implement config explain

### ID
config-explain

### Parent
epic-v01

### Dependencies
- go-foundation
- envelope-rendering

### Type
task

### Priority
P2

### Labels
v0-1, config, verb

### Description
`infer config explain` is the built-in way for users and agents to understand or create a valid config, and it must work before any config file exists.

### Design
Embed an annotated default config source and generate or reflect a `ConfigKeyDef[]` catalog from the config schema. Support `--key` filtering and `--format toml|md`. Bypass the normal config loader for this verb so a missing config file never blocks explanation.

### Acceptance
- `infer config explain --json` works with no config file on disk.
- Output includes `format`, `annotated_source`, `keys`, and `schema_version`.
- `--key` narrows to one key or wildcard section and unknown keys emit `E_INVALID_ARG`.
- `toml` and `md` formats render from the same key catalog.
- The key catalog includes type, requiredness/default, valid set, description, and examples for documented config keys.

## Implement doctor verb

### ID
doctor-verb

### Parent
epic-v01

### Dependencies
- list-verbs
- config-show
- config-validation
- testserver-fixtures

### Type
task

### Priority
P1

### Labels
v0-1, verb, diagnose

### Description
`infer doctor` is the DIAGNOSE mega-command and the primary install-moment experience. It should summarize backend health, loaded models, routes, system constraints, warnings, and the single best next action.

### Design
Compose backend status, loaded model data, route summaries, and system info. System VRAM probing is best-effort and capped at 250ms per source using `system_profiler`, `nvidia-smi`, Apple Silicon unified-memory estimation, or `profile.vram_total_bytes_hint`. Backend failures are diagnostic content, not command failure, unless no meaningful partial result exists. Populate `recommended_action` and up to six `commands[]` entries deterministically.

### Acceptance
- JSON output matches the doctor schema and includes summary, backends, loaded models, routes, system, and nullable `recommended_action`.
- Down or degraded backends appear as rows and warnings while doctor still exits 0 when partial diagnosis is useful.
- `commands[]` ranks unreachable backend inspections before route follow-ups and future-version commands include `available_in_version`.
- Human output contains readable sections but is not byte-diffed as a contract.
- Fixture-backed golden tests cover clean, degraded, fallback, and no-backends scenarios.

## Implement route verb

### ID
route-verb

### Parent
epic-v01

### Dependencies
- list-verbs
- config-validation
- testserver-fixtures

### Type
task

### Priority
P1

### Labels
v0-1, verb, plan

### Description
`infer route <task>` is the PLAN mega-command that selects the first available candidate in a configured fallback chain and explains the decision without running inference.

### Design
Implement prompt sourcing via `--prompt`, `--prompt-file`, and stdin; estimate tokens with the v0.1 `chars/4` heuristic. Walk the configured primary and fallback candidates in order and select the first candidate that is installed, reachable, and not in backoff. Route estimates stay `null` in v0.1 because no inference stats are collected. Populate `commands[]` with future warmup, primary-backend inspection, context-limit inspection, and selected-model inspection according to deterministic rules.

### Acceptance
- `infer route <task> --json` emits task, input, decision, candidates, and constraints.
- `--explain` and `--quiet` affect human rendering only; JSON shape remains invariant.
- Unknown task, missing task, and no-route-available cases emit documented errors.
- Fallback selections emit `W_FALLBACK_USED`; unloaded selected models can emit `W_MODEL_NOT_LOADED`; near-limit inputs can emit `W_CONTEXT_NEAR_LIMIT`.
- Fixture-backed goldens cover primary success, fallback success, no route available, prompt-file, inline prompt, and stdin cases.

## Implement error catalog

### ID
error-catalog

### Parent
epic-v01

### Dependencies
- envelope-rendering
- route-verb
- doctor-verb
- config-validation

### Type
task

### Priority
P1

### Labels
v0-1, errors, testing

### Description
Agents need stable error and warning codes, complete retry metadata, and copy-pasteable corrections for every v0.1 failure or warning path.

### Design
Implement typed constructors for every `E_*` and `W_*` code, export code constants for library consumers, add the `did_you_mean` engine for verbs and flags, and implement human stderr rendering. Redirect removed spellings: `infer explain <task>` to `infer route <task> --explain`, and `infer capabilities <model>` to `infer model <model>`.

### Acceptance
- `infer doctr --json` emits `E_UNKNOWN_VERB` with `did_you_mean: "infer doctor"`.
- `infer route --json` emits `E_MISSING_ARG`.
- `E_VERB_RENAMED` is emitted for the old `explain` and per-model `capabilities` forms with v0.1 runnable replacements.
- Every v0.1-status error and warning code has at least one test path; `E_CONFIG_WRITE_FAILED` is reserved and not emitted.
- Human error rendering includes the message, optional `try:` line, exit-code name, and retryable boolean.

## Add demo scripts

### ID
demo-scripts

### Parent
epic-v01

### Dependencies
- doctor-verb
- route-verb
- testserver-fixtures

### Type
task

### Priority
P2

### Labels
v0-1, examples, testing

### Description
The three v0.1 demos should be runnable repo artifacts and double as integration tests against deterministic fixtures.

### Design
Create `examples/demo-1-install-moment.sh`, `examples/demo-2-route-explained.sh`, and `examples/demo-3-agent-loop.sh`. Each script boots or targets an `internal/testserver` fixture, invokes the real `infer` binary with `--json`, and asserts structural JSON properties with `jq`. Human text snapshots may exist for readability but must not fail builds on formatting changes.

### Acceptance
- All three demo scripts run green locally against fixture servers.
- Demo 1 validates doctor summary and a present recommended action.
- Demo 2 validates fallback route behavior and unavailable primary candidate.
- Demo 3 extracts a recommended follow-up command from doctor JSON, runs it, and verifies success.
- Scripts never depend on real local Ollama, llama.cpp, or OpenAI-compatible services.

## Add CI and release config

### ID
ci-release-config

### Parent
epic-v01

### Dependencies
- error-catalog
- demo-scripts

### Type
task

### Priority
P1

### Labels
v0-1, ci, release

### Description
The repo needs automated verification of tests, golden contracts, demos, and release builds before a v0.1 release candidate can be trusted.

### Design
Add `.github/workflows/ci.yml` running `go vet`, `go test ./...`, `go build`, and demo scripts. Add golden enforcement tests for each verb's JSON data and document the regeneration flow using `INFERCTL_UPDATE_GOLDEN=1`. Add `.goreleaser.yaml` for `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`; Windows is deferred. Embed build commit, date, and Go version through ldflags.

### Acceptance
- CI runs on pushes and verifies vet, tests, build, golden diffs, and demos.
- Each verb has a JSON golden under the repo testdata/contract structure.
- Golden update flow is documented and requires reviewable diffs.
- `goreleaser release --snapshot --clean` succeeds locally and produces four platform tarballs in `dist/`.
- No CI path makes outbound network calls except tests explicitly covering `infer version --check` with controlled behavior.

## Generate user docs

### ID
user-docs

### Parent
epic-v01

### Dependencies
- ci-release-config

### Type
task

### Priority
P2

### Labels
v0-1, docs

### Description
The repository should be understandable from README and generated docs without requiring a reviewer to inspect planning documents outside the repo.

### Design
Rewrite README for private v0.1 evaluation, install placeholders, quickstart, and design-doc pointers. Generate `docs/errors.md` from `internal/errors/catalog.go` and `docs/verbs.md` from Cobra metadata plus schema data. Draft the `CHANGELOG.md` `[0.1.0]` section with features, known limits, and deferred v0.2/v0.5 items.

### Acceptance
- README explains what inferctl is, how to try it privately, licensing status, and the supported backend scope.
- `docs/errors.md` lists every active and reserved code with meaning, exit code, retryability, details, and examples where applicable.
- `docs/verbs.md` lists every v0.1 command form, flags, JSON data schema summary, exit codes, and examples.
- Generated docs are reproducible through `go generate`.
- CHANGELOG has a drafted `[0.1.0]` section.

## Prepare release candidate

### ID
release-candidate

### Parent
epic-v01

### Dependencies
- user-docs

### Type
task

### Priority
P2

### Labels
v0-1, release

### Description
The private repository should be ready to cut a v0.1.0 release candidate as soon as external publication gates permit, without creating public-release side effects during implementation.

### Design
Create a private `v0.1.0-rc.1` tag when all repo artifacts are complete, run the release process in skip-publish or snapshot mode, smoke-test the resulting binary, and write repo-local release handoff notes or `RELEASING.md` instructions sufficient for a future thread to publish after external gates clear.

### Acceptance
- `v0.1.0-rc.1` can be tagged privately from the completed repo state.
- `goreleaser release --skip-publish` or equivalent dry-run succeeds against the tag.
- At least one produced binary is smoke-tested outside the normal build path.
- `RELEASING.md` or equivalent repo-local handoff documents the exact command sequence to publish once external gates clear.
- No public release, public visibility flip, or external outreach is performed as part of this issue.
