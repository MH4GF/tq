#!/bin/bash
# Stop hook: push back the agent when the session is about to end with work
# left half-done (uncommitted, no PR, PR open / pending / failing / conflict).
#
# Output contract:
#   exit 0 (no stdout): allow stop
#   stdout {"decision":"block","reason":"..."}: push back
#
# Fail open: any tooling error exits 0 to avoid getting the session stuck.
#
# Loop guard: persist (session_id, reason_key) to .git/info/tq-stop-hook-last-reason
# so identical consecutive push-backs are suppressed while reason transitions re-fire.
#
# Operator-only safety valve (intentionally NOT documented in agent skills):
#   TQ_SKIP_COMPLETION_CHECK=1 claude ...

set -uo pipefail

input=$(cat 2>/dev/null || echo '{}')
session_id=$(jq -r '.session_id // ""' <<<"$input" 2>/dev/null || echo "")

git_dir=$(git rev-parse --git-dir 2>/dev/null) || exit 0
marker="$git_dir/info/tq-stop-hook-last-reason"

read_marker() { [ -f "$marker" ] && cat "$marker" 2>/dev/null || printf ''; }
write_marker() {
  mkdir -p "$(dirname "$marker")" 2>/dev/null || return 0
  printf '%s\t%s' "$session_id" "$1" > "$marker" 2>/dev/null || return 0
}
clear_marker() { rm -f "$marker" 2>/dev/null || true; }

branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null) || exit 0
case "$branch" in
  main|master|HEAD|"")
    clear_marker
    exit 0
    ;;
esac

[ "${TQ_SKIP_COMPLETION_CHECK:-}" = "1" ] && exit 0

# emit_block <reason_key> <reason_text>
# Suppresses if (session_id, reason_key) matches the previous push-back.
emit_block() {
  local key="$1" reason="$2"
  local current prev
  current=$(printf '%s\t%s' "$session_id" "$key")
  prev=$(read_marker)
  if [ "$prev" = "$current" ]; then
    exit 0
  fi
  write_marker "$key"
  jq -nc --arg reason "$reason" '{decision:"block", reason:$reason}'
  exit 0
}

# Wrap external calls so we cap latency on Linux (where `timeout` exists) but
# still work on macOS dev boxes (which ship neither `timeout` nor `gtimeout`).
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

if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
  emit_block "uncommitted" "未コミット変更があります。\`/commit-and-pr\` で commit して PR まで進めてください。"
fi

# Single gh call: --state all so MERGED/CLOSED PRs surface (we exit 0 for them).
# All evaluation fields included; no follow-up `gh pr view` needed.
pr_json=$(with_timeout 5 gh pr list --state all --head "$branch" --limit 1 \
  --json number,state,isDraft,statusCheckRollup,mergeable 2>/dev/null) || exit 0
pr_count=$(jq 'length' <<<"$pr_json" 2>/dev/null || echo 0)

if [ "$pr_count" = "0" ]; then
  ahead=$(git rev-list --count origin/main..HEAD 2>/dev/null || echo 0)
  if [ "${ahead:-0}" -gt 0 ]; then
    emit_block "pr-missing" "commit が origin/main より進んでいますが PR がありません。\`/commit-and-pr\` で PR を作成してください。"
  fi
  clear_marker
  exit 0
fi

# Single jq pass to extract every field we need. statusCheckRollup entries
# carry one of three result fields depending on shape:
#   - CheckRun completed: .conclusion (SUCCESS/FAILURE/...)
#   - CheckRun running:   .status (IN_PROGRESS/QUEUED/...) — .conclusion is ""
#   - StatusContext:      .state (SUCCESS/FAILURE/...) — no .conclusion/.status
# `//` treats "" as truthy, so explicitly skip empty strings.
IFS=$'\t' read -r pr_number state is_draft mergeable ci_states < <(
  jq -r '.[0] | [
    .number,
    .state,
    (.isDraft | tostring),
    .mergeable,
    ([.statusCheckRollup[]? |
      if (.conclusion // "") != "" then .conclusion
      elif (.state // "") != ""      then .state
      else                                (.status // "")
      end
    ] | join(" "))
  ] | @tsv' <<<"$pr_json"
)

if [ "$state" != "OPEN" ]; then
  clear_marker
  exit 0
fi

has_failure=false
has_pending=false
for c in $ci_states; do
  case "$c" in
    FAILURE|TIMED_OUT|ACTION_REQUIRED|STARTUP_FAILURE|STALE) has_failure=true ;;
    PENDING|QUEUED|IN_PROGRESS|WAITING|REQUESTED) has_pending=true ;;
  esac
done

if $has_failure; then
  emit_block "ci-fail#${pr_number}" "PR #${pr_number} の CI が失敗しています。\`/gh-ops:fix-ci\` で修正してください。"
fi
if $has_pending; then
  emit_block "ci-pending#${pr_number}" "PR #${pr_number} の CI 待ちです。\`/gh-ops:wait-pr-checks\` で完了を待ってください。"
fi

case "$mergeable" in
  MERGEABLE)
    if [ "$is_draft" = "true" ]; then
      emit_block "mergeable-draft#${pr_number}" "PR #${pr_number} は draft のままです。\`gh pr ready ${pr_number}\` で ready 化してから \`/gh-ops:merge-pr\` を実行してください。"
    else
      emit_block "mergeable#${pr_number}" "PR #${pr_number} は merge 可能です。\`/gh-ops:merge-pr\` を実行してください。"
    fi
    ;;
  CONFLICTING)
    emit_block "conflict#${pr_number}" "PR #${pr_number} に conflict があります。\`/gh-ops:fix-conflict\` で解消してください。"
    ;;
  UNKNOWN)
    emit_block "mergeable-unknown#${pr_number}" "PR #${pr_number} の mergeable 判定が pending です。\`/gh-ops:wait-pr-checks\` で再評価を待ってください。"
    ;;
esac

clear_marker
exit 0
