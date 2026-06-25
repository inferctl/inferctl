# Public-Readiness Decision Memo

This memo records the v0.2.2 public-readiness posture.

## Current Posture

- Repo/module: `github.com/inferctl/inferctl`.
- Command package: `cmd/inferctl`.
- Binary and shell command: `inferctl`.
- License status: Apache License 2.0 (`LICENSE` and `NOTICE`).
- Default install posture: Go toolchain install only.
- Remote CI posture: manual-only via `workflow_dispatch`.
- Archive posture: no release archives for any platform.
- Windows posture: Go toolchain install only; no installer or zip.
- Provider evidence: curated verified-run artifacts now cover Ollama,
  llama.cpp, generic `openai_compat`, LM Studio, and MLX local provider paths.

## Decisions Deferred On Purpose

- Confirm `go install github.com/inferctl/inferctl/cmd/inferctl@latest` works
  from a clean public shell before announcement.
- Whether announcement timing and any external legal/name gates are ready.
- Whether a lighter automatic CI workflow is justified before public launch.
- Whether recurring provider/model matrix artifacts should live in external
  public artifact storage for `inferctl.dev`.

## Non-Goals For v0.2.2

- No signing or notarization work.
- No Homebrew tap or formula work.
- No release archives or prebuilt binaries for any platform.
- No Windows installer reintroduction.

## Default Next-Step Recommendation

Keep v0.2.2 as a narrow public-readiness release:

- maintain changelog and docs consistency,
- keep `infer-testserver` as an explicitly documented internal helper name,
- validate the public `go install` path before announcement,
- preserve the verified-run artifacts as curated bootstrap evidence, not a
  recurring benchmark corpus,
- preserve manual-only hosted CI unless a concrete need appears,
- revisit packaged release mechanics only under a separate, explicit decision.
