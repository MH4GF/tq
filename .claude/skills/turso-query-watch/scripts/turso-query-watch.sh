#!/usr/bin/env bash
set -euo pipefail

# Periodic Turso rows-read regression watcher.
#
# Captures the top queries by rows-read from `turso db inspect <db> --queries`,
# compares each against a stored baseline, and files a tq tracking action when a
# query's rows-read grows past both a percentage and an absolute-floor gate.
# Detection only -- it never edits code or "fixes" anything.
#
# Docs: docs/turso-query-watch.md

usage() {
  cat <<'EOF'
Usage: turso-query-watch.sh [--dry-run] [--help]

  --dry-run   Parse + compare + print JSON/report, but do not file a tq action
              and do not rewrite the baseline. Safe for ad-hoc human runs.
  --help      Show this help.

Environment:
  TURSO_DB_NAME              Turso database name            (default: tq)
  TURSO_INSPECT_FIXTURE      Read this file instead of running `turso db
                             inspect` (testing / offline ad-hoc runs)
  TURSO_QUERY_BASELINE       Baseline JSON path
                             (default: ~/.config/tq/turso-query-baseline.json)
  TURSO_QUERY_IGNORE         Ignore-pattern file, one substring per line, '#'
                             comments (default: ~/.config/tq/turso-query-ignore.txt)
  TURSO_QUERY_TOP_N          Compare top N by rows-read   (default: 10)
  TURSO_QUERY_PCT            Fractional growth gate       (default: 0.5  = +50%)
  TURSO_QUERY_ABS_FLOOR      Absolute rows-read delta gate(default: 50000000)
  TURSO_WATCH_TASK_ID        tq task the action is filed under (default: 698)
  TURSO_QUERY_NO_BASELINE_UPDATE=1  Do not rewrite the baseline this run
  TQ_BIN                     tq binary                    (default: tq)
EOF
}

DRY_RUN=0
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    --help|-h) usage; exit 0 ;;
    *) echo "turso-query-watch: unknown argument: $arg" >&2; usage >&2; exit 2 ;;
  esac
done

DB_NAME="${TURSO_DB_NAME:-tq}"
BASELINE="${TURSO_QUERY_BASELINE:-$HOME/.config/tq/turso-query-baseline.json}"
IGNORE_FILE="${TURSO_QUERY_IGNORE:-$HOME/.config/tq/turso-query-ignore.txt}"
TOP_N="${TURSO_QUERY_TOP_N:-10}"
PCT="${TURSO_QUERY_PCT:-0.5}"
ABS_FLOOR="${TURSO_QUERY_ABS_FLOOR:-50000000}"
WATCH_TASK_ID="${TURSO_WATCH_TASK_ID:-698}"
TQ_BIN="${TQ_BIN:-tq}"

# --- 1. Acquire raw `turso db inspect --queries` text ------------------------
RAW=""
if [[ -n "${TURSO_INSPECT_FIXTURE:-}" ]]; then
  if [[ ! -f "$TURSO_INSPECT_FIXTURE" ]]; then
    echo "turso-query-watch: TURSO_INSPECT_FIXTURE not found: $TURSO_INSPECT_FIXTURE" >&2
    exit 1
  fi
  RAW="$(cat "$TURSO_INSPECT_FIXTURE")"
else
  if ! command -v turso >/dev/null 2>&1; then
    echo "turso-query-watch: 'turso' CLI not found on PATH." >&2
    echo "  Install it (https://docs.turso.tech/cli) or set TURSO_INSPECT_FIXTURE." >&2
    exit 1
  fi
  if ! RAW="$(turso db inspect "$DB_NAME" --queries 2>&1)"; then
    echo "turso-query-watch: 'turso db inspect $DB_NAME --queries' failed:" >&2
    echo "$RAW" >&2
    echo "  This usually means the CLI is not authenticated to the Turso" >&2
    echo "  platform. Export a platform token as TURSO_API_TOKEN (mint via" >&2
    echo "  'turso auth api-tokens mint <name>') or run 'turso auth login'." >&2
    exit 1
  fi
fi

# --- 2-5. Parse, compare, report, maybe file action + update baseline --------
# All structured work happens in python3 for robust parsing/JSON handling.
RAW="$RAW" \
BASELINE="$BASELINE" IGNORE_FILE="$IGNORE_FILE" TOP_N="$TOP_N" \
PCT="$PCT" ABS_FLOOR="$ABS_FLOOR" DB_NAME="$DB_NAME" DRY_RUN="$DRY_RUN" \
NO_BL_UPDATE="${TURSO_QUERY_NO_BASELINE_UPDATE:-0}" \
python3 - "$TQ_BIN" "$WATCH_TASK_ID" <<'PY'
import json, os, re, subprocess, sys, datetime

tq_bin, watch_task_id = sys.argv[1], sys.argv[2]
raw = os.environ["RAW"]
baseline_path = os.environ["BASELINE"]
ignore_file = os.environ["IGNORE_FILE"]
top_n = int(os.environ["TOP_N"])
pct = float(os.environ["PCT"])
abs_floor = int(os.environ["ABS_FLOOR"])
db_name = os.environ["DB_NAME"]
dry_run = os.environ["DRY_RUN"] == "1"
no_bl_update = os.environ["NO_BL_UPDATE"] == "1"

# --- Parse the inspect output -------------------------------------------------
# Layout (turso-cli, rodaine/table):
#   QUERY            ROWS WRITTEN  ROWS READ
#   <sql ...>        <written>     <read>
# `--queries` alone prints no "Total ..." preamble. Every line is space-padded
# to the widest query. A multi-line SQL spans several physical lines but the two
# numeric columns appear only on its FIRST physical line; continuation lines
# carry no trailing numbers.
# Robust row rule: the last two whitespace-separated integer tokens on a line
# are ROWS WRITTEN then ROWS READ; everything before is the query text. The
# greedy query group keeps trailing numeric SQL literals (e.g. `LIMIT 100`)
# because the wide padding gap always separates the query from the columns.
lines = raw.splitlines()
hdr_idx = None
for i, ln in enumerate(lines):
    u = ln.upper()
    if "QUERY" in u and "ROWS READ" in u and "ROWS WRITTEN" in u:
        hdr_idx = i
        break
if hdr_idx is None:
    sys.stderr.write(
        "turso-query-watch: could not find the QUERY/ROWS table header in "
        "inspect output. Raw output was:\n" + raw + "\n")
    sys.exit(1)

row_re = re.compile(r"^(.*\S)\s+(\d+)\s+(\d+)\s*$")
current = {}  # sql -> rows_read
for ln in lines[hdr_idx + 1:]:
    if not ln.strip():
        continue
    m = row_re.match(ln)
    if not m:
        continue
    sql = m.group(1).strip()
    rows_read = int(m.group(3))  # group(2)=written, group(3)=read
    if sql not in current or rows_read > current[sql]:
        current[sql] = rows_read

if not current:
    sys.stderr.write(
        "turso-query-watch: header found but no query rows parsed. Raw:\n"
        + raw + "\n")
    sys.exit(1)

ranked = sorted(current.items(), key=lambda kv: kv[1], reverse=True)[:top_n]

# --- Ignore list --------------------------------------------------------------
ignore_patterns = []
if os.path.isfile(ignore_file):
    with open(ignore_file) as fh:
        for line in fh:
            s = line.strip()
            if s and not s.startswith("#"):
                ignore_patterns.append(s)

def ignored(sql):
    return any(pat in sql for pat in ignore_patterns)

# --- Baseline -----------------------------------------------------------------
baseline = {}
if os.path.isfile(baseline_path):
    try:
        with open(baseline_path) as fh:
            baseline = json.load(fh).get("queries", {})
    except (ValueError, OSError):
        baseline = {}

regressions = []
report_rows = []
for sql, now in ranked:
    base = baseline.get(sql, {}).get("rows_read", 0)
    skip = ignored(sql)
    delta = now - base
    grew_pct = base == 0 or now >= base * (1.0 + pct)
    grew_abs = delta >= abs_floor
    is_reg = (not skip) and grew_pct and grew_abs
    report_rows.append({
        "query": sql,
        "rows_read": now,
        "baseline_rows_read": base,
        "delta": delta,
        "pct_increase": (None if base == 0 else round(delta / base, 4)),
        "new_query": base == 0,
        "ignored": skip,
        "regression": is_reg,
    })
    if is_reg:
        regressions.append(report_rows[-1])

summary = {
    "db": db_name,
    "checked_at": datetime.datetime.now(datetime.timezone.utc)
        .strftime("%Y-%m-%dT%H:%M:%SZ"),
    "baseline_path": baseline_path,
    "top_n": top_n,
    "threshold_pct": pct,
    "abs_floor": abs_floor,
    "dry_run": dry_run,
    "regression_count": len(regressions),
    "queries": report_rows,
}
print(json.dumps(summary, indent=2, ensure_ascii=False))

# --- File a tq action on regression ------------------------------------------
if regressions and not dry_run:
    def fmt(n):
        return f"{n:,}"
    body = [
        "## Turso rows-read regression detected",
        "",
        f"`turso db inspect {db_name} --queries` shows {len(regressions)} "
        f"query(ies) past both gates "
        f"(>= +{int(pct * 100)}% **and** >= +{fmt(abs_floor)} rows-read vs "
        f"baseline `{baseline_path}`).",
        "",
        "| query | baseline | now | delta |",
        "|---|--:|--:|--:|",
    ]
    for r in regressions:
        q = r["query"].replace("|", "\\|")
        if len(q) > 160:
            q = q[:157] + "..."
        body.append(
            f"| `{q}` | {fmt(r['baseline_rows_read'])} | "
            f"{fmt(r['rows_read'])} | +{fmt(r['delta'])} |")
    body += [
        "",
        "**Detection only.** Investigate which change inflated these queries "
        "(git log around the affected tables / hot paths), then propose a "
        "remediation for human approval. Do NOT auto-fix. If this is an "
        "expected/benign growth, add a substring of the query to the ignore "
        f"file (`{ignore_file}`) to silence future alerts. See "
        "docs/turso-query-watch.md.",
    ]
    instruction = "\n".join(body)
    title = (f"Turso rows-read regression: {len(regressions)} query(ies) "
             f"over threshold")[:100]
    try:
        out = subprocess.run(
            [tq_bin, "action", "create", instruction,
             "--task", str(watch_task_id), "--title", title,
             "--meta", json.dumps({"mode": "experimental_bg"})],
            capture_output=True, text=True, check=True)
        sys.stderr.write("turso-query-watch: filed action -> "
                         + out.stdout.strip() + "\n")
    except subprocess.CalledProcessError as e:
        sys.stderr.write("turso-query-watch: FAILED to file tq action: "
                         + (e.stderr or e.stdout or "") + "\n")
        sys.exit(1)
elif regressions:
    sys.stderr.write(f"turso-query-watch: {len(regressions)} regression(s) "
                     "detected (dry-run: no action filed)\n")
else:
    sys.stderr.write("turso-query-watch: no regressions\n")

# --- Update baseline ----------------------------------------------------------
if not dry_run and not no_bl_update:
    os.makedirs(os.path.dirname(baseline_path) or ".", exist_ok=True)
    payload = {
        "updated_at": summary["checked_at"],
        "db": db_name,
        "queries": {sql: {"rows_read": rr} for sql, rr in current.items()},
    }
    tmp = baseline_path + ".tmp"
    with open(tmp, "w") as fh:
        json.dump(payload, fh, indent=2, ensure_ascii=False)
    os.replace(tmp, baseline_path)
    sys.stderr.write(f"turso-query-watch: baseline updated -> {baseline_path}\n")
PY
