# Installing inferctl

inferctl is private-evaluation software. Do not publish or redistribute binaries until the external legal, naming, and publication gates are cleared.

## Local Source Builds

Use a local checkout build for normal private evaluation:

```sh
go build -o ./bin/inferctl ./cmd/inferctl
./bin/inferctl version --json | jq .data.tool_version
```

Expected result: local source builds normally report `tool_version: "dev"`.

## Private Tagged `go install` Validation

When validating a private tag, keep the module fetch scoped to private GitHub access instead of the public module proxy:

```sh
export GOPRIVATE=github.com/Ozhiaki/*
export GONOSUMDB=github.com/Ozhiaki/*
go install github.com/Ozhiaki/inferctl/cmd/inferctl@v0.2.1
inferctl version --json | jq .data.tool_version
```

Expected result: tagged private installs should report `tool_version: "0.2.1"`.

Before running that command, verify the shell can read the private repository through `gh auth status`, `.netrc`, or your configured Git credential helper.

Broad public `go install ...@latest` instructions are intentionally deferred until the final public module-path decision is made.

## Private Archive Validation

Download the archive for your platform, unpack it, and place the extracted directory on `PATH`.

Linux and macOS archives are currently the supported release-archive targets:

```sh
mkdir -p "$HOME/.local/inferctl"
tar -xzf inferctl_VERSION_linux_amd64.tar.gz -C "$HOME/.local/inferctl"
export PATH="$HOME/.local/inferctl:$PATH"
inferctl version --json
```

For a persistent shell setup, add the PATH line to your shell profile:

```sh
printf '\nexport PATH="$HOME/.local/inferctl:$PATH"\n' >> "$HOME/.zshrc"
```

## Windows Source Builds

Windows support is best-effort and source-first in v0.2.1. Build the CLI locally rather than depending on a packaged installer:

```powershell
go build -o .\bin\inferctl.exe .\cmd\inferctl
.\bin\inferctl.exe version --json
```

No Windows installer, Scoop manifest, or PATH mutation workflow is promised in this release.

## Packaged Files

Release archives for the supported archive targets contain:

- `inferctl`
- `README.md`
- `CHANGELOG.md`
- `LICENSE_PENDING.md`
- `docs/install.md`
- `docs/agent-guide.md`
- `docs/verbs.md`
- `docs/errors.md`
- `testdata/contract/README.md`

The `examples/` scripts remain source-only checkout artifacts for v0.2 and are intentionally not packaged in release archives.
