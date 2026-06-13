## Ship inferctl v0.2 release

### ID
v02-epic

### Type
epic

### Priority
P1

### Labels
v0-2, release

### Description
inferctl v0.2 needs a dependency-aware implementation track that preserves v0.1 compatibility while adding Windows support, config mutation, discovery, TRIAGE, documentation, packaging, and release validation. The work is constrained by an additive contract: the v0.2 contract keeps `contract_version = "0.1"` and treats the v0.2 contract docs as the implementation source once frozen.

### Acceptance
- The v0.2 contract artifact set is stable and internally consistent.
- Capabilities indexes every shipped verb, schema, code, backend kind, and guide reference.
- The test suite proves v0.1 verb-shape compatibility.
- Windows support is present in CI and has passed the release stability gate.
- Config writes are atomic, validated, and round-trip safe.
- Discovery is local, deterministic, protocol-verified, and proven on at least three backend kinds.
- TRIAGE provides deterministic next actions without relying on future verbs.
- v0.2 release packaging, docs, examples decision, and CHANGELOG entries are complete.

## Freeze v0.2 contracts

### ID
phase-0-contract-freeze

### Parent
v02-epic

### Type
task

### Priority
P1

### Labels
v0-2, contracts

### Description
Implementation work needs a stable contract baseline before test and golden updates can be trusted. The v0.2 planning set currently depends on a draft contract bundle and must be frozen before downstream phases can declare completion.

### Design
- Mark the v0.2 contract docs as stable for implementation.
- Verify that `infer version --check` exists in current source and in the capabilities golden.
- If `infer version --check` is missing from source or the capabilities golden, open or update the Phase 1 work to implement or restore it.
- Check the v0.2 planning directory for the literal section-symbol character and remove it from planning docs if present.

### Acceptance
- Contract docs are marked stable for implementation.
- `infer version --check` is confirmed in source and the capabilities golden, or the Phase 1 issue explicitly owns the missing implementation/golden work.
- The v0.2 planning directory contains no section-symbol character.

## Align v0.1 contract carry-forward

### ID
phase-1-contract-alignment

### Parent
v02-epic

### Dependencies
- phase-0-contract-freeze

### Type
task

### Priority
P1

### Labels
v0-2, contracts

### Description
v0.2 needs to carry forward v0.1 runtime lessons so existing contract surfaces remain accurate and new work starts from canonical live behavior. Esiban findings exposed naming, recommendation, validation, enum, and render-name gaps that should be corrected before later error, TRIAGE, and guide examples depend on them.

### Design
- Treat live `selected_*`, `provenance`, and `backends.<name>.default` names as canonical in tests and docs.
- Fix `doctor.recommended_action` so it does not reference `infer warmup`.
- Deduplicate `recommended_action.alternatives`.
- Make `config validate` return `findings: []` on clean config.
- Add useful `did_you_mean` payloads for enum `E_INVALID_ARG` and `E_CONFIG_VALIDATION_FAILED`.
- Make `infer config valid --json` return an error or redirect instead of namespace help with exit `0`.
- Complete render names for exit codes `2` and `5`.
- Keep `infer version --check`; implement or restore it only if Phase 0 finds the current source or capabilities golden lacks it.

### Acceptance
- Phase 0 acceptance is green.
- Existing v0.1 goldens remain unchanged except where this issue explicitly requires a v0.2 correction.
- Regression tests cover every Esiban finding carried into v0.2.
- `config_validate.clean.golden.json` exists and asserts clean config emits `findings: []`.
- `doctor.recommended_action.no_future_verbs.golden.json` exists and asserts recommendations do not point at unavailable future verbs.

## Add Windows path and CI foundation

### ID
phase-2-windows-foundation

### Parent
v02-epic

### Dependencies
- phase-0-contract-freeze

### Type
task

### Priority
P1

### Labels
v0-2, windows

### Description
Windows support is a v0.2 commitment, and later config writes, packaging, and release validation depend on deterministic cross-platform path behavior. The project needs CI, fixtures, path normalization, and filesystem error contracts before write-path features build on top of them.

### Design
- Add a Windows CI runner.
- Audit config path handling, including `%APPDATA%`.
- Add Windows test fixtures for config path resolution.
- Normalize platform-specific paths in deterministic contract goldens to placeholders such as `<config_path>`, `<home>`, and `<tmp>`.
- Implement same-volume temp-file replacement helpers for future config writes.
- Define filesystem error normalization for POSIX and Windows using `os_error` and `os_error_code`, not POSIX-only `errno`.
- Add a CI check that every code in the capabilities golden has a corresponding docs entry and every docs entry exists in the golden.

### Acceptance
- Phase 0 acceptance is green.
- Contract tests pass on macOS, Linux, and Windows.
- Windows CI is green for at least 10 consecutive main-branch runs with zero re-runs before v0.2 release.
- Config path resolution tests cover env override, repo-local config, XDG fallback, home fallback, and Windows appdata fallback.
- Path-bearing goldens are stable across POSIX and Windows.

## Implement config write commands

### ID
phase-3-config-writes

### Parent
v02-epic

### Dependencies
- phase-1-contract-alignment
- phase-2-windows-foundation

### Type
feature

### Priority
P1

### Labels
v0-2, config

### Description
inferctl v0.2 needs safe config mutation so discovery, setup workflows, TRIAGE recommendations, and agent-guide examples can move users from detected state to working configuration. Config writes must preserve human-maintained TOML structure, avoid partial writes, redact secrets, and keep schema-version behavior compatible.

### Design
- Add TOML round-trip editing with `github.com/pelletier/go-toml/v2`.
- Implement `infer config init`.
- Implement `infer config set <key> <value>`.
- Implement `infer config patch <toml-fragment>`.
- Add `--from-stdin` to `infer config patch`.
- Add specified dry-run support.
- Add redaction for auth and secret-looking keys.
- Activate config write errors, including `E_CONFIG_PATCH_DELETE_UNSUPPORTED`.
- Preserve `"0.1"` schema files when edits use only v0.1 keys; bump to `"0.2"` only when a v0.2-only key is introduced.

### Acceptance
- Phase 1 and Phase 2 write-path acceptances are green.
- Comments and ordering are preserved in representative fixture edits.
- Invalid edits do not write partial files.
- Windows and POSIX write tests pass.
- Secret values do not appear in JSON, logs, diffs, or goldens.
- `config_init.print.golden.json`, `config_set.change.golden.json`, and `config_patch.stdin.golden.json` exist.

## Add local backend adapters

### ID
phase-4a-local-backends

### Parent
v02-epic

### Dependencies
- phase-1-contract-alignment

### Type
feature

### Priority
P1

### Labels
v0-2, backends

### Description
v0.2 needs a broader local backend baseline so discovery and routing can support five backend kinds instead of only the v0.1 set. LM Studio and MLX must be represented as first-class backend kinds with shared model schemas and identity probes.

### Design
- Implement the LM Studio adapter.
- Implement the MLX server adapter.
- Expand the backend kind enum and capabilities manifest.
- Add identity probes for LM Studio and MLX sufficient for discovery.

### Acceptance
- Phase 1 acceptance is green.
- Ollama, llama.cpp, openai_compat, LM Studio, and MLX appear in capabilities.
- LM Studio and MLX adapters expose models through the same shared schema types as v0.1 backends.
- Discovery can verify LM Studio and MLX without relying on `openai_compat` auth or remote support.

## Support openai_compat auth and remote URLs

### ID
phase-4b-openai-compat-auth

### Parent
v02-epic

### Dependencies
- phase-1-contract-alignment
- phase-2-windows-foundation

### Type
feature

### Priority
P1

### Labels
v0-2, backends

### Description
openai_compat auth and remote backend support are v0.2 baseline scope. Backend-reading verbs need explicit auth-failure and remote-opt-in behavior, and every auth-aware surface must enforce secret redaction.

### Design
- Complete `openai_compat` auth header support.
- Complete remote URL support with explicit `remote_allowed`.
- Add `E_BACKEND_AUTH_FAILED` and `E_BACKEND_REMOTE_NOT_ALLOWED` paths across every backend-reading verb.
- Add secret-redaction tests across all auth-aware outputs.

### Acceptance
- Phase 1 and Phase 2 redaction acceptances are green.
- `doctor`, `backends`, `models`, `model`, `route`, and `discover` have tests for `E_BACKEND_AUTH_FAILED` where applicable.
- `doctor`, `backends`, `models`, `model`, `route`, and `discover` have tests for `E_BACKEND_REMOTE_NOT_ALLOWED` where applicable.
- CI fails if a configured secret appears unredacted in any envelope, log line, golden, warning, or error.
- Remote endpoints require explicit opt-in.

## Implement discovery v1

### ID
phase-5-discovery-v1

### Parent
v02-epic

### Dependencies
- phase-3-config-writes
- phase-4a-local-backends

### Type
feature

### Priority
P1

### Labels
v0-2, discovery

### Description
inferctl needs local, deterministic discovery so agents and users can find supported local backends and convert verified candidates into safe config changes. Discovery must avoid broad host inspection and produce patchable output only when identity is verified.

### Design
- Implement fixed localhost port scanning.
- Verify protocol identity per backend kind.
- Emit `DiscoveryCandidate` records.
- Emit TOML patch suggestions.
- Add `--format toml` for patchable output.
- Add `--deliver file:<path>` where useful.
- Add partial and ambiguous discovery warnings.

### Acceptance
- Phase 3 and Phase 4a acceptances are green.
- Discovery is proven on at least three backend kinds.
- No process-table or subnet scanning is used.
- Ambiguous ports do not produce unsafe config patches.
- `discover.empty.golden.json` and `discover.ollama.golden.json` exist.

## Implement deterministic triage

### ID
phase-6-triage

### Parent
v02-epic

### Dependencies
- phase-1-contract-alignment
- phase-5-discovery-v1

### Type
feature

### Priority
P1

### Labels
v0-2, triage

### Description
inferctl needs a TRIAGE verb that gives agents deterministic next actions from existing diagnostic and config-validation outputs. v0.2 triage may consume discovery output supplied by the caller, but it must not run discovery inline.

### Design
- Implement `infer triage`.
- Run `doctor` and config validation as triage inputs.
- Accept prior JSON envelopes through `--input-file`.
- Do not invoke `infer discover` inline in v0.2.
- Rank by severity, code, then subject.
- Emit `TriageItem` records.
- Deduplicate command alternatives.
- Add focused filters by backend, severity, and limit.

### Acceptance
- Phase 1 and Phase 5 acceptances are green.
- Clean machine returns zero triage items.
- Broken config ranks config errors above backend warnings.
- Ranking is deterministic across repeated runs in deterministic mode.
- `triage.clean.golden.json` and `triage.errors.golden.json` exist.

## Add composability and delivery

### ID
phase-7-composability-delivery

### Parent
v02-epic

### Dependencies
- phase-3-config-writes
- phase-4a-local-backends
- phase-4b-openai-compat-auth
- phase-5-discovery-v1
- phase-6-triage

### Type
feature

### Priority
P1

### Labels
v0-2, delivery

### Description
v0.2 workflows need composable command output and artifact delivery without breaking JSON consumers. Delivery metadata must be structured predictably, and commands must continue to surface warnings and errors even when artifacts are written elsewhere.

### Design
- Add `--deliver` to selected verbs.
- Put structured delivery metadata only in `data.delivery`.
- Ensure JSON mode always emits envelopes even when delivery writes artifacts.
- Add artifact paths to payloads.
- Add pipeline examples to tests.

### Acceptance
- Phases 3 through 6 are green, including both backend phases.
- `infer discover --format toml | infer config patch --from-stdin` works.
- Delivery never hides errors or warnings from JSON consumers.
- `commands[]` may suggest follow-up commands, but tests assert that delivery metadata lives in `data.delivery`.

## Write agent guide and examples

### ID
phase-8a-agent-guide

### Parent
v02-epic

### Dependencies
- phase-7-composability-delivery

### Type
docs

### Priority
P1

### Labels
v0-2, docs

### Description
v0.2 needs an agent-facing guide that ties setup, config mutation, discovery, triage, routing, auth, and model-family quirks into reliable workflows. The guide must be discoverable from capabilities and must clearly explain the examples packaging decision.

### Design
- Write `docs/agent-guide.md`.
- Reference `docs/agent-guide.md` from capabilities.
- Cover setup, config mutation, discovery, route-to-backend loops, triage, auth, and model-family quirks.
- Decide whether examples are source-only or release-package artifacts.
- If examples stay source-only, document that clearly and keep source checkout scripts passing.
- If examples are packaged, include examples that do not require a Go toolchain on the target machine.

### Acceptance
- Phases 1 through 7 are green.
- Agent guide exists and is linked from capabilities.
- Agent guide describes triage/discovery composition explicitly: triage does not run discovery inline in v0.2.
- Auth sections reflect completed `openai_compat` auth and remote support behavior.
- Example packaging decision is reflected in docs and tests.

## Add packaging and distribution

### ID
phase-8b-packaging-distribution

### Parent
v02-epic

### Dependencies
- phase-2-windows-foundation
- phase-8a-agent-guide

### Type
task

### Priority
P1

### Labels
v0-2, packaging

### Description
v0.2 release artifacts need distribution paths that match the documented docs and examples set, including Windows installation coverage. Tarball users need a postinstall PATH hint, and Scoop packaging must be smoke-tested.

### Design
- Add a postinstall PATH hint for tarball users.
- Add Scoop bucket packaging.
- Verify release archive contents against the examples decision.
- Ensure the Windows release path is covered.

### Acceptance
- Phase 2 and Phase 8a acceptances are green.
- Release archives contain the documented docs/examples set.
- Scoop install path works in a Windows smoke test.

## Validate v0.2 release

### ID
phase-9-release-validation

### Parent
v02-epic

### Dependencies
- phase-8b-packaging-distribution

### Type
task

### Priority
P1

### Labels
v0-2, release

### Description
Before v0.2 ships, the project needs a full validation pass across tests, packaging, runtime conformance, Windows smoke coverage, documentation alignment, and release notes. This issue is the final release gate after all implementation phases are complete.

### Design
- Run the full unit and contract test suite on macOS, Linux, and Windows.
- Run snapshot release packaging.
- Re-run Esiban-style Linux runtime conformance.
- Run at least one Windows smoke test against a local or fixture backend.
- Verify capabilities, errors docs, verbs docs, and agent guide are aligned.
- Write `CHANGELOG.md` v0.2.0 entries for added verbs, flags, backend kinds, error codes, warning codes, config keys, Windows support, packaging changes, and the agent guide.

### Acceptance
- All prior phase acceptances are green.
- All v0.1 verb shapes remain compatible.
- New v0.2 verbs are golden-pinned.
- Windows CI stability gate has passed.
- Discovery works on at least three backend kinds.
- TRIAGE ranking is deterministic.
- Config writes are atomic and validated.
- `CHANGELOG.md` has a complete v0.2.0 section.
