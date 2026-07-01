#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP"
}
trap cleanup EXIT

if [[ "$#" -eq 0 ]]; then
  echo "usage: $0 fixtures/*.snapshot.json" >&2
  exit 2
fi

cd "$ROOT"
go build -o "$TMP/inferctl" ./cmd/inferctl

for fixture in "$@"; do
  if [[ ! -f "$fixture" ]]; then
    echo "missing fixture: $fixture" >&2
    exit 1
  fi

  jq -e '
    .snapshot_schema_version != null and
    .contract_version != null and
    .inferctl_version != null and
    .captured_at_iso != null and
    .task != null and
    .prompt != null and
    .route_decision != null and
    (.route_candidates | type == "array") and
    (.backend_reachability | type == "array") and
    (.installed_models | type == "array") and
    (.loaded_models | type == "array") and
    (.warnings | type == "array") and
    (.errors | type == "array")
  ' "$fixture" >/dev/null

  "$TMP/inferctl" diff --before "$fixture" --after "$fixture" --json >"$TMP/self-diff.json"
  jq -e '.data.summary.total == 0' "$TMP/self-diff.json" >/dev/null

  jq -e '
    (.prompt.source_kind | IN("none", "inline", "file", "stdin")) and
    (.prompt.source | IN("none", "inline", "file", "stdin")) and
    (.prompt.content == null) and
    (.prompt.text == null) and
    (.prompt.prompt == null)
  ' "$fixture" >/dev/null

  if grep -E 'Authorization|Bearer|api[_-]?key|auth[_-]?header|http://(localhost|127\.|10\.|172\.(1[6-9]|2[0-9]|3[0-1])|192\.168\.)|https://(localhost|127\.|10\.|172\.(1[6-9]|2[0-9]|3[0-1])|192\.168\.)|/Users/|/home/|/private/var/|/tmp/' "$fixture" >/dev/null; then
    echo "fixture contains private endpoint, credential-looking text, or local path: $fixture" >&2
    exit 1
  fi
done

echo "agent drift snapshot fixtures ok"
