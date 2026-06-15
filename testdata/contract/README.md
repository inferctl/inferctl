# Contract Goldens

This directory stores reviewable JSON examples for every v0.1 verb data shape.
The capabilities data golden is enforced byte-for-byte against
`inferctl capabilities --json | jq .data` by `scripts/check-contract-goldens.sh`.

To refresh generated goldens:

```sh
INFERCTL_UPDATE_GOLDEN=1 scripts/update-contract-goldens.sh
scripts/check-contract-goldens.sh
git diff -- internal/contract/capabilities.golden.json testdata/contract
```

Review the diff before committing. Human-rendered output is intentionally not
goldened; only JSON contract data belongs here.
