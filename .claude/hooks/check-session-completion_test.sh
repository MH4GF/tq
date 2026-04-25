#!/bin/bash
# Test harness for check-session-completion.sh.
# Builds a throwaway git repo per case and stubs `gh` via PATH override.
# Run from the repo root: ./.claude/hooks/check-session-completion_test.sh

set -uo pipefail

HOOK="$(cd "$(dirname "$0")" && pwd)/check-session-completion.sh"
[ -x "$HOOK" ] || { echo "FATAL: $HOOK is not executable"; exit 1; }

PASS=0
FAIL=0
FAILED_CASES=()

setup_repo() {
  WORK=$(mktemp -d)
  cd "$WORK"
  git init --quiet --initial-branch=main
  git config user.email "test@example.com"
  git config user.name "Test"
  git config commit.gpgsign false
  git config tag.gpgsign false
  echo "init" > README.md
  git add README.md
  git commit --quiet -m "init"
  # Fake an origin/main ref so `git rev-list --count origin/main..HEAD` works.
  git update-ref refs/remotes/origin/main HEAD
}

setup_gh_stub() {
  STUB_DIR=$(mktemp -d)
  cat >"$STUB_DIR/gh" <<'STUB'
#!/bin/bash
case "$1 $2" in
  "pr list")
    [ "${GH_PR_LIST_FAIL:-}" = "1" ] && exit 1
    echo "${GH_PR_LIST_OUT:-[]}"
    ;;
  *)
    echo "unexpected gh call: $*" >&2
    exit 2
    ;;
esac
STUB
  chmod +x "$STUB_DIR/gh"
  export PATH="$STUB_DIR:$PATH"
}

# Most PR-state cases need: feature branch + one commit ahead of origin/main.
setup_feature_branch_with_commit() {
  setup_repo
  setup_gh_stub
  git checkout -q -b feat/x
  echo "y" >> README.md
  git add README.md
  git commit --quiet -m "feat"
}

teardown() {
  cd /
  rm -rf "$WORK" "$STUB_DIR"
  unset GH_PR_LIST_OUT GH_PR_LIST_FAIL TQ_SKIP_COMPLETION_CHECK
}

# run_case <name> <stdin> <expected_exit> <expect_decision: block|none> [reason_substr]
run_case() {
  local name="$1" stdin="$2" want_exit="$3" want_decision="$4" want_reason="${5:-}"
  local out exit_code decision reason
  out=$(echo "$stdin" | "$HOOK" 2>/dev/null)
  exit_code=$?
  if [ "$exit_code" != "$want_exit" ]; then
    FAIL=$((FAIL + 1))
    FAILED_CASES+=("$name: exit $exit_code (want $want_exit), out=$out")
    return
  fi
  if [ "$want_decision" = "none" ]; then
    if [ -n "$out" ]; then
      FAIL=$((FAIL + 1))
      FAILED_CASES+=("$name: expected no output, got $out")
      return
    fi
  else
    decision=$(jq -r '.decision // empty' <<<"$out" 2>/dev/null)
    reason=$(jq -r '.reason // empty' <<<"$out" 2>/dev/null)
    if [ "$decision" != "$want_decision" ]; then
      FAIL=$((FAIL + 1))
      FAILED_CASES+=("$name: decision=$decision (want $want_decision), out=$out")
      return
    fi
    if [ -n "$want_reason" ] && ! echo "$reason" | grep -qF "$want_reason"; then
      FAIL=$((FAIL + 1))
      FAILED_CASES+=("$name: reason missing '$want_reason', got: $reason")
      return
    fi
  fi
  PASS=$((PASS + 1))
}

# ---------- T1: stop_hook_active ----------
setup_repo; setup_gh_stub
git checkout -q -b feat/x
run_case "T1 stop_hook_active" '{"stop_hook_active":true}' 0 none
teardown

# ---------- T2: branch=main (still on main, ignore everything) ----------
setup_repo; setup_gh_stub
echo "x" >> README.md
run_case "T2 branch=main" '{}' 0 none
teardown

# ---------- T3: opt-out env ----------
setup_repo; setup_gh_stub
git checkout -q -b feat/x
echo "x" >> README.md
export TQ_SKIP_COMPLETION_CHECK=1
run_case "T3 TQ_SKIP_COMPLETION_CHECK" '{}' 0 none
teardown

# ---------- T4: uncommitted on feature branch ----------
setup_repo; setup_gh_stub
git checkout -q -b feat/x
echo "x" >> README.md
run_case "T4 uncommitted" '{}' 0 block "未コミット変更"
teardown

# ---------- T5: clean, no ahead, no PR ----------
setup_repo; setup_gh_stub
git checkout -q -b feat/x
export GH_PR_LIST_OUT="[]"
run_case "T5 clean no-ahead no-PR" '{}' 0 none
teardown

# ---------- T6: commits ahead, no PR ----------
setup_feature_branch_with_commit
export GH_PR_LIST_OUT="[]"
run_case "T6 ahead no-PR" '{}' 0 block "PR がありません"
teardown

# ---------- T7: PR open, SUCCESS, MERGEABLE, not draft ----------
setup_feature_branch_with_commit
export GH_PR_LIST_OUT='[{"number":42,"state":"OPEN","isDraft":false,"mergeable":"MERGEABLE","statusCheckRollup":[{"conclusion":"SUCCESS"}]}]'
run_case "T7 PR open mergeable" '{}' 0 block "merge 可能"
teardown

# ---------- T8: PR open, SUCCESS, MERGEABLE, draft ----------
setup_feature_branch_with_commit
export GH_PR_LIST_OUT='[{"number":43,"state":"OPEN","isDraft":true,"mergeable":"MERGEABLE","statusCheckRollup":[{"conclusion":"SUCCESS"}]}]'
run_case "T8 PR draft mergeable" '{}' 0 block "draft"
teardown

# ---------- T9: PR open, FAILURE ----------
setup_feature_branch_with_commit
export GH_PR_LIST_OUT='[{"number":44,"state":"OPEN","isDraft":false,"mergeable":"MERGEABLE","statusCheckRollup":[{"conclusion":"SUCCESS"},{"conclusion":"FAILURE"}]}]'
run_case "T9 PR CI failure" '{}' 0 block "fix-ci"
teardown

# ---------- T10: PR open, PENDING ----------
setup_feature_branch_with_commit
export GH_PR_LIST_OUT='[{"number":45,"state":"OPEN","isDraft":false,"mergeable":"MERGEABLE","statusCheckRollup":[{"status":"IN_PROGRESS"}]}]'
run_case "T10 PR CI pending" '{}' 0 block "wait-pr-checks"
teardown

# ---------- T11: PR open, SUCCESS, CONFLICTING ----------
setup_feature_branch_with_commit
export GH_PR_LIST_OUT='[{"number":46,"state":"OPEN","isDraft":false,"mergeable":"CONFLICTING","statusCheckRollup":[{"conclusion":"SUCCESS"}]}]'
run_case "T11 PR conflict" '{}' 0 block "fix-conflict"
teardown

# ---------- T12: PR open, SUCCESS, UNKNOWN ----------
setup_feature_branch_with_commit
export GH_PR_LIST_OUT='[{"number":47,"state":"OPEN","isDraft":false,"mergeable":"UNKNOWN","statusCheckRollup":[{"conclusion":"SUCCESS"}]}]'
run_case "T12 PR mergeable unknown" '{}' 0 block "wait-pr-checks"
teardown

# ---------- T13: PR MERGED ----------
setup_feature_branch_with_commit
export GH_PR_LIST_OUT='[{"number":48,"state":"MERGED","isDraft":false,"mergeable":"MERGEABLE","statusCheckRollup":[]}]'
run_case "T13 PR merged" '{}' 0 none
teardown

# ---------- T14: PR CLOSED ----------
setup_feature_branch_with_commit
export GH_PR_LIST_OUT='[{"number":49,"state":"CLOSED","isDraft":false,"mergeable":"MERGEABLE","statusCheckRollup":[]}]'
run_case "T14 PR closed" '{}' 0 none
teardown

# ---------- T16: CheckRun in-progress (conclusion="" + status=IN_PROGRESS) ----------
# Regresses against jq `//` treating empty string as truthy and missing pending state.
setup_feature_branch_with_commit
export GH_PR_LIST_OUT='[{"number":50,"state":"OPEN","isDraft":false,"mergeable":"MERGEABLE","statusCheckRollup":[{"conclusion":"","status":"IN_PROGRESS"},{"conclusion":"SUCCESS"}]}]'
run_case "T16 CheckRun in-progress empty conclusion" '{}' 0 block "wait-pr-checks"
teardown

# ---------- T17: StatusContext (uses .state, not .conclusion/.status) ----------
setup_feature_branch_with_commit
export GH_PR_LIST_OUT='[{"number":51,"state":"OPEN","isDraft":false,"mergeable":"MERGEABLE","statusCheckRollup":[{"state":"PENDING"}]}]'
run_case "T17 StatusContext pending" '{}' 0 block "wait-pr-checks"
teardown

# ---------- T18: StatusContext SUCCESS treated as success ----------
setup_feature_branch_with_commit
export GH_PR_LIST_OUT='[{"number":52,"state":"OPEN","isDraft":false,"mergeable":"MERGEABLE","statusCheckRollup":[{"conclusion":"SUCCESS"},{"state":"SUCCESS"}]}]'
run_case "T18 StatusContext success → mergeable" '{}' 0 block "merge 可能"
teardown

# ---------- T15: gh fails (fail open) ----------
setup_feature_branch_with_commit
export GH_PR_LIST_FAIL=1
run_case "T15 gh fail open" '{}' 0 none
teardown

# ---------- Report ----------
echo
echo "Passed: $PASS"
echo "Failed: $FAIL"
if [ $FAIL -gt 0 ]; then
  echo
  printf 'FAILED:\n'
  for c in "${FAILED_CASES[@]}"; do printf '  - %s\n' "$c"; done
  exit 1
fi
