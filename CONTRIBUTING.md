# Contributing to inferctl

External PRs are welcome. Please open an issue first for broad design or
behavior changes; the command contract is intentionally tight.

## Development

Requires Go 1.22+. No external services needed for unit/integration tests; a
local Ollama install is useful for adapter work but not required.

```sh
git clone https://github.com/inferctl/inferctl
cd inferctl
go test ./...
go build -o bin/inferctl ./cmd/inferctl
```

`bin/inferctl version --json` should report `tool_version: "dev"` on
checkout builds. See `README.md` for the full local workflow.

## Style

- `gofmt` and `go vet` must pass; CI enforces both.
- No new dependencies without discussion. Standard library first.
- Tests for new behavior. Adapter changes need recorded fixtures, not live
  network calls.
- JSON envelopes are a stable contract: do not break field shapes without a
  version bump and migration note.

## Commits

- Conventional-ish: `area: short summary` (e.g. `route: explain tie-break for
  equal scores`).
- Keep commits scoped; rebase noisy WIP locally before opening a PR.
- No `wip`, `fix`, or `lol` in landed commits.

## DCO Sign-off (required)

All commits must be signed off under the [Developer Certificate of Origin
1.1](https://developercertificate.org). Append the trailer with `git commit
-s`:

```
Signed-off-by: Your Name <you@example.com>
```

That's the contributor agreement. By signing off you confirm the contribution
is yours to give and may be released under Apache 2.0. No separate CLA.

## License

Contributions are licensed under Apache 2.0 (see `LICENSE` and `NOTICE`).

## Security

Do not file security issues publicly. See `SECURITY.md`.
