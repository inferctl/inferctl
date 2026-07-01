# Team Requirements Verify Workflow

This example lets a repository declare the local inference state it expects and
fail fast when a teammate or CI runner has drifted.

```sh
python examples/team-requirements/verify-local-llm.py
```

The verifier is read-only. It runs:

```sh
inferctl config validate --json
inferctl status --json
inferctl preflight <task> --prompt-file <file> --json
```

It reports drift; it does not pull models, start daemons, load models, rewrite
config, or repair backend state.

## Requirements vs. Lockfiles

`inferctl.requirements.toml` is author intent: a hand-editable statement of what
the project needs. A future lockfile would be a generated snapshot of a known
local inference state. This example does not require `inferctl lock` or
`inferctl verify`; those can graduate later if the requirements pattern proves
useful.

## CI Sketch

```yaml
name: local-inference-preflight
on:
  workflow_dispatch:
jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - run: go install github.com/inferctl/inferctl/cmd/inferctl@latest
      - run: python examples/team-requirements/verify-local-llm.py
```

For hosted CI without real local model daemons, use deterministic fixture
backends in the same pattern as `examples/team-requirements/test.sh`.
