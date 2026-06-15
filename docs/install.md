# Installing inferctl

inferctl is private-evaluation software. Do not publish or redistribute binaries until the external legal, naming, and publication gates are cleared.

## Release Archives

Download the archive for your platform, unpack it, and place the extracted directory on `PATH`.

Linux and macOS archives are `tar.gz` files:

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

Windows archives are zip files. Extract the archive to a stable directory, then add that directory to the user PATH:

```powershell
Expand-Archive .\inferctl_VERSION_windows_amd64.zip -DestinationPath "$env:LOCALAPPDATA\inferctl" -Force
[Environment]::SetEnvironmentVariable("Path", "$env:LOCALAPPDATA\inferctl;$env:Path", "User")
inferctl version --json
```

Open a new terminal after changing PATH.

## Scoop

The GoReleaser config generates a Scoop manifest for the `infer` command. During private evaluation, the manifest is generated locally with upload disabled. A public bucket update must wait for the external release gates.

Expected smoke test:

```powershell
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean --skip=publish
.\scripts\smoke-scoop.ps1
```

The smoke test rewrites the generated manifest to point at the local Windows zip artifact before running `scoop install`.

## Packaged Files

Release archives contain:

- `infer` or `infer.exe`
- `README.md`
- `CHANGELOG.md`
- `LICENSE_PENDING.md`
- `docs/install.md`
- `docs/agent-guide.md`
- `docs/verbs.md`
- `docs/errors.md`
- `testdata/contract/README.md`

The `examples/` scripts remain source-only checkout artifacts for v0.2 and are intentionally not packaged in release archives.
