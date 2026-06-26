# inferctl

Explain your local LLM stack.

`inferctl` is a CLI for inspecting local inference backends, loaded models, and
routing decisions. It is designed for agent use first: every command has opt-in
JSON envelopes via `--json`, stable error codes, and copy-pasteable follow-up
commands.

Licensed under Apache 2.0 (see LICENSE and NOTICE).

![inferctl terminal demo](docs/img/inferctl-demo.gif)

## What It Does

- Diagnoses backend health with `inferctl doctor`.
- Lists configured backends and models without running inference.
- Explains route selection with `inferctl route <task>`.
- Shows, validates, and explains the v0.1 config format.
- Emits a machine-readable contract with `inferctl capabilities --json`.

v0.2.2 supports read-only adapters for Ollama, llama.cpp, generic
OpenAI-compatible `/v1/models` servers, LM Studio, and MLX. Remote
authenticated `openai_compat` configuration is supported, but warmup, release,
lock management, latency collection, and live inference execution are
intentionally deferred.

## Verified Provider Runs

The `verified-runs/` directory stores curated, redacted provider workflow
captures. These are bootstrap evidence for local provider discovery, config
validation, diagnosis, model listing, route explanation, and triage behavior;
they are not model quality benchmarks.

Current curated provider coverage:

| Provider path | Environment | Model | Result |
| --- | --- | --- | --- |
| Ollama | Linux localhost | `qwen3:8b` | Pass with `W_MODEL_NOT_LOADED` caveat |
| llama.cpp | Linux localhost | `qwen2.5-0.5b-instruct-q4_k_m` | Pass |
| `openai_compat` | Linux localhost | `qwen2.5-0.5b-instruct-q4_k_m-openai-compat` | Pass with expected loaded-model caveat |
| LM Studio | Linux localhost, headless daemon | `qwen2.5-0.5b-instruct-q8_0-lmstudio` | Pass |
| MLX | macOS arm64 localhost | `mlx-community/Qwen2.5-0.5B-Instruct-4bit` | Pass |

See [verified-runs/README.md](verified-runs/README.md) for the artifact index,
redaction policy, and per-run summaries.

## Install

Install from the public module path on macOS, Linux, or Windows:

```sh
go install github.com/inferctl/inferctl/cmd/inferctl@latest
inferctl version --json | jq .data.tool_version
```

No release binaries, Homebrew formula, Scoop manifest, installer, or archive
builds are published for v0.2.2.

### Local Checkout Build

Use a local checkout for development and validation:

```sh
go test ./...
go build -o bin/inferctl ./cmd/inferctl
bin/inferctl version --json | jq .data.tool_version
bin/inferctl capabilities --json | jq .data.verbs
bin/inferctl config explain
```

Local checkout builds are expected to report `tool_version: "dev"` when no tag
or release ldflags are involved.

Remote CI is intentionally manual-only at this stage. Use local verification
as the default loop, then trigger `.github/workflows/ci.yml` with
`workflow_dispatch` only when you specifically want a hosted re-run.

The demo scripts run against deterministic fixture servers and do not require
local Ollama or llama.cpp:

```sh
examples/demo-1-install-moment.sh
examples/demo-2-route-explained.sh
examples/demo-3-agent-loop.sh
```

The fixture helper intentionally keeps the internal legacy name
`cmd/infer-testserver`. The product rename applies to the user-facing CLI
binary, not to this repo-local test utility.

## Config

Create a TOML config from:

```sh
inferctl config explain
```

Then point the CLI at it:

```sh
INFERCTL_CONFIG=/path/to/config.toml inferctl doctor --json
```

The minimum useful config defines `[meta]`, `[profile]`, at least one
`[backends.<name>]`, and any `[routing.<task>]` entries you want `inferctl route`
to resolve.

## Docs

- [Verbs](docs/verbs.md)
- [Errors](docs/errors.md)
- [Agent guide](docs/agent-guide.md)
- [Install](docs/install.md)
- [Public-readiness memo](docs/public-readiness.md)
- [Verified runs](verified-runs/README.md)
- [Lineage](docs/lineage.md)
- [Contract goldens](testdata/contract/README.md)

Regenerate generated docs with:

```sh
go generate ./internal/contract
```

## Release Checks

```sh
scripts/check-contract-goldens.sh
go test ./...
go vet ./...
go build ./...
```

Public release, name availability, and legal review are outside this repo's
implementation scope. Homebrew, signed binaries, direct-download installers,
release archives, and public binary builds are intentionally out of scope for
v0.2.2.

## License

Apache License 2.0. See [LICENSE](LICENSE).

The inference-router idea that kicked off inferctl came out of tinkering with
[Foxforge](https://github.com/GuideboardLabs/foxforge). inferctl itself is an
independent Go implementation.
