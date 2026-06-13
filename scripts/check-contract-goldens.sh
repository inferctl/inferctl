#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP"
}
trap cleanup EXIT

cd "$ROOT"
go build -o "$TMP/infer" ./cmd/infer

"$TMP/infer" capabilities --json | jq .data >"$TMP/capabilities.golden.json"
tr -d '\r' <internal/contract/capabilities.golden.json >"$TMP/capabilities.expected.json"
tr -d '\r' <"$TMP/capabilities.golden.json" >"$TMP/capabilities.actual.json"
diff -u "$TMP/capabilities.expected.json" "$TMP/capabilities.actual.json"

find testdata/contract -name '*.golden.json' -print0 | sort -z | while IFS= read -r -d '' file; do
  jq empty "$file"
done

go test ./internal/contract -run 'TestVerbGoldens'
