#!/usr/bin/env bash
# PostToolUse hook (Write|Edit|MultiEdit): auto-format edited .go files via
# `golangci-lint fmt`, then surface package-level lint diagnostics back into
# the agent's context via hookSpecificOutput.additionalContext.
#
# Always non-blocking (exit 0). Diagnostics inform; they do not gate.
# Fail-open on any tooling absence so the harness never bricks the session.

set -uo pipefail

input="$(cat 2>/dev/null || echo '{}')"
file="$(jq -r '.tool_input.file_path // empty' <<<"$input" 2>/dev/null)"

[ -n "$file" ] || exit 0
case "$file" in
  *.go) ;;
  *) exit 0 ;;
esac
case "$file" in
  */vendor/*|*/.git/*|*/node_modules/*) exit 0 ;;
esac
[ -f "$file" ] || exit 0

command -v golangci-lint >/dev/null 2>&1 || {
  echo "post-edit-go-quality: golangci-lint not found; skipping" >&2
  exit 0
}

repo_root="$(git -C "$(dirname "$file")" rev-parse --show-toplevel 2>/dev/null)" || exit 0
cd "$repo_root" || exit 0

rel="${file#$repo_root/}"
pkg_dir="$(dirname "$rel")"
pkg_pattern="./${pkg_dir}/..."

with_timeout() {
  local sec="$1"; shift
  if command -v timeout >/dev/null 2>&1; then
    timeout "$sec" "$@"
  elif command -v gtimeout >/dev/null 2>&1; then
    gtimeout "$sec" "$@"
  else
    "$@"
  fi
}

with_timeout 10 golangci-lint fmt "$file" >/dev/null 2>&1 || true

diag="$(with_timeout 30 golangci-lint run "$pkg_pattern" 2>&1 || true)"

if echo "$diag" | grep -qE '^[^[:space:]].*:[0-9]+:[0-9]+:'; then
  trimmed="$(echo "$diag" | head -40)"
  jq -Rn --arg msg "lint violations in ${pkg_dir} (fix before continuing):

$trimmed" '{
    hookSpecificOutput: {
      hookEventName: "PostToolUse",
      additionalContext: $msg
    }
  }'
fi

exit 0
