# CI Markdown Summary

Append inferctl's existing preflight Markdown to the CI summary surface your
runner already provides:

```sh
inferctl preflight code_review \
  --prompt-file sample-pr-review-prompt.txt \
  --format markdown
```

Or use the thin wrapper from this directory:

```sh
./run-preflight-markdown.sh sample-pr-review-prompt.txt
```

This example packages product output. It does not render Markdown, define a
Markdown schema, ship a template, call a provider API, or own pull request
comment behavior. Markdown layout and redaction are preflight product behavior;
this example consumes stdout exactly as CI would.

## GitHub Actions

Use the step summary file that GitHub Actions already exposes:

```yaml
jobs:
  local-inference-preflight:
    runs-on: self-hosted
    steps:
      - uses: actions/checkout@v4
      - name: Append inferctl preflight summary
        run: |
          set -euo pipefail
          inferctl preflight code_review \
            --prompt-file examples/ci-markdown-summary/sample-pr-review-prompt.txt \
            --format markdown >> "$GITHUB_STEP_SUMMARY"
```

This requires no pull request write permission because it does not post or edit
comments.

## Buildkite Or Generic Shell

Use the same command and let your CI system decide how to publish the file:

```sh
./buildkite-command.sh
```

The script writes `inferctl-preflight.md` by default. Uploading that file as an
annotation or artifact is CI-owned behavior, not inferctl-owned behavior.

Generated samples and regression checks must come from real
`inferctl preflight --format markdown` output or checked fixture output. If the
preflight formatter changes, update this example from command output rather than
editing an independent Markdown contract.
