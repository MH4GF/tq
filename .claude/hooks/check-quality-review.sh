#!/bin/bash
# Block `gh pr create` if /quality-review has not been run on the current HEAD.
# Reads SHA from .claude/tmp/quality-review-state.json (written by /quality-review).

set -euo pipefail

input=$(cat)
cmd=$(printf '%s' "$input" | jq -r '.tool_input.command // empty')

# Tokenize via shlex so quoted args (e.g., a git commit message containing
# "gh pr create") do not produce false positives. Match only when `gh`, `pr`,
# `create` appear as three consecutive shell tokens.
match=$(printf '%s' "$cmd" | python3 -c '
import sys, shlex
try:
    tokens = shlex.split(sys.stdin.read(), comments=False, posix=True)
except ValueError:
    sys.exit(0)
for i in range(len(tokens) - 2):
    if tokens[i] == "gh" and tokens[i+1] == "pr" and tokens[i+2] == "create":
        print("yes"); sys.exit(0)
')
[ "$match" = "yes" ] || exit 0

sha=$(git rev-parse HEAD 2>/dev/null) || exit 0

state_file=".claude/tmp/quality-review-state.json"
if [ -f "$state_file" ] && jq -e --arg sha "$sha" '.reviewed[]? | select(.sha == $sha)' "$state_file" >/dev/null 2>&1; then
  exit 0
fi

short=${sha:0:8}
cat <<EOF
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"HEAD ${short} に対して /quality-review が未実行です。CLAUDE.md の Quality Gate に従い、/quality-review を実行してから gh pr create を再試行してください。"}}
EOF
