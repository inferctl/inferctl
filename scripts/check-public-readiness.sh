#!/usr/bin/env bash
set -euo pipefail

failures=0
grep_out=$(mktemp "${TMPDIR:-/tmp}/inferctl-public-readiness-grep.XXXXXX")
beads_out=$(mktemp "${TMPDIR:-/tmp}/inferctl-public-readiness-beads.XXXXXX")
trap 'rm -f "$grep_out" "$beads_out"' EXIT

section() {
  printf '\n== %s ==\n' "$1"
}

fail() {
  failures=$((failures + 1))
  printf 'FAIL: %s\n' "$1" >&2
}

require_clean_grep() {
  local label=$1
  shift

  if git grep -n -E "$@" >"$grep_out"; then
    fail "$label"
    cat "$grep_out" >&2
  else
    printf 'ok: %s\n' "$label"
  fi
}

section "tracked artifacts"

if git ls-files 'dist/**' 'bin/**' '.doctor/**' '.beads/imports/**' '.beads/.br_history/**' '.beads/.br_recovery/**' | grep -q .; then
  fail "generated or local runtime artifacts are tracked"
  git ls-files 'dist/**' 'bin/**' '.doctor/**' '.beads/imports/**' '.beads/.br_history/**' '.beads/.br_recovery/**' >&2
else
  printf 'ok: generated and local runtime artifacts are untracked\n'
fi

section "public naming and install posture"

if git grep -n 'go install github.com/inferctl/inferctl/cmd/inferctl@latest' -- README.md docs/install.md docs/agent-guide.md >/dev/null; then
  printf 'ok: public go install path is documented\n'
else
  fail "public go install path is missing from primary docs"
fi

for removed in .goreleaser.yaml .goreleaser.yml goreleaser.yaml goreleaser.yml; do
  if git ls-files -- "$removed" | grep -q .; then
    fail "GoReleaser config is tracked: $removed"
  fi
done

section "private residue"

require_clean_grep "private module and launch-gate residue" \
  'GOPRIVATE|GONOSUMDB|LICENSE_PENDING|github\.com/Ozhiaki|Makakoons|makakoon|private-tag|private validation|private cleanup|private evaluation|launch-gate|launch gate' \
  -- \
  ':(exclude)scripts/check-public-readiness.sh'

require_clean_grep "developer-local absolute paths" \
  '/Users/dave|/home/dave|/var/folders|/private/var/folders' \
  -- \
  ':(exclude)scripts/check-public-readiness.sh'

require_clean_grep "generic absolute home paths outside intentional tests" \
  '/Users/|/home/' \
  -- \
  ':(exclude)scripts/check-public-readiness.sh' \
  ':(exclude)internal/contract/capabilities_test.go'

section "beads"

if ! command -v jq >/dev/null 2>&1; then
  fail "jq is required for Beads JSONL validation"
else
  jq -e . .beads/issues.jsonl >/dev/null
  printf 'ok: .beads/issues.jsonl parses as JSONL\n'

  if jq -r '[.id, .title, .description, .close_reason, (.labels // [] | join(" "))] | @tsv' .beads/issues.jsonl \
    | grep -n -E 'GOPRIVATE|GONOSUMDB|LICENSE_PENDING|github\.com/Ozhiaki|Makakoons|makakoon|/Users/|/home/|private-tag|private validation|private cleanup|private evaluation|launch-gate|launch gate|agent mail|subagent|handoff|scratch|raw prompt' >"$beads_out"; then
    fail "Beads public tracker contains private or process residue"
    cat "$beads_out" >&2
  else
    printf 'ok: Beads public tracker fields are residue-clean\n'
  fi
fi

if (( failures > 0 )); then
  printf '\npublic-readiness check failed with %d issue(s)\n' "$failures" >&2
  exit 1
fi

printf '\npublic-readiness check passed\n'
