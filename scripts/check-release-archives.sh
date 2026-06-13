#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

required=(
  "README.md"
  "CHANGELOG.md"
  "LICENSE_PENDING.md"
  "docs/install.md"
  "docs/agent-guide.md"
  "docs/verbs.md"
  "docs/errors.md"
  "testdata/contract/README.md"
)

shopt -s nullglob
archives=(dist/inferctl_*.tar.gz dist/inferctl_*.zip)
if ((${#archives[@]} == 0)); then
  echo "no release archives found under dist/" >&2
  exit 1
fi

for archive in "${archives[@]}"; do
  case "$archive" in
    *.tar.gz) listing="$(tar -tzf "$archive")" ;;
    *.zip) listing="$(unzip -Z1 "$archive")" ;;
    *) echo "unsupported archive $archive" >&2; exit 1 ;;
  esac
  for path in "${required[@]}"; do
    if ! grep -Eq "(^|/)${path}$" <<<"$listing"; then
      echo "$archive missing $path" >&2
      exit 1
    fi
  done
  if grep -Eq "(^|/)examples/" <<<"$listing"; then
    echo "$archive unexpectedly contains source-only examples" >&2
    exit 1
  fi
done

echo "release archive contents ok"
