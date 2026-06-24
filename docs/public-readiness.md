# Public-Readiness Decision Memo

This memo records the decisions intentionally deferred after the private
`v0.2.1` cleanup release.

## Current Posture

- Repo status: private-evaluation only.
- License status: no public license grant yet.
- Default install posture: Go toolchain install only.
- Remote CI posture: manual-only via `workflow_dispatch`.
- Archive posture: no release archives for any platform.
- Windows posture: Go toolchain install only; no installer or zip.

## Decisions Deferred On Purpose

- Confirm `go install github.com/inferctl/inferctl/cmd/inferctl@latest` works
  once the repo is public.
- Whether repo visibility, announcement, and legal gates are ready.
- Whether a lighter automatic CI workflow is justified before public launch.

## Non-Goals For v0.2.2

- No public distribution rollout.
- No signing or notarization work.
- No Homebrew tap or formula work.
- No release archives or prebuilt binaries for any platform.
- No Windows installer reintroduction.
- No license grant or repo visibility change.

## Default Next-Step Recommendation

Keep v0.2.2 as a narrow polish release:

- maintain changelog and docs consistency,
- keep `infer-testserver` as an explicitly documented internal helper name,
- validate the public `go install` path before announcement,
- preserve manual-only hosted CI unless a concrete need appears,
- revisit packaged release mechanics only under a separate, explicit decision.
