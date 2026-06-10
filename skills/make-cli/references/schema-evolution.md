# schema-evolution.md — Versioning the Agent Contract

A CLI's `capabilities --json` and per-verb output schemas are the agent contract. Once shipped, agents depend on them. This file gives the patterns for evolving the contract without breaking the ecosystem.

## Three classes of change

### Additive (always safe)

- New optional field in capabilities or in any output
- New verb
- New optional flag
- New exit code (only for new conditions; existing codes unchanged)
- New env var
- New entry in `features` array

Old agents ignore new fields. No deprecation needed. Minor `contract_version` bump optional.

### Renaming (deprecation needed)

- Renamed verb
- Renamed flag
- Renamed env var
- Renamed JSON output key
- Renamed exit code's semantic meaning

Both names work for ≥ 1 release. Old name emits a deprecation warning. Eventually removed in a major bump.

### Breaking (major version bump)

- Removed verb or flag
- Reused exit code for a different meaning
- Changed default behavior
- Changed output schema structure (moved fields, changed types)

Bump `contract_version` from `"1"` → `"2"`. Ship `--contract-version=N` compat flag for ≥ 6 months.

## `contract_version` semantics

Always present in `capabilities` AND every response's `meta.contract_version`:

| Change type | Bump |
|---|---|
| Additive | `1` → `1.1` (minor; optional) |
| Renaming, with deprecation | `1.1` → `1.2` (minor) |
| Breaking | `1` → `2` (major) |

Agents check major-version compatibility:

```python
caps = json.loads(subprocess.check_output(["<tool>", "capabilities", "--json"]))
major = int(caps["contract_version"].split(".")[0])
if major != 1:
    raise NotImplementedError(f"agent supports contract v1; tool ships v{major}")
```

Per-verb `schema_version` can evolve independently for additive changes to a single verb's output, without bumping tool-level `contract_version`. Surface both in `meta`:

```json
"meta": {
  "tool_version":     "0.5.0",
  "contract_version": "1",
  "schema_version":   "2"
}
```

## Capabilities pinning

Pin capabilities to a golden file in the regression suite:

```
regression_tests/
├── capabilities-golden.json
└── R-001__capabilities_contract.test.sh
```

The test:

```bash
got=$("$TOOL" capabilities --json | jq -S .)
want=$(cat regression_tests/capabilities-golden.json | jq -S .)
diff <(echo "$got") <(echo "$want") || {
  echo "REGRESSION: capabilities drifted; bump contract_version OR re-pin golden" >&2
  exit 1
}
```

When intentionally bumping `contract_version`:

1. Change the schema in source.
2. Bump `contract_version`.
3. Re-pin: `<tool> capabilities --json | jq -S . > capabilities-golden.json`.
4. Document the change in CHANGELOG.
5. Test re-passes.

This is the discipline that catches accidental drift while permitting intentional evolution.

## Per-verb output pinning

Per-verb output schemas pin similarly, with volatile fields stripped before comparison:

```bash
# regression_tests/SCHEMA-list.test.sh
got=$("$TOOL" list --json --limit=1 | jq 'del(.meta.request_id, .meta.ts_iso, .meta.elapsed_ms, .data.items[]._hash)')
want=$(cat regression_tests/list-golden.json | jq 'del(.meta.request_id, .meta.ts_iso, .meta.elapsed_ms, .data.items[]._hash)')
diff <(echo "$got" | jq -S .) <(echo "$want" | jq -S .)
```

Strip `request_id`, `ts_iso`, `elapsed_ms`, and any hash that varies per-run.

## The `--contract-version=N` compat flag

For widely-deployed CLIs, support old contract versions for ≥ 6 months after a major bump:

```bash
# Default: current contract version
$ <tool> list --json
{"meta": {"contract_version": "2"}, ...}

# Force old contract version
$ <tool> list --json --contract-version=1
{"meta": {"contract_version": "1"}, ...}
```

For capabilities:

```bash
$ <tool> capabilities --json --contract-version=1
# returns the v1 schema
```

After 6 months, emit a deprecation warning when the flag is used. After 12 months, remove the compat path.

Pre-1.0 tools may skip the compat flag — agents shouldn't depend on a pre-stable contract. Document this in CHANGELOG.

## Migration scripts

For non-trivial schema changes, ship an explicit migration tool:

```bash
$ <tool> migrate-output --from=1 --to=2 < old-output.json > new-output.json
```

Pipelines mid-migration:

```bash
<tool> list --json | <tool> migrate-output --from=1 --to=2 | downstream-consumer
```

Eventually the downstream consumer updates to v2 directly.

## CHANGELOG discipline

Every contract change gets a CHANGELOG entry. The CHANGELOG *is* the migration documentation.

```markdown
## v0.5.0

### Breaking changes (contract_version 1 → 2)

- The `list --json` output schema changed:
  - `data.items[].labels` (array of strings) renamed to `data.items[].tags`
  - `data.results` (deprecated alias for `data.items`) removed
- Migration: `sed -i 's/\.labels/.tags/g; s/\.results/.items/g' your-script.sh`
- Compat mode: `<tool> list --json --contract-version=1` emits old schema. Deprecated; removed in v0.7.0.

### Additive (contract_version 2 → 2.1)

- New field `data.items[].priority` (integer 0-9, default 5).
- New env var `<TOOL>_DEFAULT_PRIORITY`.
```

Without this, agents can't follow the bumps.

## Pre-1.0 discipline

Before v1.0, schema changes are expected. Use `contract_version: "0.x"` to signal pre-stability. Document in CHANGELOG that minor bumps may break before v1.0. Don't promise backward-compat across pre-1.0 minor versions.

Once v1.0 ships, the contract locks. Breaking changes require major bumps + compat flag.

## Old-version detection in clients

Agents read `contract_version` and adapt:

```python
caps = get_capabilities()
contract = caps["contract_version"].split(".")[0]

if contract == "1":
    use_v1_logic()
elif contract == "2":
    use_v2_logic()
    if "new_feature" in caps["features"]:
        use_new_feature()
else:
    raise NotImplementedError(f"unsupported contract {contract}")
```

This is what the schema-evolution machinery is *for* — letting agents detect and adapt without breaking.

## Anti-patterns

- **Silent schema changes.** Adding a required field without bumping version. Breaks agents.
- **Reusing exit codes.** "Exit 1 used to mean X; now means Y." Unknowable from outside without CHANGELOG diving.
- **Renaming without deprecation.** Old name silently dropped. Agents using old name fail in obscure ways.
- **Bumping major on additive changes.** Alert fatigue. Major bumps signal real breakage; over-bumping desensitizes.
- **Removing the compat flag too soon.** 6 months minimum. 12 months better.
- **Accumulating dead compat code forever.** After 12 months, remove. Don't carry multiple compat versions indefinitely.
- **CHANGELOG without migration instructions.** "Breaking change to schema" with no `sed`/`jq` recipe forces every consumer to derive their own.
- **Per-verb schema bumps without surfacing in `meta`.** Without `meta.schema_version`, agents can't detect a verb-local schema change inside an unchanged `contract_version`.

## Cross-references

- `envelope.md` — `contract_version` and `schema_version` in `meta`.
- `introspection.md` — capabilities is the contract being versioned.
- `errors.md` — exit code dictionary versions with the contract.
- `config.md` — config schema has its own `_schema_version`, evolved separately.
