# Releasing inferctl

This repo is still private-evaluation only. Do not publish binaries, flip repo
visibility, or announce a release until the external legal/name/publication gates
are cleared.

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
git tag v0.1.0-rc.1
go run github.com/goreleaser/goreleaser/v2@latest release --clean --skip=publish
./dist/infer_darwin_arm64_v8.0/infer version --json | jq '.data.tool_version'
```

If validating on a different host architecture, smoke-test the matching binary
under `dist/infer_<os>_<arch>*/infer`.

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
git tag -a v0.1.0 -m "inferctl v0.1.0"
git push origin main
git push origin v0.1.0
goreleaser release --clean
```

The `goreleaser release --clean` command is the first command in this document
that can publish release artifacts. Run it only after the external gates clear.

## Rollback

For a local-only RC tag:

```sh
git tag -d v0.1.0-rc.1
rm -rf dist/
```

For a pushed public tag, delete only after deciding how to communicate the
replacement release:

```sh
git push origin :refs/tags/v0.1.0
git tag -d v0.1.0
```
