#!/bin/bash
# Regression test: record-quality-review.sh must not lose entries under
# concurrent invocations across worktrees. Spawns N parallel runs with
# distinct SHAs and asserts all N appear in the final state JSON.
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
script="$repo_root/.claude/scripts/record-quality-review.sh"

if [[ ! -x "$script" ]]; then
  echo "error: $script is not executable" >&2
  exit 1
fi

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

mkdir -p "$work_dir/bin"
cat > "$work_dir/bin/git" <<'EOF'
#!/bin/bash
# Strict stub: only `git rev-parse HEAD` is expected from the script under
# test. Any other git invocation should fail the test loudly, not pass
# through to the real git.
if [[ "${1:-}" != "rev-parse" || "${2:-}" != "HEAD" ]]; then
  echo "stub git: unexpected args: $*" >&2
  exit 99
fi
echo "${QR_TEST_SHA:?QR_TEST_SHA must be set}"
EOF
chmod +x "$work_dir/bin/git"

mkdir -p "$work_dir/run/.claude/tmp"

n=20
pids=()
for i in $(seq 1 "$n"); do
  sha=$(printf 'deadbeef%032d' "$i")
  (
    cd "$work_dir/run"
    export PATH="$work_dir/bin:$PATH"
    export QR_TEST_SHA="$sha"
    "$script"
  ) &
  pids+=($!)
done

fail=0
for pid in "${pids[@]}"; do
  wait "$pid" || fail=1
done
if [[ $fail -ne 0 ]]; then
  echo "FAIL: at least one parallel invocation exited non-zero" >&2
  exit 1
fi

state_file="$work_dir/run/.claude/tmp/quality-review-state.json"
if [[ ! -f "$state_file" ]]; then
  echo "FAIL: state file not created at $state_file" >&2
  exit 1
fi

count=$(jq '.reviewed | length' "$state_file")
if [[ "$count" -ne "$n" ]]; then
  echo "FAIL: expected $n entries, got $count" >&2
  jq '.' "$state_file" >&2
  exit 1
fi

for i in $(seq 1 "$n"); do
  sha=$(printf 'deadbeef%032d' "$i")
  if ! jq -e --arg sha "$sha" '.reviewed[] | select(.sha == $sha)' "$state_file" >/dev/null; then
    echo "FAIL: SHA $sha missing from state" >&2
    exit 1
  fi
done

echo "PASS: all $n parallel SHAs persisted"
