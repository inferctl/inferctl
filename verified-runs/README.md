# Verified Runs

This directory stores a small number of curated, publishable provider validation runs for inferctl.

These artifacts are bootstrap evidence for the source-first public release and sample inputs for future `inferctl.dev` provider/model coverage pages. This directory is not intended to become the long-term storage layer for recurring scheduled verification.

## What Belongs Here

- Curated public sample runs tied to a specific inferctl commit.
- Redacted JSON outputs that prove provider workflow behavior.
- A `manifest.json` describing the run, commands, caveats, and redaction review.
- A `summary.md` that explains the result for humans.

## What Does Not Belong Here

- Raw unredacted captures.
- Routine scheduled run history.
- Private machine names, usernames, home paths, Tailscale details, LAN IPs, API keys, tokens, or private model provenance.
- Large artifact sets better suited for object storage.
- Model quality benchmark results.

## Required Review Before Adding A Run

Before committing a run, verify:

- All JSON artifacts parse.
- The artifact directory has a neutral public name.
- `scripts/check-public-readiness.sh` passes.
- A private-residue scan finds no private hostnames, user paths, network identity, or credentials.
- The run summary clearly states what was and was not validated.

Example scan:

```sh
rg -n -i 'hostname|tailnet|nodekey|/home/|/Users/|Bearer|api[_-]?key|secret|token' verified-runs/<run-id>
```

## Long-Term Plan

Recurring provider/model matrix artifacts should move to external public artifact storage, with `inferctl.dev` consuming generated index files. Keep this repo directory limited to a few intentionally curated examples.
