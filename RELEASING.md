# Releasing inferctl

This repo is still private-evaluation only. v0.2.1 is a private technical
cleanup release, not the public launch. Do not publish binaries, flip repo
visibility, or announce a release until the external legal/name/publication
gates are cleared.

GitHub Actions verification is intentionally manual-only for this phase. Treat
local verification as the default release gate and use hosted Actions runs only
when a specific cross-platform check is worth the extra churn.

## Private RC Validation

Use this sequence to validate a local release candidate without public side
effects:

```sh
git status --short
go generate ./internal/contract
scripts/check-contract-goldens.sh
go test ./...
go vet ./...
go build ./...
examples/demo-1-install-moment.sh
examples/demo-2-route-explained.sh
examples/demo-3-agent-loop.sh
git tag v0.2.1-rc.1
```

After tagging, validate private `go install` from a clean shell when credentials
can read the private repo:

```sh
export GOPRIVATE=github.com/inferctl/*
export GONOSUMDB=github.com/inferctl/*
go install github.com/inferctl/inferctl/cmd/inferctl@v0.2.1-rc.1
inferctl version --json | jq .data.tool_version
```

No platform archive, installer, Homebrew formula, or Scoop manifest is a release
gate in this cycle.

Do not publish broad public `go install github.com/inferctl/inferctl/cmd/inferctl@latest`
instructions until the repo is ready for public module proxy traffic.

The deterministic fixture helper remains `cmd/infer-testserver` by design in
this cycle. It is an internal repo utility, not part of the user-facing naming
surface.

## Publish After External Gates Clear

After the external gates clear, use a fresh reviewed commit and tag:

```sh
git status --short
go generate ./internal/contract
scripts/check-contract-goldens.sh
go test ./...
go vet ./...
go build ./...
examples/demo-1-install-moment.sh
examples/demo-2-route-explained.sh
examples/demo-3-agent-loop.sh
git tag -a v0.2.1 -m "inferctl v0.2.1"
git push origin main
git push origin v0.2.1
```

Do not run GoReleaser or upload release archives for this launch posture.
The public install path is Go toolchain only.

## Rollback

For a local-only RC tag:

```sh
git tag -d v0.2.1-rc.1
rm -rf dist/
```

For a pushed public tag, delete only after deciding how to communicate the
replacement release:

```sh
git push origin :refs/tags/v0.2.1
git tag -d v0.2.1
```
