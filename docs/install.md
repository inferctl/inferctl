# Installing inferctl

inferctl is installed from source with the Go toolchain. This project does not currently publish release binaries,
archives, installers, Homebrew formulae, or Scoop manifests for any platform.

## Public Go Install

Install from the module path on macOS, Linux, or Windows:

```sh
go install github.com/inferctl/inferctl/cmd/inferctl@latest
inferctl version --json | jq .data.tool_version
```

## Local Source Builds

Use a local checkout build for development and validation:

```sh
go build -o ./bin/inferctl ./cmd/inferctl
./bin/inferctl version --json | jq .data.tool_version
```

Expected result: local source builds normally report `tool_version: "dev"`.

## Tagged `go install` Validation

After pushing a public source tag, validate it from a clean shell:

```sh
go install github.com/inferctl/inferctl/cmd/inferctl@v0.2.2
inferctl version --json | jq .data.tool_version
```

Expected result: tagged installs should report `tool_version: "0.2.2"`.

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
