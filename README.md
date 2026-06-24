# inferctl

Explain your local LLM stack.

`inferctl` is a private-evaluation CLI for inspecting local inference backends,
loaded models, and routing decisions. It is designed for agent use first:
every command has opt-in JSON envelopes via `--json`, stable error codes, and
copy-pasteable follow-up commands.

Licensed under Apache 2.0 (see LICENSE and NOTICE). Still in private
evaluation ahead of public launch.

## What It Does

- Diagnoses backend health with `inferctl doctor`.
- Lists configured backends and models without running inference.
- Explains route selection with `inferctl route <task>`.
- Shows, validates, and explains the v0.1 config format.
- Emits a machine-readable contract with `inferctl capabilities --json`.

v0.1 supports read-only adapters for Ollama, llama.cpp, and unauthenticated
OpenAI-compatible `/v1/models` servers. Remote authenticated backends, warmup,
release, lock management, and live inference execution are intentionally
deferred.

## Source-First Use

The current v0.2.1 work is a private technical cleanup release. Use a local
checkout build for day-to-day validation; public installation will be through
`go install`, not prebuilt binaries or archives.

Remote CI is intentionally manual-only at this stage. Use local verification
as the default loop, then trigger `.github/workflows/ci.yml` with
`workflow_dispatch` only when you specifically want a hosted re-run.

### Local Checkout Build

```sh
go test ./...
go build -o bin/inferctl ./cmd/inferctl
bin/inferctl version --json | jq .data.tool_version
bin/inferctl capabilities --json | jq .data.verbs
bin/inferctl config explain
```

Local checkout builds are expected to report `tool_version: "dev"` when no tag
or release ldflags are involved.

### Public Go Install

After publication, the intended install command for macOS, Linux, and Windows is:

```sh
go install github.com/inferctl/inferctl/cmd/inferctl@latest
```

### Private Tagged Install

When Dave decides to validate a private tag, use the private-module path:

```sh
export GOPRIVATE=github.com/inferctl/*
export GONOSUMDB=github.com/inferctl/*
go install github.com/inferctl/inferctl/cmd/inferctl@v0.2.1
inferctl version --json | jq .data.tool_version
```

That flow requires GitHub credentials that can read the private repo. Broad
public `go install ...@latest` guidance depends on the repo being ready for
public module proxy traffic.

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
v0.2.1.

## License

Apache License 2.0. See [LICENSE](LICENSE).

The inference-router idea that kicked off inferctl came out of tinkering with
Foxforge. inferctl itself is an independent Go implementation.
