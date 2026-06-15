# Installing inferctl

inferctl is private-evaluation software. Do not publish or redistribute binaries until the external legal, naming, and publication gates are cleared.

## Release Archives

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
