# Installing inferctl

inferctl is private-evaluation software. Public installation is source-built
with `go install`; this project does not currently publish release binaries,
archives, installers, Homebrew formulae, or Scoop manifests for any platform.

## Public Go Install

After the repo is public, install from the module path on macOS, Linux, or
Windows:

```sh
go install github.com/inferctl/inferctl/cmd/inferctl@latest
inferctl version --json | jq .data.tool_version
```

## Local Source Builds

Use a local checkout build for private evaluation:

```sh
go build -o ./bin/inferctl ./cmd/inferctl
./bin/inferctl version --json | jq .data.tool_version
```

Expected result: local source builds normally report `tool_version: "dev"`.

## Private Tagged `go install` Validation

When validating a private tag, keep the module fetch scoped to private GitHub access instead of the public module proxy:

```sh
export GOPRIVATE=github.com/inferctl/*
export GONOSUMDB=github.com/inferctl/*
go install github.com/inferctl/inferctl/cmd/inferctl@v0.2.1
inferctl version --json | jq .data.tool_version
```

Expected result: tagged private installs should report `tool_version: "0.2.1"`.

Before running that command, verify the shell can read the private repository through `gh auth status`, `.netrc`, or your configured Git credential helper.

## Windows

```powershell
go install github.com/inferctl/inferctl/cmd/inferctl@latest
inferctl version --json
```

No Windows installer, Scoop manifest, zip archive, or PATH mutation workflow is
promised. Install through the Go toolchain.

## Packaged Builds

There are no packaged builds at this stage. Public release artifacts, when
present, should be limited to source tags and repository metadata until a
separate distribution decision changes this posture.

The `examples/` scripts remain source-only checkout artifacts for v0.2 and are
not packaged.
