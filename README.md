# inferctl

Explain your local LLM stack.

`inferctl` is a private-evaluation CLI for inspecting local inference backends,
loaded models, and routing decisions. It is designed for agent use first:
every command has opt-in JSON envelopes via `--json`, stable error codes, and
copy-pasteable follow-up commands.

No license is granted yet; private evaluation only.

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

## Try It Privately

```sh
go test ./...
go build -o bin/inferctl ./cmd/inferctl
bin/inferctl capabilities --json | jq .data.verbs
bin/inferctl config explain
```

The demo scripts run against deterministic fixture servers and do not require
local Ollama or llama.cpp:

```sh
examples/demo-1-install-moment.sh
examples/demo-2-route-explained.sh
examples/demo-3-agent-loop.sh
```

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
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
```

Public release, name availability, and legal review are outside this repo's
implementation scope.

## License

Pending. No public distribution before resolution.
