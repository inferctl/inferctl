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

Generated samples and regression checks must come from real
`inferctl preflight --format markdown` output or checked fixture output. If the
preflight formatter changes, update this example from command output rather than
editing an independent Markdown contract.
