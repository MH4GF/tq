#!/usr/bin/env bash
set -euo pipefail

# Hermetic E2E for turso-query-watch.sh: a synthetic fixture + a hand-lowered
# baseline drive a known regression through the full notification path, with a
# fake `tq` on PATH capturing the filed action. No turso auth / network needed.

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="$HERE/turso-query-watch.sh"
FIXTURE="$HERE/testdata/turso-inspect-sample.txt"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

BASELINE="$WORK/baseline.json"
IGNORE="$WORK/ignore.txt"
TQ_CALLS="$WORK/tq_calls.txt"
BIN="$WORK/bin"
mkdir -p "$BIN"

# Baseline lowered so two fixture queries clear both gates (+50% AND +50M).
cat >"$BASELINE" <<'JSON'
{
  "queries": {
    "SELECT id, title, task_id, metadata, status FROM actions WHERE task_id IN (?, ?)": {"rows_read": 10000000},
    "INSERT INTO events (entity_type, entity_id, event_type, payload) VALUES (?, ?, ?, ?)": {"rows_read": 20000},
    "SELECT id, metadata FROM actions WHERE metadata LIKE '%permission_mode%' OR metadata LIKE '%worktree%'": {"rows_read": 1000},
    "SELECT 'task' AS entity_type, t.id, t.project_id FROM tasks t WHERE t.title LIKE '%' || ? || '%'": {"rows_read": 5000000},
    "COMMIT": {"rows_read": 0}
  }
}
JSON

# Silence the LIKE-scan query even though it would otherwise clear both gates.
echo "metadata LIKE '%permission_mode%'" >"$IGNORE"

# Fake tq: record argv, succeed.
cat >"$BIN/tq" <<EOF
#!/usr/bin/env bash
printf '%s\0' "\$@" >> "$TQ_CALLS"
echo "action #9999 created"
EOF
chmod +x "$BIN/tq"

run() {
  PATH="$BIN:$PATH" \
  TURSO_INSPECT_FIXTURE="$FIXTURE" \
  TURSO_QUERY_BASELINE="$1" \
  TURSO_QUERY_IGNORE="$IGNORE" \
  TURSO_WATCH_TASK_ID=698 \
  bash "$SCRIPT" "${@:2}"
}

fail() { echo "FAIL: $1" >&2; exit 1; }
pass() { echo "PASS: $1"; }

# --- Case 1: real run files an action ----------------------------------------
BL1="$WORK/bl1.json"; cp "$BASELINE" "$BL1"
run "$BL1" >"$WORK/out1.json" 2>"$WORK/err1.txt"

python3 - "$WORK/out1.json" <<'PY' || fail "JSON assertions (case 1)"
import json, sys
d = json.load(open(sys.argv[1]))
assert d["regression_count"] == 2, d["regression_count"]
by = {r["query"]: r for r in d["queries"]}
q1 = "SELECT id, title, task_id, metadata, status FROM actions WHERE task_id IN (?, ?)"
q3 = "SELECT id, metadata FROM actions WHERE metadata LIKE '%permission_mode%' OR metadata LIKE '%worktree%'"
q4 = "SELECT 'task' AS entity_type, t.id, t.project_id FROM tasks t WHERE t.title LIKE '%' || ? || '%'"
assert by[q1]["regression"] is True, by[q1]
assert by[q3]["ignored"] is True and by[q3]["regression"] is False, by[q3]
assert by[q4]["regression"] is True, by[q4]
PY
pass "JSON: 2 regressions, ignored query excluded"

[ -f "$TQ_CALLS" ] || fail "fake tq was not invoked"
python3 - "$TQ_CALLS" <<'PY' || fail "tq action create argv assertions"
import sys
args = open(sys.argv[1], "rb").read().decode().split("\0")
assert "action" in args and "create" in args, args
assert "--task" in args and args[args.index("--task") + 1] == "698", args
mi = args.index("--meta")
import json
assert json.loads(args[mi + 1])["mode"] == "interactive", args[mi + 1]
PY
pass "tq action create: --task 698, mode interactive"

# --- Case 2: --dry-run files nothing, leaves baseline untouched ---------------
BL2="$WORK/bl2.json"; cp "$BASELINE" "$BL2"
SUM_BEFORE="$(md5 -q "$BL2" 2>/dev/null || md5sum "$BL2")"
: >"$TQ_CALLS"
run "$BL2" --dry-run >/dev/null 2>"$WORK/err2.txt"
[ ! -s "$TQ_CALLS" ] || fail "--dry-run filed a tq action"
SUM_AFTER="$(md5 -q "$BL2" 2>/dev/null || md5sum "$BL2")"
[ "$SUM_BEFORE" = "$SUM_AFTER" ] || fail "--dry-run rewrote the baseline"
pass "--dry-run: no action filed, baseline untouched"

# --- Case 3: real run updates the baseline -----------------------------------
grep -q '"updated_at"' "$BL1" || fail "baseline not rewritten after real run"
pass "baseline rewritten after real run"

echo "ALL PASS"
