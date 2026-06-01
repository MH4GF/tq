# Turso rows-read regression watch

Production-side observation of Turso `rows-read` for the `tq` database. Static
guards (forbidigo, deadcode-check, golden rules 11/16/18) are method-name
dependent and PR-time only; this is their runtime counterpart — method-name
independent, catching regressions that ship despite the static rules. Detection
only: it files a tracking action; humans triage and remediate.

## How it works

The skill `/turso-query-watch` runs
`.claude/skills/turso-query-watch/scripts/turso-query-watch.sh`, which:

1. Captures `turso db inspect tq --queries` (text table; the CLI has no JSON
   mode — only `--queries` / `--verbose`).
2. Parses the top N (default 10) queries by rows-read.
3. Compares each against the stored baseline.
4. Files **one** tq action (mode `interactive`) under task #698 when any
   query regresses. Rewrites the baseline on every non-dry-run run, whether or
   not a regression fired.

It always prints a JSON summary to stdout. It exits non-zero — never silently
no-ops — if `turso` is missing, unauthenticated, or the output cannot be parsed.

## Schedule

| | |
|---|---|
| Cadence | Weekly, Sunday 20:00 local — cron `0 20 * * 0` |
| Instruction | `/turso-query-watch` |
| Task | #698 "Turso rows-read regression watch (recurring)" (project `tq`) |
| Dispatch | `interactive`, `--model sonnet --effort low` |

Weekly is the deliberate starting default: rows-read accrues slowly and a
weekly window keeps noise low. If observed week-over-week variance turns out
large enough to warrant earlier detection, lower the cron to daily
(`0 20 * * *`) — the script and thresholds are unaffected.

`/turso-query-watch` resolves only after this skill is merged to `main` (the
scheduled `claude --bg` runs in the project's main work_dir). The bundled
script can be run by path before then.

## Threshold

A query is a regression only when it clears **both** gates versus baseline
(logical AND — the stricter "whichever fires later" reading):

- rows-read grew by **≥ +50%** (`TURSO_QUERY_PCT=0.5`), **and**
- rows-read grew by **≥ +50,000,000** absolute (`TURSO_QUERY_ABS_FLOOR`).

Requiring both suppresses two false-positive classes: large-percentage growth
on tiny queries, and small-percentage drift on already-large queries. A
brand-new query (absent from baseline) has no percentage, so only the absolute
floor applies. Tune via the env vars above if real variance dictates.

## Baseline

Lives at `~/.config/tq/turso-query-baseline.json` (override:
`TURSO_QUERY_BASELINE`). Rationale:

- `~/.config/tq/` is already tq's state directory (default DB path), so the
  scheduled `claude --bg` run — same machine, same `$HOME` — sees the same
  baseline week to week.
- Not in the repo: no git churn, no merge conflicts, and zero risk of
  committing Turso data or tokens.

The first run has no baseline, so every query is "new" and only the absolute
floor can fire. Each non-dry-run rewrites the baseline to the latest values, so
the same regression is not re-filed every week; sustained regressions are
tracked on the already-filed action instead.

## Silencing false positives

Add a substring of the offending query (one per line, `#` for comments) to
`~/.config/tq/turso-query-ignore.txt` (override: `TURSO_QUERY_IGNORE`). Any
top-N query containing a listed substring is reported with `"ignored": true`
and never counts as a regression. Use this for queries whose growth is
expected/benign (e.g. a known-unbounded analytics scan).

## Authentication

`turso db inspect` requires Turso **platform** auth (the libsql token in
`TQ_DB_URL` is DB-level and does not work for the platform stats API). The
machine must have `turso auth login` completed (config under
`~/Library/Application Support/turso`) or `TURSO_API_TOKEN` exported. The
scheduled run inherits this from the same user/HOME, so a one-time
`turso auth login` is sufficient.

## Manual / ad-hoc use

- `/turso-query-watch` — same logic the schedule runs.
- `bash .claude/skills/turso-query-watch/scripts/turso-query-watch.sh --dry-run`
  — print JSON + report, file nothing, leave baseline untouched.
- `bash .claude/skills/turso-query-watch/scripts/turso-query-watch.sh --help`
  — full env-var reference.

The hermetic test (`scripts/turso-query-watch.test.sh`, also a CI step) drives
a synthetic regression through the full notification path with a fixture and a
fake `tq`; it needs no Turso auth.
