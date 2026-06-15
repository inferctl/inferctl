#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP"
}
trap cleanup EXIT

cd "$ROOT"
go build -o "$TMP/inferctl" ./cmd/inferctl

"$TMP/inferctl" capabilities --json | jq .data >internal/contract/capabilities.golden.json
cp internal/contract/capabilities.golden.json testdata/contract/capabilities.golden.json

echo "Updated contract goldens. Review git diff before committing."
