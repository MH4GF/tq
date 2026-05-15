---
name: turso-query-watch
description: Detect production-side Turso rows-read regressions in the tq database. Runs the bundled watcher that inspects `turso db inspect tq --queries`, compares the top queries against a stored baseline, and self-files a tq action when a query's rows-read grows past both gates. Use this whenever asked to check Turso query cost, watch rows-read, run the weekly rows-read regression check, or investigate whether DB query cost regressed in production — including the scheduled "/turso-query-watch" run.
context: fork
allowed-tools: Bash(bash *turso-query-watch.sh*)
---

# Turso rows-read regression watch

A detection-only watcher. Static guards (forbidigo, deadcode-check, golden
rules) are method-name dependent and miss production regressions; this observes
the real metered metric instead. It does not fix anything — humans triage.

## Run it

```bash
bash .claude/skills/turso-query-watch/scripts/turso-query-watch.sh
```

The script does all the work deterministically: it captures
`turso db inspect tq --queries`, parses the top queries by rows-read, compares
each against `~/.config/tq/turso-query-baseline.json`, prints a JSON summary to
stdout, and — only when a query clears **both** the +50% and +50M rows-read
gates — files a tq action under task #698 and rewrites the baseline. Queries
listed in `~/.config/tq/turso-query-ignore.txt` are excluded.

Pass `--dry-run` for an ad-hoc human check that neither files an action nor
touches the baseline.

## Report

Relay the script's JSON summary, focusing on `regression_count` and any query
with `"regression": true`. If the script filed an action, say so and quote the
action ID from stderr. If `regression_count` is 0, a one-line "no regressions"
is enough.

## Do not

Do not edit code, propose fixes, commit, or auto-remediate. The script already
files the tracking action; remediation is a separate, human-triaged step
(scope of this skill ends at detection). If the script exits non-zero (missing
`turso`, auth failure, parse failure), report its stderr verbatim — never
silently treat a failed run as "no regressions".

See `docs/turso-query-watch.md` for schedule, thresholds, baseline location,
and how to silence false positives.
