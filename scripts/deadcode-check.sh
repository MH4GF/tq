#!/usr/bin/env bash
set -euo pipefail

# Detects functions/methods unreachable from main and test binaries via
# golang.org/x/tools/cmd/deadcode (RTA-based call graph). Compares findings
# against .deadcode-allowlist (one identifier per line, blank/# comments OK).
# Fails when new findings appear OR when allowlist entries are no longer dead.
#
# See docs/golden-rules.md Rule 13.

DEADCODE_VERSION="v0.44.0"
ALLOWLIST="${ALLOWLIST:-.deadcode-allowlist}"

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_root"

if [[ ! -f "$ALLOWLIST" ]]; then
  echo "error: allowlist file not found: $ALLOWLIST" >&2
  exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
current="$tmpdir/current"
expected="$tmpdir/expected"
stderr="$tmpdir/stderr"

if ! go run "golang.org/x/tools/cmd/deadcode@${DEADCODE_VERSION}" \
    -test \
    -f '{{range .Funcs}}{{$.Path}}.{{.Name}}{{"\n"}}{{end}}' \
    ./... 2>"$stderr" \
    | sort -u > "$current"; then
  echo "error: deadcode invocation failed" >&2
  cat "$stderr" >&2
  exit 1
fi

grep -vE '^\s*(#|$)' "$ALLOWLIST" | sort -u > "$expected"

new_findings="$(comm -23 "$current" "$expected")"
stale_entries="$(comm -13 "$current" "$expected")"

status=0
if [[ -n "$new_findings" ]]; then
  echo "FAIL: new dead code (not in $ALLOWLIST):"
  echo "$new_findings" | sed 's/^/  + /'
  echo
  echo "If intentional (test seam, interface impl, etc.), add the identifier to $ALLOWLIST."
  echo "Otherwise, remove the unused code."
  status=1
fi

if [[ -n "$stale_entries" ]]; then
  [[ $status -eq 0 ]] || echo
  echo "FAIL: stale entries in $ALLOWLIST (no longer reported as dead):"
  echo "$stale_entries" | sed 's/^/  - /'
  echo
  echo "Remove these lines from $ALLOWLIST."
  status=1
fi

if [[ $status -eq 0 ]]; then
  echo "deadcode: OK ($(wc -l < "$expected" | tr -d ' ') allowlisted, no new findings)"
fi

exit $status
