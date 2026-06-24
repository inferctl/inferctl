# Releasing inferctl

v0.2.2 is a public-readiness release with Go-toolchain installation only. Do
not publish binaries, installers, Homebrew formulae, Scoop manifests, GoReleaser
artifacts, or release archives for this launch posture.

GitHub Actions verification is intentionally manual-only for this phase. Treat
local verification as the default release gate and use hosted Actions runs only
when a specific cross-platform check is worth the extra churn.

## RC Validation

Use this sequence to validate a local release candidate:

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
git tag v0.2.2-rc.1
```

If you push an RC tag, validate `go install` from a clean shell:

```sh
go install github.com/inferctl/inferctl/cmd/inferctl@v0.2.2-rc.1
inferctl version --json | jq .data.tool_version
```

No platform archive, installer, Homebrew formula, or Scoop manifest is a release
gate in this cycle.

The public install path is `go install github.com/inferctl/inferctl/cmd/inferctl@latest`.

The deterministic fixture helper remains `cmd/infer-testserver` by design in
this cycle. It is an internal repo utility, not part of the user-facing naming
surface.

## Publish Source Tag

When publishing v0.2.2, use a fresh reviewed commit and tag:

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
git tag -a v0.2.2 -m "inferctl v0.2.2"
git push origin main
git push origin v0.2.2
go install github.com/inferctl/inferctl/cmd/inferctl@v0.2.2
go install github.com/inferctl/inferctl/cmd/inferctl@latest
```

Do not run GoReleaser or upload release archives for this launch posture.
The public install path is Go toolchain only.

## Rollback

For a local-only RC tag:

```sh
git tag -d v0.2.2-rc.1
rm -rf dist/
```

For a pushed public tag, delete only after deciding how to communicate the
replacement release:

```sh
git push origin :refs/tags/v0.2.2
git tag -d v0.2.2
```
