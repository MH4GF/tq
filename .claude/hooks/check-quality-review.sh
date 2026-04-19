#!/bin/bash
# Block `gh pr create` if /quality-review has not been run on the current HEAD.
# Reads SHA from .claude/tmp/quality-review-state.json (written by /quality-review).

set -euo pipefail

cmd=$(jq -r '.tool_input.command // empty')

# Fast bail-out: most Bash calls are unrelated. Substring check is cheap and
# avoids spawning python3 for ~all non-matching commands.
[[ "$cmd" == *"gh"*"pr"*"create"* ]] || exit 0

# Confirm with shlex tokenization so quoted args (e.g., a git commit message
# containing "gh pr create") do not produce false positives.
match=$(python3 -c '
import sys, shlex
try:
    tokens = shlex.split(sys.stdin.read(), comments=False, posix=True)
except ValueError:
    sys.exit(0)
for i in range(len(tokens) - 2):
    if tokens[i] == "gh" and tokens[i+1] == "pr" and tokens[i+2] == "create":
        print("yes"); sys.exit(0)
' <<<"$cmd")
[ "$match" = "yes" ] || exit 0

sha=$(git rev-parse HEAD 2>/dev/null) || exit 0
[ -n "$sha" ] || exit 0

state_file=".claude/tmp/quality-review-state.json"
if [ -f "$state_file" ] && jq -e --arg sha "$sha" '.reviewed[]? | select(.sha == $sha)' "$state_file" >/dev/null 2>&1; then
  exit 0
fi

short=${sha:0:8}
cat <<EOF
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"HEAD ${short} に対して /quality-review が未実行です。CLAUDE.md の Quality Gate に従い、/quality-review を実行してから gh pr create を再試行してください。"}}
EOF
